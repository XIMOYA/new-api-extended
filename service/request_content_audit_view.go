// service/request_content_audit_view.go
// 请求内容结构化投影：把审计正文按消息、工具调用、工具结果和高级参数拆成受限区块，正文文件仍保持完整加密留存。
package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	auditstore "github.com/QuantumNous/new-api/service/request_content_audit"
)

const (
	requestContentViewSourceLimit           = 32 << 20
	requestContentViewPreviewBytes          = 1 << 10
	requestContentViewSectionBytes          = 512 << 10
	requestContentViewMaxSections           = 256
	requestContentViewHeadSections          = 32
	requestContentViewMaxDepth              = 32
	requestContentViewMaxNodes              = 4096
	requestContentViewMaxFields             = 512
	requestContentViewOpaqueMarker          = "[encrypted content omitted]"
	requestContentViewSourceTooLargeMessage = "source_too_large"
	requestContentViewUnavailableMessage    = "content_unavailable"
)

var (
	ErrRequestContentAuditViewSourceTooLarge = errors.New("request content audit view source is too large")
	ErrRequestContentAuditViewSectionMissing = errors.New("request content audit view section not found")
)

type RequestContentAuditView struct {
	RequestID           string                            `json:"request_id"`
	SchemaVersion       int                               `json:"schema_version,omitempty"`
	RelayFormat         string                            `json:"relay_format"`
	EndpointPath        string                            `json:"endpoint_path,omitempty"`
	Kind                string                            `json:"kind"`
	SourceSize          int64                             `json:"source_size"`
	StoredSize          int64                             `json:"stored_size"`
	ProjectionAvailable bool                              `json:"projection_available"`
	ProjectionMessage   string                            `json:"projection_message,omitempty"`
	Retention           *RequestContentAuditViewRetention `json:"retention,omitempty"`
	Summary             RequestContentAuditViewSummary    `json:"summary"`
	Sections            []RequestContentAuditViewSection  `json:"sections"`
}

// RequestContentAuditViewRetention 描述这条记录按保留策略丢弃了哪些内容。
type RequestContentAuditViewRetention struct {
	Schema         int                           `json:"schema"`
	KeptBytes      int64                         `json:"kept_bytes"`
	DroppedBytes   int64                         `json:"dropped_bytes"`
	OriginalSha256 string                        `json:"original_sha256,omitempty"`
	OriginalSize   int64                         `json:"original_size,omitempty"`
	Dropped        []RequestContentPruneCategory `json:"dropped,omitempty"`
}

type RequestContentAuditViewSummary struct {
	InputItemCount      int   `json:"input_item_count"`
	MessageCount        int   `json:"message_count"`
	SectionsTruncated   bool  `json:"sections_truncated"`
	ToolCallCount       int   `json:"tool_call_count"`
	ToolOutputCount     int   `json:"tool_output_count"`
	ReasoningCount      int   `json:"reasoning_count"`
	AdvancedFieldCount  int   `json:"advanced_field_count"`
	OpaqueBytes         int64 `json:"opaque_bytes"`
	DroppedBytes        int64 `json:"dropped_bytes"`
	OmittedSectionCount int   `json:"omitted_section_count"`
	AssetCount          int   `json:"asset_count"`
}

type RequestContentAuditViewSection struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Type          string   `json:"type,omitempty"`
	Role          string   `json:"role,omitempty"`
	CallID        string   `json:"call_id,omitempty"`
	OutputType    string   `json:"output_type,omitempty"`
	Title         string   `json:"title"`
	Preview       string   `json:"preview,omitempty"`
	Content       string   `json:"content,omitempty"`
	ContentFormat string   `json:"content_format,omitempty"`
	ContentSize   int64    `json:"content_size"`
	OpaqueBytes   int64    `json:"opaque_bytes,omitempty"`
	OpaqueHash    string   `json:"opaque_hash,omitempty"`
	Opaque        bool     `json:"opaque"`
	Dropped       bool     `json:"dropped,omitempty"`
	DroppedKind   string   `json:"dropped_kind,omitempty"`
	DroppedBytes  int64    `json:"dropped_bytes,omitempty"`
	Expandable    bool     `json:"expandable"`
	Truncated     bool     `json:"truncated"`
	AssetKeys     []string `json:"asset_keys,omitempty"`
}

func BuildRequestContentAuditView(ctx context.Context, store *auditstore.Store, audit *model.RequestContentAudit, redacted bool) (RequestContentAuditView, error) {
	view := RequestContentAuditView{
		RequestID:           auditRequestID(audit),
		SchemaVersion:       0,
		RelayFormat:         auditRelayFormat(audit),
		EndpointPath:        auditEndpointPath(audit),
		SourceSize:          auditContentSize(audit),
		StoredSize:          auditStoredSize(audit),
		ProjectionAvailable: false,
		Sections:            []RequestContentAuditViewSection{},
	}
	if audit == nil || !audit.ContentAvailable() {
		view.ProjectionMessage = requestContentViewUnavailableMessage
		return view, nil
	}

	envelope, err := readRequestContentAuditEnvelope(ctx, store, audit, redacted)
	if err != nil {
		if errors.Is(err, ErrRequestContentAuditViewSourceTooLarge) {
			view.ProjectionMessage = requestContentViewSourceTooLargeMessage
			return view, nil
		}
		return view, err
	}
	view.SchemaVersion = intValue(envelope["schema_version"])
	view.Kind = stringValue(envelope["kind"])
	view.RelayFormat = firstNonEmpty(stringValue(envelope["relay_format"]), view.RelayFormat)
	view.EndpointPath = firstNonEmpty(stringValue(envelope["endpoint_path"]), view.EndpointPath)
	view.Summary.AssetCount = arrayLength(envelope["assets"])
	if audit.AssetCount > view.Summary.AssetCount {
		view.Summary.AssetCount = audit.AssetCount
	}

	payload := envelope["payload"]
	sections, summary := buildRequestContentViewSections(payload)
	summary.AssetCount = view.Summary.AssetCount
	view.Retention = requestContentRetentionFromEnvelope(envelope)
	view.Summary = summary
	view.Sections = sections
	view.ProjectionAvailable = true
	return view, nil
}

func GetRequestContentAuditViewSection(ctx context.Context, store *auditstore.Store, audit *model.RequestContentAudit, sectionID string, redacted bool) (RequestContentAuditViewSection, error) {
	if audit == nil || !audit.ContentAvailable() {
		return RequestContentAuditViewSection{}, ErrRequestContentAuditViewSectionMissing
	}
	envelope, err := readRequestContentAuditEnvelope(ctx, store, audit, redacted)
	if err != nil {
		return RequestContentAuditViewSection{}, err
	}
	value, ok := requestContentAuditSectionValue(envelope["payload"], sectionID)
	if !ok {
		return RequestContentAuditViewSection{}, ErrRequestContentAuditViewSectionMissing
	}
	section := buildRequestContentViewSection(sectionID, value)
	if section.Kind == "reasoning" || section.Opaque {
		section.Content = requestContentOpaquePreview(value)
		section.ContentFormat = "text"
		section.Truncated = false
		return section, nil
	}
	section.Content, section.ContentFormat, section.Truncated = renderRequestContentSection(value, requestContentViewSectionBytes)
	return section, nil
}

func readRequestContentAuditEnvelope(ctx context.Context, store *auditstore.Store, audit *model.RequestContentAudit, redacted bool) (map[string]any, error) {
	if store == nil || audit == nil {
		return nil, errors.New("request content audit view storage is unavailable")
	}
	if audit.ContentSize > requestContentViewSourceLimit {
		return nil, ErrRequestContentAuditViewSourceTooLarge
	}
	reader, err := store.OpenValidatedAuditContent(ctx, audit)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, requestContentViewSourceLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > requestContentViewSourceLimit {
		return nil, ErrRequestContentAuditViewSourceTooLarge
	}
	var envelope map[string]any
	if err := common.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if redacted {
		redactedValue := auditstore.RedactValueLimited(
			envelope,
			requestContentViewMaxDepth,
			requestContentViewMaxNodes,
		)
		var ok bool
		envelope, ok = redactedValue.(map[string]any)
		if !ok {
			return nil, errors.New("request content audit envelope redaction failed")
		}
	}
	return envelope, nil
}

func buildRequestContentViewSections(payload any) ([]RequestContentAuditViewSection, RequestContentAuditViewSummary) {
	sections := make([]RequestContentAuditViewSection, 0)
	summary := RequestContentAuditViewSummary{}
	if payloadMap, ok := payload.(map[string]any); ok {
		sequenceKeys := make([]string, 0, len(requestContentViewSequenceKeys))
		for _, key := range requestContentViewSequenceKeys {
			if _, ok := payloadMap[key].([]any); ok {
				sequenceKeys = append(sequenceKeys, key)
			}
		}
		for _, key := range sequenceKeys {
			items := payloadMap[key].([]any)
			summary.InputItemCount += len(items)
			budget := requestContentViewMaxSections - len(sections)
			if budget <= 0 {
				summary.SectionsTruncated = true
				summary.OmittedSectionCount += len(items)
				continue
			}
			// agent 客户端每轮重发整段对话，最新一轮排在末尾，所以超额时保留开头的上下文
			// 和结尾的最近若干条，中间跳过并记录数量，否则站长在 UI 里根本看不到刚发的消息。
			head, tailStart := selectRequestContentViewRange(len(items), budget)
			if tailStart > head {
				summary.SectionsTruncated = true
				summary.OmittedSectionCount += tailStart - head
			}
			for index, item := range items {
				if index >= head && index < tailStart {
					continue
				}
				sectionID := sequenceSectionID(key, index)
				if key == "input" {
					sectionID = fmt.Sprintf("input-%d", index)
				}
				section := buildRequestContentViewSection(sectionID, item)
				sections = append(sections, section)
				updateRequestContentViewSummary(&summary, section)
			}
		}

		keys := make([]string, 0, len(payloadMap))
		for key := range payloadMap {
			if isRequestContentViewSequenceKey(key) {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > requestContentViewMaxSections-len(sections) {
			summary.SectionsTruncated = true
		}
		for _, key := range keys {
			if len(sections) >= requestContentViewMaxSections {
				break
			}
			section := buildRequestContentViewSection(fieldSectionID(key), payloadMap[key])
			sections = append(sections, section)
			summary.AdvancedFieldCount++
			summary.OpaqueBytes += section.OpaqueBytes
		}
		return sections, summary
	}

	section := buildRequestContentViewSection("payload", payload)
	return []RequestContentAuditViewSection{section}, RequestContentAuditViewSummary{AdvancedFieldCount: 1, OpaqueBytes: section.OpaqueBytes}
}

var requestContentViewSequenceKeys = []string{"input", "messages", "items", "contents"}

// selectRequestContentViewRange 决定超额序列保留哪些下标：前 head 条给上下文，
// 后面从 tailStart 开始的都是最近的，中间跳过。总数没超额时返回 (total, total)。
func selectRequestContentViewRange(total int, budget int) (int, int) {
	if budget <= 0 {
		return 0, total
	}
	if total <= budget {
		return total, total
	}
	head := requestContentViewHeadSections
	if head > budget/2 {
		head = budget / 2
	}
	if head < 0 {
		head = 0
	}
	tail := budget - head
	tailStart := total - tail
	if tailStart < head {
		tailStart = head
	}
	return head, tailStart
}

func isRequestContentViewSequenceKey(key string) bool {
	for _, candidate := range requestContentViewSequenceKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

func buildRequestContentViewSection(id string, value any) RequestContentAuditViewSection {
	section := RequestContentAuditViewSection{
		ID:          id,
		Kind:        "field",
		Title:       "Other parameter",
		ContentSize: jsonValueSize(value),
		Expandable:  true,
		AssetKeys:   assetKeys(value),
	}
	if isRequestContentViewItemID(id) {
		section.Kind = "item"
		section.Title = "Input item"
		if item, ok := value.(map[string]any); ok {
			section.Type = stringValue(item["type"])
			section.Role = stringValue(item["role"])
			section.CallID = stringValue(item["call_id"])
			section.OutputType = firstNonEmpty(stringValue(item["output_type"]), jsonValueKind(item["output"]))
			switch {
			case section.Type == "message" || section.Role != "" || item["content"] != nil:
				section.Kind = "message"
				section.Title = firstNonEmpty(section.Role, "Message")
				section.Preview = messagePreview(item)
			case section.Type == "function_call":
				section.Kind = "tool_call"
				section.Title = firstNonEmpty(stringValue(item["name"]), "Function call")
				section.Preview = previewValue(item["arguments"])
			case section.Type == "function_call_output":
				section.Kind = "tool_output"
				section.Title = "Function result"
				section.Preview = previewValue(item["output"])
			case section.Type == "reasoning":
				section.Kind = "reasoning"
				section.Title = "Encrypted reasoning"
				section.Preview = reasoningPreview(item)
			default:
				section.Title = firstNonEmpty(section.Type, "Input item")
				section.Preview = previewValue(item)
			}
		} else {
			section.Preview = previewValue(value)
		}
	} else if strings.HasPrefix(id, "field-") {
		if key, ok := decodeFieldSectionID(id); ok {
			section.Title = key
			section.Kind = requestContentFieldKind(key)
		}
		section.Preview = previewValue(value)
	} else {
		section.Preview = previewValue(value)
	}
	section.OpaqueBytes = opaqueBytes(value, 0)
	section.Opaque = section.OpaqueBytes > 0
	if section.OpaqueBytes > 0 {
		section.OpaqueHash = opaqueHash(value)
	}
	if droppedBytes, droppedKind := requestContentDroppedStats(value, 0); droppedBytes > 0 {
		section.Dropped = true
		section.DroppedKind = droppedKind
		section.DroppedBytes = droppedBytes
	}
	if section.Kind == "reasoning" {
		section.Opaque = true
		section.Expandable = false
	}
	if section.Preview == "" {
		section.Preview = previewValue(value)
	}
	return section
}

func updateRequestContentViewSummary(summary *RequestContentAuditViewSummary, section RequestContentAuditViewSection) {
	summary.OpaqueBytes += section.OpaqueBytes
	summary.DroppedBytes += section.DroppedBytes
	switch section.Kind {
	case "message":
		summary.MessageCount++
	case "tool_call":
		summary.ToolCallCount++
	case "tool_output":
		summary.ToolOutputCount++
	case "reasoning":
		summary.ReasoningCount++
	}
}

func requestContentAuditSectionValue(payload any, sectionID string) (any, bool) {
	if sectionID == "payload" {
		return payload, true
	}
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	if strings.HasPrefix(sectionID, "input-") {
		index, err := strconv.Atoi(strings.TrimPrefix(sectionID, "input-"))
		// 下标上界由序列长度本身把关：投影只返回头尾区块，但尾部区块的真实下标会超过
		// 区块数上限，这里再按上限拦就会让最新那几条消息取不到详情。
		if err != nil || index < 0 {
			return nil, false
		}
		input, ok := payloadMap["input"].([]any)
		if !ok || index >= len(input) {
			return nil, false
		}
		return input[index], true
	}
	if key, index, ok := decodeSequenceSectionID(sectionID); ok {
		items, ok := payloadMap[key].([]any)
		if !ok || index >= len(items) {
			return nil, false
		}
		return items[index], true
	}
	if key, ok := decodeFieldSectionID(sectionID); ok {
		value, exists := payloadMap[key]
		return value, exists
	}
	return nil, false
}

func renderRequestContentSection(value any, limit int) (string, string, bool) {
	if text, ok := value.(string); ok {
		truncated := len(text) > limit
		return truncateUTF8(text, limit), "text", truncated
	}
	data, err := marshalStructuredJSON(sanitizeStructuredValue(value, true, 0), true)
	if err != nil {
		return "[unavailable]", "text", false
	}
	truncated := len(data) > limit
	return truncateUTF8(string(data), limit), "json", truncated
}

func previewValue(value any) string {
	if text, ok := value.(string); ok {
		return truncateUTF8(text, requestContentViewPreviewBytes)
	}
	data, err := marshalStructuredJSON(sanitizeStructuredValue(value, true, 0), true)
	if err != nil {
		return "[unavailable]"
	}
	return truncateUTF8(string(data), requestContentViewPreviewBytes)
}

func messagePreview(item map[string]any) string {
	text := collectText(item["content"], 0)
	if text != "" {
		return truncateUTF8(text, requestContentViewPreviewBytes)
	}
	return previewValue(item)
}

func reasoningPreview(item map[string]any) string {
	if summary, ok := item["summary"]; ok {
		text := collectText(summary, 0)
		if text != "" {
			return truncateUTF8(text, requestContentViewPreviewBytes)
		}
	}
	return "Encrypted reasoning content is hidden"
}

func collectText(value any, depth int) string {
	return collectTextWithBudget(value, depth, newViewBudget())
}

func collectTextWithBudget(value any, depth int, budget *viewBudget) string {
	if depth > requestContentViewMaxDepth || !budget.takeNode() {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, child := range typed {
			if text := collectTextWithBudget(child, depth+1, budget); text != "" {
				parts = append(parts, text)
			}
			if budget.nodes <= 0 {
				break
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content", "output", "summary"} {
			if !budget.takeField() {
				break
			}
			if child, ok := typed[key]; ok {
				if text := collectTextWithBudget(child, depth+1, budget); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

type viewBudget struct {
	nodes  int
	fields int
}

func newViewBudget() *viewBudget {
	return &viewBudget{
		nodes:  requestContentViewMaxNodes,
		fields: requestContentViewMaxFields,
	}
}

func (budget *viewBudget) takeNode() bool {
	if budget.nodes <= 0 {
		return false
	}
	budget.nodes--
	return true
}

func (budget *viewBudget) takeField() bool {
	if budget.fields <= 0 {
		return false
	}
	budget.fields--
	return true
}

func sanitizeStructuredValue(value any, hideOpaque bool, depth int) any {
	return sanitizeStructuredValueWithBudget(value, hideOpaque, depth, newViewBudget())
}

func sanitizeStructuredValueWithBudget(value any, hideOpaque bool, depth int, budget *viewBudget) any {
	if depth > requestContentViewMaxDepth || !budget.takeNode() {
		return "[nested content omitted]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, child := range typed {
			if !budget.takeField() {
				result["_omitted_fields"] = "[field limit reached]"
				break
			}
			if hideOpaque && isOpaqueValue(key, child) {
				result[key] = requestContentViewOpaqueMarker
				continue
			}
			result[key] = sanitizeStructuredValueWithBudget(child, hideOpaque, depth+1, budget)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			if budget.nodes <= 0 {
				result = append(result, "[item limit reached]")
				break
			}
			result = append(result, sanitizeStructuredValueWithBudget(child, hideOpaque, depth+1, budget))
		}
		return result
	default:
		return value
	}
}

func opaqueBytes(value any, depth int) int64 {
	return opaqueBytesWithBudget(value, depth, newViewBudget())
}

func opaqueBytesWithBudget(value any, depth int, budget *viewBudget) int64 {
	if depth > requestContentViewMaxDepth || !budget.takeNode() {
		return 0
	}
	switch typed := value.(type) {
	case map[string]any:
		var total int64
		for key, child := range typed {
			if !budget.takeField() {
				break
			}
			if isOpaqueValue(key, child) {
				total += jsonValueSize(child)
				continue
			}
			total += opaqueBytesWithBudget(child, depth+1, budget)
		}
		return total
	case []any:
		var total int64
		for _, child := range typed {
			total += opaqueBytesWithBudget(child, depth+1, budget)
			if budget.nodes <= 0 {
				break
			}
		}
		return total
	case string:
		if isHighEntropyString(typed) {
			return int64(len(typed))
		}
	}
	return 0
}

func assetKeys(value any) []string {
	seen := make(map[string]struct{})
	collectAssetKeys(value, seen, 0, newViewBudget())
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func collectAssetKeys(value any, seen map[string]struct{}, depth int, budget *viewBudget) {
	if depth > requestContentViewMaxDepth || !budget.takeNode() {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		if key, ok := typed["asset_key"].(string); ok && key != "" {
			seen[key] = struct{}{}
		}
		for key, child := range typed {
			if !budget.takeField() {
				return
			}
			if key == "asset_key" {
				continue
			}
			collectAssetKeys(child, seen, depth+1, budget)
		}
	case []any:
		for _, child := range typed {
			collectAssetKeys(child, seen, depth+1, budget)
			if budget.nodes <= 0 {
				return
			}
		}
	}
}

func requestContentOpaquePreview(value any) string {
	size := opaqueBytes(value, 0)
	if size <= 0 {
		size = jsonValueSize(value)
	}
	return fmt.Sprintf("Encrypted content omitted (%s)", formatBytes(size))
}

func marshalStructuredJSON(value any, indent bool) ([]byte, error) {
	data, err := common.Marshal(value)
	if err != nil || !indent {
		return data, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return nil, err
	}
	return pretty.Bytes(), nil
}

func jsonValueSize(value any) int64 {
	return boundedJSONValueSize(value, 0, newViewBudget())
}

func boundedJSONValueSize(value any, depth int, budget *viewBudget) int64 {
	if depth > requestContentViewMaxDepth || !budget.takeNode() {
		return 0
	}
	switch typed := value.(type) {
	case string:
		return int64(len(typed) + 2)
	case []any:
		var total int64
		for _, child := range typed {
			total += boundedJSONValueSize(child, depth+1, budget)
			if budget.nodes <= 0 {
				break
			}
		}
		return total
	case map[string]any:
		var total int64
		for key, child := range typed {
			if !budget.takeField() {
				break
			}
			total += int64(len(key) + 4)
			total += boundedJSONValueSize(child, depth+1, budget)
			if budget.nodes <= 0 {
				break
			}
		}
		return total
	default:
		data, err := common.Marshal(value)
		if err != nil {
			return 0
		}
		return int64(len(data))
	}
}

func truncateUTF8(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	value = value[:limit]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + "…"
}

func sequenceSectionID(key string, index int) string {
	return "list-" + base64.RawURLEncoding.EncodeToString([]byte(key)) + "-" + strconv.Itoa(index)
}

func isRequestContentViewItemID(id string) bool {
	return strings.HasPrefix(id, "input-") || strings.HasPrefix(id, "list-")
}

func decodeSequenceSectionID(id string) (string, int, bool) {
	if !strings.HasPrefix(id, "list-") {
		return "", 0, false
	}
	value := strings.TrimPrefix(id, "list-")
	separator := strings.LastIndexByte(value, '-')
	if separator <= 0 {
		return "", 0, false
	}
	keyBytes, err := base64.RawURLEncoding.DecodeString(value[:separator])
	if err != nil {
		return "", 0, false
	}
	index, err := strconv.Atoi(value[separator+1:])
	// 同 input-<index>：真实下标可能大于区块数上限，越界交给序列长度判断。
	if err != nil || index < 0 {
		return "", 0, false
	}
	return string(keyBytes), index, true
}

func fieldSectionID(key string) string {
	return "field-" + base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeFieldSectionID(id string) (string, bool) {
	if !strings.HasPrefix(id, "field-") {
		return "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "field-"))
	if err != nil || len(decoded) == 0 {
		return "", false
	}
	return string(decoded), true
}

func requestContentFieldKind(key string) string {
	switch strings.ToLower(key) {
	case "tools":
		return "tools"
	case "client_metadata", "metadata":
		return "metadata"
	case "instructions":
		return "instructions"
	default:
		return "field"
	}
}

func isOpaqueKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "encrypted_content" || strings.Contains(key, "encrypted_content")
}

func isOpaqueValue(key string, value any) bool {
	if isOpaqueKey(key) {
		return true
	}
	text, ok := value.(string)
	return ok && isHighEntropyString(text)
}

func isHighEntropyString(value string) bool {
	if len(value) < 256 || strings.IndexFunc(value, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\r' || r == '\t'
	}) >= 0 {
		return false
	}
	unique := make(map[rune]struct{}, 64)
	for _, char := range value {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_+/=", char) {
			return false
		}
		unique[char] = struct{}{}
	}
	return len(unique) >= 32
}

func opaqueHash(value any) string {
	if text, ok := value.(string); ok {
		digest := sha256.Sum256([]byte(text))
		return hex.EncodeToString(digest[:])
	}
	data, err := common.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := strconv.Atoi(string(typed))
		return parsed
	default:
		return 0
	}
}

func jsonValueKind(value any) string {
	switch value.(type) {
	case nil:
		return ""
	case string:
		return "text"
	case bool, float64, json.Number:
		return "scalar"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "value"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func arrayLength(value any) int {
	list, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(list)
}

func auditRequestID(audit *model.RequestContentAudit) string {
	if audit == nil {
		return ""
	}
	return audit.RequestId
}

func auditRelayFormat(audit *model.RequestContentAudit) string {
	if audit == nil {
		return ""
	}
	return audit.RelayFormat
}

func auditEndpointPath(audit *model.RequestContentAudit) string {
	if audit == nil {
		return ""
	}
	return audit.EndpointPath
}

func auditContentSize(audit *model.RequestContentAudit) int64 {
	if audit == nil {
		return 0
	}
	return audit.ContentSize
}

func auditStoredSize(audit *model.RequestContentAudit) int64 {
	if audit == nil {
		return 0
	}
	return audit.StoredSize
}

func formatBytes(value int64) string {
	if value < 1024 {
		return strconv.FormatInt(value, 10) + " B"
	}
	units := []string{"KB", "MB", "GB"}
	floatValue := float64(value)
	unitIndex := -1
	for floatValue >= 1024 && unitIndex < len(units)-1 {
		floatValue /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.1f %s", floatValue, units[unitIndex])
}
