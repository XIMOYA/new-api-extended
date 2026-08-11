// relay/common/narrafork_quota.go
// NarraFork 额度事件的请求激活、渠道覆盖合并与重复事件状态。
package common

import (
	"strings"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/gin-gonic/gin"
)

const NarraForkQuotaEventName = "quotaBalanceEvent"
const NarraForkQuotaEventHeader = "X-NarraFork-Quota-Event"

const (
	NarraForkActivationModeNever             = narrafork_setting.ActivationModeNever
	NarraForkActivationModeHeaderOnly        = narrafork_setting.ActivationModeHeaderOnly
	NarraForkActivationModeUserAgentOnly     = narrafork_setting.ActivationModeUserAgentOnly
	NarraForkActivationModeHeaderOrUserAgent = narrafork_setting.ActivationModeHeaderOrUserAgent
	NarraForkActivationModeAlways            = narrafork_setting.ActivationModeAlways
)

type NarraForkQuotaEventConfig struct {
	Enabled                    bool
	ActivationMode             string
	BalanceSource              string
	IncludeDetailed            bool
	DuplicatePolicy            string
	ExposeExtra                bool
	TokenDisplayMode           string
	CacheHitRateScope          string
	CacheHitRateDays           int
	ShowBalance                bool
	ShowRequestQuota           bool
	ShowTodayQuota             bool
	ShowTodayTokens            bool
	ShowMonthQuota             bool
	ShowMonthTokens            bool
	ShowTotalQuota             bool
	ShowUsedQuota              bool
	ShowInputTokens            bool
	ShowOutputTokens           bool
	ShowTotalTokens            bool
	ShowCacheHitTokens         bool
	ShowCacheHitRate           bool
	ShowReasoningTokens        bool
	ShowLatency                bool
	ShowTTFT                   bool
	ShowRequestID              bool
	ShowRetryCount             bool
	ShowModel                  bool
	ShowBillingSource          bool
	ShowUnavailableFields      bool
	DetailTemplate             string
	CustomQuotaBalance         string
	CustomDetailedQuotaBalance string
	Activated                  bool
	UpstreamEventSeen          bool
	EventSent                  bool
}

func NewNarraForkQuotaEventConfig(global narrafork_setting.NarraForkSetting) NarraForkQuotaEventConfig {
	return NarraForkQuotaEventConfig{
		Enabled:                    global.Enabled,
		ActivationMode:             global.ActivationMode,
		BalanceSource:              global.BalanceSource,
		IncludeDetailed:            global.IncludeDetailed,
		DuplicatePolicy:            global.DuplicatePolicy,
		ExposeExtra:                global.ExposeExtra,
		TokenDisplayMode:           global.TokenDisplayMode,
		CacheHitRateScope:          global.CacheHitRateScope,
		CacheHitRateDays:           global.CacheHitRateDays,
		ShowBalance:                global.ShowBalance,
		ShowRequestQuota:           global.ShowRequestQuota,
		ShowTodayQuota:             global.ShowTodayQuota,
		ShowTodayTokens:            global.ShowTodayTokens,
		ShowMonthQuota:             global.ShowMonthQuota,
		ShowMonthTokens:            global.ShowMonthTokens,
		ShowTotalQuota:             global.ShowTotalQuota,
		ShowUsedQuota:              global.ShowUsedQuota,
		ShowInputTokens:            global.ShowInputTokens,
		ShowOutputTokens:           global.ShowOutputTokens,
		ShowTotalTokens:            global.ShowTotalTokens,
		ShowCacheHitTokens:         global.ShowCacheHitTokens,
		ShowCacheHitRate:           global.ShowCacheHitRate,
		ShowReasoningTokens:        global.ShowReasoningTokens,
		ShowLatency:                global.ShowLatency,
		ShowTTFT:                   global.ShowTTFT,
		ShowRequestID:              global.ShowRequestID,
		ShowRetryCount:             global.ShowRetryCount,
		ShowModel:                  global.ShowModel,
		ShowBillingSource:          global.ShowBillingSource,
		ShowUnavailableFields:      global.ShowUnavailableFields,
		DetailTemplate:             global.DetailTemplate,
		CustomQuotaBalance:         global.CustomQuotaBalance,
		CustomDetailedQuotaBalance: global.CustomDetailedQuotaBalance,
	}
}

func ResolveNarraForkQuotaEvent(c *gin.Context, info *RelayInfo) NarraForkQuotaEventConfig {
	global := narrafork_setting.GetSettings()
	resolved := NewNarraForkQuotaEventConfig(global)
	if info != nil {
		ApplyNarraForkUserDisplayPreference(
			&resolved,
			info.UserSetting.NarraForkDisplayMode,
			global.AllowUserDisplayOverride,
		)
	}

	if overrides, ok := appcommon.GetContextKeyType[narrafork_setting.NarraForkPolicyOverrides](c, constant.ContextKeyNarraForkPolicyOverrides); ok {
		ApplyNarraForkPolicyPatch(&resolved, overrides.Group)
		ApplyNarraForkPolicyPatch(&resolved, overrides.User)
	}
	if info != nil && info.ChannelMeta != nil {
		ApplyNarraForkChannelSettings(&resolved, info.ChannelOtherSettings.NarraFork)
	}

	resolved.Activated = resolved.Enabled &&
		info != nil &&
		info.IsStream &&
		SupportsNarraForkQuotaEventFormat(info.RelayFormat) &&
		matchesNarraForkActivation(c, resolved.ActivationMode)
	return resolved
}

func ApplyNarraForkUserDisplayPreference(config *NarraForkQuotaEventConfig, mode string, allowed bool) {
	if !allowed || config == nil || narrafork_setting.NormalizeUserDisplayMode(mode) != narrafork_setting.UserDisplayModeHide {
		return
	}
	config.Enabled = false
}

func ApplyNarraForkChannelSettings(config *NarraForkQuotaEventConfig, settings *dto.NarraForkQuotaEventSettings) {
	if settings == nil {
		return
	}
	patch := &narrafork_setting.NarraForkPolicyPatch{
		Enabled:                    settings.Enabled,
		ActivationMode:             settings.ActivationMode,
		BalanceSource:              settings.BalanceSource,
		IncludeDetailed:            settings.IncludeDetailed,
		DuplicatePolicy:            settings.DuplicatePolicy,
		ExposeExtra:                settings.ExposeExtra,
		TokenDisplayMode:           settings.TokenDisplayMode,
		CacheHitRateScope:          settings.CacheHitRateScope,
		CacheHitRateDays:           settings.CacheHitRateDays,
		ShowBalance:                settings.ShowBalance,
		ShowRequestQuota:           settings.ShowRequestQuota,
		ShowTodayQuota:             settings.ShowTodayQuota,
		ShowTodayTokens:            settings.ShowTodayTokens,
		ShowMonthQuota:             settings.ShowMonthQuota,
		ShowMonthTokens:            settings.ShowMonthTokens,
		ShowTotalQuota:             settings.ShowTotalQuota,
		ShowUsedQuota:              settings.ShowUsedQuota,
		ShowInputTokens:            settings.ShowInputTokens,
		ShowOutputTokens:           settings.ShowOutputTokens,
		ShowTotalTokens:            settings.ShowTotalTokens,
		ShowCacheHitTokens:         settings.ShowCacheHitTokens,
		ShowCacheHitRate:           settings.ShowCacheHitRate,
		ShowReasoningTokens:        settings.ShowReasoningTokens,
		ShowLatency:                settings.ShowLatency,
		ShowTTFT:                   settings.ShowTTFT,
		ShowRequestID:              settings.ShowRequestID,
		ShowRetryCount:             settings.ShowRetryCount,
		ShowModel:                  settings.ShowModel,
		ShowBillingSource:          settings.ShowBillingSource,
		ShowUnavailableFields:      settings.ShowUnavailableFields,
		DetailTemplate:             settings.DetailTemplate,
		CustomQuotaBalance:         settings.CustomQuotaBalance,
		CustomDetailedQuotaBalance: settings.CustomDetailedQuotaBalance,
	}
	ApplyNarraForkPolicyPatch(config, patch)
}

func ApplyNarraForkPolicyPatch(config *NarraForkQuotaEventConfig, patch *narrafork_setting.NarraForkPolicyPatch) {
	if config == nil || patch == nil {
		return
	}
	if patch.Enabled != nil {
		config.Enabled = *patch.Enabled
	}
	if mode := strings.ToLower(strings.TrimSpace(patch.ActivationMode)); isValidActivationMode(mode) {
		config.ActivationMode = mode
	}
	if source := strings.ToLower(strings.TrimSpace(patch.BalanceSource)); isValidBalanceSource(source) {
		config.BalanceSource = source
	}
	if patch.IncludeDetailed != nil {
		config.IncludeDetailed = *patch.IncludeDetailed
	}
	if policy := strings.ToLower(strings.TrimSpace(patch.DuplicatePolicy)); isValidDuplicatePolicy(policy) {
		config.DuplicatePolicy = policy
	}
	if patch.ExposeExtra != nil {
		config.ExposeExtra = *patch.ExposeExtra
	}
	if mode := strings.ToLower(strings.TrimSpace(patch.TokenDisplayMode)); isValidTokenDisplayMode(mode) {
		config.TokenDisplayMode = mode
	}
	if scope := strings.ToLower(strings.TrimSpace(patch.CacheHitRateScope)); isValidCacheHitRateScope(scope) {
		config.CacheHitRateScope = scope
	}
	if patch.CacheHitRateDays != nil && isValidCacheHitRateDays(*patch.CacheHitRateDays) {
		config.CacheHitRateDays = *patch.CacheHitRateDays
	}
	if patch.ShowBalance != nil {
		config.ShowBalance = *patch.ShowBalance
	}
	if patch.ShowRequestQuota != nil {
		config.ShowRequestQuota = *patch.ShowRequestQuota
	}
	if patch.ShowTodayQuota != nil {
		config.ShowTodayQuota = *patch.ShowTodayQuota
	}
	if patch.ShowTodayTokens != nil {
		config.ShowTodayTokens = *patch.ShowTodayTokens
	}
	if patch.ShowMonthQuota != nil {
		config.ShowMonthQuota = *patch.ShowMonthQuota
	}
	if patch.ShowMonthTokens != nil {
		config.ShowMonthTokens = *patch.ShowMonthTokens
	}
	if patch.ShowTotalQuota != nil {
		config.ShowTotalQuota = *patch.ShowTotalQuota
	}
	if patch.ShowUsedQuota != nil {
		config.ShowUsedQuota = *patch.ShowUsedQuota
	}
	if patch.ShowInputTokens != nil {
		config.ShowInputTokens = *patch.ShowInputTokens
	}
	if patch.ShowOutputTokens != nil {
		config.ShowOutputTokens = *patch.ShowOutputTokens
	}
	if patch.ShowTotalTokens != nil {
		config.ShowTotalTokens = *patch.ShowTotalTokens
	}
	if patch.ShowCacheHitTokens != nil {
		config.ShowCacheHitTokens = *patch.ShowCacheHitTokens
	}
	if patch.ShowCacheHitRate != nil {
		config.ShowCacheHitRate = *patch.ShowCacheHitRate
	}
	if patch.ShowReasoningTokens != nil {
		config.ShowReasoningTokens = *patch.ShowReasoningTokens
	}
	if patch.ShowLatency != nil {
		config.ShowLatency = *patch.ShowLatency
	}
	if patch.ShowTTFT != nil {
		config.ShowTTFT = *patch.ShowTTFT
	}
	if patch.ShowRequestID != nil {
		config.ShowRequestID = *patch.ShowRequestID
	}
	if patch.ShowRetryCount != nil {
		config.ShowRetryCount = *patch.ShowRetryCount
	}
	if patch.ShowModel != nil {
		config.ShowModel = *patch.ShowModel
	}
	if patch.ShowBillingSource != nil {
		config.ShowBillingSource = *patch.ShowBillingSource
	}
	if patch.ShowUnavailableFields != nil {
		config.ShowUnavailableFields = *patch.ShowUnavailableFields
	}
	if patch.DetailTemplate != nil {
		config.DetailTemplate = *patch.DetailTemplate
	}
	if patch.CustomQuotaBalance != nil && strings.TrimSpace(*patch.CustomQuotaBalance) != "" {
		config.CustomQuotaBalance = *patch.CustomQuotaBalance
	}
	if patch.CustomDetailedQuotaBalance != nil && strings.TrimSpace(*patch.CustomDetailedQuotaBalance) != "" {
		config.CustomDetailedQuotaBalance = *patch.CustomDetailedQuotaBalance
	}
}

func isValidTokenDisplayMode(value string) bool {
	return value == narrafork_setting.TokenDisplayModeExact || value == narrafork_setting.TokenDisplayModeCompact
}

func isValidCacheHitRateScope(value string) bool {
	return value == narrafork_setting.CacheHitRateScopeRequest ||
		value == narrafork_setting.CacheHitRateScopeToday ||
		value == narrafork_setting.CacheHitRateScopeRecentDays
}

func isValidCacheHitRateDays(value int) bool {
	return value >= narrafork_setting.MinCacheHitRateDays && value <= narrafork_setting.MaxCacheHitRateDays
}

func SupportsNarraForkQuotaEventFormat(format types.RelayFormat) bool {
	switch format {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatClaude:
		return true
	default:
		return false
	}
}

func (config *NarraForkQuotaEventConfig) ObserveUpstreamEvent() bool {
	if config == nil {
		return false
	}
	config.UpstreamEventSeen = true
	if !config.Activated {
		return false
	}
	return config.DuplicatePolicy != narrafork_setting.DuplicatePolicyReplace
}

func (config *NarraForkQuotaEventConfig) ShouldSendLocalEvent() bool {
	if config == nil || !config.Activated || config.EventSent {
		return false
	}
	if config.DuplicatePolicy == narrafork_setting.DuplicatePolicySkip && config.UpstreamEventSeen {
		return false
	}
	return true
}

// CanWriteNarraForkQuotaEvent prevents a late balance event after a stream error,
// timeout, client disconnect, or panic. A terminal protocol event can still be
// injected while the scanner is processing it, when EndReason is still None.
func CanWriteNarraForkQuotaEvent(info *RelayInfo) bool {
	if info == nil || !info.NarraForkQuotaEvent.ShouldSendLocalEvent() {
		return false
	}
	if info.StreamStatus == nil {
		return true
	}
	switch info.StreamStatus.EndReason {
	case StreamEndReasonNone, StreamEndReasonDone, StreamEndReasonEOF:
		return true
	default:
		return false
	}
}

func (config *NarraForkQuotaEventConfig) MarkEventSent() {
	if config != nil {
		config.EventSent = true
	}
}

func NarraForkActivationMatches(c *gin.Context) (headerMatched bool, userAgentMatched bool) {
	if c == nil || c.Request == nil {
		return false, false
	}
	return strings.TrimSpace(c.GetHeader(NarraForkQuotaEventHeader)) == "true",
		strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "narrafork")
}

func matchesNarraForkActivation(c *gin.Context, mode string) bool {
	mode = narrafork_setting.NormalizeActivationMode(mode)
	if mode == narrafork_setting.ActivationModeNever {
		return false
	}
	if mode == narrafork_setting.ActivationModeAlways {
		return true
	}

	headerMatched, userAgentMatched := NarraForkActivationMatches(c)
	switch mode {
	case narrafork_setting.ActivationModeHeaderOnly:
		return headerMatched
	case narrafork_setting.ActivationModeUserAgentOnly:
		return userAgentMatched
	case narrafork_setting.ActivationModeHeaderOrUserAgent:
		return headerMatched || userAgentMatched
	default:
		return false
	}
}

func isValidActivationMode(value string) bool {
	switch value {
	case narrafork_setting.ActivationModeNever,
		narrafork_setting.ActivationModeHeaderOnly,
		narrafork_setting.ActivationModeUserAgentOnly,
		narrafork_setting.ActivationModeHeaderOrUserAgent,
		narrafork_setting.ActivationModeAlways:
		return true
	default:
		return false
	}
}

func isValidBalanceSource(value string) bool {
	switch value {
	case narrafork_setting.BalanceSourceEffective,
		narrafork_setting.BalanceSourceUserQuota,
		narrafork_setting.BalanceSourceTokenQuota,
		narrafork_setting.BalanceSourceCustom:
		return true
	default:
		return false
	}
}

func isValidDuplicatePolicy(value string) bool {
	switch value {
	case narrafork_setting.DuplicatePolicySkip,
		narrafork_setting.DuplicatePolicyReplace,
		narrafork_setting.DuplicatePolicyAlways:
		return true
	default:
		return false
	}
}
