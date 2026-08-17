// service/request_content_audit/pack_store_test.go
// 内容寻址 pack 存储测试：批内/跨批去重、pack 滚动、篡改检测、引用释放与回收、
// 单项大小上限、元数据提交失败后的文件回退，以及并发写入不产生重复块。
package request_content_audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var packTestNow = time.Date(2026, time.August, 15, 10, 30, 0, 0, time.UTC)

func newTestPackStore(t *testing.T, options ...PackStoreOption) (*PackStore, *gorm.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "pack.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 单连接让并发用例排队访问同一个 sqlite 文件，避免 SQLITE_BUSY 掩盖真正要验证的行为
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
	})
	repository := model.NewRequestContentPackRepository(db)
	require.NoError(t, repository.Migrate(context.Background()))
	options = append([]PackStoreOption{WithPackClock(func() time.Time { return packTestNow })}, options...)
	store, err := NewPackStore(root, repository, &StaticKeyProvider{
		ActiveKeyId: "pack-key-v1",
		Keys: map[string][]byte{
			"pack-key-v1": bytes.Repeat([]byte{0x3c}, 32),
		},
	}, options...)
	require.NoError(t, err)
	return store, db, root
}

func packFileSize(t *testing.T, store *PackStore, relativePath string) int64 {
	t.Helper()
	path, err := store.resolve(relativePath)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

func listPacks(t *testing.T, db *gorm.DB) []model.RequestContentPack {
	t.Helper()
	var packs []model.RequestContentPack
	require.NoError(t, db.Order("id ASC").Find(&packs).Error)
	return packs
}

func listBlobs(t *testing.T, db *gorm.DB) map[string]model.RequestContentBlob {
	t.Helper()
	var blobs []model.RequestContentBlob
	require.NoError(t, db.Order("id ASC").Find(&blobs).Error)
	indexed := make(map[string]model.RequestContentBlob, len(blobs))
	for index := range blobs {
		indexed[blobs[index].Sha256] = blobs[index]
	}
	require.Len(t, indexed, len(blobs))
	return indexed
}

func TestPackStoreDeduplicatesRepeatedItemsInOneBatch(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	expiresAt := packTestNow.Add(24 * time.Hour).Unix()
	items := [][]byte{
		[]byte(`{"role":"user","text":"窗前听雨的第一条消息"}`),
		[]byte(`{"role":"assistant","text":"收到"}`),
		[]byte(`{"role":"user","text":"窗前听雨的第一条消息"}`),
	}

	refs, err := store.PutItems(ctx, items, expiresAt)
	require.NoError(t, err)
	require.Len(t, refs, 3)
	assert.Equal(t, refs[0], refs[2])
	assert.NotEqual(t, refs[0].Sha256, refs[1].Sha256)
	assert.EqualValues(t, len(items[0]), refs[0].PlainSize)

	blobs := listBlobs(t, db)
	require.Len(t, blobs, 2)
	assert.EqualValues(t, 2, blobs[refs[0].Sha256].RefCount)
	assert.EqualValues(t, 1, blobs[refs[1].Sha256].RefCount)
	assert.EqualValues(t, expiresAt, blobs[refs[0].Sha256].ExpiresAt)

	packs := listPacks(t, db)
	require.Len(t, packs, 1)
	assert.False(t, packs[0].Sealed)
	assert.EqualValues(t, 2, packs[0].BlobCount)
	assert.EqualValues(t, blobs[refs[0].Sha256].Length+blobs[refs[1].Sha256].Length, packs[0].LiveBytes)
	assert.Equal(t, packs[0].Size, packFileSize(t, store, packs[0].StoragePath))
	assert.True(t, strings.HasPrefix(packs[0].StoragePath, "packs/2026/08/"))
	assert.True(t, strings.HasSuffix(packs[0].StoragePath, ".rap"))

	contents, err := store.GetItems(ctx, refs)
	require.NoError(t, err)
	require.Len(t, contents, 3)
	for index := range items {
		assert.Equal(t, items[index], contents[index])
	}

	path, err := store.resolve(packs[0].StoragePath)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "窗前听雨的第一条消息")
}

func TestPackStoreDeduplicatesAcrossBatchesWithoutGrowingPack(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	items := [][]byte{[]byte("shared-conversation-head"), []byte("shared-conversation-tail")}
	firstExpiry := packTestNow.Add(time.Hour).Unix()

	firstRefs, err := store.PutItems(ctx, items, firstExpiry)
	require.NoError(t, err)
	packs := listPacks(t, db)
	require.Len(t, packs, 1)
	sizeAfterFirst := packFileSize(t, store, packs[0].StoragePath)
	require.Equal(t, packs[0].Size, sizeAfterFirst)

	laterExpiry := packTestNow.Add(48 * time.Hour).Unix()
	secondRefs, err := store.PutItems(ctx, items, laterExpiry)
	require.NoError(t, err)
	assert.Equal(t, firstRefs, secondRefs)

	blobs := listBlobs(t, db)
	require.Len(t, blobs, 2)
	for _, ref := range secondRefs {
		assert.EqualValues(t, 2, blobs[ref.Sha256].RefCount)
		assert.EqualValues(t, laterExpiry, blobs[ref.Sha256].ExpiresAt)
	}

	packs = listPacks(t, db)
	require.Len(t, packs, 1)
	assert.EqualValues(t, laterExpiry, packs[0].ExpiresAt)
	assert.EqualValues(t, 2, packs[0].BlobCount)
	assert.Equal(t, sizeAfterFirst, packs[0].Size)
	assert.Equal(t, sizeAfterFirst, packFileSize(t, store, packs[0].StoragePath))

	contents, err := store.GetItems(ctx, secondRefs)
	require.NoError(t, err)
	assert.Equal(t, items, contents)
}

func TestPackStoreRollsToNewPackWhenSizeLimitReached(t *testing.T) {
	store, db, _ := newTestPackStore(t, WithPackMaxSize(64))
	ctx := context.Background()
	expiresAt := packTestNow.Add(time.Hour).Unix()

	var refs []BlobRef
	var expected [][]byte
	for round := 0; round < 4; round++ {
		items := [][]byte{
			[]byte(fmt.Sprintf("round-%d-first-message", round)),
			[]byte(fmt.Sprintf("round-%d-second-message", round)),
		}
		batchRefs, err := store.PutItems(ctx, items, expiresAt)
		require.NoError(t, err)
		refs = append(refs, batchRefs...)
		expected = append(expected, items...)
	}

	packs := listPacks(t, db)
	require.Len(t, packs, 4)
	for index := range packs {
		assert.True(t, packs[index].Sealed, "pack %d should be sealed", packs[index].Id)
		assert.EqualValues(t, 2, packs[index].BlobCount)
		assert.Equal(t, packs[index].Size, packFileSize(t, store, packs[index].StoragePath))
	}
	require.Len(t, listBlobs(t, db), 8)

	contents, err := store.GetItems(ctx, refs)
	require.NoError(t, err)
	assert.Equal(t, expected, contents)

	reversedRefs := make([]BlobRef, 0, len(refs))
	reversedExpected := make([][]byte, 0, len(refs))
	for index := len(refs) - 1; index >= 0; index-- {
		reversedRefs = append(reversedRefs, refs[index])
		reversedExpected = append(reversedExpected, expected[index])
	}
	contents, err = store.GetItems(ctx, reversedRefs)
	require.NoError(t, err)
	assert.Equal(t, reversedExpected, contents)
}

func TestPackStoreGetItemsRejectsTamperedPack(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	refs, err := store.PutItems(ctx, [][]byte{
		[]byte(strings.Repeat("integrity-", 32)),
		[]byte(strings.Repeat("tamper-", 32)),
	}, packTestNow.Add(time.Hour).Unix())
	require.NoError(t, err)

	packs := listPacks(t, db)
	require.Len(t, packs, 1)
	path, err := store.resolve(packs[0].StoragePath)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw[len(raw)-1] ^= 0xff
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	contents, err := store.GetItems(ctx, refs)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrIntegrityMismatch))
	assert.Nil(t, contents)
}

func TestPackStoreReclaimsSharedBlobOnlyAfterLastRelease(t *testing.T) {
	store, db, root := newTestPackStore(t, WithPackMaxSize(64))
	ctx := context.Background()
	expiresAt := packTestNow.Add(time.Hour).Unix()
	shared := []byte("shared-conversation-prefix")

	firstRefs, err := store.PutItems(ctx, [][]byte{shared, []byte("first-only-tail")}, expiresAt)
	require.NoError(t, err)
	secondRefs, err := store.PutItems(ctx, [][]byte{shared, []byte("second-only-tail")}, expiresAt)
	require.NoError(t, err)
	assert.Equal(t, firstRefs[0], secondRefs[0])
	require.Len(t, listPacks(t, db), 2)
	require.Len(t, listBlobs(t, db), 3)

	cutoff := packTestNow.Add(2 * time.Hour)
	require.NoError(t, store.ReleaseItems(ctx, firstRefs))
	report, err := store.CleanupBlobs(ctx, cutoff, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.BlobCount)
	assert.EqualValues(t, 0, report.PackCount)
	assert.Empty(t, report.Paths)

	blobs := listBlobs(t, db)
	require.Len(t, blobs, 2)
	assert.EqualValues(t, 1, blobs[secondRefs[0].Sha256].RefCount)
	contents, err := store.GetItems(ctx, secondRefs)
	require.NoError(t, err)
	require.Len(t, contents, 2)
	assert.Equal(t, shared, contents[0])

	require.NoError(t, store.ReleaseItems(ctx, secondRefs))
	report, err = store.CleanupBlobs(ctx, cutoff, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 2, report.BlobCount)
	assert.EqualValues(t, 2, report.PackCount)
	require.Len(t, report.Paths, 2)
	assert.Empty(t, listBlobs(t, db))
	assert.Empty(t, listPacks(t, db))
	for _, relativePath := range report.Paths {
		assert.True(t, strings.HasPrefix(relativePath, "packs/"))
		assert.FileExists(t, filepath.Join(root, filepath.FromSlash(relativePath)))
	}

	_, err = store.GetItems(ctx, secondRefs)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBlobNotFound))

	removed, err := store.RemovePackFiles(report.Paths)
	require.NoError(t, err)
	assert.EqualValues(t, 2, removed)
	for _, relativePath := range report.Paths {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(relativePath)))
		assert.ErrorIs(t, err, fs.ErrNotExist)
	}
}

func TestPackStoreRejectsItemsOverTheSizeLimit(t *testing.T) {
	store, db, _ := newTestPackStore(t, WithPackMaxItemSize(64))
	ctx := context.Background()
	expiresAt := packTestNow.Add(time.Hour).Unix()

	refs, err := store.PutItems(ctx, [][]byte{
		[]byte("small-enough"),
		bytes.Repeat([]byte("x"), 65),
	}, expiresAt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSizeLimitExceeded))
	assert.Nil(t, refs)
	assert.Empty(t, listPacks(t, db))
	assert.Empty(t, listBlobs(t, db))

	refs, err = store.PutItems(ctx, [][]byte{bytes.Repeat([]byte("x"), 64)}, expiresAt)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	contents, err := store.GetItems(ctx, refs)
	require.NoError(t, err)
	assert.Equal(t, bytes.Repeat([]byte("x"), 64), contents[0])
}

func TestPackStoreTruncatesAppendWhenMetadataCommitFails(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	expiresAt := packTestNow.Add(time.Hour).Unix()

	committedRefs, err := store.PutItems(ctx, [][]byte{[]byte("committed-message")}, expiresAt)
	require.NoError(t, err)
	packs := listPacks(t, db)
	require.Len(t, packs, 1)
	sizeBefore := packFileSize(t, store, packs[0].StoragePath)
	require.Equal(t, packs[0].Size, sizeBefore)

	// 只挡住块记录插入：查询仍然可用，于是文件已经落盘、元数据提交失败，正是需要回退文件的场景
	require.NoError(t, db.Exec(
		"CREATE TRIGGER block_blob_insert BEFORE INSERT ON request_content_blobs BEGIN SELECT RAISE(ABORT, 'blocked'); END",
	).Error)
	_, err = store.PutItems(ctx, [][]byte{[]byte("rolled-back-message")}, expiresAt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")

	packs = listPacks(t, db)
	require.Len(t, packs, 1)
	assert.Equal(t, sizeBefore, packs[0].Size)
	assert.Equal(t, sizeBefore, packFileSize(t, store, packs[0].StoragePath))
	assert.True(t, packs[0].Sealed, "回退后应封存旧 pack，避免复用同一组 nonce")
	require.Len(t, listBlobs(t, db), 1)

	require.NoError(t, db.Exec("DROP TRIGGER block_blob_insert").Error)
	recoveredRefs, err := store.PutItems(ctx, [][]byte{[]byte("message-after-recovery")}, expiresAt)
	require.NoError(t, err)
	packs = listPacks(t, db)
	require.Len(t, packs, 2)
	for index := range packs {
		assert.Equal(t, packs[index].Size, packFileSize(t, store, packs[index].StoragePath))
	}

	contents, err := store.GetItems(ctx, append(append([]BlobRef{}, committedRefs...), recoveredRefs...))
	require.NoError(t, err)
	require.Len(t, contents, 2)
	assert.Equal(t, []byte("committed-message"), contents[0])
	assert.Equal(t, []byte("message-after-recovery"), contents[1])
}

// 崩溃残留：文件比 pack.size 长说明上一次写入没提交完，下一次追加前必须先截掉这段尾巴。
func TestPackStoreDropsUncommittedTailBeforeAppending(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	expiresAt := packTestNow.Add(time.Hour).Unix()

	firstRefs, err := store.PutItems(ctx, [][]byte{[]byte("survivor-message")}, expiresAt)
	require.NoError(t, err)
	packs := listPacks(t, db)
	require.Len(t, packs, 1)
	committedSize := packs[0].Size

	path, err := store.resolve(packs[0].StoragePath)
	require.NoError(t, err)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	_, err = file.Write(bytes.Repeat([]byte{0x7f}, 128))
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.Greater(t, packFileSize(t, store, packs[0].StoragePath), committedSize)

	secondRefs, err := store.PutItems(ctx, [][]byte{[]byte("message-written-after-crash")}, expiresAt)
	require.NoError(t, err)
	packs = listPacks(t, db)
	require.Len(t, packs, 1)
	assert.Equal(t, packs[0].Size, packFileSize(t, store, packs[0].StoragePath))

	contents, err := store.GetItems(ctx, append(append([]BlobRef{}, firstRefs...), secondRefs...))
	require.NoError(t, err)
	require.Len(t, contents, 2)
	assert.Equal(t, []byte("survivor-message"), contents[0])
	assert.Equal(t, []byte("message-written-after-crash"), contents[1])
}

// 追加写靠 previousSize 做乐观校验：偏移过期就整体回滚，不能把块记录指到别人写过的字节上。
func TestPackRepositoryRejectsAppendWithStalePackSize(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	refs, err := store.PutItems(ctx, [][]byte{[]byte("first-writer-message")}, packTestNow.Add(time.Hour).Unix())
	require.NoError(t, err)
	packs := listPacks(t, db)
	require.Len(t, packs, 1)

	repository := model.NewRequestContentPackRepository(db)
	stale := []model.RequestContentBlob{{
		Sha256:    strings.Repeat("a", 64),
		PackId:    packs[0].Id,
		Offset:    packs[0].Size,
		Length:    64,
		PlainSize: 16,
		RefCount:  1,
		CreatedAt: packTestNow.Unix(),
	}}
	err = repository.AppendBlobs(ctx, packs[0].Id, stale, packs[0].Size-1, packs[0].Size+64, packs[0].ChunkIndex+1, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrRequestContentPackConflict))

	require.Len(t, listBlobs(t, db), 1)
	after := listPacks(t, db)
	require.Len(t, after, 1)
	assert.Equal(t, packs[0].Size, after[0].Size)
	assert.Equal(t, packs[0].ChunkIndex, after[0].ChunkIndex)

	contents, err := store.GetItems(ctx, refs)
	require.NoError(t, err)
	assert.Equal(t, []byte("first-writer-message"), contents[0])
}

func TestPackStoreConcurrentPutItemsKeepsOneBlobPerContent(t *testing.T) {
	store, db, _ := newTestPackStore(t)
	ctx := context.Background()
	expiresAt := packTestNow.Add(time.Hour).Unix()
	shared := [][]byte{
		[]byte("shared-system-prompt"),
		[]byte("shared-first-turn"),
		[]byte("shared-second-turn"),
	}
	const workers = 8

	results := make([][]BlobRef, workers)
	payloads := make([][][]byte, workers)
	failures := make([]error, workers)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			items := append([][]byte{}, shared...)
			items = append(items, []byte(fmt.Sprintf("worker-%d-unique-turn", index)))
			refs, err := store.PutItems(ctx, items, expiresAt)
			payloads[index] = items
			results[index] = refs
			failures[index] = err
		}(worker)
	}
	group.Wait()
	for _, err := range failures {
		require.NoError(t, err)
	}

	blobs := listBlobs(t, db)
	require.Len(t, blobs, len(shared)+workers)
	var totalRefs int64
	for _, blob := range blobs {
		totalRefs += blob.RefCount
	}
	assert.EqualValues(t, workers*(len(shared)+1), totalRefs)
	for _, ref := range results[0][:len(shared)] {
		assert.EqualValues(t, workers, blobs[ref.Sha256].RefCount, "共享内容 %s 的引用数不对", ref.Sha256)
	}

	packs := listPacks(t, db)
	require.Len(t, packs, 1)
	assert.EqualValues(t, len(blobs), packs[0].BlobCount)
	assert.Equal(t, packs[0].Size, packFileSize(t, store, packs[0].StoragePath))

	for worker := 0; worker < workers; worker++ {
		contents, err := store.GetItems(ctx, results[worker])
		require.NoError(t, err)
		assert.Equal(t, payloads[worker], contents)
	}
}
