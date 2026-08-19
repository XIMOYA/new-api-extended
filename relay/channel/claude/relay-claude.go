package claude

import (
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

func stopReasonClaude2OpenAI(reason string) string {
	return relayconvert.StopReasonClaudeToOpenAI(reason)
}

func maybeMarkClaudeRefusal(c *gin.Context, stopReason string) {
	if c == nil {
		return
	}
	if strings.EqualFold(stopReason, "refusal") {
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "claude_stop_reason=refusal")
	}
}

func StreamResponseClaude2OpenAI(claudeResponse *dto.ClaudeResponse) *dto.ChatCompletionsStreamResponse {
	return relayconvert.StreamResponseClaude2OpenAI(claudeResponse)
}

func ResponseClaude2OpenAI(claudeResponse *dto.ClaudeResponse) *dto.OpenAITextResponse {
	return relayconvert.ResponseClaude2OpenAI(claudeResponse)
}

type ClaudeResponseInfo = relayconvert.ClaudeResponseInfo

func cacheCreationTokensForOpenAIUsage(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	openAIUsage := relayconvert.UsageFromClaudeUsage(usage)
	if openAIUsage == nil {
		return 0
	}
	return openAIUsage.PromptTokens - usage.PromptTokens - usage.PromptTokensDetails.CachedTokens
}

func buildOpenAIStyleUsageFromClaudeUsage(usage *dto.Usage) dto.Usage {
	mapped := relayconvert.UsageFromClaudeUsage(usage)
	if mapped == nil {
		return dto.Usage{}
	}
	return *mapped
}

func buildMessageDeltaPatchUsage(claudeResponse *dto.ClaudeResponse, claudeInfo *ClaudeResponseInfo) *dto.ClaudeUsage {
	return relayconvert.BuildMessageDeltaPatchUsage(claudeResponse, claudeInfo)
}

func shouldSkipClaudeMessageDeltaUsagePatch(info *relaycommon.RelayInfo) bool {
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	if info == nil {
		return false
	}
	return info.ChannelSetting.PassThroughBodyEnabled
}

func patchClaudeMessageDeltaUsageData(data string, usage *dto.ClaudeUsage) string {
	return relayconvert.PatchClaudeMessageDeltaUsageData(data, usage)
}

func FormatClaudeResponseInfo(claudeResponse *dto.ClaudeResponse, oaiResponse *dto.ChatCompletionsStreamResponse, claudeInfo *ClaudeResponseInfo) bool {
	return relayconvert.FormatClaudeResponseInfo(claudeResponse, oaiResponse, claudeInfo)
}

func HandleStreamResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, data string) *types.NewAPIError {
	var claudeResponse dto.ClaudeResponse
	err := common.UnmarshalJsonStr(data, &claudeResponse)
	if err != nil {
		common.SysLog("error unmarshalling stream response: " + err.Error())
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		streamErr := types.WithClaudeError(*claudeError, http.StatusInternalServerError)
		// 上游也可能在流中途以 error 事件报余额耗尽。这里改标记成渠道错误，
		// 否则它会以固定 500 的形态混在普通上游错误里，无法触发渠道禁用与后续避让。
		// 判定复用 operation_setting.IsUpstreamExhaustedError，与渠道自动禁用同一套特征。
		if operation_setting.IsUpstreamExhaustedError(streamErr) && !types.IsChannelError(streamErr) {
			streamErr.SetErrorCode(types.ErrorCodeChannelUpstreamQuotaExhausted)
		}
		return streamErr
	}
	if claudeResponse.StopReason != "" {
		maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	}
	if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
		maybeMarkClaudeRefusal(c, *claudeResponse.Delta.StopReason)
	}
	if info.RelayFormat == types.RelayFormatClaude {
		FormatClaudeResponseInfo(&claudeResponse, nil, claudeInfo)

		if claudeResponse.Type == "message_start" {
			// message_start, 获取usage
			if claudeResponse.Message != nil {
				info.UpstreamModelName = claudeResponse.Message.Model
			}
		} else if claudeResponse.Type == "message_delta" {
			// 确保 message_delta 的 usage 包含完整的 input_tokens 和 cache 相关字段
			// 解决 AWS Bedrock 等上游返回的 message_delta 缺少这些字段的问题
			if !shouldSkipClaudeMessageDeltaUsagePatch(info) {
				data = patchClaudeMessageDeltaUsageData(data, buildMessageDeltaPatchUsage(&claudeResponse, claudeInfo))
			}
		}
		countClaudeStreamBillableTools(c, info, &claudeResponse)
		if claudeResponse.Type == "message_stop" {
			ensureClaudeFinalUsage(c, info, claudeInfo)
			if err := helper.WriteNarraForkQuotaEvent(c, info, claudeInfo.Usage); err != nil {
				common.SysLog("error writing NarraFork quota event: " + err.Error())
			}
		}
		// 接力续写时第二段流会重新发一遍 message_start，客户端会因此看到两次消息起始。
		// 只抑制这一个消息级事件；block 级事件靠 index 偏移延续，不能抑制。
		if shouldSuppressClaudeHandoffEvent(info, claudeResponse.Type) {
			return nil
		}
		if patched, ok := rewriteClaudeHandoffBlockIndex(info, &claudeResponse, data); ok {
			data = patched
		}
		helper.ClaudeChunkData(c, claudeResponse, data)
		trackClaudeDeliveredContent(info, &claudeResponse)
	} else if info.RelayFormat == types.RelayFormatOpenAI {
		response := StreamResponseClaude2OpenAI(&claudeResponse)

		if !FormatClaudeResponseInfo(&claudeResponse, response, claudeInfo) {
			return nil
		}

		countClaudeStreamBillableTools(c, info, &claudeResponse)

		err = helper.ObjectData(c, response)
		if err != nil {
			logger.LogError(c, "send_stream_response_failed: "+err.Error())
		}
		trackClaudeDeliveredContent(info, &claudeResponse)
	}
	return nil
}

// shouldSuppressClaudeHandoffEvent 判断一个事件是否应在接力续写时被抑制。
//
// 只抑制 message_start：它是消息级前导，一条 SSE 连接里只应出现一次。
// 绝不能把 content_block_start 一并抑制——那是块级事件，一条消息里
// text / thinking / tool_use 各自都会发一次，抑制它会让所有 Claude 原生流式
// 请求（哪怕接力从未发生）都收到没有 start 打头的 delta，破坏 Messages API 契约。
// 第二段流的 block 起始事件靠 index 偏移延续，见 rewriteClaudeHandoffBlockIndex。
func shouldSuppressClaudeHandoffEvent(info *relaycommon.RelayInfo, eventType string) bool {
	if info == nil || info.StreamRelay == nil {
		return false
	}
	if eventType != "message_start" {
		return false
	}
	return info.StreamRelay.IsMessageStarted()
}

// claudeBlockIndexedEventTypes 是携带 content block index 的事件。
// 接力后第二段上游重新从 index 0 编号，必须加偏移才能延续第一段的 block 序列。
var claudeBlockIndexedEventTypes = map[string]struct{}{
	"content_block_start": {},
	"content_block_delta": {},
	"content_block_stop":  {},
}

// rewriteClaudeHandoffBlockIndex 把上游的 block index 平移到接力后的连续区间。
//
// 返回是否改写过 data。第一段流（偏移为 0）不做任何改动，保持零开销。
func rewriteClaudeHandoffBlockIndex(info *relaycommon.RelayInfo, resp *dto.ClaudeResponse, data string) (string, bool) {
	if info == nil || info.StreamRelay == nil || resp == nil {
		return data, false
	}
	if _, indexed := claudeBlockIndexedEventTypes[resp.Type]; !indexed {
		return data, false
	}
	offset := info.StreamRelay.BlockIndexOffset()
	if offset == 0 {
		return data, false
	}
	// Index 是可选字段，缺省时没有可平移的目标。
	if resp.Index == nil {
		return data, false
	}

	shifted := *resp.Index + offset
	resp.SetIndex(shifted)
	// data 是要原样透传的上游报文，index 必须同步改写，否则客户端读到的仍是旧值。
	patched, err := sjson.Set(data, "index", shifted)
	if err != nil {
		return data, false
	}
	return patched, true
}


// trackClaudeDeliveredContent 记录已经真实发给客户端的文本增量与前导事件状态，
// 供跨渠道接力续写时构造 assistant 前缀、并避免重复发送消息起始事件。
func trackClaudeDeliveredContent(info *relaycommon.RelayInfo, claudeResponse *dto.ClaudeResponse) {
	if info == nil || info.StreamRelay == nil || claudeResponse == nil {
		return
	}
	switch claudeResponse.Type {
	case "message_start":
		// 消息级前导，接力时第二段要抑制的就是它。
		info.StreamRelay.MarkMessageStarted()
		return
	case "content_block_start":
		// 记录已转发的 block index（此时已含接力偏移），供接力时计算下一段的起点。
		if claudeResponse.Index != nil {
			info.StreamRelay.ObserveBlockIndex(*claudeResponse.Index)
		}
		return
	case "content_block_stop":
		info.StreamRelay.CloseBlock()
		return
	case "content_block_delta":
		if claudeResponse.Index != nil {
			info.StreamRelay.ObserveBlockIndex(*claudeResponse.Index)
		}
	default:
		return
	}
	if claudeResponse.Delta == nil {
		return
	}
	// 只累积正文文本；thinking 增量不属于最终回答内容，不能作为续写前缀。
	if text := claudeResponse.Delta.GetText(); text != "" {
		info.StreamRelay.AppendDeliveredText(text)
	}
}

func countClaudeStreamBillableTools(c *gin.Context, info *relaycommon.RelayInfo, claudeResponse *dto.ClaudeResponse) {
	if claudeResponse == nil {
		return
	}
	if claudeResponse.Type == "content_block_start" &&
		claudeResponse.ContentBlock != nil &&
		claudeResponse.ContentBlock.Type == "tool_use" {
		info.CountBillableToolCall(dto.BuildInCallToolUse, claudeResponse.ContentBlock.Name)
	}
	if claudeResponse.Type == "message_delta" &&
		claudeResponse.Usage != nil &&
		claudeResponse.Usage.ServerToolUse != nil &&
		claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}
}

func ensureClaudeFinalUsage(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) {
	if claudeInfo == nil || claudeInfo.Usage == nil {
		return
	}
	if claudeInfo.Usage.PromptTokens == 0 {
		// 上游出错
	}
	if claudeInfo.Usage.CompletionTokens == 0 || !claudeInfo.Done {
		if common.DebugEnabled {
			common.SysLog("claude response usage is not complete, maybe upstream error")
		}
		// 只补缺失字段，不整份覆盖——保留 message_start 已拿到的 cache 字段
		fallback := service.ResponseText2Usage(c, claudeInfo.ResponseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		if claudeInfo.Usage.CompletionTokens == 0 ||
			(!claudeInfo.Done && fallback.CompletionTokens > claudeInfo.Usage.CompletionTokens) {
			claudeInfo.Usage.CompletionTokens = fallback.CompletionTokens
		}
		if claudeInfo.Usage.PromptTokens == 0 {
			claudeInfo.Usage.PromptTokens = fallback.PromptTokens
		}
		claudeInfo.Usage.TotalTokens = claudeInfo.Usage.PromptTokens + claudeInfo.Usage.CompletionTokens
	}
	claudeInfo.Usage.UsageSemantic = "anthropic"
	if claudeInfo.Usage.BillingUsage == nil {
		claudeInfo.Usage.BillingUsage = dto.NewClaudeMessagesBillingUsage(buildMessageDeltaPatchUsage(nil, claudeInfo))
	}
}

func HandleStreamFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) {
	ensureClaudeFinalUsage(c, info, claudeInfo)

	if info.RelayFormat == types.RelayFormatClaude {
		if err := helper.WriteNarraForkQuotaEvent(c, info, claudeInfo.Usage); err != nil {
			common.SysLog("error writing NarraFork quota event: " + err.Error())
		}
	} else if info.RelayFormat == types.RelayFormatOpenAI {
		if info.ShouldIncludeUsage {
			openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
			response := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.UpstreamModelName, openAIUsage)
			err := helper.ObjectData(c, response)
			if err != nil {
				common.SysLog("send final response failed: " + err.Error())
			}
		}
		if err := helper.WriteNarraForkQuotaEvent(c, info, claudeInfo.Usage); err != nil {
			common.SysLog("error writing NarraFork quota event: " + err.Error())
		}
		helper.Done(c)
	}
}

func ClaudeStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	var err *types.NewAPIError
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		err = HandleStreamResponseData(c, info, claudeInfo, data)
		if err != nil {
			sr.Stop(err)
		}
	})
	if err != nil {
		// 已经吐给客户端的内容必须照实结算：上层失败路径会整笔退还预扣费，
		// 若不在这里落账，被打断的半个回答就变成完全免费，成本全部由平台承担。
		// 判据用 HasSentToClient：原生 Claude→Claude 透传不会递增 SendResponseCount，
		// 只看计数会让最常见的原生流式漏计费。
		// 注意这里不能调用 HandleStreamFinalResponse——它会发送 [DONE] 等结束事件，
		// 而接力续写还要在同一条连接上继续写，提前收尾会让客户端以为流已结束。
		if info.HasSentToClient(c) {
			ensureClaudeFinalUsage(c, info, claudeInfo)
			service.PostTextConsumeQuota(c, info, claudeInfo.Usage, nil)
		}
		return nil, err
	}

	HandleStreamFinalResponse(c, info, claudeInfo)
	return claudeInfo.Usage, nil
}

func HandleClaudeResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, httpResp *http.Response, data []byte) *types.NewAPIError {
	var claudeResponse dto.ClaudeResponse
	err := common.Unmarshal(data, &claudeResponse)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &dto.Usage{}
	}
	if claudeResponse.Usage != nil {
		claudeInfo.Usage.PromptTokens = claudeResponse.Usage.InputTokens
		claudeInfo.Usage.CompletionTokens = claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.TotalTokens = claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.UsageSemantic = "anthropic"
		claudeInfo.Usage.BillingUsage = dto.NewClaudeMessagesBillingUsage(claudeResponse.Usage)
		claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Usage.CacheReadInputTokens
		claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Usage.CacheCreationInputTokens
		claudeInfo.Usage.ClaudeCacheCreation5mTokens = claudeResponse.Usage.GetCacheCreation5mTokens()
		claudeInfo.Usage.ClaudeCacheCreation1hTokens = claudeResponse.Usage.GetCacheCreation1hTokens()
	}
	var responseData []byte
	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		openaiResponse := ResponseClaude2OpenAI(&claudeResponse)
		openaiResponse.Usage = buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
		responseData, err = common.Marshal(openaiResponse)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
	case types.RelayFormatClaude:
		responseData = data
	}

	if claudeResponse.Usage != nil && claudeResponse.Usage.ServerToolUse != nil && claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}

	for _, block := range claudeResponse.Content {
		if block.Type == "tool_use" {
			info.CountBillableToolCall(dto.BuildInCallToolUse, block.Name)
		}
	}

	service.IOCopyBytesGracefully(c, httpResp, responseData)
	return nil
}

func ClaudeHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	logger.LogDebug(c, "responseBody: %s", responseBody)
	handleErr := HandleClaudeResponseData(c, info, claudeInfo, resp, responseBody)
	if handleErr != nil {
		return nil, handleErr
	}
	return claudeInfo.Usage, nil
}
