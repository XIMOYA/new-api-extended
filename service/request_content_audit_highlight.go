// service/request_content_audit_highlight.go
// 请求记录的"本轮最新用户消息"提取：agent 客户端每轮重发整段对话，
// 列表页需要一眼看到这条记录里最后一条用户输入，而不是从头翻几百个区块。
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/model"
	auditstore "github.com/QuantumNous/new-api/service/request_content_audit"
)

const requestContentHighlightPreviewBytes = 240

type RequestContentAuditHighlight struct {
	RequestID    string `json:"request_id"`
	Available    bool   `json:"available"`
	SectionID    string `json:"section_id,omitempty"`
	SectionIndex int    `json:"section_index,omitempty"`
	SectionCount int    `json:"section_count"`
	Role         string `json:"role,omitempty"`
	Preview      string `json:"preview,omitempty"`
	ContentSize  int64  `json:"content_size,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	Message      string `json:"message,omitempty"`
}

// BuildRequestContentAuditHighlight 读取正文并返回最后一条用户消息的定位信息。
func BuildRequestContentAuditHighlight(ctx context.Context, store *auditstore.Store, audit *model.RequestContentAudit, redacted bool) (RequestContentAuditHighlight, error) {
	highlight := RequestContentAuditHighlight{RequestID: auditRequestID(audit)}
	if audit == nil || !audit.ContentAvailable() {
		highlight.Message = requestContentViewUnavailableMessage
		return highlight, nil
	}

	envelope, err := readRequestContentAuditEnvelope(ctx, store, audit, redacted)
	if err != nil {
		// 正文超过投影上限时不算错误，前端只是不显示摘要。
		if errors.Is(err, ErrRequestContentAuditViewSourceTooLarge) {
			highlight.Message = requestContentViewSourceTooLargeMessage
			return highlight, nil
		}
		return highlight, err
	}

	// 不能复用 buildRequestContentViewSections 的结果：它有区块上限，几百条的 agent 记录
	// 会被截断，最新那条用户消息根本不在里面。这里直接从正文序列倒着扫。
	section, index, total, ok := latestUserMessagePosition(envelope["payload"])
	highlight.SectionCount = total
	if !ok {
		return highlight, nil
	}
	highlight.Available = true
	highlight.SectionID = section.ID
	highlight.SectionIndex = index
	highlight.Role = section.Role
	highlight.ContentSize = section.ContentSize
	trimmed := truncateUTF8(section.Preview, requestContentHighlightPreviewBytes)
	highlight.Preview = trimmed
	highlight.Truncated = len(trimmed) < len(section.Preview)
	return highlight, nil
}

// latestUserMessagePosition 从正文的消息序列末尾往前找最后一条用户消息，
// 返回的区块 ID 与结构化视图一致，前端可以直接定位。
func latestUserMessagePosition(payload any) (RequestContentAuditViewSection, int, int, bool) {
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		return RequestContentAuditViewSection{}, 0, 0, false
	}
	sequenceKeys := make([]string, 0, len(requestContentViewSequenceKeys))
	total := 0
	for _, key := range requestContentViewSequenceKeys {
		items, ok := payloadMap[key].([]any)
		if !ok {
			continue
		}
		sequenceKeys = append(sequenceKeys, key)
		total += len(items)
	}

	for keyIndex := len(sequenceKeys) - 1; keyIndex >= 0; keyIndex-- {
		key := sequenceKeys[keyIndex]
		items := payloadMap[key].([]any)
		for index := len(items) - 1; index >= 0; index-- {
			sectionID := sequenceSectionID(key, index)
			if key == "input" {
				sectionID = fmt.Sprintf("input-%d", index)
			}
			section := buildRequestContentViewSection(sectionID, items[index])
			if section.Kind != "message" || section.Role != "user" {
				continue
			}
			return section, index + 1, total, true
		}
	}
	return RequestContentAuditViewSection{}, 0, total, false
}

// latestUserMessageSection 返回最后一条用户消息区块及其序号（从 1 开始）。
func latestUserMessageSection(sections []RequestContentAuditViewSection) (RequestContentAuditViewSection, int, bool) {
	for index := len(sections) - 1; index >= 0; index-- {
		section := sections[index]
		if section.Kind != "message" || section.Role != "user" {
			continue
		}
		return section, index + 1, true
	}
	return RequestContentAuditViewSection{}, 0, false
}
