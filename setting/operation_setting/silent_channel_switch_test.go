// setting/operation_setting/silent_channel_switch_test.go
// 验证静默切换的判定边界：本站侧错误必须原样透出，上游错误才替换文案，
// 以及重试上限的取值与硬上限保护。
package operation_setting

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withSilentChannelSwitch(t *testing.T, enabled bool, message string, maxAttempts int) {
	t.Helper()
	previousEnabled := SilentChannelSwitchEnabled
	previousMessage := SilentChannelSwitchMessage
	previousAttempts := SilentChannelSwitchMaxAttempts
	t.Cleanup(func() {
		SilentChannelSwitchEnabled = previousEnabled
		SilentChannelSwitchMessage = previousMessage
		SilentChannelSwitchMaxAttempts = previousAttempts
	})
	SilentChannelSwitchEnabled = enabled
	SilentChannelSwitchMessage = message
	SilentChannelSwitchMaxAttempts = maxAttempts
}

func TestShouldHideUpstreamErrorKeepsLocalErrorsVisible(t *testing.T) {
	withSilentChannelSwitch(t, true, "", 0)

	localCodes := []types.ErrorCode{
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed,
		types.ErrorCodeInvalidRequest,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeGetChannelFailed,
	}
	for _, code := range localCodes {
		err := types.NewError(errors.New("本站侧错误"), code)
		assert.False(t, ShouldHideUpstreamError(err, true), "code %s 不应被隐藏", code)
	}

	upstream := types.NewOpenAIError(errors.New("You exceeded your current quota"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	assert.True(t, ShouldHideUpstreamError(upstream, true))
}

func TestShouldHideUpstreamErrorRequiresAttemptAndSwitch(t *testing.T) {
	upstream := types.NewOpenAIError(errors.New("upstream exploded"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)

	withSilentChannelSwitch(t, true, "", 0)
	assert.False(t, ShouldHideUpstreamError(upstream, false), "没有真正请求过渠道时不隐藏")
	assert.False(t, ShouldHideUpstreamError(nil, true))

	withSilentChannelSwitch(t, false, "", 0)
	assert.False(t, ShouldHideUpstreamError(upstream, true), "开关关闭时行为不变")
}

func TestSilentChannelSwitchMessageOrDefault(t *testing.T) {
	withSilentChannelSwitch(t, true, "   ", 0)
	assert.Equal(t, DefaultSilentChannelSwitchMessage, SilentChannelSwitchMessageOrDefault())

	withSilentChannelSwitch(t, true, "上游忙，等等再来", 0)
	assert.Equal(t, "上游忙，等等再来", SilentChannelSwitchMessageOrDefault())
}

func TestSilentChannelSwitchAttempts(t *testing.T) {
	withSilentChannelSwitch(t, false, "", 20)
	assert.Equal(t, 6, SilentChannelSwitchAttempts(5), "开关关闭时仍按 RetryTimes+1")

	withSilentChannelSwitch(t, true, "", 0)
	assert.Equal(t, 6, SilentChannelSwitchAttempts(5), "未配置上限时跟随 RetryTimes")

	withSilentChannelSwitch(t, true, "", 20)
	assert.Equal(t, 20, SilentChannelSwitchAttempts(5))

	withSilentChannelSwitch(t, true, "", MaxSilentChannelSwitchAttempts+50)
	assert.Equal(t, MaxSilentChannelSwitchAttempts, SilentChannelSwitchAttempts(5), "必须受硬上限保护")

	withSilentChannelSwitch(t, true, "", 0)
	assert.Equal(t, 1, SilentChannelSwitchAttempts(-3), "非法 RetryTimes 至少尝试一次")
}

func TestIsUpstreamExhaustedError(t *testing.T) {
	withSilentChannelSwitch(t, true, "", 0)
	require.Contains(t, AutomaticDisableKeywords, "You exceeded your current quota")

	// 上游把额度不足回成 400，默认状态码规则不会重试，必须靠关键词识别。
	quota := types.NewOpenAIError(errors.New("You exceeded your current quota, please check your plan"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	assert.True(t, IsUpstreamExhaustedError(quota))

	credit := types.NewOpenAIError(errors.New("Your credit balance is too low to access the API"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	assert.True(t, IsUpstreamExhaustedError(credit))

	disabledByStatus := types.NewOpenAIError(errors.New("invalid api key"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized)
	assert.True(t, IsUpstreamExhaustedError(disabledByStatus))

	ordinary := types.NewOpenAIError(errors.New("internal server error"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	assert.False(t, IsUpstreamExhaustedError(ordinary))

	// 本站用户额度不足绝不能被当成上游耗尽，否则会被静默吞掉。
	userQuota := types.NewError(errors.New("用户额度不足"), types.ErrorCodeInsufficientUserQuota)
	assert.False(t, IsUpstreamExhaustedError(userQuota))
	assert.False(t, IsUpstreamExhaustedError(nil))
}
