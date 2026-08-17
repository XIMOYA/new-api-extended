// service/request_content_audit_prune_test.go
// 验证审计正文裁剪策略：保留用户与模型可读内容，工具调用与加密推理只留指纹。
package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func droppedPlaceholder(t *testing.T, value any) (string, int64, string) {
	t.Helper()
	kind, size, hash, ok := requestContentDroppedPlaceholder(value)
	require.True(t, ok, "expected a dropped placeholder, got %#v", value)
	return kind, size, hash
}

func TestPruneResponsesPayloadKeepsReadableContent(t *testing.T) {
	payload := map[string]any{
		"model":        "gpt-5.6-luna",
		"instructions": "follow the project conventions",
		"tools": []any{
			map[string]any{"type": "function", "name": "shell", "parameters": map[string]any{"a": "b"}},
		},
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "请修复这个 bug"},
				},
			},
			map[string]any{
				"type":              "reasoning",
				"summary":           []any{map[string]any{"type": "summary_text", "text": "先定位限流中间件"}},
				"encrypted_content": "AAAABBBBCCCCDDDD",
			},
			map[string]any{
				"type":      "function_call",
				"name":      "shell",
				"call_id":   "call_1",
				"arguments": `{"command":["rg","429"]}`,
			},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "very long shell output",
			},
			map[string]any{
				"type": "message",
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "output_text", "text": "已定位到 CriticalRateLimit"},
				},
			},
		},
	}

	pruned, summary := pruneRequestContentPayload(payload)
	result, ok := pruned.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "gpt-5.6-luna", result["model"])
	assert.Equal(t, "follow the project conventions", result["instructions"])

	toolsKind, toolsSize, toolsHash := droppedPlaceholder(t, result["tools"])
	assert.Equal(t, RequestContentDroppedToolsSchema, toolsKind)
	assert.Positive(t, toolsSize)
	assert.Len(t, toolsHash, 64)

	items, ok := result["input"].([]any)
	require.True(t, ok)
	require.Len(t, items, 5)

	userItem := items[0].(map[string]any)
	userBlocks := userItem["content"].([]any)
	assert.Equal(t, "请修复这个 bug", userBlocks[0].(map[string]any)["text"])

	reasoningItem := items[1].(map[string]any)
	assert.NotNil(t, reasoningItem["summary"])
	reasoningKind, _, _ := droppedPlaceholder(t, reasoningItem["encrypted_content"])
	assert.Equal(t, RequestContentDroppedReasoning, reasoningKind)

	callItem := items[2].(map[string]any)
	assert.Equal(t, "shell", callItem["name"])
	assert.Equal(t, "call_1", callItem["call_id"])
	callKind, _, _ := droppedPlaceholder(t, callItem["arguments"])
	assert.Equal(t, RequestContentDroppedToolCall, callKind)

	outputItem := items[3].(map[string]any)
	outputKind, _, _ := droppedPlaceholder(t, outputItem["output"])
	assert.Equal(t, RequestContentDroppedToolResult, outputKind)

	assistantItem := items[4].(map[string]any)
	assistantBlocks := assistantItem["content"].([]any)
	assert.Equal(t, "已定位到 CriticalRateLimit", assistantBlocks[0].(map[string]any)["text"])

	assert.Equal(t, requestContentPruneSchema, summary.Schema)
	assert.Positive(t, summary.DroppedBytes)
	assert.Positive(t, summary.KeptBytes)
	assert.Len(t, summary.Dropped, 4)
}

func TestPruneAnthropicPayloadDropsToolContentAndSignature(t *testing.T) {
	payload := map[string]any{
		"system": []any{map[string]any{"type": "text", "text": "you are a coding agent"}},
		"tools":  []any{map[string]any{"name": "Bash", "input_schema": map[string]any{"x": "y"}}},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "帮我看下这段代码"},
					map[string]any{"type": "image", "source": map[string]any{"asset_key": "asset-0"}},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "thinking": "需要先读文件", "signature": "SIGSIGSIG"},
					map[string]any{"type": "tool_use", "id": "tu_1", "name": "Bash", "input": map[string]any{"command": "ls"}},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "tool_result", "tool_use_id": "tu_1", "content": "1000 行输出"},
				},
			},
		},
	}

	pruned, summary := pruneRequestContentPayload(payload)
	result := pruned.(map[string]any)

	systemBlocks := result["system"].([]any)
	assert.Equal(t, "you are a coding agent", systemBlocks[0].(map[string]any)["text"])
	toolsKind, _, _ := droppedPlaceholder(t, result["tools"])
	assert.Equal(t, RequestContentDroppedToolsSchema, toolsKind)

	messages := result["messages"].([]any)
	userBlocks := messages[0].(map[string]any)["content"].([]any)
	assert.Equal(t, "帮我看下这段代码", userBlocks[0].(map[string]any)["text"])
	assert.NotNil(t, userBlocks[1].(map[string]any)["source"])

	assistantBlocks := messages[1].(map[string]any)["content"].([]any)
	thinking := assistantBlocks[0].(map[string]any)
	assert.Equal(t, "需要先读文件", thinking["thinking"])
	signatureKind, _, _ := droppedPlaceholder(t, thinking["signature"])
	assert.Equal(t, RequestContentDroppedSignature, signatureKind)

	toolUse := assistantBlocks[1].(map[string]any)
	assert.Equal(t, "Bash", toolUse["name"])
	inputKind, _, _ := droppedPlaceholder(t, toolUse["input"])
	assert.Equal(t, RequestContentDroppedToolCall, inputKind)

	toolResultBlocks := messages[2].(map[string]any)["content"].([]any)
	toolResult := toolResultBlocks[0].(map[string]any)
	assert.Equal(t, "tu_1", toolResult["tool_use_id"])
	resultKind, _, _ := droppedPlaceholder(t, toolResult["content"])
	assert.Equal(t, RequestContentDroppedToolResult, resultKind)

	assert.Positive(t, summary.DroppedBytes)
}

func TestPruneChatCompletionsDropsToolRoleAndToolCallArguments(t *testing.T) {
	payload := map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "be concise"},
			map[string]any{"role": "user", "content": "读一下配置"},
			map[string]any{
				"role": "assistant",
				"tool_calls": []any{
					map[string]any{
						"id":       "call_9",
						"type":     "function",
						"function": map[string]any{"name": "read", "arguments": `{"path":"config.yaml"}`},
					},
				},
			},
			map[string]any{"role": "tool", "tool_call_id": "call_9", "content": "文件内容很长"},
		},
	}

	pruned, summary := pruneRequestContentPayload(payload)
	messages := pruned.(map[string]any)["messages"].([]any)

	assert.Equal(t, "be concise", messages[0].(map[string]any)["content"])
	assert.Equal(t, "读一下配置", messages[1].(map[string]any)["content"])

	toolCalls := messages[2].(map[string]any)["tool_calls"].([]any)
	function := toolCalls[0].(map[string]any)["function"].(map[string]any)
	assert.Equal(t, "read", function["name"])
	argumentsKind, _, _ := droppedPlaceholder(t, function["arguments"])
	assert.Equal(t, RequestContentDroppedToolCall, argumentsKind)

	toolMessage := messages[3].(map[string]any)
	assert.Equal(t, "call_9", toolMessage["tool_call_id"])
	resultKind, _, _ := droppedPlaceholder(t, toolMessage["content"])
	assert.Equal(t, RequestContentDroppedToolResult, resultKind)

	assert.Positive(t, summary.DroppedBytes)
}

func TestRequestContentDroppedStatsAndRetentionRoundTrip(t *testing.T) {
	payload := map[string]any{
		"input": []any{
			map[string]any{"type": "function_call_output", "output": "0123456789"},
			map[string]any{"type": "function_call", "arguments": "ab"},
		},
	}
	pruned, summary := pruneRequestContentPayload(payload)

	total, dominant := requestContentDroppedStats(pruned, 0)
	assert.Equal(t, summary.DroppedBytes, total)
	assert.Equal(t, RequestContentDroppedToolResult, dominant)

	envelope := map[string]any{
		"payload":     pruned,
		"audit_prune": summary,
		"original":    RequestContentOriginal{Sha256: "abc", Size: 4096},
	}
	encoded, err := json.Marshal(envelope)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	retention := requestContentRetentionFromEnvelope(decoded)
	require.NotNil(t, retention)
	assert.Equal(t, requestContentPruneSchema, retention.Schema)
	assert.Equal(t, summary.DroppedBytes, retention.DroppedBytes)
	assert.Equal(t, summary.KeptBytes, retention.KeptBytes)
	assert.Equal(t, "abc", retention.OriginalSha256)
	assert.EqualValues(t, 4096, retention.OriginalSize)
	assert.Len(t, retention.Dropped, len(summary.Dropped))
}
