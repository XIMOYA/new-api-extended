package service

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// maxStreamHandoffs 限制单个请求内的接力次数。
// 每次接力都会把已输出内容重新作为 prompt 发给新上游，成本随次数线性上升，
// 而且断点越多回答的连贯性越差，因此必须有硬上限。
const maxStreamHandoffs = 2

// CanHandoffStream 判断一个流中途失败的请求能否交给别的渠道接着写。
//
// 只有满足全部条件才允许接力：
//   - 功能已启用（默认关闭，属于会改变计费与回答连贯性的行为）；
//   - 当前是流式请求，且已经有正文内容发给客户端（否则应该走普通重试，那样才是真正无感）；
//   - 失败原因是上游侧问题（余额耗尽这类渠道错误），而不是客户端断开或请求本身非法；
//   - 接力次数未超上限。
func CanHandoffStream(info *relaycommon.RelayInfo, newAPIError *types.NewAPIError) bool {
	if info == nil || newAPIError == nil {
		return false
	}
	if !operation_setting.StreamHandoffEnabled {
		return false
	}
	if !info.IsStream {
		return false
	}
	// 只支持已实现续写语义的格式，其余格式（Gemini 原生流、Responses、Realtime 等）
	// 前导事件与增量结构差异较大，贸然拼接会产出客户端无法解析的流。
	switch info.RelayFormat {
	case types.RelayFormatClaude, types.RelayFormatOpenAI:
	default:
		return false
	}
	if info.StreamRelay == nil || !info.StreamRelay.HasDeliveredText() {
		return false
	}
	if info.StreamRelay.HandoffCount() >= maxStreamHandoffs {
		return false
	}
	// 客户端自己断开时重试没有意义，接力只针对上游故障。
	if info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonClientGone {
		return false
	}
	// 请求本身非法（参数错误等）换渠道也不会成功。
	if types.IsSkipRetryError(newAPIError) {
		return false
	}
	return types.IsChannelError(newAPIError) || newAPIError.StatusCode >= 500
}

// ApplyStreamHandoffPrefix 把已经发给客户端的内容作为 assistant 前缀写入请求，
// 让接替的上游从断点继续生成，而不是从头重写一遍。
//
// Claude 与 OpenAI 都支持在 messages 末尾放一条 assistant 消息作为续写起点
// （Claude 称为 assistant prefill）。返回值表示是否真的改写了请求。
func ApplyStreamHandoffPrefix(info *relaycommon.RelayInfo, request any) bool {
	if info == nil || info.StreamRelay == nil {
		return false
	}
	delivered := info.StreamRelay.DeliveredText()
	if strings.TrimSpace(delivered) == "" {
		return false
	}

	switch req := request.(type) {
	case *dto.ClaudeRequest:
		return appendClaudeAssistantPrefill(req, delivered)
	case *dto.GeneralOpenAIRequest:
		return appendOpenAIAssistantPrefill(req, delivered)
	default:
		return false
	}
}

func appendClaudeAssistantPrefill(request *dto.ClaudeRequest, delivered string) bool {
	if request == nil || len(request.Messages) == 0 {
		return false
	}
	last := &request.Messages[len(request.Messages)-1]
	// 末条已是 assistant 前缀说明前一次接力写过，直接把内容替换成最新的完整前缀，
	// 避免多次接力时前缀被重复追加。
	if last.Role == "assistant" {
		last.SetStringContent(delivered)
		return true
	}
	prefill := dto.ClaudeMessage{Role: "assistant"}
	prefill.SetStringContent(delivered)
	request.Messages = append(request.Messages, prefill)
	return true
}

func appendOpenAIAssistantPrefill(request *dto.GeneralOpenAIRequest, delivered string) bool {
	if request == nil || len(request.Messages) == 0 {
		return false
	}
	last := &request.Messages[len(request.Messages)-1]
	if last.Role == "assistant" {
		last.SetStringContent(delivered)
		return true
	}
	prefill := dto.Message{Role: "assistant"}
	prefill.SetStringContent(delivered)
	request.Messages = append(request.Messages, prefill)
	return true
}
