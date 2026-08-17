// service/request_content_audit_capture.go
// 请求提交内容规范化：只处理已通过接口校验的请求，并将 Base64/Multipart 原件拆分为加密资源引用。
package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	auditstore "github.com/QuantumNous/new-api/service/request_content_audit"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	requestContentEnvelopeVersion    = 1
	requestContentThumbnailLimit     = 512
	requestContentThumbnailMaxPixels = 16_000_000
	requestContentCaptureMemoryLimit = 128 << 20
)

type RequestContentEnvelope struct {
	SchemaVersion int                         `json:"schema_version"`
	Kind          string                      `json:"kind"`
	RequestID     string                      `json:"request_id"`
	RelayFormat   string                      `json:"relay_format"`
	ContentType   string                      `json:"content_type"`
	EndpointPath  string                      `json:"endpoint_path"`
	Payload       any                         `json:"payload,omitempty"`
	Assets        []RequestContentAsset       `json:"assets,omitempty"`
	Original      *RequestContentOriginal     `json:"original,omitempty"`
	Prune         *RequestContentPruneSummary `json:"audit_prune,omitempty"`
}

type RequestContentAsset struct {
	AssetKey  string `json:"asset_key"`
	FileName  string `json:"file_name,omitempty"`
	AssetType string `json:"asset_type"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

func CaptureRelayRequestContent(c *gin.Context, relayFormat types.RelayFormat, info *relaycommon.RelayInfo) error {
	if !RequestContentAuditEnabled() {
		return nil
	}
	// Realtime 的用户内容在 WebSocket 帧中，不属于这个 HTTP 提交体边界；避免落一条空握手记录。
	if relayFormat == types.RelayFormatOpenAIRealtime {
		return nil
	}
	if c == nil || info == nil {
		return fmt.Errorf("request content audit capture context is incomplete")
	}
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		requestID = info.RequestId
	}
	if requestID == "" {
		return fmt.Errorf("request content audit request id is empty")
	}

	settings := RequestContentAuditSettings()
	store, err := EnsureRequestContentAuditStore(c.Request.Context())
	if err != nil {
		return err
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	reader, err := storage.NewReader()
	if err != nil {
		return err
	}
	defer reader.Close()
	readLimit := settings.MaxRecordBytes
	if readLimit > requestContentCaptureMemoryLimit {
		readLimit = requestContentCaptureMemoryLimit
	}
	if storage.Size() > readLimit {
		return auditstore.ErrSizeLimitExceeded
	}
	body, err := io.ReadAll(io.LimitReader(reader, readLimit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > readLimit {
		return auditstore.ErrSizeLimitExceeded
	}

	contentType := c.Request.Header.Get("Content-Type")
	envelope, assets, err := buildRequestContentEnvelope(c, requestID, relayFormat, contentType, body, settings.MaxAssetBytes)
	if err != nil {
		return err
	}

	createdAt := time.Now()
	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelMeta.ChannelId
	}
	_, err = store.StoreJSON(c.Request.Context(), auditstore.RecordInput{
		Audit: model.RequestContentAudit{
			RequestId:            requestID,
			UserId:               info.UserId,
			Username:             c.GetString("username"),
			TokenId:              info.TokenId,
			ChannelId:            channelID,
			ModelName:            info.OriginModelName,
			RequestType:          string(relayFormat),
			RelayFormat:          string(relayFormat),
			EndpointPath:         c.Request.URL.Path,
			ContentType:          contentType,
			CreatedAt:            createdAt.Unix(),
			ExpiresAt:            createdAt.AddDate(0, 0, settings.RetentionDays).Unix(),
			MessageCount:         requestContentMessageCount(envelope.Payload),
			NormalizationVersion: requestContentEnvelopeVersion,
			CaptureStatus:        model.RequestContentCaptureStatusComplete,
		},
		Assets: assets,
	}, envelope)
	return err
}

func RecordRequestContentAuditFailure(c *gin.Context, relayFormat types.RelayFormat, info *relaycommon.RelayInfo, captureErr error) error {
	if !RequestContentAuditEnabled() {
		return nil
	}
	if c == nil || info == nil {
		return fmt.Errorf("request content audit failure context is incomplete")
	}
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		requestID = info.RequestId
	}
	if requestID == "" {
		return fmt.Errorf("request content audit failure request id is empty")
	}
	store, err := EnsureRequestContentAuditStore(c.Request.Context())
	if err != nil {
		return err
	}
	settings := RequestContentAuditSettings()
	now := time.Now()
	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelMeta.ChannelId
	}
	failurePayload := map[string]any{
		"schema_version": requestContentEnvelopeVersion,
		"kind":           "capture_failure",
		"request_id":     requestID,
		"error_code":     RequestContentAuditErrorCode(captureErr),
	}
	_, err = store.StoreJSON(c.Request.Context(), auditstore.RecordInput{
		Audit: model.RequestContentAudit{
			RequestId:            requestID,
			UserId:               info.UserId,
			Username:             c.GetString("username"),
			TokenId:              info.TokenId,
			ChannelId:            channelID,
			ModelName:            info.OriginModelName,
			RequestType:          string(relayFormat),
			RelayFormat:          string(relayFormat),
			EndpointPath:         c.Request.URL.Path,
			ContentType:          c.Request.Header.Get("Content-Type"),
			CreatedAt:            now.Unix(),
			ExpiresAt:            now.AddDate(0, 0, settings.RetentionDays).Unix(),
			CaptureStatus:        model.RequestContentCaptureStatusFailed,
			CaptureErrorCode:     RequestContentAuditErrorCode(captureErr),
			NormalizationVersion: requestContentEnvelopeVersion,
		},
	}, failurePayload)
	return err
}

func RequestContentAuditErrorCode(err error) string {
	if errors.Is(err, auditstore.ErrSizeLimitExceeded) {
		return "size_limit_exceeded"
	}
	if err == nil {
		return "capture_failed"
	}
	return "capture_failed"
}

func buildRequestContentEnvelope(c *gin.Context, requestID string, relayFormat types.RelayFormat, contentType string, body []byte, maxAssetBytes int64) (RequestContentEnvelope, []auditstore.AssetInput, error) {
	envelope := RequestContentEnvelope{
		SchemaVersion: requestContentEnvelopeVersion,
		Kind:          "json",
		RequestID:     requestID,
		RelayFormat:   string(relayFormat),
		ContentType:   contentType,
		EndpointPath:  c.Request.URL.Path,
	}
	if len(body) > 0 {
		digest := sha256.Sum256(body)
		envelope.Original = &RequestContentOriginal{
			Sha256: hex.EncodeToString(digest[:]),
			Size:   int64(len(body)),
		}
	}

	if strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		return buildMultipartEnvelope(c, envelope, maxAssetBytes)
	}
	if strings.HasPrefix(strings.ToLower(contentType), "application/json") || strings.Contains(strings.ToLower(contentType), "+json") {
		var payload any
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err == nil {
			assets := make([]auditstore.AssetInput, 0)
			refs := make([]RequestContentAsset, 0)
			normalized, normalizeErr := normalizeRequestPayload(payload, &assets, &refs, maxAssetBytes)
			if normalizeErr != nil {
				return RequestContentEnvelope{}, nil, normalizeErr
			}
			pruned, pruneSummary := pruneRequestContentPayload(normalized)
			envelope.Payload = pruned
			envelope.Prune = &pruneSummary
			envelope.Assets = refs
			return envelope, assets, nil
		}
	}

	asset, ref, err := newRequestContentAsset(0, "request-body", "asset-0", contentType, body, maxAssetBytes)
	if err != nil {
		return RequestContentEnvelope{}, nil, err
	}
	envelope.Kind = "raw"
	envelope.Payload = map[string]any{
		"asset_key": ref.AssetKey,
		"mime_type": ref.MimeType,
		"size":      ref.Size,
	}
	envelope.Assets = []RequestContentAsset{ref}
	return envelope, []auditstore.AssetInput{asset}, nil
}

func buildMultipartEnvelope(c *gin.Context, envelope RequestContentEnvelope, maxAssetBytes int64) (RequestContentEnvelope, []auditstore.AssetInput, error) {
	form := c.Request.MultipartForm
	if form == nil {
		defer func() {
			if c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			return RequestContentEnvelope{}, nil, err
		}
		form = c.Request.MultipartForm
	}
	if form == nil {
		return RequestContentEnvelope{}, nil, fmt.Errorf("multipart form is unavailable")
	}

	assets := make([]auditstore.AssetInput, 0)
	refs := make([]RequestContentAsset, 0)
	fields := make(map[string][]any, len(form.Value))
	for key, values := range form.Value {
		for _, value := range values {
			fieldValue := any(value)
			trimmed := strings.TrimSpace(value)
			if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
				decoder := json.NewDecoder(strings.NewReader(trimmed))
				decoder.UseNumber()
				var parsed any
				if err := decoder.Decode(&parsed); err == nil {
					fieldValue = parsed
				}
			}
			normalized, err := normalizeRequestContentField(fieldValue, &assets, &refs, maxAssetBytes, key)
			if err != nil {
				return RequestContentEnvelope{}, nil, err
			}
			fields[key] = append(fields[key], normalized)
		}
	}
	fileKeys := make([]string, 0, len(form.File))
	for key := range form.File {
		fileKeys = append(fileKeys, key)
	}
	sort.Strings(fileKeys)

	files := make([]map[string]any, 0)
	for _, fieldName := range fileKeys {
		fileHeaders := form.File[fieldName]
		for _, header := range fileHeaders {
			if header == nil {
				continue
			}
			assetReadLimit := maxAssetBytes
			if assetReadLimit > requestContentCaptureMemoryLimit {
				assetReadLimit = requestContentCaptureMemoryLimit
			}
			if header.Size > assetReadLimit {
				return RequestContentEnvelope{}, nil, auditstore.ErrSizeLimitExceeded
			}
			file, err := header.Open()
			if err != nil {
				return RequestContentEnvelope{}, nil, err
			}
			data, readErr := io.ReadAll(io.LimitReader(file, assetReadLimit+1))
			closeErr := file.Close()
			if readErr != nil {
				return RequestContentEnvelope{}, nil, readErr
			}
			if closeErr != nil {
				return RequestContentEnvelope{}, nil, closeErr
			}
			if int64(len(data)) > assetReadLimit {
				return RequestContentEnvelope{}, nil, auditstore.ErrSizeLimitExceeded
			}
			contentType := header.Header.Get("Content-Type")
			if contentType == "" {
				contentType = http.DetectContentType(data)
			}
			assetKey := fmt.Sprintf("asset-%d", len(assets))
			asset, ref, err := newRequestContentAsset(len(assets), "file", assetKey, contentType, data, maxAssetBytes)
			if err != nil {
				return RequestContentEnvelope{}, nil, err
			}
			asset.FileName = header.Filename
			ref.FileName = header.Filename
			assets = append(assets, asset)
			refs = append(refs, ref)
			files = append(files, map[string]any{
				"field":     fieldName,
				"file_name": header.Filename,
				"asset_key": assetKey,
				"mime_type": ref.MimeType,
				"size":      ref.Size,
			})
		}
	}
	envelope.Kind = "multipart"
	prunedFields, pruneSummary := pruneRequestContentPayload(map[string]any{
		"fields": fields,
		"files":  files,
	})
	envelope.Payload = prunedFields
	envelope.Prune = &pruneSummary
	envelope.Assets = refs
	return envelope, assets, nil
}

func normalizeRequestContentField(value any, assets *[]auditstore.AssetInput, refs *[]RequestContentAsset, maxAssetBytes int64, fieldName string) (any, error) {
	return normalizeRequestPayloadAt(value, assets, refs, maxAssetBytes, fieldName, "multipart", "")
}

func normalizeRequestPayload(value any, assets *[]auditstore.AssetInput, refs *[]RequestContentAsset, maxAssetBytes int64) (any, error) {
	return normalizeRequestPayloadAt(value, assets, refs, maxAssetBytes, "", "", "")
}

func normalizeRequestPayloadAt(value any, assets *[]auditstore.AssetInput, refs *[]RequestContentAsset, maxAssetBytes int64, fieldName string, parentName string, mimeHint string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		mapMimeHint := firstStringField(typed, "mime_type", "mimeType", "content_type", "contentType", "media_type")
		if mapMimeHint == "" {
			mapMimeHint = mimeHint
		}
		for key, child := range typed {
			normalized, err := normalizeRequestPayloadAt(child, assets, refs, maxAssetBytes, key, fieldName, mapMimeHint)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			normalized, err := normalizeRequestPayloadAt(child, assets, refs, maxAssetBytes, fieldName, parentName, mimeHint)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(strings.ToLower(trimmed), "data:") {
			mimeType, reader, err := auditstore.DecodeDataURI(trimmed)
			if err != nil {
				return map[string]any{"asset_error": "invalid_data_uri"}, nil
			}
			assetReadLimit := maxAssetBytes
			if assetReadLimit > requestContentCaptureMemoryLimit {
				assetReadLimit = requestContentCaptureMemoryLimit
			}
			data, err := io.ReadAll(io.LimitReader(reader, assetReadLimit+1))
			if err != nil {
				return nil, err
			}
			if int64(len(data)) > assetReadLimit {
				return nil, auditstore.ErrSizeLimitExceeded
			}
			return appendRequestContentAsset(assets, refs, mimeType, data, maxAssetBytes)
		}
		if !isBase64PayloadField(fieldName, parentName) {
			return typed, nil
		}
		data, normalizedMime, err := decodeBase64Payload(trimmed, mimeHint, maxAssetBytes)
		if err != nil {
			if errors.Is(err, auditstore.ErrSizeLimitExceeded) {
				return nil, err
			}
			return map[string]any{"asset_error": "invalid_base64"}, nil
		}
		return appendRequestContentAsset(assets, refs, normalizedMime, data, maxAssetBytes)
	default:
		return value, nil
	}
}

func appendRequestContentAsset(assets *[]auditstore.AssetInput, refs *[]RequestContentAsset, mimeType string, data []byte, maxAssetBytes int64) (any, error) {
	assetKey := fmt.Sprintf("asset-%d", len(*assets))
	asset, ref, err := newRequestContentAsset(len(*assets), "file", assetKey, mimeType, data, maxAssetBytes)
	if err != nil {
		return nil, err
	}
	*assets = append(*assets, asset)
	*refs = append(*refs, ref)
	return map[string]any{
		"asset_key":  ref.AssetKey,
		"asset_type": ref.AssetType,
		"mime_type":  ref.MimeType,
		"size":       ref.Size,
		"width":      ref.Width,
		"height":     ref.Height,
	}, nil
}

func isBase64PayloadField(fieldName string, parentName string) bool {
	fieldName = strings.ToLower(strings.TrimSpace(fieldName))
	parentName = strings.ToLower(strings.TrimSpace(parentName))
	switch fieldName {
	case "b64_json", "b64json", "base64", "base64_data", "encoded_data", "data_base64":
		return true
	case "data":
		switch parentName {
		case "inline_data", "inlinedata", "source", "image_data", "audio_data", "file_data", "document_data", "video_data":
			return true
		}
	}
	return false
}

func firstStringField(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := value[key]; ok {
			if text, ok := raw.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func decodeBase64Payload(value string, mimeHint string, maxAssetBytes int64) ([]byte, string, error) {
	if value == "" {
		return nil, "", errors.New("empty base64 payload")
	}
	compact := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
	if maxAssetBytes > 0 && int64(base64.StdEncoding.DecodedLen(len(compact))) > maxAssetBytes+3 {
		return nil, "", auditstore.ErrSizeLimitExceeded
	}
	var data []byte
	var err error
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		data, err = encoding.DecodeString(compact)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxAssetBytes {
		return nil, "", auditstore.ErrSizeLimitExceeded
	}
	return data, normalizeContentType(mimeHint, data), nil
}

func newRequestContentAsset(position int, assetType string, assetKey string, mimeType string, data []byte, maxAssetBytes int64) (auditstore.AssetInput, RequestContentAsset, error) {
	if int64(len(data)) > maxAssetBytes {
		return auditstore.AssetInput{}, RequestContentAsset{}, auditstore.ErrSizeLimitExceeded
	}
	mimeType = normalizeContentType(mimeType, data)
	if assetType == "file" && strings.HasPrefix(mimeType, "image/") {
		assetType = "image"
	}
	width, height := imageDimensions(data, mimeType)
	asset := auditstore.AssetInput{
		Position:  position,
		AssetKey:  assetKey,
		AssetType: assetType,
		MimeType:  mimeType,
		Width:     width,
		Height:    height,
		Original:  bytes.NewReader(data),
	}
	if thumbnail := buildRequestThumbnail(data, mimeType); thumbnail != nil {
		asset.Thumbnail = thumbnail
	}
	return asset, RequestContentAsset{
		AssetKey:  assetKey,
		AssetType: assetType,
		MimeType:  mimeType,
		Size:      int64(len(data)),
		Width:     width,
		Height:    height,
	}, nil
}

func normalizeContentType(contentType string, data []byte) string {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil && mediaType != "" {
		return strings.ToLower(mediaType)
	}
	if contentType != "" && !strings.Contains(contentType, ";") {
		return strings.ToLower(contentType)
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
}

func imageDimensions(data []byte, mimeType string) (int, int) {
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return 0, 0
	}
	if config, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		return config.Width, config.Height
	}
	if config, err := webp.DecodeConfig(bytes.NewReader(data)); err == nil {
		return config.Width, config.Height
	}
	return 0, 0
}

func buildRequestThumbnail(data []byte, mimeType string) *auditstore.AssetVariantInput {
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return nil
	}
	configuredWidth, configuredHeight := imageDimensions(data, mimeType)
	if configuredWidth <= 0 || configuredHeight <= 0 || int64(configuredWidth)*int64(configuredHeight) > requestContentThumbnailMaxPixels {
		return nil
	}
	decoded, err := decodeRequestImage(data, mimeType)
	if err != nil {
		return nil
	}
	bounds := decoded.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil
	}
	width, height := bounds.Dx(), bounds.Dy()
	if width > requestContentThumbnailLimit || height > requestContentThumbnailLimit {
		scale := float64(requestContentThumbnailLimit) / float64(width)
		if height > width {
			scale = float64(requestContentThumbnailLimit) / float64(height)
		}
		width = max(1, int(float64(width)*scale))
		height = max(1, int(float64(height)*scale))
	}
	destination := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(destination, destination.Bounds(), decoded, decoded.Bounds(), draw.Over, nil)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, destination); err != nil {
		return nil
	}
	return &auditstore.AssetVariantInput{
		MimeType: "image/png",
		Width:    width,
		Height:   height,
		Content:  bytes.NewReader(encoded.Bytes()),
	}
}

func decodeRequestImage(data []byte, mimeType string) (image.Image, error) {
	if strings.EqualFold(mimeType, "image/webp") {
		return webp.Decode(bytes.NewReader(data))
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return decoded, nil
	}
	return webp.Decode(bytes.NewReader(data))
}

func requestContentMessageCount(payload any) int {
	if object, ok := payload.(map[string]any); ok {
		for _, key := range []string{"messages", "input", "contents", "items"} {
			if list, ok := object[key].([]any); ok && len(list) > 0 {
				return len(list)
			}
		}
	}
	return 1
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
