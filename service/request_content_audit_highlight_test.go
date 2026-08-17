// service/request_content_audit_highlight_test.go
// 验证"本轮最新用户消息"定位：取最后一条用户消息、无用户消息时不可用、预览按字节截断。
package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestUserMessageSectionPicksLastUserTurn(t *testing.T) {
	payload := map[string]any{
		"input": []any{
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": []any{map[string]any{"type": "input_text", "text": "第一句提问"}},
			},
			map[string]any{
				"type":    "message",
				"role":    "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": "第一次回答"}},
			},
			map[string]any{"type": "function_call", "name": "shell", "arguments": "{}"},
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": []any{map[string]any{"type": "input_text", "text": "最后这句才是我刚发的"}},
			},
			map[string]any{"type": "reasoning", "encrypted_content": strings.Repeat("x", 512)},
		},
	}

	sections, summary := buildRequestContentViewSections(payload)
	require.Equal(t, 3, summary.MessageCount)

	section, index, total, ok := latestUserMessagePosition(payload)
	require.True(t, ok)
	assert.Equal(t, "message", section.Kind)
	assert.Equal(t, "user", section.Role)
	assert.Equal(t, 4, index)
	assert.Equal(t, 5, total)
	assert.Contains(t, section.Preview, "最后这句才是我刚发的")
	assert.NotContains(t, section.Preview, "第一句提问")
	assert.Equal(t, sections[3].ID, section.ID)
}

func TestLatestUserMessagePositionSurvivesSectionCap(t *testing.T) {
	items := make([]any, 0, 600)
	for index := 0; index < 600; index++ {
		role := "assistant"
		text := fmt.Sprintf("历史消息 %d", index)
		if index%2 == 0 {
			role = "user"
		}
		if index == 598 {
			text = "这是我刚刚发出的那句话"
		}
		items = append(items, map[string]any{
			"type":    "message",
			"role":    role,
			"content": []any{map[string]any{"type": "input_text", "text": text}},
		})
	}
	payload := map[string]any{"input": items}

	sections, summary := buildRequestContentViewSections(payload)
	require.Equal(t, requestContentViewMaxSections, len(sections))
	assert.True(t, summary.SectionsTruncated)
	assert.Equal(t, 600-requestContentViewMaxSections, summary.OmittedSectionCount)
	// 头部保留最初的上下文，尾部保留最近的若干条。
	assert.Equal(t, "input-0", sections[0].ID)
	assert.Equal(t, "input-599", sections[len(sections)-1].ID)

	section, index, total, ok := latestUserMessagePosition(payload)
	require.True(t, ok)
	assert.Equal(t, 599, index)
	assert.Equal(t, 600, total)
	assert.Contains(t, section.Preview, "这是我刚刚发出的那句话")

	// 定位到的区块必须真的出现在返回的列表里，否则前端跳不过去。
	found := false
	for _, candidate := range sections {
		if candidate.ID == section.ID {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestLatestUserMessageSectionMissingUserTurn(t *testing.T) {
	payload := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "assistant",
				"content": []any{map[string]any{"type": "text", "text": "只有模型输出"}},
			},
		},
	}
	_, _, _, ok := latestUserMessagePosition(payload)
	assert.False(t, ok)
}

func TestSectionValueResolvesIndexBeyondSectionCap(t *testing.T) {
	items := make([]any, 0, 600)
	for index := 0; index < 600; index++ {
		items = append(items, map[string]any{
			"type":    "message",
			"role":    "user",
			"content": []any{map[string]any{"type": "input_text", "text": fmt.Sprintf("消息 %d", index)}},
		})
	}

	inputPayload := map[string]any{"input": items}
	value, ok := requestContentAuditSectionValue(inputPayload, "input-599")
	require.True(t, ok)
	assert.Contains(t, previewValue(value), "消息 599")
	_, ok = requestContentAuditSectionValue(inputPayload, "input-600")
	assert.False(t, ok)

	messagePayload := map[string]any{"messages": items}
	value, ok = requestContentAuditSectionValue(messagePayload, sequenceSectionID("messages", 590))
	require.True(t, ok)
	assert.Contains(t, previewValue(value), "消息 590")
}

func TestHighlightPreviewTruncatesLongUserMessage(t *testing.T) {
	long := strings.Repeat("窗", 400)
	payload := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": long},
		},
	}
	sections, _ := buildRequestContentViewSections(payload)
	section, _, ok := latestUserMessageSection(sections)
	require.True(t, ok)

	// truncateUTF8 会在截断处补一个省略号，所以长度上限是 limit + 省略号本身。
	trimmed := truncateUTF8(section.Preview, requestContentHighlightPreviewBytes)
	assert.LessOrEqual(t, len(trimmed), requestContentHighlightPreviewBytes+len("…"))
	assert.True(t, strings.HasSuffix(trimmed, "…"))
	assert.True(t, strings.HasPrefix(long, strings.TrimSuffix(trimmed, "…")))
	assert.Less(t, len(trimmed), len(section.Preview))
}
