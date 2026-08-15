// service/request_content_audit_view_test.go
// 验证 Responses 请求可以按消息、工具调用、工具结果和加密推理分类，并限制结构化视图输出。
package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestContentViewSectionsClassifiesResponsesItems(t *testing.T) {
	longEncryptedContent := strings.Repeat("encrypted-content-", 256)
	longToolOutput := strings.Repeat("skill documentation ", 256)
	payload := map[string]any{
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "生成一张二次元猫娘图片"},
				},
			},
			map[string]any{
				"type":              "reasoning",
				"encrypted_content": longEncryptedContent,
			},
			map[string]any{
				"type":      "function_call",
				"name":      "Skill",
				"call_id":   "call-1",
				"arguments": `{"skill":"baoyu-image-gen"}`,
			},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call-1",
				"output":  longToolOutput,
			},
		},
		"tools": []any{map[string]any{"type": "function", "name": "Skill"}},
	}

	sections, summary := buildRequestContentViewSections(payload)

	require.Len(t, sections, 5)
	assert.Equal(t, 4, summary.InputItemCount)
	assert.Equal(t, 1, summary.MessageCount)
	assert.Equal(t, 1, summary.ToolCallCount)
	assert.Equal(t, 1, summary.ToolOutputCount)
	assert.Equal(t, 1, summary.ReasoningCount)
	assert.Equal(t, "message", sections[0].Kind)
	assert.Equal(t, "tool_call", sections[2].Kind)
	assert.Equal(t, "call-1", sections[2].CallID)
	assert.Equal(t, "tool_output", sections[3].Kind)
	assert.Equal(t, "text", sections[3].OutputType)
	assert.Equal(t, "tools", sections[4].Kind)
	assert.NotEmpty(t, sections[1].OpaqueHash)
	assert.Contains(t, sections[0].Preview, "猫娘")
	assert.NotContains(t, sections[1].Preview, longEncryptedContent)
	assert.Less(t, len(sections[3].Preview), len(longToolOutput))
	assert.Greater(t, summary.OpaqueBytes, int64(0))
}

func TestBuildRequestContentViewSectionsSupportsChatMessages(t *testing.T) {
	payload := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
			map[string]any{"role": "assistant", "content": "world"},
		},
		"tools": []any{map[string]any{"type": "function", "name": "Skill"}},
	}

	sections, summary := buildRequestContentViewSections(payload)

	require.Len(t, sections, 3)
	assert.Equal(t, 2, summary.InputItemCount)
	assert.Equal(t, 2, summary.MessageCount)
	assert.Equal(t, "message", sections[0].Kind)
	assert.Equal(t, "message", sections[1].Kind)
	assert.Equal(t, "user", sections[0].Role)
	assert.Contains(t, sections[1].Preview, "world")

	value, ok := requestContentAuditSectionValue(payload, sections[0].ID)
	require.True(t, ok)
	assert.Equal(t, "user", value.(map[string]any)["role"])
}

func TestRequestContentAuditSectionValueUsesStableSectionIDs(t *testing.T) {
	payload := map[string]any{
		"input": []any{
			map[string]any{"type": "message", "role": "user"},
		},
		"client_metadata": map[string]any{"session_id": "session-1"},
	}

	input, ok := requestContentAuditSectionValue(payload, "input-0")
	require.True(t, ok)
	assert.Equal(t, "message", input.(map[string]any)["type"])

	fieldID := fieldSectionID("client_metadata")
	metadata, ok := requestContentAuditSectionValue(payload, fieldID)
	require.True(t, ok)
	assert.Equal(t, "session-1", metadata.(map[string]any)["session_id"])

	_, ok = requestContentAuditSectionValue(payload, "input-invalid")
	assert.False(t, ok)
}

func TestRenderRequestContentSectionHidesEncryptedFieldsAndTruncatesUTF8(t *testing.T) {
	value := map[string]any{
		"summary":           "safe summary",
		"output":            strings.Repeat("safe output ", 128),
		"encrypted_content": strings.Repeat("x", 1024),
	}

	content, format, truncated := renderRequestContentSection(value, 128)
	assert.Equal(t, "json", format)
	assert.True(t, truncated)
	assert.NotContains(t, content, strings.Repeat("x", 32))
	assert.Contains(t, content, requestContentViewOpaqueMarker)

	text := "猫娘"
	assert.Equal(t, "猫…", truncateUTF8(text, 4))
}
