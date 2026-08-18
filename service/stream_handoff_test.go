package service

import (
	"errors"
	"net/http"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func withStreamHandoffEnabled(t *testing.T, enabled bool) {
	t.Helper()
	original := operation_setting.StreamHandoffEnabled
	operation_setting.StreamHandoffEnabled = enabled
	t.Cleanup(func() {
		operation_setting.StreamHandoffEnabled = original
	})
}

func newHandoffReadyInfo() *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		IsStream:     true,
		RelayFormat:  types.RelayFormatClaude,
		StreamRelay:  relaycommon.NewStreamRelayState(),
		StreamStatus: relaycommon.NewStreamStatus(),
	}
	info.StreamRelay.AppendDeliveredText("已经发给客户端的前半段回答")
	return info
}

// 接力会把已输出内容重新发给新上游（prompt 重复计费）并可能造成断点跳变，
// 因此准入条件必须严格：只在真正的上游故障且确有内容已发出时才允许。
func TestCanHandoffStream(t *testing.T) {
	upstreamQuotaErr := types.NewErrorWithStatusCode(
		errors.New("credit balance is too low"),
		types.ErrorCodeChannelUpstreamQuotaExhausted,
		http.StatusBadRequest,
	)

	t.Run("allows handoff for upstream channel error after content delivered", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		require.True(t, CanHandoffStream(newHandoffReadyInfo(), upstreamQuotaErr))
	})

	t.Run("disabled by default switch", func(t *testing.T) {
		withStreamHandoffEnabled(t, false)
		require.False(t, CanHandoffStream(newHandoffReadyInfo(), upstreamQuotaErr))
	})

	t.Run("rejects when nothing delivered yet", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		info := newHandoffReadyInfo()
		info.StreamRelay = relaycommon.NewStreamRelayState()
		// 没有内容发出时应该走普通重试，那样才是真正的无感切换。
		require.False(t, CanHandoffStream(info, upstreamQuotaErr))
	})

	t.Run("rejects non stream request", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		info := newHandoffReadyInfo()
		info.IsStream = false
		require.False(t, CanHandoffStream(info, upstreamQuotaErr))
	})

	t.Run("rejects unsupported relay format", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		info := newHandoffReadyInfo()
		info.RelayFormat = types.RelayFormatGemini
		require.False(t, CanHandoffStream(info, upstreamQuotaErr))
	})

	t.Run("rejects client disconnect", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		info := newHandoffReadyInfo()
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, nil)
		// 客户端自己走了，换渠道续写没有意义。
		require.False(t, CanHandoffStream(info, upstreamQuotaErr))
	})

	t.Run("rejects skip retry error", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		invalidReq := types.NewErrorWithStatusCode(
			errors.New("max_tokens too large"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
		require.False(t, CanHandoffStream(newHandoffReadyInfo(), invalidReq))
	})

	t.Run("rejects ordinary client error", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		badReq := types.NewErrorWithStatusCode(
			errors.New("invalid tool schema"),
			types.ErrorCodeBadResponseStatusCode,
			http.StatusBadRequest,
		)
		// 4xx 且非渠道错误说明请求本身有问题，换渠道也不会成功。
		require.False(t, CanHandoffStream(newHandoffReadyInfo(), badReq))
	})

	t.Run("stops after reaching handoff limit", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		info := newHandoffReadyInfo()
		for i := 0; i < maxStreamHandoffs; i++ {
			require.True(t, CanHandoffStream(info, upstreamQuotaErr))
			info.StreamRelay.BeginHandoff()
		}
		// 每次接力都会重复计费一遍 prompt，必须有硬上限。
		require.False(t, CanHandoffStream(info, upstreamQuotaErr))
	})

	t.Run("nil inputs are safe", func(t *testing.T) {
		withStreamHandoffEnabled(t, true)
		require.False(t, CanHandoffStream(nil, upstreamQuotaErr))
		require.False(t, CanHandoffStream(newHandoffReadyInfo(), nil))
	})
}

// 续写前缀决定接替的上游从哪里继续写。前缀必须是完整的已输出内容，
// 且多次接力时不能重复堆叠。
func TestApplyStreamHandoffPrefix(t *testing.T) {
	t.Run("appends assistant prefill to claude request", func(t *testing.T) {
		info := newHandoffReadyInfo()
		request := &dto.ClaudeRequest{
			Model: "claude-opus-4-8",
			Messages: []dto.ClaudeMessage{
				{Role: "user", Content: "写一篇长文"},
			},
		}

		require.True(t, ApplyStreamHandoffPrefix(info, request))
		require.Len(t, request.Messages, 2)
		require.Equal(t, "assistant", request.Messages[1].Role)
		require.Equal(t, "已经发给客户端的前半段回答", request.Messages[1].GetStringContent())
	})

	t.Run("replaces existing assistant prefill instead of stacking", func(t *testing.T) {
		info := newHandoffReadyInfo()
		request := &dto.ClaudeRequest{
			Model: "claude-opus-4-8",
			Messages: []dto.ClaudeMessage{
				{Role: "user", Content: "写一篇长文"},
			},
		}
		require.True(t, ApplyStreamHandoffPrefix(info, request))

		// 第二次接力：又多输出了一段内容，前缀应被整体替换为最新的完整内容。
		info.StreamRelay.AppendDeliveredText("，以及第二段接力产生的内容")
		require.True(t, ApplyStreamHandoffPrefix(info, request))

		require.Len(t, request.Messages, 2, "多次接力不能不断追加 assistant 消息")
		require.Equal(t,
			"已经发给客户端的前半段回答，以及第二段接力产生的内容",
			request.Messages[1].GetStringContent(),
		)
	})

	t.Run("appends assistant prefill to openai request", func(t *testing.T) {
		info := newHandoffReadyInfo()
		info.RelayFormat = types.RelayFormatOpenAI
		request := &dto.GeneralOpenAIRequest{
			Model: "gpt-4o",
			Messages: []dto.Message{
				{Role: "user", Content: "写一篇长文"},
			},
		}

		require.True(t, ApplyStreamHandoffPrefix(info, request))
		require.Len(t, request.Messages, 2)
		require.Equal(t, "assistant", request.Messages[1].Role)
		require.Equal(t, "已经发给客户端的前半段回答", request.Messages[1].StringContent())
	})

	t.Run("no delivered content leaves request untouched", func(t *testing.T) {
		info := newHandoffReadyInfo()
		info.StreamRelay = relaycommon.NewStreamRelayState()
		request := &dto.ClaudeRequest{
			Model:    "claude-opus-4-8",
			Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
		}

		require.False(t, ApplyStreamHandoffPrefix(info, request))
		require.Len(t, request.Messages, 1)
	})

	t.Run("nil state is safe", func(t *testing.T) {
		require.False(t, ApplyStreamHandoffPrefix(nil, &dto.ClaudeRequest{}))
		require.False(t, ApplyStreamHandoffPrefix(&relaycommon.RelayInfo{}, &dto.ClaudeRequest{}))
	})
}
