// service/request_content_audit/manifest_test.go
// 验证去重正文：相同会话前缀跨请求共享内容块，读取仍逐字节还原，且引用释放后才回收。
package request_content_audit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDeduplicatedStore(t *testing.T) (*Store, *PackStore, *gorm.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "audit.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })

	keys := &StaticKeyProvider{
		ActiveKeyId: "test-key-v1",
		Keys:        map[string][]byte{"test-key-v1": bytes.Repeat([]byte{0x5a}, 32)},
	}
	packs, err := NewPackStore(root, model.NewRequestContentPackRepository(db), keys, WithPackClock(func() time.Time {
		return auditTestNow
	}))
	require.NoError(t, err)
	store, err := NewStore(root, model.NewRequestContentAuditRepository(db), keys,
		WithPackStore(packs),
		WithClock(func() time.Time { return auditTestNow }),
	)
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.Background()))
	return store, packs, db, root
}

func conversationPayload(turns int) []byte {
	var builder strings.Builder
	builder.WriteString(`{"model":"gpt-5.6-luna","input":[`)
	for index := 0; index < turns; index++ {
		if index > 0 {
			builder.WriteString(",")
		}
		fmt.Fprintf(&builder, `{"type":"message","role":"user","content":[{"type":"input_text","text":"第 %d 轮 `, index)
		// 用与轮次相关的伪随机文本，既保证内容多样（分块边界能出现），又保证同一轮内容可重现。
		state := uint64(index)*2654435761 + 12345
		for count := 0; count < 1500; count++ {
			state ^= state << 13
			state ^= state >> 7
			state ^= state << 17
			builder.WriteByte(byte('a' + state%26))
		}
		builder.WriteString(`"}]}`)
	}
	builder.WriteString(`]}`)
	return []byte(builder.String())
}

func packBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(filepath.Join(root, "packs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	return total
}

func TestStoreDeduplicatesSharedConversationPrefix(t *testing.T) {
	store, _, db, root := newDeduplicatedStore(t)
	ctx := context.Background()
	expiresAt := auditTestNow.Add(24 * time.Hour).Unix()

	first := conversationPayload(40)
	firstAudit, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-dedup-a", ExpiresAt: expiresAt},
		Content: bytes.NewReader(first),
	})
	require.NoError(t, err)
	assert.Equal(t, manifestIntegrityVersion, firstAudit.IntegrityVersion)
	assert.EqualValues(t, len(first), firstAudit.ContentSize)
	afterFirst := packBytes(t, root)
	require.Positive(t, afterFirst)

	second := conversationPayload(41)
	secondAudit, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-dedup-b", ExpiresAt: expiresAt},
		Content: bytes.NewReader(second),
	})
	require.NoError(t, err)
	afterSecond := packBytes(t, root)

	// 第二轮只多出一条消息，新写入的字节必须远小于整段正文。
	assert.Less(t, afterSecond-afterFirst, int64(len(second))/4)

	var restored bytes.Buffer
	require.NoError(t, store.ReadAuditContent(ctx, firstAudit, &restored))
	assert.Equal(t, first, restored.Bytes())
	restored.Reset()
	require.NoError(t, store.ReadAuditContent(ctx, secondAudit, &restored))
	assert.Equal(t, second, restored.Bytes())

	var sharedBlobs int64
	require.NoError(t, db.Model(&model.RequestContentBlob{}).Where("ref_count > 1").Count(&sharedBlobs).Error)
	assert.Positive(t, sharedBlobs)
}

func TestCleanupExpiredReclaimsBlobsAfterLastManifestReference(t *testing.T) {
	store, _, db, root := newDeduplicatedStore(t)
	ctx := context.Background()
	payload := conversationPayload(30)

	// 两条记录共享全部内容块，保留期不同：块的过期时间取较晚的那条。
	early, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-early", ExpiresAt: auditTestNow.Add(-2 * time.Hour).Unix()},
		Content: bytes.NewReader(payload),
	})
	require.NoError(t, err)
	late, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-late", ExpiresAt: auditTestNow.Add(-time.Hour).Unix()},
		Content: bytes.NewReader(payload),
	})
	require.NoError(t, err)
	require.NotEqual(t, early.ContentPath, late.ContentPath)

	report, err := store.CleanupExpired(ctx, auditTestNow.Add(-90*time.Minute), 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.AuditCount)
	assert.EqualValues(t, 0, report.BlobCount)
	assert.Positive(t, packBytes(t, root))

	var restored bytes.Buffer
	require.NoError(t, store.ReadAuditContent(ctx, late, &restored))
	assert.Equal(t, payload, restored.Bytes())

	report, err = store.CleanupExpired(ctx, auditTestNow, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.AuditCount)
	assert.Positive(t, report.BlobCount)
	assert.Positive(t, report.PackCount)

	var remainingBlobs int64
	require.NoError(t, db.Model(&model.RequestContentBlob{}).Count(&remainingBlobs).Error)
	assert.Zero(t, remainingBlobs)
	assert.Zero(t, packBytes(t, root))
}
