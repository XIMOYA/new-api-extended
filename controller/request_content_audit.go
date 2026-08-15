// controller/request_content_audit.go
// 请求内容审计接口：分页元数据、授权详情、流式正文、脱敏预览和原件资源访问。
package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	auditstore "github.com/QuantumNous/new-api/service/request_content_audit"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const requestContentAuditPreviewLimit = 16 << 10

var (
	redactedContentTooLarge    = []byte(`{"redacted":true,"truncated":true,"content":"[REDACTED]"}`)
	redactedContentUnavailable = []byte(`{"redacted":true,"content":"[REDACTED]"}`)
)

func RequestContentAuditNoStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Next()
}

type requestContentAuditAccess struct {
	FullContent bool
	Redacted    bool
}

type requestContentAuditSummary struct {
	ID                   int64  `json:"id"`
	RequestID            string `json:"request_id"`
	UpstreamRequestID    string `json:"upstream_request_id,omitempty"`
	UserID               int    `json:"user_id"`
	Username             string `json:"username,omitempty"`
	TokenID              int    `json:"token_id,omitempty"`
	ChannelID            int    `json:"channel_id,omitempty"`
	ModelName            string `json:"model_name"`
	RequestType          string `json:"request_type"`
	RelayFormat          string `json:"relay_format"`
	EndpointPath         string `json:"endpoint_path"`
	ContentType          string `json:"content_type"`
	CreatedAt            int64  `json:"created_at"`
	ExpiresAt            int64  `json:"expires_at"`
	ContentSize          int64  `json:"content_size"`
	StoredSize           int64  `json:"stored_size"`
	MessageCount         int    `json:"message_count"`
	AssetCount           int    `json:"asset_count"`
	CaptureStatus        string `json:"capture_status"`
	NormalizationVersion int    `json:"normalization_version"`
	ContentAvailable     bool   `json:"content_available"`
	CanViewFullContent   bool   `json:"can_view_full_content"`
	IsRedacted           bool   `json:"is_redacted"`
}

type requestContentAuditAssetSummary struct {
	AssetKey           string `json:"asset_key"`
	FileName           string `json:"file_name,omitempty"`
	AssetType          string `json:"asset_type"`
	MimeType           string `json:"mime_type"`
	OriginalSize       int64  `json:"original_size"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	ThumbnailAvailable bool   `json:"thumbnail_available"`
	OriginalAvailable  bool   `json:"original_available"`
}

type requestContentAuditDetail struct {
	requestContentAuditSummary
	Assets []requestContentAuditAssetSummary `json:"assets"`
}

type requestContentAuditPreview struct {
	RequestID string `json:"request_id"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Redacted  bool   `json:"redacted"`
}

func ListRequestContentAudits(c *gin.Context) {
	store, err := service.EnsureRequestContentAuditStore(c.Request.Context())
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	filter := model.RequestContentAuditListFilter{
		RequestId:   strings.TrimSpace(c.Query("request_id")),
		ModelName:   strings.TrimSpace(c.Query("model_name")),
		RequestType: strings.TrimSpace(c.Query("request_type")),
		Offset:      pageInfo.GetStartIdx(),
		Limit:       pageInfo.GetPageSize(),
	}
	filter.StartTime, _ = strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	filter.EndTime, _ = strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser
	isAllowlistedAdmin := isAdmin && service.RequestContentAuditAdminAllowlisted(c.GetInt("id"))
	if role < common.RoleRootUser && !isAllowlistedAdmin {
		if !isAdmin && !service.RequestContentAuditUserViewEnabled() {
			writeRequestContentAuditError(c, http.StatusForbidden, errors.New("request content audit user viewing is disabled"))
			return
		}
		filter.UserId = c.GetInt("id")
	}

	audits, total, err := store.ListAudits(c.Request.Context(), filter)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	items := make([]requestContentAuditSummary, 0, len(audits))
	for index := range audits {
		access, allowed := requestContentAuditAccessFor(c, &audits[index])
		if !allowed {
			continue
		}
		items = append(items, makeRequestContentAuditSummary(&audits[index], access))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetRequestContentAudit(c *gin.Context) {
	store, audit, access, ok := loadAuthorizedRequestContentAudit(c, c.Param("id"))
	if !ok {
		return
	}
	assets, err := store.GetAssets(c.Request.Context(), audit.Id)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	detail := requestContentAuditDetail{
		requestContentAuditSummary: makeRequestContentAuditSummary(audit, access),
		Assets:                     make([]requestContentAuditAssetSummary, 0, len(assets)),
	}
	for index := range assets {
		detail.Assets = append(detail.Assets, requestContentAuditAssetSummary{
			AssetKey:           assets[index].AssetKey,
			FileName:           requestContentAuditAssetFileName(&assets[index], access),
			AssetType:          assets[index].AssetType,
			MimeType:           assets[index].MimeType,
			OriginalSize:       assets[index].OriginalSize,
			Width:              assets[index].Width,
			Height:             assets[index].Height,
			ThumbnailAvailable: assets[index].ThumbnailObjectId > 0 && (access.FullContent || !access.Redacted),
			OriginalAvailable:  assets[index].OriginalObjectId > 0 && access.FullContent,
		})
	}
	common.ApiSuccess(c, detail)
}

func GetRequestContentAuditView(c *gin.Context) {
	store, audit, access, ok := loadAuthorizedRequestContentAudit(c, c.Param("id"))
	if !ok {
		return
	}
	view, err := service.BuildRequestContentAuditView(c.Request.Context(), store, audit, access.Redacted)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, view)
}

func GetRequestContentAuditViewSection(c *gin.Context) {
	store, audit, access, ok := loadAuthorizedRequestContentAudit(c, c.Param("id"))
	if !ok {
		return
	}
	section, err := service.GetRequestContentAuditViewSection(
		c.Request.Context(),
		store,
		audit,
		c.Param("section_id"),
		access.Redacted,
	)
	if err != nil {
		if errors.Is(err, service.ErrRequestContentAuditViewSectionMissing) {
			writeRequestContentAuditError(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, service.ErrRequestContentAuditViewSourceTooLarge) {
			writeRequestContentAuditError(c, http.StatusRequestEntityTooLarge, err)
			return
		}
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, section)
}

func GetRequestContentAuditByRequestID(c *gin.Context) {
	store, err := service.EnsureRequestContentAuditStore(c.Request.Context())
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	audit, err := store.GetAuditByRequestId(c.Request.Context(), c.Param("request_id"))
	if err != nil {
		writeRequestContentAuditLookupError(c, err)
		return
	}
	access, allowed := requestContentAuditAccessFor(c, audit)
	if !allowed {
		writeRequestContentAuditError(c, http.StatusNotFound, errors.New("request content audit record not found"))
		return
	}
	assets, err := store.GetAssets(c.Request.Context(), audit.Id)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	detail := requestContentAuditDetail{
		requestContentAuditSummary: makeRequestContentAuditSummary(audit, access),
		Assets:                     make([]requestContentAuditAssetSummary, 0, len(assets)),
	}
	for index := range assets {
		detail.Assets = append(detail.Assets, requestContentAuditAssetSummary{
			AssetKey:           assets[index].AssetKey,
			FileName:           requestContentAuditAssetFileName(&assets[index], access),
			AssetType:          assets[index].AssetType,
			MimeType:           assets[index].MimeType,
			OriginalSize:       assets[index].OriginalSize,
			Width:              assets[index].Width,
			Height:             assets[index].Height,
			ThumbnailAvailable: assets[index].ThumbnailObjectId > 0 && (access.FullContent || !access.Redacted),
			OriginalAvailable:  assets[index].OriginalObjectId > 0 && access.FullContent,
		})
	}
	common.ApiSuccess(c, detail)
}

func GetRequestContentAuditPreview(c *gin.Context) {
	store, audit, access, ok := loadAuthorizedRequestContentAudit(c, c.Param("id"))
	if !ok {
		return
	}
	preview, err := readRequestContentAuditPreview(c, store, audit, access)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func GetRequestContentAuditPreviewByRequestID(c *gin.Context) {
	store, err := service.EnsureRequestContentAuditStore(c.Request.Context())
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	audit, err := store.GetAuditByRequestId(c.Request.Context(), c.Param("request_id"))
	if err != nil {
		writeRequestContentAuditLookupError(c, err)
		return
	}
	access, allowed := requestContentAuditAccessFor(c, audit)
	if !allowed {
		writeRequestContentAuditError(c, http.StatusNotFound, errors.New("request content audit record not found"))
		return
	}
	preview, err := readRequestContentAuditPreview(c, store, audit, access)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func StreamRequestContentAuditContent(c *gin.Context) {
	store, audit, access, ok := loadAuthorizedRequestContentAudit(c, c.Param("id"))
	if !ok {
		return
	}
	if !access.FullContent && !access.Redacted {
		writeRequestContentAuditError(c, http.StatusForbidden, errors.New("request content audit content access denied"))
		return
	}
	if !audit.ContentAvailable() {
		writeRequestContentAuditError(c, http.StatusNotFound, errors.New("request content audit content is unavailable"))
		return
	}
	reader, err := store.OpenValidatedAuditContent(c.Request.Context(), audit)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Accel-Buffering", "no")
	if access.FullContent {
		writer := io.Writer(c.Writer)
		if flusher, ok := c.Writer.(http.Flusher); ok {
			writer = requestContentFlushWriter{writer: c.Writer, flusher: flusher}
		}
		if _, err := io.Copy(writer, reader); err != nil {
			return
		}
		return
	}

	maxRecordBytes := service.RequestContentAuditSettings().MaxRecordBytes
	if audit.ContentSize > maxRecordBytes {
		_, _ = c.Writer.Write(redactedContentTooLarge)
		return
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxRecordBytes+1))
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	if int64(len(content)) > maxRecordBytes {
		_, _ = c.Writer.Write(redactedContentTooLarge)
		return
	}
	var value any
	if err := decodeRequestContentJSON(content, &value); err != nil {
		_, _ = c.Writer.Write(redactedContentUnavailable)
		return
	}
	content, err = common.Marshal(auditstore.RedactValue(value))
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	_, _ = c.Writer.Write(content)
}

func StreamRequestContentAuditAsset(c *gin.Context) {
	store, audit, access, ok := loadAuthorizedRequestContentAudit(c, c.Param("id"))
	if !ok {
		return
	}
	asset, err := store.GetAssetByKey(c.Request.Context(), audit.Id, c.Param("asset_key"))
	if err != nil {
		writeRequestContentAuditLookupError(c, err)
		return
	}
	variant := c.Param("variant")
	objectID := asset.OriginalObjectId
	contentType := asset.MimeType
	contentLength := asset.OriginalSize
	if variant == "thumbnail" {
		if !access.FullContent {
			writeRequestContentAuditError(c, http.StatusForbidden, errors.New("request content audit thumbnail access denied"))
			return
		}
		if asset.ThumbnailObjectId <= 0 {
			writeRequestContentAuditError(c, http.StatusNotFound, errors.New("request content audit thumbnail is unavailable"))
			return
		}
		objectID = asset.ThumbnailObjectId
		contentType = asset.ThumbnailMimeType
		contentLength = asset.ThumbnailSize
	} else if variant != "original" {
		writeRequestContentAuditError(c, http.StatusBadRequest, errors.New("invalid request content audit asset variant"))
		return
	} else if !access.FullContent {
		writeRequestContentAuditError(c, http.StatusForbidden, errors.New("request content audit original access denied"))
		return
	}
	object, err := store.GetObjectById(c.Request.Context(), objectID)
	if err != nil {
		writeRequestContentAuditLookupError(c, err)
		return
	}
	reader, err := store.OpenValidatedAsset(c.Request.Context(), object)
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Disposition", `attachment; filename="request-content-asset"`)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Accel-Buffering", "no")
	writer := io.Writer(c.Writer)
	if flusher, ok := c.Writer.(http.Flusher); ok {
		writer = requestContentFlushWriter{writer: c.Writer, flusher: flusher}
	}
	if _, err := io.Copy(writer, reader); err != nil {
		return
	}
}

func loadAuthorizedRequestContentAudit(c *gin.Context, identifier string) (*auditstore.Store, *model.RequestContentAudit, requestContentAuditAccess, bool) {
	store, err := service.EnsureRequestContentAuditStore(c.Request.Context())
	if err != nil {
		writeRequestContentAuditError(c, http.StatusInternalServerError, err)
		return nil, nil, requestContentAuditAccess{}, false
	}
	id, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil || id <= 0 {
		writeRequestContentAuditError(c, http.StatusBadRequest, errors.New("invalid request content audit ID"))
		return nil, nil, requestContentAuditAccess{}, false
	}
	audit, err := store.GetAuditById(c.Request.Context(), id)
	if err != nil {
		writeRequestContentAuditLookupError(c, err)
		return nil, nil, requestContentAuditAccess{}, false
	}
	access, allowed := requestContentAuditAccessFor(c, audit)
	if !allowed {
		writeRequestContentAuditError(c, http.StatusNotFound, errors.New("request content audit record not found"))
		return nil, nil, requestContentAuditAccess{}, false
	}
	return store, audit, access, true
}

func requestContentAuditAccessFor(c *gin.Context, audit *model.RequestContentAudit) (requestContentAuditAccess, bool) {
	if audit == nil {
		return requestContentAuditAccess{}, false
	}
	role := c.GetInt("role")
	userID := c.GetInt("id")
	if role >= common.RoleRootUser {
		return requestContentAuditAccess{FullContent: true}, true
	}
	if audit.UserId == userID {
		if role >= common.RoleAdminUser || service.RequestContentAuditUserViewEnabled() {
			return requestContentAuditAccess{FullContent: true}, true
		}
		return requestContentAuditAccess{}, false
	}
	if role >= common.RoleAdminUser && service.RequestContentAuditAdminAllowlisted(userID) {
		return requestContentAuditAccess{Redacted: true}, true
	}
	return requestContentAuditAccess{}, false
}

func requestContentAuditAssetFileName(asset *model.RequestContentAsset, access requestContentAuditAccess) string {
	if asset == nil || access.Redacted {
		return ""
	}
	return asset.FileName
}

func makeRequestContentAuditSummary(audit *model.RequestContentAudit, access requestContentAuditAccess) requestContentAuditSummary {
	return requestContentAuditSummary{
		ID:                   audit.Id,
		RequestID:            audit.RequestId,
		UpstreamRequestID:    audit.UpstreamRequestId,
		UserID:               audit.UserId,
		Username:             audit.Username,
		TokenID:              audit.TokenId,
		ChannelID:            audit.ChannelId,
		ModelName:            audit.ModelName,
		RequestType:          audit.RequestType,
		RelayFormat:          audit.RelayFormat,
		EndpointPath:         audit.EndpointPath,
		ContentType:          audit.ContentType,
		CreatedAt:            audit.CreatedAt,
		ExpiresAt:            audit.ExpiresAt,
		ContentSize:          audit.ContentSize,
		StoredSize:           audit.StoredSize,
		MessageCount:         audit.MessageCount,
		AssetCount:           audit.AssetCount,
		CaptureStatus:        audit.CaptureStatus,
		NormalizationVersion: audit.NormalizationVersion,
		ContentAvailable:     audit.ContentAvailable(),
		CanViewFullContent:   access.FullContent,
		IsRedacted:           access.Redacted,
	}
}

func readRequestContentAuditPreview(c *gin.Context, store *auditstore.Store, audit *model.RequestContentAudit, access requestContentAuditAccess) (requestContentAuditPreview, error) {
	result := requestContentAuditPreview{RequestID: audit.RequestId, Redacted: access.Redacted}
	if !audit.ContentAvailable() {
		return result, nil
	}
	reader, err := store.OpenValidatedAuditContent(c.Request.Context(), audit)
	if err != nil {
		return result, err
	}
	defer reader.Close()

	var content []byte
	if access.Redacted {
		maxRecordBytes := service.RequestContentAuditSettings().MaxRecordBytes
		if audit.ContentSize > maxRecordBytes {
			result.Content = "[REDACTED]"
			result.Truncated = true
			return result, nil
		}
		content, err = io.ReadAll(io.LimitReader(reader, maxRecordBytes+1))
		if err != nil {
			return result, err
		}
		if int64(len(content)) > maxRecordBytes {
			result.Content = "[REDACTED]"
			result.Truncated = true
			return result, nil
		}
		var value any
		if err := decodeRequestContentJSON(content, &value); err != nil {
			result.Content = "[REDACTED]"
			result.Truncated = true
			return result, nil
		}
		content, err = common.Marshal(auditstore.RedactValue(value))
		if err != nil {
			return result, err
		}
	} else {
		limit := requestContentAuditPreviewLimit * 4
		content, err = io.ReadAll(io.LimitReader(reader, int64(limit+1)))
		if err != nil {
			return result, err
		}
		if len(content) > limit {
			content = content[:limit]
			result.Truncated = true
		}
	}
	if len(content) > requestContentAuditPreviewLimit {
		content = content[:requestContentAuditPreviewLimit]
		result.Truncated = true
	}
	result.Content = string(content)
	return result, nil
}

type requestContentFlushWriter struct {
	writer  io.Writer
	flusher http.Flusher
}

func (writer requestContentFlushWriter) Write(data []byte) (int, error) {
	written, err := writer.writer.Write(data)
	if err == nil {
		writer.flusher.Flush()
	}
	return written, err
}

func decodeRequestContentJSON(data []byte, value *any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(value)
}

func writeRequestContentAuditLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeRequestContentAuditError(c, http.StatusNotFound, errors.New("request content audit record not found"))
		return
	}
	writeRequestContentAuditError(c, http.StatusInternalServerError, err)
}

func writeRequestContentAuditError(c *gin.Context, status int, err error) {
	message := "request content audit request failed"
	if errors.Is(err, auditstore.ErrEncryptionKeyUnavailable) {
		status = http.StatusServiceUnavailable
		message = "request content audit encryption key is unavailable"
	} else if err != nil && status < http.StatusInternalServerError {
		message = err.Error()
	}
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
	})
}
