// service/request_content_audit/pack_store.go
// 请求内容审计的内容寻址存储层：内容按 sha256 去重后追加进共享的加密 pack 文件，
// 相同内容只落一份，多条审计记录靠引用计数共享同一段字节。
// 容器头、AEAD 分块规则、错误值都复用 crypto_container.go，只是记录语义换成"一条记录 = 一个独立压缩块"。
package request_content_audit

import (
	"bytes"
	"cmp"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
)

const (
	defaultPackMaxSize     = 8 << 20
	defaultPackMaxItemSize = 4 << 20
	// 记录前缀：1 字节记录类型 + 4 字节大端密文长度。
	packRecordHeaderSize = 1 + 4
	packDirectory        = "packs"
	packFileExtension    = ".rap"
	// 解压上限的下界，避免把限制压到比 zstd 窗口还小。
	packDecoderMinMemory = 8 << 20
)

var (
	// ErrBlobNotFound 表示引用指向的内容块在库里已经查不到（被提前回收，或元数据与清单不一致）。
	ErrBlobNotFound = errors.New("request content audit blob is not found")
	// errPackUnusable 表示活动 pack 的文件不能继续追加，需要封存后换一个新的。
	errPackUnusable = errors.New("request content audit pack file is unusable")
)

type PackStore struct {
	root        string
	repository  *model.RequestContentPackRepository
	keys        KeyProvider
	maxPackSize int64
	maxItemSize int64
	now         func() time.Time
	appendMu    sync.Mutex
}

type PackStoreOption func(*PackStore) error

// BlobRef 是清单里引用一段内容的最小信息。
type BlobRef struct {
	Sha256    string `json:"sha256"`
	PlainSize int64  `json:"plain_size"`
}

type PackCleanupReport struct {
	BlobCount int64
	PackCount int64
	Paths     []string
}

// packItem 是一次写入里的一份去重内容，count 记它在本批次出现的次数。
type packItem struct {
	sha   string
	data  []byte
	count int64
}

// packAppendHandle 是一次追加写要用的 pack 上下文：文件句柄、容器头、AEAD 与写入起点。
type packAppendHandle struct {
	pack        *model.RequestContentPack
	file        *os.File
	header      []byte
	noncePrefix [8]byte
	aead        cipher.AEAD
	offset      int64
	chunkIndex  uint32
}

func WithPackMaxSize(size int64) PackStoreOption {
	return func(store *PackStore) error {
		if size <= 0 {
			return errors.New("request content pack maximum size must be positive")
		}
		store.maxPackSize = size
		return nil
	}
}

func WithPackClock(now func() time.Time) PackStoreOption {
	return func(store *PackStore) error {
		if now == nil {
			return errors.New("request content pack clock is nil")
		}
		store.now = now
		return nil
	}
}

func WithPackMaxItemSize(size int64) PackStoreOption {
	return func(store *PackStore) error {
		if size <= 0 {
			return errors.New("request content pack maximum item size must be positive")
		}
		store.maxItemSize = size
		return nil
	}
}

func NewPackStore(root string, repository *model.RequestContentPackRepository, keys KeyProvider, options ...PackStoreOption) (*PackStore, error) {
	if root == "" {
		return nil, errors.New("request content pack storage root is empty")
	}
	if repository == nil {
		return nil, errors.New("request content pack repository is nil")
	}
	if keys == nil {
		return nil, errors.New("request content audit key provider is nil")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	store := &PackStore{
		root:        absoluteRoot,
		repository:  repository,
		keys:        keys,
		maxPackSize: defaultPackMaxSize,
		maxItemSize: defaultPackMaxItemSize,
		now:         time.Now,
	}
	for _, option := range options {
		if err := option(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// PutItems 把一批内容写入去重存储，返回与输入等长、顺序一致的引用。
// 相同内容（含同一批次内重复）只写一次，已存在的只增加引用计数并抬高过期时间。
func (store *PackStore) PutItems(ctx context.Context, items [][]byte, expiresAt int64) ([]BlobRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	refs := make([]BlobRef, len(items))
	if len(items) == 0 {
		return refs, nil
	}
	if expiresAt < 0 {
		expiresAt = 0
	}
	pending := make([]*packItem, 0, len(items))
	indexed := make(map[string]*packItem, len(items))
	for position, data := range items {
		if store.maxItemSize > 0 && int64(len(data)) > store.maxItemSize {
			return nil, fmt.Errorf("request content audit item %d is %d bytes: %w", position, len(data), ErrSizeLimitExceeded)
		}
		digest := sha256.Sum256(data)
		sha := hex.EncodeToString(digest[:])
		refs[position] = BlobRef{Sha256: sha, PlainSize: int64(len(data))}
		if item, exists := indexed[sha]; exists {
			item.count++
			continue
		}
		item := &packItem{sha: sha, data: data, count: 1}
		indexed[sha] = item
		pending = append(pending, item)
	}
	if err := store.ensureDirectories(); err != nil {
		return nil, err
	}
	if err := store.storeItems(ctx, pending, expiresAt); err != nil {
		return nil, err
	}
	return refs, nil
}

// storeItems 在锁内完成"确认去重 → 选活动 pack → 追加写 → 更新元数据"。
// 去重复查放在锁内，是为了让同进程的并发写入不会把同一份内容写进两个 pack。
func (store *PackStore) storeItems(ctx context.Context, items []*packItem, expiresAt int64) error {
	store.appendMu.Lock()
	defer store.appendMu.Unlock()

	shas := make([]string, 0, len(items))
	for _, item := range items {
		shas = append(shas, item.sha)
	}
	existing, err := store.repository.FindBlobsBySha(ctx, shas)
	if err != nil {
		return err
	}
	referenced := make([]string, 0, len(items))
	pending := make([]*packItem, 0, len(items))
	packIds := make([]int64, 0, len(items))
	for _, item := range items {
		blob, hit := existing[item.sha]
		if !hit {
			pending = append(pending, item)
			continue
		}
		for repeat := int64(0); repeat < item.count; repeat++ {
			referenced = append(referenced, item.sha)
		}
		if blob.PackId > 0 && !slices.Contains(packIds, blob.PackId) {
			packIds = append(packIds, blob.PackId)
		}
	}
	if len(referenced) > 0 {
		if err := store.repository.AddBlobReferences(ctx, referenced, expiresAt); err != nil {
			return err
		}
		if err := store.repository.RaisePackExpiry(ctx, packIds, expiresAt); err != nil {
			return err
		}
	}
	if len(pending) == 0 {
		return nil
	}
	return store.appendItems(ctx, pending, expiresAt)
}

// appendItems 把这批内容压缩加密成记录追加到活动 pack：先落盘再提交元数据，
// 元数据失败就把文件截回写入前的长度，不留没有块记录指向的字节。
func (store *PackStore) appendItems(ctx context.Context, items []*packItem, expiresAt int64) error {
	handle, err := store.openActivePack(ctx, len(items), expiresAt)
	if err != nil {
		return err
	}
	defer handle.file.Close()

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		return err
	}
	defer encoder.Close()

	createdAt := store.now().Unix()
	payload := bytes.NewBuffer(make([]byte, 0, 64<<10))
	blobs := make([]model.RequestContentBlob, 0, len(items))
	offset := handle.offset
	chunkIndex := handle.chunkIndex
	for _, item := range items {
		sealed := handle.aead.Seal(
			nil,
			makeRecordNonce(handle.noncePrefix, chunkIndex),
			encoder.EncodeAll(item.data, nil),
			makeRecordAdditionalData(handle.header, containerRecordData, chunkIndex),
		)
		if int64(len(sealed)) > math.MaxUint32 {
			return fmt.Errorf("request content audit record is %d bytes: %w", len(sealed), ErrSizeLimitExceeded)
		}
		recordHeader := make([]byte, packRecordHeaderSize)
		recordHeader[0] = containerRecordData
		binary.BigEndian.PutUint32(recordHeader[1:], uint32(len(sealed)))
		payload.Write(recordHeader)
		payload.Write(sealed)
		length := int64(packRecordHeaderSize + len(sealed))
		blobs = append(blobs, model.RequestContentBlob{
			Sha256:     item.sha,
			PackId:     handle.pack.Id,
			Offset:     offset,
			Length:     length,
			ChunkIndex: chunkIndex,
			PlainSize:  int64(len(item.data)),
			RefCount:   item.count,
			CreatedAt:  createdAt,
			ExpiresAt:  expiresAt,
		})
		offset += length
		chunkIndex++
	}
	return store.commitAppend(ctx, handle, blobs, payload.Bytes(), offset, chunkIndex, expiresAt)
}

// commitAppend 是追加写的原子性边界：先 write + Sync，再提交块记录与 pack 元数据。
// 元数据失败就回退文件长度；只有"另有写入方推进了同一个 pack"时不能截断，那会削掉对方已提交的数据。
func (store *PackStore) commitAppend(ctx context.Context, handle *packAppendHandle, blobs []model.RequestContentBlob, payload []byte, size int64, chunkIndex uint32, expiresAt int64) error {
	if _, err := handle.file.WriteAt(payload, handle.offset); err != nil {
		return errors.Join(err, store.rollbackAppend(ctx, handle))
	}
	if err := handle.file.Sync(); err != nil {
		return errors.Join(err, store.rollbackAppend(ctx, handle))
	}
	if err := store.repository.AppendBlobs(ctx, handle.pack.Id, blobs, handle.offset, size, chunkIndex, expiresAt); err != nil {
		if errors.Is(err, model.ErrRequestContentPackConflict) {
			return err
		}
		return errors.Join(err, store.rollbackAppend(ctx, handle))
	}
	if size >= store.maxPackSize {
		if err := store.repository.SealPack(ctx, handle.pack.Id); err != nil {
			return err
		}
	}
	return store.reconcileAppendedBlobs(ctx, handle.pack.Id, size, chunkIndex, blobs, expiresAt)
}

// rollbackAppend 撤销这次追加：文件截回原长度，并把 pack 封存。
// 封存是为了不再复用这批记录已经用掉的 chunk index —— 那些字节已经落过盘（快照、备份里可能还留着），
// 换一个 noncePrefix 全新的 pack 比拿同一组 nonce 再加密一份明文安全。
func (store *PackStore) rollbackAppend(ctx context.Context, handle *packAppendHandle) error {
	return errors.Join(store.truncatePack(handle), store.repository.SealPack(ctx, handle.pack.Id))
}

// truncatePack 把 pack 文件截回追加前的长度。
func (store *PackStore) truncatePack(handle *packAppendHandle) error {
	if err := handle.file.Truncate(handle.offset); err != nil {
		return err
	}
	return handle.file.Sync()
}

// reconcileAppendedBlobs 处理另一个实例抢先插入同一个 sha 的情况：
// 我们的插入被唯一索引冲突跳过时引用计数没加上，要补一次；本 pack 里那段字节成了死区，从计数里扣掉。
func (store *PackStore) reconcileAppendedBlobs(ctx context.Context, packId int64, size int64, chunkIndex uint32, blobs []model.RequestContentBlob, expiresAt int64) error {
	shas := make([]string, 0, len(blobs))
	for index := range blobs {
		shas = append(shas, blobs[index].Sha256)
	}
	stored, err := store.repository.FindBlobsBySha(ctx, shas)
	if err != nil {
		return err
	}
	lost := make([]string, 0)
	var lostCount, lostBytes int64
	for index := range blobs {
		blob := blobs[index]
		saved, exists := stored[blob.Sha256]
		if !exists {
			return fmt.Errorf("request content audit blob %s disappeared right after append: %w", blob.Sha256, ErrBlobNotFound)
		}
		if saved.PackId == blob.PackId && saved.Offset == blob.Offset {
			continue
		}
		for repeat := int64(0); repeat < blob.RefCount; repeat++ {
			lost = append(lost, blob.Sha256)
		}
		lostCount++
		lostBytes += blob.Length
	}
	if len(lost) == 0 {
		return nil
	}
	if err := store.repository.AddBlobReferences(ctx, lost, expiresAt); err != nil {
		return err
	}
	return store.repository.UpdatePackAfterAppend(ctx, packId, size, chunkIndex, -lostCount, -lostBytes, expiresAt)
}

// openActivePack 选出可追加的 pack 并打开文件。活动 pack 的文件不可用（丢失、被截短、头部与元数据不符）时
// 先封存它再换一个新的：写入链路不能被单个坏文件堵死，坏 pack 里的块会在读取时以完整性错误暴露出来。
func (store *PackStore) openActivePack(ctx context.Context, recordCount int, expiresAt int64) (*packAppendHandle, error) {
	for attempt := 0; attempt < 3; attempt++ {
		pack, err := store.repository.ActivePack(ctx, store.maxPackSize)
		if err != nil {
			return nil, err
		}
		if pack == nil {
			return store.createPack(ctx, expiresAt)
		}
		if uint64(pack.ChunkIndex)+uint64(recordCount) >= uint64(math.MaxUint32) {
			if err := store.repository.SealPack(ctx, pack.Id); err != nil {
				return nil, err
			}
			continue
		}
		handle, err := store.openPack(ctx, pack)
		if err == nil {
			return handle, nil
		}
		if !errors.Is(err, errPackUnusable) {
			return nil, err
		}
		if sealErr := store.repository.SealPack(ctx, pack.Id); sealErr != nil {
			return nil, errors.Join(err, sealErr)
		}
	}
	return store.createPack(ctx, expiresAt)
}

func (store *PackStore) openPack(ctx context.Context, pack *model.RequestContentPack) (*packAppendHandle, error) {
	path, err := store.resolve(pack.StoragePath)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("open request content pack %q: %w", pack.StoragePath, errPackUnusable)
		}
		return nil, err
	}
	handle := &packAppendHandle{pack: pack, file: file, offset: pack.Size, chunkIndex: pack.ChunkIndex}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if info.Size() < pack.Size {
		return nil, errors.Join(
			fmt.Errorf("request content pack %q is shorter than the recorded %d bytes: %w", pack.StoragePath, pack.Size, errPackUnusable),
			file.Close(),
		)
	}
	if info.Size() > pack.Size {
		// 上一次写入落了盘但元数据没提交，尾部是没有块记录指向的残留字节。
		if err := store.truncatePack(handle); err != nil {
			return nil, errors.Join(err, file.Close())
		}
	}
	header, keyId, _, noncePrefix, err := readContainerHeader(file)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("read request content pack header %q: %w", pack.StoragePath, errPackUnusable),
			file.Close(),
		)
	}
	if len(header) != pack.HeaderSize || keyId != pack.KeyId {
		return nil, errors.Join(
			fmt.Errorf("request content pack header %q does not match metadata: %w", pack.StoragePath, errPackUnusable),
			file.Close(),
		)
	}
	aead, err := newPackAEAD(ctx, store.keys, keyId)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	handle.header = header
	handle.noncePrefix = noncePrefix
	handle.aead = aead
	return handle, nil
}

// createPack 新建一个 pack 文件并登记元数据；容器头写完就 Sync，noncePrefix 只存在文件头里，读取时再取回。
func (store *PackStore) createPack(ctx context.Context, expiresAt int64) (*packAppendHandle, error) {
	key, err := store.keys.ActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	aead, err := newPackAEADFromKey(key.Value)
	if err != nil {
		return nil, err
	}
	createdAt := store.now().UTC()
	relativePath := filepath.ToSlash(filepath.Join(
		packDirectory,
		createdAt.Format("2006"),
		createdAt.Format("01"),
		uuid.NewString()+packFileExtension,
	))
	path, err := store.resolve(relativePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	header, noncePrefix, err := writeContainerHeader(file, key.Id, defaultChunkSize)
	if err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	pack := &model.RequestContentPack{
		StoragePath: relativePath,
		KeyId:       key.Id,
		HeaderSize:  len(header),
		Size:        int64(len(header)),
		CreatedAt:   store.now().Unix(),
		ExpiresAt:   expiresAt,
	}
	if err := store.repository.CreatePack(ctx, pack); err != nil {
		return nil, err
	}
	committed = true
	return &packAppendHandle{
		pack:        pack,
		file:        file,
		header:      header,
		noncePrefix: noncePrefix,
		aead:        aead,
		offset:      pack.Size,
	}, nil
}

// GetItems 按引用顺序取回原始内容；同一个 pack 的读取合并到一次文件打开里。
func (store *PackStore) GetItems(ctx context.Context, refs []BlobRef) ([][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := make([][]byte, len(refs))
	if len(refs) == 0 {
		return results, nil
	}
	shas := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Sha256 == "" {
			return nil, errors.New("request content audit blob reference sha256 is empty")
		}
		shas = append(shas, ref.Sha256)
	}
	blobs, err := store.repository.FindBlobsBySha(ctx, shas)
	if err != nil {
		return nil, err
	}
	grouped := make(map[int64][]model.RequestContentBlob, len(blobs))
	for _, sha := range shas {
		if _, exists := blobs[sha]; !exists {
			return nil, fmt.Errorf("request content audit blob %s: %w", sha, ErrBlobNotFound)
		}
	}
	for _, blob := range blobs {
		grouped[blob.PackId] = append(grouped[blob.PackId], blob)
	}
	packIds := make([]int64, 0, len(grouped))
	for packId := range grouped {
		packIds = append(packIds, packId)
	}
	slices.Sort(packIds)
	contents := make(map[string][]byte, len(blobs))
	for _, packId := range packIds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items := grouped[packId]
		slices.SortFunc(items, func(first, second model.RequestContentBlob) int {
			return cmp.Compare(first.Offset, second.Offset)
		})
		if err := store.readPackBlobs(ctx, packId, items, contents); err != nil {
			return nil, err
		}
	}
	delivered := make(map[string]struct{}, len(contents))
	for position, ref := range refs {
		data := contents[ref.Sha256]
		if ref.PlainSize > 0 && ref.PlainSize != int64(len(data)) {
			return nil, fmt.Errorf("request content audit blob %s size does not match the reference: %w", ref.Sha256, ErrIntegrityMismatch)
		}
		if _, repeated := delivered[ref.Sha256]; repeated {
			data = append([]byte(nil), data...)
		} else {
			delivered[ref.Sha256] = struct{}{}
		}
		results[position] = data
	}
	return results, nil
}

// readPackBlobs 打开一个 pack，把这批记录解密解压后按 sha 填进 contents，
// 解压结果的 sha256 与长度都要和块记录对得上，否则算完整性失败。
func (store *PackStore) readPackBlobs(ctx context.Context, packId int64, blobs []model.RequestContentBlob, contents map[string][]byte) error {
	pack, err := store.repository.PackById(ctx, packId)
	if err != nil {
		return fmt.Errorf("load request content pack %d: %w", packId, err)
	}
	path, err := store.resolve(pack.StoragePath)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	limit := info.Size()
	if pack.Size > 0 && pack.Size < limit {
		limit = pack.Size
	}
	header, keyId, _, noncePrefix, err := readContainerHeader(file)
	if err != nil {
		return err
	}
	if (pack.HeaderSize > 0 && len(header) != pack.HeaderSize) || (pack.KeyId != "" && keyId != pack.KeyId) {
		return fmt.Errorf("request content pack %q header does not match metadata: %w", pack.StoragePath, ErrIntegrityMismatch)
	}
	aead, err := newPackAEAD(ctx, store.keys, keyId)
	if err != nil {
		return err
	}
	maxDecoded := store.maxItemSize
	for index := range blobs {
		maxDecoded = max(maxDecoded, blobs[index].PlainSize)
	}
	decoder, err := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxMemory(uint64(max(maxDecoded, packDecoderMinMemory))),
	)
	if err != nil {
		return err
	}
	defer decoder.Close()
	for index := range blobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		content, err := readPackRecord(file, decoder, aead, header, noncePrefix, blobs[index], limit)
		if err != nil {
			return err
		}
		contents[blobs[index].Sha256] = content
	}
	return nil
}

// readPackRecord 读一条记录并还原内容：偏移越界、记录头不符、AEAD 失败、解压失败、摘要不符都归为完整性错误，
// 由调用方决定降级策略。
func readPackRecord(file *os.File, decoder *zstd.Decoder, aead cipher.AEAD, header []byte, noncePrefix [8]byte, blob model.RequestContentBlob, limit int64) ([]byte, error) {
	if blob.Length <= packRecordHeaderSize || blob.Offset < int64(len(header)) || blob.Offset+blob.Length > limit {
		return nil, fmt.Errorf("request content audit blob %s points outside its pack: %w", blob.Sha256, ErrIntegrityMismatch)
	}
	record := make([]byte, blob.Length)
	if _, err := file.ReadAt(record, blob.Offset); err != nil {
		return nil, err
	}
	sealedSize := int64(binary.BigEndian.Uint32(record[1:packRecordHeaderSize]))
	if record[0] != containerRecordData || sealedSize != blob.Length-packRecordHeaderSize {
		return nil, fmt.Errorf("request content audit blob %s record header is invalid: %w", blob.Sha256, ErrIntegrityMismatch)
	}
	plaintext, err := aead.Open(
		nil,
		makeRecordNonce(noncePrefix, blob.ChunkIndex),
		record[packRecordHeaderSize:],
		makeRecordAdditionalData(header, containerRecordData, blob.ChunkIndex),
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt request content audit blob %s: %w", blob.Sha256, ErrIntegrityMismatch)
	}
	content, err := decoder.DecodeAll(plaintext, nil)
	if err != nil {
		return nil, fmt.Errorf("decompress request content audit blob %s: %w", blob.Sha256, ErrIntegrityMismatch)
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != blob.Sha256 || int64(len(content)) != blob.PlainSize {
		return nil, fmt.Errorf("request content audit blob %s failed verification: %w", blob.Sha256, ErrIntegrityMismatch)
	}
	return content, nil
}

// ReleaseItems 释放一批引用（审计记录过期时调用）。
func (store *PackStore) ReleaseItems(ctx context.Context, refs []BlobRef) error {
	if len(refs) == 0 {
		return nil
	}
	shas := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Sha256 == "" {
			continue
		}
		shas = append(shas, ref.Sha256)
	}
	if len(shas) == 0 {
		return nil
	}
	return store.repository.ReleaseBlobReferences(ctx, shas)
}

// CleanupBlobs 回收 ref_count 归零且已过期的块，返回可删除的 pack 文件相对路径。
// 文件删除交给调用方，数据库删除和文件删除因此可以分别重试。
func (store *PackStore) CleanupBlobs(ctx context.Context, cutoff time.Time, batchSize int) (PackCleanupReport, error) {
	report := PackCleanupReport{}
	if batchSize <= 0 {
		return report, errors.New("request content pack cleanup batch size must be positive")
	}
	store.appendMu.Lock()
	defer store.appendMu.Unlock()

	blobs, err := store.repository.ReclaimableBlobs(ctx, cutoff.Unix(), batchSize)
	if err != nil {
		return report, err
	}
	if len(blobs) > 0 {
		blobIds := make([]int64, 0, len(blobs))
		for index := range blobs {
			blobIds = append(blobIds, blobs[index].Id)
		}
		deleted, err := store.repository.DeleteBlobs(ctx, blobIds)
		if err != nil {
			return report, err
		}
		report.BlobCount = deleted
	}
	packs, err := store.repository.EmptyPacks(ctx, batchSize)
	if err != nil {
		return report, err
	}
	if len(packs) == 0 {
		return report, nil
	}
	packIds := make([]int64, 0, len(packs))
	paths := make([]string, 0, len(packs))
	for index := range packs {
		packIds = append(packIds, packs[index].Id)
		if packs[index].StoragePath != "" {
			paths = append(paths, packs[index].StoragePath)
		}
	}
	deleted, err := store.repository.DeletePacks(ctx, packIds)
	if err != nil {
		return report, err
	}
	report.PackCount = deleted
	report.Paths = paths
	return report, nil
}

// RemovePackFiles 删除 CleanupBlobs 交回来的 pack 文件，已经不存在的算删过。
// ReferencedPackPaths 返回仍有数据库记录指向的 pack 相对路径，用于孤儿文件扫描。
func (store *PackStore) ReferencedPackPaths(ctx context.Context) (map[string]struct{}, error) {
	packs, err := store.repository.AllPackPaths(ctx)
	if err != nil {
		return nil, err
	}
	referenced := make(map[string]struct{}, len(packs))
	for _, path := range packs {
		if path == "" {
			continue
		}
		referenced[path] = struct{}{}
	}
	return referenced, nil
}

func (store *PackStore) RemovePackFiles(paths []string) (int64, error) {
	var removed int64
	var failures []error
	for _, relativePath := range paths {
		path, err := store.resolve(relativePath)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove request content pack %q: %w", relativePath, err))
			continue
		}
		removed++
	}
	return removed, errors.Join(failures...)
}

func newPackAEAD(ctx context.Context, keys KeyProvider, keyId string) (cipher.AEAD, error) {
	key, err := keys.Key(ctx, keyId)
	if err != nil {
		return nil, err
	}
	return newPackAEADFromKey(key)
}

func newPackAEADFromKey(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create request content pack cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create request content pack gcm: %w", err)
	}
	return aead, nil
}

// resolve 把 pack 的相对路径映射到审计根目录下的绝对路径，并挡掉逃出根目录的写法。
func (store *PackStore) resolve(relativePath string) (string, error) {
	if relativePath == "" {
		return "", errors.New("request content pack storage path is empty")
	}
	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
	if filepath.IsAbs(cleanRelativePath) || cleanRelativePath == "." || cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return "", errors.New("request content pack storage path escapes root")
	}
	path := filepath.Join(store.root, cleanRelativePath)
	relativeToRoot, err := filepath.Rel(store.root, path)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("request content pack storage path escapes root")
	}
	return path, nil
}

func (store *PackStore) ensureDirectories() error {
	for _, directory := range []string{store.root, filepath.Join(store.root, packDirectory)} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}
