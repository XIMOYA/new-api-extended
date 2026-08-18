// setting/operation_setting/silent_channel_switch.go
// 静默切换渠道：上游渠道报错时只在后台记录真实原因，对用户返回统一文案，
// 并允许把重试上限单独放开，直到确实没有可用渠道再报错。
package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
)

const (
	DefaultSilentChannelSwitchMessage = "上游服务暂时不可用，请稍后重试"
	// 单个请求最多尝试的渠道数硬上限，避免配置写大后一个请求长时间占用连接。
	MaxSilentChannelSwitchAttempts = 32
)

var (
	SilentChannelSwitchEnabled = false
	SilentChannelSwitchMessage = DefaultSilentChannelSwitchMessage
	// 0 表示沿用 RetryTimes，大于 0 时按这个值决定最多尝试多少个渠道。
	SilentChannelSwitchMaxAttempts = 0
)

// 这些错误由本站自身产生，与上游渠道无关，必须原样返回，否则用户无法自查。
var localFacingErrorCodes = map[types.ErrorCode]struct{}{
	types.ErrorCodeInsufficientUserQuota:      {},
	types.ErrorCodePreConsumeTokenQuotaFailed: {},
	types.ErrorCodeInvalidRequest:             {},
	types.ErrorCodeSensitiveWordsDetected:     {},
	types.ErrorCodeCountTokenFailed:           {},
	types.ErrorCodeModelPriceError:            {},
	types.ErrorCodeReadRequestBodyFailed:      {},
	types.ErrorCodeAccessDenied:               {},
	types.ErrorCodeGenRelayInfoFailed:         {},
	types.ErrorCodeGetChannelFailed:           {},
	types.ErrorCodeViolationFeeGrokCSAM:       {},
}

// SilentChannelSwitchMessageOrDefault 返回配置的统一文案，留空时回落默认值。
func SilentChannelSwitchMessageOrDefault() string {
	message := strings.TrimSpace(SilentChannelSwitchMessage)
	if message == "" {
		return DefaultSilentChannelSwitchMessage
	}
	return message
}

// SilentChannelSwitchAttempts 返回静默切换下允许的渠道尝试次数上限（含首次尝试）。
func SilentChannelSwitchAttempts(retryTimes int) int {
	attempts := retryTimes + 1
	if SilentChannelSwitchEnabled && SilentChannelSwitchMaxAttempts > 0 {
		attempts = SilentChannelSwitchMaxAttempts
	}
	if attempts < 1 {
		attempts = 1
	}
	if attempts > MaxSilentChannelSwitchAttempts {
		attempts = MaxSilentChannelSwitchAttempts
	}
	return attempts
}

// IsLocalFacingError 判断错误是否属于本站侧，本站侧错误永远不隐藏。
func IsLocalFacingError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	_, exists := localFacingErrorCodes[err.GetErrorCode()]
	return exists
}

// ShouldHideUpstreamError 判断是否要把上游错误替换成统一文案。
// attempted 表示这次请求确实把流量发给过渠道，避免把本地校验错误也一并隐藏。
func ShouldHideUpstreamError(err *types.NewAPIError, attempted bool) bool {
	if !SilentChannelSwitchEnabled || err == nil || !attempted {
		return false
	}
	return !IsLocalFacingError(err)
}

// IsUpstreamExhaustedError 识别"上游自己没额度/被封禁"这类错误：
// 复用自动禁用渠道的判定特征，这类错误即使状态码不在重试范围内也应该换渠道。
func IsUpstreamExhaustedError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if IsLocalFacingError(err) {
		return false
	}
	if ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}
	message := strings.ToLower(err.Error())
	if message == "" {
		return false
	}
	for _, keyword := range AutomaticDisableKeywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}
