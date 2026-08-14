// service/request_content_audit/store_test.go
// 请求内容审计存储端到端测试：覆盖流式加密读取、原子提交、资源去重、完整性校验与保留期清理。
package request_content_audit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var auditTestNow = time.Date(2026, time.August, 14, 9, 0, 0, 0, time.UTC)

func newTestStore(t *testing.T) (*Store, *model.RequestContentAuditRepository, *gorm.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "audit.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
	})
	repository := model.NewRequestContentAuditRepository(db)
	store, err := NewStore(root, repository, &StaticKeyProvider{
		ActiveKeyId: "test-key-v1",
		Keys: map[string][]byte{
			"test-key-v1": bytes.Repeat([]byte{0x5a}, 32),
		},
	}, WithClock(func() time.Time {
		return auditTestNow
	}))
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.Background()))
	return store, repository, db, root
}

func TestStoreStreamsEncryptedJSONAndCommitsMetadata(t *testing.T) {
	store, repository, _, root := newTestStore(t)
	ctx := context.Background()
	message := strings.Repeat("窗前听雨的审计内容-", 20000)
	payload := map[string]any{
		"request_type": "chat",
		"messages": []map[string]any{{
			"role": "user",
			"text": message,
		}},
	}
	thumbnail := []byte("thumbnail-content")
	audit, err := store.StoreJSON(ctx, RecordInput{
		Audit: model.RequestContentAudit{
			RequestId:            "req-stream-1",
			UserId:               7,
			Username:             "rain",
			ModelName:            "gpt-test",
			RequestType:          "chat",
			MessageCount:         1,
			NormalizationVersion: 1,
			ExpiresAt:            auditTestNow.Add(24 * time.Hour).Unix(),
		},
		Assets: []AssetInput{{
			Position:  0,
			AssetType: "image",
			MimeType:  "image/png",
			Width:     16,
			Height:    16,
			Original:  bytes.NewReader([]byte("original-resource-content")),
			Thumbnail: &AssetVariantInput{
				MimeType: "image/png",
				Width:    8,
				Height:   8,
				Content:  bytes.NewReader(thumbnail),
			},
		}},
	}, payload)
	require.NoError(t, err)
	require.NotZero(t, audit.Id)
	assert.Equal(t, model.RequestContentCaptureStatusComplete, audit.CaptureStatus)
	assert.Equal(t, integrityVersion, audit.IntegrityVersion)
	assert.Positive(t, audit.ContentSize)
	assert.Positive(t, audit.StoredSize)
	assert.Less(t, audit.StoredSize, audit.ContentSize)

	var decoded map[string]any
	require.NoError(t, store.ReadAuditJSON(ctx, audit, &decoded))
	messages, ok := decoded["messages"].([]any)
	require.True(t, ok)
	firstMessage, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, message, firstMessage["text"])

	stream, err := store.OpenAuditContent(ctx, audit)
	require.NoError(t, err)
	streamed, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.NoError(t, stream.Close())
	expected, err := common.Marshal(payload)
	require.NoError(t, err)
	assert.Equal(t, expected, streamed)

	assets, err := repository.GetAssets(ctx, audit.Id)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.NotZero(t, assets[0].OriginalObjectId)
	assert.NotZero(t, assets[0].ThumbnailObjectId)

	bodyPath, err := store.resolve(audit.ContentPath)
	require.NoError(t, err)
	encryptedBody, err := os.ReadFile(bodyPath)
	require.NoError(t, err)
	assert.NotContains(t, string(encryptedBody), message[:64])
	assert.NoError(t, assertNoTemporaryFiles(root))
}

func TestStoreDeduplicatesBase64AssetsAndRejectsDuplicateMetadataCommit(t *testing.T) {
	store, repository, db, root := newTestStore(t)
	ctx := context.Background()
	dataURI := "data:image/png;base64,c2hhcmVkLWltYWdlLWJ5dGVz"

	firstAsset, err := AssetFromDataURI(0, "image", dataURI, 32, 24)
	require.NoError(t, err)
	first, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-dedup-1", ExpiresAt: auditTestNow.Add(time.Hour).Unix()},
		Content: bytes.NewReader([]byte("first request")),
		Assets:  []AssetInput{firstAsset},
	})
	require.NoError(t, err)

	secondAsset, err := AssetFromDataURI(0, "image", dataURI, 32, 24)
	require.NoError(t, err)
	second, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-dedup-2", ExpiresAt: auditTestNow.Add(time.Hour).Unix()},
		Content: bytes.NewReader([]byte("second request")),
		Assets:  []AssetInput{secondAsset},
	})
	require.NoError(t, err)

	firstAssets, err := repository.GetAssets(ctx, first.Id)
	require.NoError(t, err)
	secondAssets, err := repository.GetAssets(ctx, second.Id)
	require.NoError(t, err)
	require.Len(t, firstAssets, 1)
	require.Len(t, secondAssets, 1)
	assert.Equal(t, firstAssets[0].OriginalObjectId, secondAssets[0].OriginalObjectId)

	object, err := repository.GetObjectById(ctx, firstAssets[0].OriginalObjectId)
	require.NoError(t, err)
	assert.EqualValues(t, 2, object.RefCount)
	var objectCount int64
	require.NoError(t, db.Model(&model.RequestContentObject{}).Count(&objectCount).Error)
	assert.EqualValues(t, 1, objectCount)

	duplicate, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: first.RequestId, ExpiresAt: auditTestNow.Add(time.Hour).Unix()},
		Content: bytes.NewReader([]byte("must not become an audit row")),
	})
	require.Error(t, err)
	assert.Nil(t, duplicate)
	var auditCount int64
	require.NoError(t, db.Model(&model.RequestContentAudit{}).Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount)
	assert.NoError(t, assertNoTemporaryFiles(root))
	assert.Equal(t, 2, countAuditFiles(t, filepath.Join(root, "content")))
}

func TestStoreCleansStagedAssetFilesAfterMetadataFailure(t *testing.T) {
	store, _, db, root := newTestStore(t)
	ctx := context.Background()
	_, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-staged-duplicate"},
		Content: bytes.NewReader([]byte("first body")),
	})
	require.NoError(t, err)

	_, err = store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-staged-duplicate"},
		Content: bytes.NewReader([]byte("second body")),
		Assets: []AssetInput{{
			Position:  0,
			AssetType: "image",
			MimeType:  "image/png",
			Original:  bytes.NewReader([]byte("must be removed after rollback")),
		}},
	})
	require.Error(t, err)

	var auditCount int64
	require.NoError(t, db.Model(&model.RequestContentAudit{}).Count(&auditCount).Error)
	assert.EqualValues(t, 1, auditCount)
	assert.Equal(t, 1, countAuditFiles(t, filepath.Join(root, "content")))
	assert.Equal(t, 0, countAuditFiles(t, filepath.Join(root, "assets")))
	assert.NoError(t, assertNoTemporaryFiles(root))
}

func TestReadAuditContentRejectsTampering(t *testing.T) {
	store, _, _, root := newTestStore(t)
	ctx := context.Background()
	audit, err := store.Store(ctx, RecordInput{
		Audit:   model.RequestContentAudit{RequestId: "req-integrity-1", ExpiresAt: auditTestNow.Add(time.Hour).Unix()},
		Content: bytes.NewReader([]byte(strings.Repeat("integrity", 1000))),
	})
	require.NoError(t, err)

	path, err := store.resolve(audit.ContentPath)
	require.NoError(t, err)
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.Seek(-1, io.SeekEnd)
	require.NoError(t, err)
	_, err = file.Write([]byte{0})
	require.NoError(t, err)
	require.NoError(t, file.Close())

	var content bytes.Buffer
	err = store.ReadAuditContent(ctx, audit, &content)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrIntegrityMismatch))
	validated, err := store.OpenValidatedAuditContent(ctx, audit)
	require.Error(t, err)
	assert.Nil(t, validated)
	assert.NoError(t, assertNoTemporaryFiles(root))
}

func TestCleanupExpiresBodiesAndOnlyRemovesDeduplicatedObjectAfterLastReference(t *testing.T) {
	store, repository, db, root := newTestStore(t)
	ctx := context.Background()
	now := auditTestNow
	sharedAsset := func() AssetInput {
		return AssetInput{
			Position:  0,
			AssetType: "image",
			MimeType:  "image/jpeg",
			Original:  bytes.NewReader([]byte("shared-retention-asset")),
		}
	}

	first, err := store.Store(ctx, RecordInput{
		Audit: model.RequestContentAudit{
			RequestId: "req-expired-first",
			ExpiresAt: now.Add(-2 * time.Hour).Unix(),
		},
		Content: bytes.NewReader([]byte("expired body")),
		Assets:  []AssetInput{sharedAsset()},
	})
	require.NoError(t, err)
	second, err := store.Store(ctx, RecordInput{
		Audit: model.RequestContentAudit{
			RequestId: "req-expired-second",
			ExpiresAt: now.Add(time.Hour).Unix(),
		},
		Content: bytes.NewReader([]byte("remaining body")),
		Assets:  []AssetInput{sharedAsset()},
	})
	require.NoError(t, err)

	secondAssets, err := repository.GetAssets(ctx, second.Id)
	require.NoError(t, err)
	require.Len(t, secondAssets, 1)
	object, err := repository.GetObjectById(ctx, secondAssets[0].OriginalObjectId)
	require.NoError(t, err)
	objectPath, err := store.resolve(object.StoragePath)
	require.NoError(t, err)
	firstBodyPath, err := store.resolve(first.ContentPath)
	require.NoError(t, err)

	report, err := store.CleanupExpired(ctx, now, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.AuditCount)
	assert.EqualValues(t, 1, report.AssetCount)
	assert.EqualValues(t, 0, report.ObjectCount)
	assert.FileExists(t, objectPath)
	_, err = os.Stat(firstBodyPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	remainingObject, err := repository.GetObjectById(ctx, secondAssets[0].OriginalObjectId)
	require.NoError(t, err)
	assert.EqualValues(t, 1, remainingObject.RefCount)
	require.NoError(t, db.Model(&model.RequestContentAudit{}).
		Where("id = ?", second.Id).
		Update("expires_at", now.Add(-time.Minute).Unix()).Error)

	report, err = store.CleanupExpired(ctx, now, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.AuditCount)
	assert.EqualValues(t, 1, report.ObjectCount)
	_, err = repository.GetObjectById(ctx, secondAssets[0].OriginalObjectId)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = os.Stat(objectPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	orphanPath := filepath.Join(root, "assets", "original", "aa", "orphan.rac")
	require.NoError(t, os.MkdirAll(filepath.Dir(orphanPath), 0o700))
	require.NoError(t, os.WriteFile(orphanPath, []byte("orphan"), 0o600))
	require.NoError(t, os.Chtimes(orphanPath, auditTestNow, auditTestNow))
	removed, err := store.CleanupOrphans(ctx, auditTestNow.Add(time.Minute))
	require.NoError(t, err)
	assert.EqualValues(t, 1, removed)
	_, err = os.Stat(orphanPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func assertNoTemporaryFiles(root string) error {
	entries, err := filepath.Glob(filepath.Join(root, ".tmp", "*.tmp"))
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("request content audit temporary files remain after commit")
	}
	return nil
}

func countAuditFiles(t *testing.T, directory string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(directory, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".rac" {
			count++
		}
		return nil
	})
	require.NoError(t, err)
	return count
}
