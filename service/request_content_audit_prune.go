// service/request_content_audit_prune.go
// 审计正文裁剪策略：只保留用户消息、AI 文本、可读推理摘要与系统提示，
// 工具定义、工具调用参数、工具执行结果和不可读的加密推理只留类型、字节数与哈希指纹。
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
)

const requestContentPruneSchema = 1

// 占位对象里的固定字段名，视图层和前端都依赖它们判断"内容未保留"。
const (
	requestContentDroppedFlagKey = "audit_dropped"
	requestContentDroppedKindKey = "audit_kind"
	requestContentDroppedSizeKey = "size"
	requestContentDroppedHashKey = "sha256"
)

// 被丢弃内容的分类，同时用于统计和前端展示。
const (
	RequestContentDroppedToolsSchema = "tools_schema"
	RequestContentDroppedToolCall    = "tool_call"
	RequestContentDroppedToolResult  = "tool_result"
	RequestContentDroppedReasoning   = "reasoning_opaque"
	RequestContentDroppedSignature   = "signature"
)

type RequestContentPruneSummary struct {
	Schema       int                           `json:"schema"`
	KeptBytes    int64                         `json:"kept_bytes"`
	DroppedBytes int64                         `json:"dropped_bytes"`
	Dropped      []RequestContentPruneCategory `json:"dropped,omitempty"`
}

type RequestContentPruneCategory struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

type RequestContentOriginal struct {
	Sha256 string `json:"sha256,omitempty"`
	Size   int64  `json:"size"`
}

type requestContentPruner struct {
	counts map[string]*RequestContentPruneCategory
	order  []string
}

func newRequestContentPruner() *requestContentPruner {
	return &requestContentPruner{counts: make(map[string]*RequestContentPruneCategory, 8)}
}

// drop 把一段内容替换成只含指纹的占位对象，原文不落盘。
func (pruner *requestContentPruner) drop(kind string, value any) any {
	data, err := common.Marshal(value)
	if err != nil {
		data = nil
	}
	size := int64(len(data))
	placeholder := map[string]any{
		requestContentDroppedFlagKey: true,
		requestContentDroppedKindKey: kind,
		requestContentDroppedSizeKey: size,
	}
	if size > 0 {
		digest := sha256.Sum256(data)
		placeholder[requestContentDroppedHashKey] = hex.EncodeToString(digest[:])
	}

	category, ok := pruner.counts[kind]
	if !ok {
		category = &RequestContentPruneCategory{Kind: kind}
		pruner.counts[kind] = category
		pruner.order = append(pruner.order, kind)
	}
	category.Count++
	category.Bytes += size
	return placeholder
}

func (pruner *requestContentPruner) summary(kept any) RequestContentPruneSummary {
	summary := RequestContentPruneSummary{Schema: requestContentPruneSchema}
	for _, kind := range pruner.order {
		category := pruner.counts[kind]
		summary.Dropped = append(summary.Dropped, *category)
		summary.DroppedBytes += category.Bytes
	}
	if data, err := common.Marshal(kept); err == nil {
		summary.KeptBytes = int64(len(data))
	}
	return summary
}

// pruneRequestContentPayload 返回裁剪后的正文与丢弃统计，输入值来自 normalizeRequestPayload。
func pruneRequestContentPayload(payload any) (any, RequestContentPruneSummary) {
	pruner := newRequestContentPruner()
	kept := pruner.pruneValue(payload, 0)
	return kept, pruner.summary(kept)
}

func (pruner *requestContentPruner) pruneValue(value any, depth int) any {
	if depth > requestContentViewMaxDepth {
		return value
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = pruner.pruneField(key, child, depth)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = pruner.pruneValue(child, depth+1)
		}
		return result
	default:
		return value
	}
}

func (pruner *requestContentPruner) pruneField(key string, value any, depth int) any {
	switch {
	case isRequestContentToolsKey(key) && !isEmptyValue(value):
		return pruner.drop(RequestContentDroppedToolsSchema, value)
	case isOpaqueKey(key) && !isEmptyValue(value):
		return pruner.drop(RequestContentDroppedReasoning, value)
	case isRequestContentSignatureKey(key) && !isEmptyValue(value):
		return pruner.drop(RequestContentDroppedSignature, value)
	case isRequestContentArgumentsKey(key) && !isEmptyValue(value):
		return pruner.drop(RequestContentDroppedToolCall, value)
	case isRequestContentViewSequenceKey(key):
		if items, ok := value.([]any); ok {
			result := make([]any, len(items))
			for index, item := range items {
				result[index] = pruner.pruneItem(item, depth+1)
			}
			return result
		}
		return pruner.pruneValue(value, depth+1)
	default:
		return pruner.pruneValue(value, depth+1)
	}
}

// pruneItem 处理消息序列里的单项，既覆盖 Responses 的顶层 item，也覆盖 Anthropic 的 content 区块。
func (pruner *requestContentPruner) pruneItem(value any, depth int) any {
	item, ok := value.(map[string]any)
	if !ok || depth > requestContentViewMaxDepth {
		return pruner.pruneValue(value, depth)
	}
	itemType := stringValue(item["type"])
	role := stringValue(item["role"])
	result := make(map[string]any, len(item))

	switch {
	case isRequestContentToolResultType(itemType) || isRequestContentToolRole(role):
		for key, child := range item {
			if isRequestContentToolResultPayloadKey(key) && !isEmptyValue(child) {
				result[key] = pruner.drop(RequestContentDroppedToolResult, child)
				continue
			}
			result[key] = pruner.pruneField(key, child, depth)
		}
	case isRequestContentToolCallType(itemType):
		for key, child := range item {
			if isRequestContentToolCallPayloadKey(key) && !isEmptyValue(child) {
				result[key] = pruner.drop(RequestContentDroppedToolCall, child)
				continue
			}
			result[key] = pruner.pruneField(key, child, depth)
		}
	default:
		for key, child := range item {
			if key == "content" {
				if blocks, ok := child.([]any); ok {
					pruned := make([]any, len(blocks))
					for index, block := range blocks {
						pruned[index] = pruner.pruneItem(block, depth+1)
					}
					result[key] = pruned
					continue
				}
			}
			result[key] = pruner.pruneField(key, child, depth)
		}
	}
	return result
}

var (
	requestContentToolCallTypes = map[string]bool{
		"function_call": true, "custom_tool_call": true, "tool_use": true,
		"server_tool_use": true, "computer_call": true, "local_shell_call": true,
		"code_interpreter_call": true, "mcp_call": true, "shell_call": true,
	}
	requestContentToolResultTypes = map[string]bool{
		"function_call_output": true, "custom_tool_call_output": true, "tool_result": true,
		"computer_call_output": true, "local_shell_call_output": true,
		"code_interpreter_call_output": true, "web_search_tool_result": true,
		"mcp_call_output": true, "shell_call_output": true,
	}
	requestContentToolCallPayloadKeys = map[string]bool{
		"arguments": true, "input": true, "params": true, "parameters": true, "action": true,
	}
	requestContentToolResultPayloadKeys = map[string]bool{
		"output": true, "content": true, "result": true, "results": true, "outputs": true,
	}
)

func isRequestContentToolCallType(itemType string) bool {
	return requestContentToolCallTypes[itemType]
}

func isRequestContentToolResultType(itemType string) bool {
	return requestContentToolResultTypes[itemType]
}

func isRequestContentToolCallPayloadKey(key string) bool {
	return requestContentToolCallPayloadKeys[key]
}

func isRequestContentToolResultPayloadKey(key string) bool {
	return requestContentToolResultPayloadKeys[key]
}

func isRequestContentToolRole(role string) bool {
	return role == "tool" || role == "function"
}

func isRequestContentToolsKey(key string) bool {
	return key == "tools" || key == "functions"
}

func isRequestContentSignatureKey(key string) bool {
	return key == "signature" || key == "thinking_signature"
}

func isRequestContentArgumentsKey(key string) bool {
	return key == "arguments"
}

func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// requestContentDroppedPlaceholder 识别裁剪占位对象，供视图层还原展示信息。
func requestContentDroppedPlaceholder(value any) (kind string, size int64, hash string, ok bool) {
	item, isMap := value.(map[string]any)
	if !isMap {
		return "", 0, "", false
	}
	flag, hasFlag := item[requestContentDroppedFlagKey].(bool)
	if !hasFlag || !flag {
		return "", 0, "", false
	}
	return stringValue(item[requestContentDroppedKindKey]),
		requestContentDroppedSize(item[requestContentDroppedSizeKey]),
		stringValue(item[requestContentDroppedHashKey]),
		true
}

func requestContentDroppedSize(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// requestContentDroppedStats 统计一个区块内被裁剪掉的字节数，并返回占比最大的分类。
func requestContentDroppedStats(value any, depth int) (int64, string) {
	totals := make(map[string]int64, 4)
	collectRequestContentDropped(value, depth, totals)
	var total, best int64
	dominant := ""
	for kind, bytes := range totals {
		total += bytes
		if bytes > best || (bytes == best && kind < dominant) {
			best = bytes
			dominant = kind
		}
	}
	return total, dominant
}

func collectRequestContentDropped(value any, depth int, totals map[string]int64) {
	if depth > requestContentViewMaxDepth {
		return
	}
	if kind, size, _, ok := requestContentDroppedPlaceholder(value); ok {
		totals[kind] += size
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			collectRequestContentDropped(child, depth+1, totals)
		}
	case []any:
		for _, child := range typed {
			collectRequestContentDropped(child, depth+1, totals)
		}
	}
}

// requestContentRetentionFromEnvelope 把正文里的裁剪统计还原成视图结构。
func requestContentRetentionFromEnvelope(envelope map[string]any) *RequestContentAuditViewRetention {
	summary, ok := envelope["audit_prune"].(map[string]any)
	if !ok {
		return nil
	}
	retention := &RequestContentAuditViewRetention{
		Schema:       intValue(summary["schema"]),
		KeptBytes:    requestContentDroppedSize(summary["kept_bytes"]),
		DroppedBytes: requestContentDroppedSize(summary["dropped_bytes"]),
	}
	if original, ok := envelope["original"].(map[string]any); ok {
		retention.OriginalSha256 = stringValue(original["sha256"])
		retention.OriginalSize = requestContentDroppedSize(original["size"])
	}
	categories, _ := summary["dropped"].([]any)
	for _, entry := range categories {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		retention.Dropped = append(retention.Dropped, RequestContentPruneCategory{
			Kind:  stringValue(item["kind"]),
			Count: intValue(item["count"]),
			Bytes: requestContentDroppedSize(item["bytes"]),
		})
	}
	return retention
}
