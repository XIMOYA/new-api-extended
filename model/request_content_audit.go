// model/request_content_audit.go
// 请求内容审计的轻量元数据模型与跨数据库仓储操作；正文和资源文件由 service 层持久化。
package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RequestContentCaptureStatusComplete = "complete"
	RequestContentCaptureStatusPartial  = "partial"
	RequestContentCaptureStatusFailed   = "failed"

	RequestContentObjectKindOriginal  = "original"
	RequestContentObjectKindThumbnail = "thumbnail"
)

// RequestContentAudit stores searchable request metadata while keeping request content out of the database.
type RequestContentAudit struct {
	Id                   int64  `json:"id" gorm:"primaryKey"`
	RequestId            string `json:"request_id" gorm:"size:128;uniqueIndex:idx_request_content_audits_request_id"`
	UpstreamRequestId    string `json:"upstream_request_id" gorm:"size:128;index:idx_request_content_audits_upstream_id"`
	UserId               int    `json:"user_id" gorm:"index:idx_request_content_audits_user_created,priority:1"`
	Username             string `json:"username" gorm:"size:128"`
	TokenId              int    `json:"token_id" gorm:"index:idx_request_content_audits_token_id"`
	ChannelId            int    `json:"channel_id" gorm:"index:idx_request_content_audits_channel_id"`
	ModelName            string `json:"model_name" gorm:"size:128;index:idx_request_content_audits_model_name"`
	RequestType          string `json:"request_type" gorm:"size:32;index:idx_request_content_audits_request_type"`
	RelayFormat          string `json:"relay_format" gorm:"size:64;index:idx_request_content_audits_relay_format"`
	EndpointPath         string `json:"endpoint_path" gorm:"size:256"`
	ContentType          string `json:"content_type" gorm:"size:128"`
	CreatedAt            int64  `json:"created_at" gorm:"index:idx_request_content_audits_user_created,priority:2"`
	ExpiresAt            int64  `json:"expires_at" gorm:"index:idx_request_content_audits_expiry"`
	ContentPath          string `json:"-" gorm:"size:512"`
	ContentSha256        string `json:"content_sha256" gorm:"size:64;index:idx_request_content_audits_content_sha"`
	ContentSize          int64  `json:"content_size"`
	StoredSize           int64  `json:"stored_size"`
	MessageCount         int    `json:"message_count"`
	AssetCount           int    `json:"asset_count"`
	RiskLevel            string `json:"risk_level" gorm:"size:32;index:idx_request_content_audits_risk_level"`
	RedactionVersion     int    `json:"redaction_version"`
	CaptureStatus        string `json:"capture_status" gorm:"size:16;index:idx_request_content_audits_capture_status"`
	CaptureErrorCode     string `json:"capture_error_code,omitempty" gorm:"size:64"`
	NormalizationVersion int    `json:"normalization_version"`
	IntegrityVersion     int    `json:"integrity_version"`
	LegalHold            bool   `json:"legal_hold" gorm:"index:idx_request_content_audits_expiry"`
}

func (RequestContentAudit) TableName() string {
	return "request_content_audits"
}

func (audit *RequestContentAudit) ContentAvailable() bool {
	return audit != nil && audit.ContentPath != "" && audit.CaptureStatus == RequestContentCaptureStatusComplete
}

// RequestContentObject identifies one deduplicated encrypted resource file.
type RequestContentObject struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	ObjectKind  string `json:"object_kind" gorm:"size:16;uniqueIndex:idx_request_content_objects_kind_sha,priority:1"`
	Sha256      string `json:"sha256" gorm:"size:64;uniqueIndex:idx_request_content_objects_kind_sha,priority:2"`
	StoragePath string `json:"-" gorm:"size:512;uniqueIndex:idx_request_content_objects_path"`
	MimeType    string `json:"mime_type" gorm:"size:128"`
	PlainSize   int64  `json:"plain_size"`
	StoredSize  int64  `json:"stored_size"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	RefCount    int64  `json:"ref_count" gorm:"index:idx_request_content_objects_ref_count"`
	CreatedAt   int64  `json:"created_at"`
}

func (RequestContentObject) TableName() string {
	return "request_content_objects"
}

// RequestContentAsset links an audit record to its original and optional thumbnail objects.
type RequestContentAsset struct {
	Id                int64  `json:"id" gorm:"primaryKey"`
	AuditId           int64  `json:"audit_id" gorm:"uniqueIndex:idx_request_content_assets_audit_position,priority:1;index:idx_request_content_assets_audit_id"`
	Position          int    `json:"position" gorm:"uniqueIndex:idx_request_content_assets_audit_position,priority:2"`
	AssetKey          string `json:"asset_key" gorm:"size:64"`
	FileName          string `json:"file_name,omitempty" gorm:"size:256"`
	AssetType         string `json:"asset_type" gorm:"size:32"`
	MimeType          string `json:"mime_type" gorm:"size:128"`
	OriginalObjectId  int64  `json:"original_object_id" gorm:"index:idx_request_content_assets_original_object"`
	ThumbnailObjectId int64  `json:"thumbnail_object_id,omitempty" gorm:"index:idx_request_content_assets_thumbnail_object"`
	OriginalSize      int64  `json:"original_size"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	Sha256            string `json:"sha256" gorm:"size:64;index:idx_request_content_assets_sha"`
	ThumbnailMimeType string `json:"thumbnail_mime_type,omitempty" gorm:"size:128"`
	ThumbnailSize     int64  `json:"thumbnail_size,omitempty"`
	ThumbnailWidth    int    `json:"thumbnail_width,omitempty"`
	ThumbnailHeight   int    `json:"thumbnail_height,omitempty"`
	ThumbnailSha256   string `json:"thumbnail_sha256,omitempty" gorm:"size:64"`
	CreatedAt         int64  `json:"created_at"`
	ExpiresAt         int64  `json:"expires_at" gorm:"index:idx_request_content_assets_expiry"`
}

func (RequestContentAsset) TableName() string {
	return "request_content_assets"
}

type RequestContentAssetCreate struct {
	Asset     RequestContentAsset
	Original  RequestContentObject
	Thumbnail *RequestContentObject
}

type DeletedRequestContentFiles struct {
	AuditCount  int64
	AssetCount  int64
	ObjectCount int64
	Paths       []string
}

type RequestContentAuditRepository struct {
	db *gorm.DB
}

type RequestContentAuditListFilter struct {
	UserId      int
	RequestId   string
	ModelName   string
	RequestType string
	StartTime   int64
	EndTime     int64
	Offset      int
	Limit       int
}

func NewRequestContentAuditRepository(db *gorm.DB) *RequestContentAuditRepository {
	return &RequestContentAuditRepository{db: db}
}

func (repository *RequestContentAuditRepository) Migrate(ctx context.Context) error {
	if repository == nil || repository.db == nil {
		return errors.New("request content audit repository database is nil")
	}
	return repository.db.WithContext(ctx).AutoMigrate(
		&RequestContentAudit{},
		&RequestContentObject{},
		&RequestContentAsset{},
	)
}

func (repository *RequestContentAuditRepository) Create(ctx context.Context, audit *RequestContentAudit, assets []RequestContentAssetCreate) error {
	if repository == nil || repository.db == nil {
		return errors.New("request content audit repository database is nil")
	}
	if audit == nil {
		return errors.New("request content audit is nil")
	}

	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		audit.AssetCount = len(assets)
		if err := tx.Create(audit).Error; err != nil {
			return err
		}

		for i := range assets {
			item := &assets[i]
			original, err := upsertRequestContentObject(tx, &item.Original)
			if err != nil {
				return err
			}

			item.Asset.AuditId = audit.Id
			item.Asset.OriginalObjectId = original.Id
			item.Asset.OriginalSize = original.PlainSize
			item.Asset.Sha256 = original.Sha256
			if item.Asset.MimeType == "" {
				item.Asset.MimeType = original.MimeType
			}
			if item.Asset.Width == 0 {
				item.Asset.Width = original.Width
			}
			if item.Asset.Height == 0 {
				item.Asset.Height = original.Height
			}

			if item.Thumbnail != nil {
				thumbnail, err := upsertRequestContentObject(tx, item.Thumbnail)
				if err != nil {
					return err
				}
				item.Asset.ThumbnailObjectId = thumbnail.Id
				item.Asset.ThumbnailMimeType = thumbnail.MimeType
				item.Asset.ThumbnailSize = thumbnail.PlainSize
				item.Asset.ThumbnailWidth = thumbnail.Width
				item.Asset.ThumbnailHeight = thumbnail.Height
				item.Asset.ThumbnailSha256 = thumbnail.Sha256
			}

			if err := tx.Create(&item.Asset).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertRequestContentObject(tx *gorm.DB, candidate *RequestContentObject) (*RequestContentObject, error) {
	if candidate == nil {
		return nil, errors.New("request content object is nil")
	}
	candidate.RefCount = 0
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "object_kind"}, {Name: "sha256"}},
		DoNothing: true,
	}).Create(candidate).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&RequestContentObject{}).
		Where("object_kind = ? AND sha256 = ?", candidate.ObjectKind, candidate.Sha256).
		UpdateColumn("ref_count", gorm.Expr("ref_count + ?", 1)).Error; err != nil {
		return nil, err
	}

	var object RequestContentObject
	if err := tx.Where("object_kind = ? AND sha256 = ?", candidate.ObjectKind, candidate.Sha256).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (repository *RequestContentAuditRepository) GetAuditById(ctx context.Context, id int64) (*RequestContentAudit, error) {
	var audit RequestContentAudit
	if err := repository.db.WithContext(ctx).First(&audit, id).Error; err != nil {
		return nil, err
	}
	return &audit, nil
}

func (repository *RequestContentAuditRepository) GetAuditByRequestId(ctx context.Context, requestId string) (*RequestContentAudit, error) {
	var audit RequestContentAudit
	if err := repository.db.WithContext(ctx).Where("request_id = ?", requestId).First(&audit).Error; err != nil {
		return nil, err
	}
	return &audit, nil
}

func (repository *RequestContentAuditRepository) List(ctx context.Context, filter RequestContentAuditListFilter) ([]RequestContentAudit, int64, error) {
	if repository == nil || repository.db == nil {
		return nil, 0, errors.New("request content audit repository database is nil")
	}
	query := repository.db.WithContext(ctx).Model(&RequestContentAudit{})
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.RequestId != "" {
		query = query.Where("request_id = ?", filter.RequestId)
	}
	if filter.ModelName != "" {
		query = query.Where("model_name = ?", filter.ModelName)
	}
	if filter.RequestType != "" {
		query = query.Where("request_type = ?", filter.RequestType)
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	var audits []RequestContentAudit
	if err := query.Order("created_at DESC, id DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&audits).Error; err != nil {
		return nil, 0, err
	}
	return audits, total, nil
}

func (repository *RequestContentAuditRepository) GetAssets(ctx context.Context, auditId int64) ([]RequestContentAsset, error) {
	var assets []RequestContentAsset
	err := repository.db.WithContext(ctx).Where("audit_id = ?", auditId).Order("position ASC").Find(&assets).Error
	return assets, err
}

func (repository *RequestContentAuditRepository) GetObjectById(ctx context.Context, id int64) (*RequestContentObject, error) {
	var object RequestContentObject
	if err := repository.db.WithContext(ctx).First(&object, id).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (repository *RequestContentAuditRepository) GetAssetByKey(ctx context.Context, auditId int64, assetKey string) (*RequestContentAsset, error) {
	var asset RequestContentAsset
	if err := repository.db.WithContext(ctx).
		Where("audit_id = ? AND asset_key = ?", auditId, assetKey).
		First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (repository *RequestContentAuditRepository) DeleteExpiredBatch(ctx context.Context, cutoff int64, batchSize int) (*DeletedRequestContentFiles, error) {
	if repository == nil || repository.db == nil {
		return nil, errors.New("request content audit repository database is nil")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("request content audit cleanup batch size must be positive")
	}

	result := &DeletedRequestContentFiles{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var audits []RequestContentAudit
		if err := tx.Where("expires_at > 0 AND expires_at <= ? AND legal_hold = ?", cutoff, false).
			Order("expires_at ASC, id ASC").Limit(batchSize).Find(&audits).Error; err != nil {
			return err
		}
		if len(audits) == 0 {
			return nil
		}

		auditIds := make([]int64, 0, len(audits))
		for i := range audits {
			auditIds = append(auditIds, audits[i].Id)
			if audits[i].ContentPath != "" {
				result.Paths = append(result.Paths, audits[i].ContentPath)
			}
		}

		var assets []RequestContentAsset
		if err := tx.Where("audit_id IN ?", auditIds).Find(&assets).Error; err != nil {
			return err
		}
		objectReferences := make(map[int64]int64)
		for i := range assets {
			objectReferences[assets[i].OriginalObjectId]++
			if assets[i].ThumbnailObjectId > 0 {
				objectReferences[assets[i].ThumbnailObjectId]++
			}
		}

		if len(assets) > 0 {
			deleteResult := tx.Where("audit_id IN ?", auditIds).Delete(&RequestContentAsset{})
			if deleteResult.Error != nil {
				return deleteResult.Error
			}
			result.AssetCount = deleteResult.RowsAffected
		}

		touchedObjectIds := make([]int64, 0, len(objectReferences))
		for objectId, count := range objectReferences {
			if objectId <= 0 {
				continue
			}
			touchedObjectIds = append(touchedObjectIds, objectId)
			if err := tx.Model(&RequestContentObject{}).Where("id = ?", objectId).
				UpdateColumn("ref_count", gorm.Expr("ref_count - ?", count)).Error; err != nil {
				return err
			}
		}

		if len(touchedObjectIds) > 0 {
			var unusedObjects []RequestContentObject
			if err := tx.Where("id IN ? AND ref_count <= ?", touchedObjectIds, 0).Find(&unusedObjects).Error; err != nil {
				return err
			}
			if len(unusedObjects) > 0 {
				unusedIds := make([]int64, 0, len(unusedObjects))
				for i := range unusedObjects {
					unusedIds = append(unusedIds, unusedObjects[i].Id)
					result.Paths = append(result.Paths, unusedObjects[i].StoragePath)
				}
				deleteResult := tx.Where("id IN ?", unusedIds).Delete(&RequestContentObject{})
				if deleteResult.Error != nil {
					return deleteResult.Error
				}
				result.ObjectCount = deleteResult.RowsAffected
			}
		}

		deleteResult := tx.Where("id IN ?", auditIds).Delete(&RequestContentAudit{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		result.AuditCount = deleteResult.RowsAffected
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (repository *RequestContentAuditRepository) ReferencedPaths(ctx context.Context) (map[string]struct{}, error) {
	if repository == nil || repository.db == nil {
		return nil, errors.New("request content audit repository database is nil")
	}
	paths := make(map[string]struct{})

	var auditPaths []string
	if err := repository.db.WithContext(ctx).Model(&RequestContentAudit{}).
		Where("content_path <> ?", "").Pluck("content_path", &auditPaths).Error; err != nil {
		return nil, err
	}
	for _, path := range auditPaths {
		paths[path] = struct{}{}
	}

	var objectPaths []string
	if err := repository.db.WithContext(ctx).Model(&RequestContentObject{}).
		Where("storage_path <> ?", "").Pluck("storage_path", &objectPaths).Error; err != nil {
		return nil, err
	}
	for _, path := range objectPaths {
		paths[path] = struct{}{}
	}
	return paths, nil
}
