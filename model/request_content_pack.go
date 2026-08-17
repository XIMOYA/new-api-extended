// model/request_content_pack.go
// 请求内容审计的内容寻址存储元数据：打包容器（pack）与去重内容块（blob）的模型、跨库仓储操作。
// 内容按 sha256 去重后追加进 pack 文件，多条审计记录靠引用计数共享同一份内容；
// 引用归零且过期的块由清理任务回收，空 pack 的文件路径交回 service 层删除。
package model

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 单条语句里携带的 sha 数量上限，避免占位符规模失控。
	requestContentShaBatchSize = 500
	// 单条语句里携带的主键数量上限。
	requestContentIdBatchSize = 500
	// 批量插入块记录的分片大小。
	requestContentBlobInsertBatchSize = 200
)

// ErrRequestContentPackConflict 表示 pack 元数据已被另一个写入方推进，本次追加写使用的偏移已经失效。
var ErrRequestContentPackConflict = errors.New("request content pack was modified by another writer")

// RequestContentPack 是一个追加写的加密容器文件，内部按记录存放多个去重后的内容块。
type RequestContentPack struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	StoragePath string `json:"-" gorm:"size:512;uniqueIndex:idx_request_content_packs_path"`
	KeyId       string `json:"key_id" gorm:"size:128"`
	HeaderSize  int    `json:"header_size"`
	Size        int64  `json:"size"`
	ChunkIndex  uint32 `json:"chunk_index"` // 下一个记录使用的 AEAD chunk index
	BlobCount   int64  `json:"blob_count"`
	LiveBytes   int64  `json:"live_bytes"` // 仍被引用的块的存储字节数，用于碎片率判断
	Sealed      bool   `json:"sealed" gorm:"index:idx_request_content_packs_sealed"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at" gorm:"index:idx_request_content_packs_expiry"`
}

func (RequestContentPack) TableName() string {
	return "request_content_packs"
}

// RequestContentBlob 是一段内容寻址的去重内容（一条消息、一个内容块）。
type RequestContentBlob struct {
	Id         int64  `json:"id" gorm:"primaryKey"`
	Sha256     string `json:"sha256" gorm:"size:64;uniqueIndex:idx_request_content_blobs_sha"`
	PackId     int64  `json:"pack_id" gorm:"index:idx_request_content_blobs_pack"`
	Offset     int64  `json:"offset"`      // 记录起始偏移（记录类型字节的位置）
	Length     int64  `json:"length"`      // 记录总长度（1 + 4 + sealed）
	ChunkIndex uint32 `json:"chunk_index"` // 该记录在 pack 中的 AEAD chunk index
	PlainSize  int64  `json:"plain_size"`
	RefCount   int64  `json:"ref_count" gorm:"index:idx_request_content_blobs_ref_count"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at" gorm:"index:idx_request_content_blobs_expiry"`
}

func (RequestContentBlob) TableName() string {
	return "request_content_blobs"
}

type RequestContentPackRepository struct {
	db *gorm.DB
}

func NewRequestContentPackRepository(db *gorm.DB) *RequestContentPackRepository {
	return &RequestContentPackRepository{db: db}
}

func (repository *RequestContentPackRepository) session(ctx context.Context) (*gorm.DB, error) {
	if repository == nil || repository.db == nil {
		return nil, errors.New("request content pack repository database is nil")
	}
	return repository.db.WithContext(ctx), nil
}

func (repository *RequestContentPackRepository) Migrate(ctx context.Context) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	return db.AutoMigrate(&RequestContentPack{}, &RequestContentBlob{})
}

// ActivePack 返回还能继续追加的 pack；没有可用的返回 nil, nil，由调用方新建。
func (repository *RequestContentPackRepository) ActivePack(ctx context.Context, maxPackSize int64) (*RequestContentPack, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return nil, err
	}
	if maxPackSize <= 0 {
		return nil, errors.New("request content pack maximum size must be positive")
	}
	var pack RequestContentPack
	err = db.Where("sealed = ? AND size < ?", false, maxPackSize).Order("id ASC").First(&pack).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pack, nil
}

func (repository *RequestContentPackRepository) CreatePack(ctx context.Context, pack *RequestContentPack) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	if pack == nil {
		return errors.New("request content pack is nil")
	}
	if pack.StoragePath == "" {
		return errors.New("request content pack storage path is empty")
	}
	return db.Create(pack).Error
}

func (repository *RequestContentPackRepository) PackById(ctx context.Context, id int64) (*RequestContentPack, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return nil, err
	}
	var pack RequestContentPack
	if err := db.First(&pack, id).Error; err != nil {
		return nil, err
	}
	return &pack, nil
}

func (repository *RequestContentPackRepository) SealPack(ctx context.Context, packId int64) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	if packId <= 0 {
		return errors.New("request content pack id is invalid")
	}
	return db.Model(&RequestContentPack{}).Where("id = ?", packId).UpdateColumn("sealed", true).Error
}

// UpdatePackAfterAppend 用一条语句推进 pack 元数据：size 与 chunk_index 直接覆盖，
// 计数列按增量累加（允许负增量做回退），expires_at 只向后延长。
func (repository *RequestContentPackRepository) UpdatePackAfterAppend(ctx context.Context, packId int64, size int64, chunkIndex uint32, addedBlobs int64, addedLiveBytes int64, expiresAt int64) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	if packId <= 0 {
		return errors.New("request content pack id is invalid")
	}
	return db.Model(&RequestContentPack{}).Where("id = ?", packId).
		UpdateColumns(requestContentPackAppendUpdates(size, chunkIndex, addedBlobs, addedLiveBytes, expiresAt)).Error
}

// AppendBlobs 把新块记录与 pack 元数据放进同一个事务：块记录一旦提交，pack.size 必须已经覆盖它们指向的文件区间，
// 否则下一次追加写会把这段字节当成崩溃残留截掉。previousSize 用作乐观校验，命中冲突说明另有写入方推进过同一个 pack。
func (repository *RequestContentPackRepository) AppendBlobs(ctx context.Context, packId int64, blobs []RequestContentBlob, previousSize int64, size int64, chunkIndex uint32, expiresAt int64) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	if packId <= 0 {
		return errors.New("request content pack id is invalid")
	}
	if len(blobs) == 0 {
		return errors.New("request content blob list is empty")
	}
	var addedLiveBytes int64
	for index := range blobs {
		addedLiveBytes += blobs[index].Length
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := createRequestContentBlobs(tx, blobs); err != nil {
			return err
		}
		result := tx.Model(&RequestContentPack{}).Where("id = ? AND size = ?", packId, previousSize).
			UpdateColumns(requestContentPackAppendUpdates(size, chunkIndex, int64(len(blobs)), addedLiveBytes, expiresAt))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrRequestContentPackConflict
		}
		return nil
	})
}

func (repository *RequestContentPackRepository) CreateBlobs(ctx context.Context, blobs []RequestContentBlob) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	return createRequestContentBlobs(db, blobs)
}

func requestContentPackAppendUpdates(size int64, chunkIndex uint32, addedBlobs int64, addedLiveBytes int64, expiresAt int64) map[string]any {
	return map[string]any{
		"size":        size,
		"chunk_index": chunkIndex,
		"blob_count":  requestContentCounterExpr("blob_count", addedBlobs),
		"live_bytes":  requestContentCounterExpr("live_bytes", addedLiveBytes),
		"expires_at":  requestContentExpiryValue("expires_at", expiresAt),
	}
}

// createRequestContentBlobs 允许并发写入方抢先插入同一个 sha：冲突行直接跳过，
// 因此调用方不能相信回填的主键，需要按 sha 重新读取。
func createRequestContentBlobs(tx *gorm.DB, blobs []RequestContentBlob) error {
	for start := 0; start < len(blobs); start += requestContentBlobInsertBatchSize {
		end := min(start+requestContentBlobInsertBatchSize, len(blobs))
		batch := blobs[start:end]
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sha256"}},
			DoNothing: true,
		}).Create(&batch).Error; err != nil {
			return err
		}
	}
	return nil
}

// FindBlobsBySha 按 sha 分批查询已存在的块，返回 sha 到块记录的映射。
func (repository *RequestContentPackRepository) FindBlobsBySha(ctx context.Context, shas []string) (map[string]RequestContentBlob, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return nil, err
	}
	_, unique := countRequestContentShas(shas)
	found := make(map[string]RequestContentBlob, len(unique))
	for start := 0; start < len(unique); start += requestContentShaBatchSize {
		end := min(start+requestContentShaBatchSize, len(unique))
		var blobs []RequestContentBlob
		if err := db.Where("sha256 IN ?", unique[start:end]).Find(&blobs).Error; err != nil {
			return nil, err
		}
		for index := range blobs {
			found[blobs[index].Sha256] = blobs[index]
		}
	}
	return found, nil
}

// AddBlobReferences 给已存在的块批量加引用：同一个 sha 在同一批里出现多次就按次数累加，
// 每批只发一条语句，同时把 expires_at 抬到更大值。
func (repository *RequestContentPackRepository) AddBlobReferences(ctx context.Context, shas []string, expiresAt int64) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	counts, unique := countRequestContentShas(shas)
	for start := 0; start < len(unique); start += requestContentShaBatchSize {
		end := min(start+requestContentShaBatchSize, len(unique))
		batch := unique[start:end]
		var builder strings.Builder
		args := make([]any, 0, len(batch)*2)
		builder.WriteString("ref_count + CASE sha256")
		for _, sha := range batch {
			builder.WriteString(" WHEN ? THEN ?")
			args = append(args, sha, counts[sha])
		}
		builder.WriteString(" ELSE 0 END")
		if err := db.Model(&RequestContentBlob{}).Where("sha256 IN ?", batch).UpdateColumns(map[string]any{
			"ref_count":  gorm.Expr(builder.String(), args...),
			"expires_at": requestContentExpiryValue("expires_at", expiresAt),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ReleaseBlobReferences 批量减引用，减到 0 就停住。这里用 CASE 而不是 GREATEST/MAX，
// 因为这三个数据库对那两个函数的支持并不一致。
func (repository *RequestContentPackRepository) ReleaseBlobReferences(ctx context.Context, shas []string) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	counts, unique := countRequestContentShas(shas)
	for start := 0; start < len(unique); start += requestContentShaBatchSize {
		end := min(start+requestContentShaBatchSize, len(unique))
		batch := unique[start:end]
		var builder strings.Builder
		args := make([]any, 0, len(batch)*3)
		builder.WriteString("CASE sha256")
		for _, sha := range batch {
			builder.WriteString(" WHEN ? THEN CASE WHEN ref_count > ? THEN ref_count - ? ELSE 0 END")
			args = append(args, sha, counts[sha], counts[sha])
		}
		builder.WriteString(" ELSE ref_count END")
		if err := db.Model(&RequestContentBlob{}).Where("sha256 IN ?", batch).
			UpdateColumn("ref_count", gorm.Expr(builder.String(), args...)).Error; err != nil {
			return err
		}
	}
	return nil
}

// ReclaimableBlobs 找出引用已归零且过期的块；expires_at 为 0 表示不受保留期限制，不参与回收。
func (repository *RequestContentPackRepository) ReclaimableBlobs(ctx context.Context, cutoff int64, limit int) ([]RequestContentBlob, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, errors.New("request content blob reclaim limit must be positive")
	}
	var blobs []RequestContentBlob
	if err := db.Where("ref_count <= ? AND expires_at > ? AND expires_at <= ?", 0, 0, cutoff).
		Order("expires_at ASC, id ASC").Limit(limit).Find(&blobs).Error; err != nil {
		return nil, err
	}
	return blobs, nil
}

// DeleteBlobs 删除块记录，并把所属 pack 的 blob_count / live_bytes 一起回退，
// 否则碎片率和空 pack 判断会一直停在写入时的数值。
func (repository *RequestContentPackRepository) DeleteBlobs(ctx context.Context, ids []int64) (int64, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	var deleted int64
	err = db.Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(ids); start += requestContentIdBatchSize {
			end := min(start+requestContentIdBatchSize, len(ids))
			var blobs []RequestContentBlob
			if err := tx.Where("id IN ?", ids[start:end]).Find(&blobs).Error; err != nil {
				return err
			}
			if len(blobs) == 0 {
				continue
			}
			blobIds := make([]int64, 0, len(blobs))
			packCounts := make(map[int64]int64, len(blobs))
			packBytes := make(map[int64]int64, len(blobs))
			for index := range blobs {
				blobIds = append(blobIds, blobs[index].Id)
				packCounts[blobs[index].PackId]++
				packBytes[blobs[index].PackId] += blobs[index].Length
			}
			result := tx.Where("id IN ?", blobIds).Delete(&RequestContentBlob{})
			if result.Error != nil {
				return result.Error
			}
			deleted += result.RowsAffected
			packIds := make([]int64, 0, len(packCounts))
			for packId := range packCounts {
				if packId > 0 {
					packIds = append(packIds, packId)
				}
			}
			slices.Sort(packIds)
			for _, packId := range packIds {
				if err := tx.Model(&RequestContentPack{}).Where("id = ?", packId).UpdateColumns(map[string]any{
					"blob_count": requestContentCounterExpr("blob_count", -packCounts[packId]),
					"live_bytes": requestContentCounterExpr("live_bytes", -packBytes[packId]),
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// RaisePackExpiry 把一批 pack 的 expires_at 抬到更大值。命中已有块的写入不会追加字节，
// 但 pack 的保留期必须跟着块一起延长，否则容器会比里面的内容先到期。
func (repository *RequestContentPackRepository) RaisePackExpiry(ctx context.Context, packIds []int64, expiresAt int64) error {
	db, err := repository.session(ctx)
	if err != nil {
		return err
	}
	if len(packIds) == 0 {
		return nil
	}
	for start := 0; start < len(packIds); start += requestContentIdBatchSize {
		end := min(start+requestContentIdBatchSize, len(packIds))
		if err := db.Model(&RequestContentPack{}).Where("id IN ?", packIds[start:end]).
			UpdateColumn("expires_at", requestContentExpiryValue("expires_at", expiresAt)).Error; err != nil {
			return err
		}
	}
	return nil
}

// EmptyPacks 找出不再持有块的 pack：blob_count 归零算空，查不到任何块记录也算空，
// 后者兜住计数被并发回退漏算的情况。未封存的空 pack 同样要回收，否则默认 8MB 的活动 pack
// 在块全部过期后会一直留着一个空文件；调用方持有追加锁，不会与写入交错。
func (repository *RequestContentPackRepository) EmptyPacks(ctx context.Context, limit int) ([]RequestContentPack, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, errors.New("request content pack reclaim limit must be positive")
	}
	blobTable := RequestContentBlob{}.TableName()
	condition := fmt.Sprintf(
		"blob_count <= ? OR NOT EXISTS (SELECT 1 FROM %s WHERE %s.pack_id = %s.id)",
		blobTable, blobTable, RequestContentPack{}.TableName(),
	)
	var packs []RequestContentPack
	if err := db.Model(&RequestContentPack{}).Where(condition, 0).
		Order("id ASC").Limit(limit).Find(&packs).Error; err != nil {
		return nil, err
	}
	return packs, nil
}

// AllPackPaths 返回所有 pack 的相对路径，供孤儿文件扫描比对。
func (repository *RequestContentPackRepository) AllPackPaths(ctx context.Context) ([]string, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return nil, err
	}
	var paths []string
	if err := db.Model(&RequestContentPack{}).Pluck("storage_path", &paths).Error; err != nil {
		return nil, err
	}
	return paths, nil
}

func (repository *RequestContentPackRepository) DeletePacks(ctx context.Context, ids []int64) (int64, error) {
	db, err := repository.session(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	var deleted int64
	for start := 0; start < len(ids); start += requestContentIdBatchSize {
		end := min(start+requestContentIdBatchSize, len(ids))
		result := db.Where("id IN ?", ids[start:end]).Delete(&RequestContentPack{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	return deleted, nil
}

// 计数列按增量累加，负增量表示回退，并且不允许落到负数。
func requestContentCounterExpr(column string, delta int64) clause.Expr {
	return gorm.Expr("CASE WHEN "+column+" + ? > 0 THEN "+column+" + ? ELSE 0 END", delta, delta)
}

// expires_at 只向后延长；0 表示不受保留期限制，出现过就必须保持 0。
func requestContentExpiryValue(column string, expiresAt int64) any {
	if expiresAt <= 0 {
		return int64(0)
	}
	return gorm.Expr("CASE WHEN "+column+" <= 0 THEN 0 WHEN "+column+" < ? THEN ? ELSE "+column+" END", expiresAt, expiresAt)
}

// countRequestContentShas 统计每个 sha 出现的次数，并按首次出现顺序返回去重后的 sha 列表。
func countRequestContentShas(shas []string) (map[string]int64, []string) {
	counts := make(map[string]int64, len(shas))
	unique := make([]string, 0, len(shas))
	for _, sha := range shas {
		if sha == "" {
			continue
		}
		if _, exists := counts[sha]; !exists {
			unique = append(unique, sha)
		}
		counts[sha]++
	}
	return counts, unique
}
