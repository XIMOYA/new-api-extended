// setting/narrafork_setting/user_override.go
// 计算 NarraFork 用户端可配置字段与全局硬上限。
package narrafork_setting

const (
	UserOverrideFieldEnabled                    = "enabled"
	UserOverrideFieldActivationMode             = "activation_mode"
	UserOverrideFieldBalanceSource              = "balance_source"
	UserOverrideFieldIncludeDetailed            = "include_detailed"
	UserOverrideFieldDuplicatePolicy            = "duplicate_policy"
	UserOverrideFieldExposeExtra                = "expose_extra"
	UserOverrideFieldTokenDisplayMode           = "token_display_mode"
	UserOverrideFieldCacheHitRateScope          = "cache_hit_rate_scope"
	UserOverrideFieldCacheHitRateDays           = "cache_hit_rate_days"
	UserOverrideFieldShowBalance                = "show_balance"
	UserOverrideFieldShowRequestQuota           = "show_request_quota"
	UserOverrideFieldShowTodayQuota             = "show_today_quota"
	UserOverrideFieldShowTodayTokens            = "show_today_tokens"
	UserOverrideFieldShowMonthQuota             = "show_month_quota"
	UserOverrideFieldShowMonthTokens            = "show_month_tokens"
	UserOverrideFieldShowTotalQuota             = "show_total_quota"
	UserOverrideFieldShowUsedQuota              = "show_used_quota"
	UserOverrideFieldShowInputTokens            = "show_input_tokens"
	UserOverrideFieldShowOutputTokens           = "show_output_tokens"
	UserOverrideFieldShowTotalTokens            = "show_total_tokens"
	UserOverrideFieldShowCacheHitTokens         = "show_cache_hit_tokens"
	UserOverrideFieldShowCacheHitRate           = "show_cache_hit_rate"
	UserOverrideFieldShowReasoningTokens        = "show_reasoning_tokens"
	UserOverrideFieldShowLatency                = "show_latency"
	UserOverrideFieldShowTTFT                   = "show_ttft"
	UserOverrideFieldShowRequestID              = "show_request_id"
	UserOverrideFieldShowRetryCount             = "show_retry_count"
	UserOverrideFieldShowModel                  = "show_model"
	UserOverrideFieldShowBillingSource          = "show_billing_source"
	UserOverrideFieldShowUnavailableFields      = "show_unavailable_fields"
	UserOverrideFieldDetailTemplate             = "detail_template"
	UserOverrideFieldCustomQuotaBalance         = "custom_quota_balance"
	UserOverrideFieldCustomDetailedQuotaBalance = "custom_detailed_quota_balance"
)

type NarraForkUserOverrideCapabilities struct {
	Visible             bool            `json:"visible"`
	AllowedFields       map[string]bool `json:"allowed_fields"`
	GlobalCaps          map[string]bool `json:"global_caps"`
	GlobalValues        map[string]any  `json:"global_values"`
	ActivationModes     []string        `json:"activation_modes"`
	BalanceSources      []string        `json:"balance_sources"`
	CacheHitRateScopes  []string        `json:"cache_hit_rate_scopes"`
	MaxCacheHitRateDays int             `json:"max_cache_hit_rate_days"`
}

func (c NarraForkUserOverrideCapabilities) Allows(field string) bool {
	return c.AllowedFields[field]
}

func BuildNarraForkUserOverrideCapabilities(global NarraForkSetting) NarraForkUserOverrideCapabilities {
	capabilities := NarraForkUserOverrideCapabilities{
		AllowedFields:       map[string]bool{},
		GlobalCaps:          map[string]bool{},
		GlobalValues:        map[string]any{},
		ActivationModes:     []string{},
		BalanceSources:      []string{},
		CacheHitRateScopes:  []string{},
		MaxCacheHitRateDays: NormalizeCacheHitRateDays(global.CacheHitRateDays),
	}

	for _, field := range []string{
		UserOverrideFieldEnabled,
		UserOverrideFieldActivationMode,
		UserOverrideFieldBalanceSource,
		UserOverrideFieldIncludeDetailed,
		UserOverrideFieldDuplicatePolicy,
		UserOverrideFieldExposeExtra,
		UserOverrideFieldTokenDisplayMode,
		UserOverrideFieldCacheHitRateScope,
		UserOverrideFieldCacheHitRateDays,
		UserOverrideFieldShowBalance,
		UserOverrideFieldShowRequestQuota,
		UserOverrideFieldShowTodayQuota,
		UserOverrideFieldShowTodayTokens,
		UserOverrideFieldShowMonthQuota,
		UserOverrideFieldShowMonthTokens,
		UserOverrideFieldShowTotalQuota,
		UserOverrideFieldShowUsedQuota,
		UserOverrideFieldShowInputTokens,
		UserOverrideFieldShowOutputTokens,
		UserOverrideFieldShowTotalTokens,
		UserOverrideFieldShowCacheHitTokens,
		UserOverrideFieldShowCacheHitRate,
		UserOverrideFieldShowReasoningTokens,
		UserOverrideFieldShowLatency,
		UserOverrideFieldShowTTFT,
		UserOverrideFieldShowRequestID,
		UserOverrideFieldShowRetryCount,
		UserOverrideFieldShowModel,
		UserOverrideFieldShowBillingSource,
		UserOverrideFieldShowUnavailableFields,
		UserOverrideFieldDetailTemplate,
		UserOverrideFieldCustomQuotaBalance,
		UserOverrideFieldCustomDetailedQuotaBalance,
	} {
		capabilities.AllowedFields[field] = false
	}

	capabilities.GlobalCaps[UserOverrideFieldEnabled] = global.Enabled
	capabilities.GlobalCaps[UserOverrideFieldIncludeDetailed] = global.Enabled && global.IncludeDetailed
	capabilities.GlobalCaps[UserOverrideFieldExposeExtra] = global.Enabled && global.ExposeExtra
	capabilities.GlobalCaps[UserOverrideFieldShowBalance] = global.Enabled && global.IncludeDetailed && global.ShowBalance
	capabilities.GlobalCaps[UserOverrideFieldShowRequestQuota] = global.Enabled && global.IncludeDetailed && global.ShowRequestQuota
	capabilities.GlobalCaps[UserOverrideFieldShowTodayQuota] = global.Enabled && global.IncludeDetailed && global.ShowTodayQuota
	capabilities.GlobalCaps[UserOverrideFieldShowTodayTokens] = global.Enabled && global.IncludeDetailed && global.ShowTodayTokens
	capabilities.GlobalCaps[UserOverrideFieldShowMonthQuota] = global.Enabled && global.IncludeDetailed && global.ShowMonthQuota
	capabilities.GlobalCaps[UserOverrideFieldShowMonthTokens] = global.Enabled && global.IncludeDetailed && global.ShowMonthTokens
	capabilities.GlobalCaps[UserOverrideFieldShowTotalQuota] = global.Enabled && global.IncludeDetailed && global.ShowTotalQuota
	capabilities.GlobalCaps[UserOverrideFieldShowUsedQuota] = global.Enabled && global.IncludeDetailed && global.ShowUsedQuota
	capabilities.GlobalCaps[UserOverrideFieldShowInputTokens] = global.Enabled && global.IncludeDetailed && global.ShowInputTokens
	capabilities.GlobalCaps[UserOverrideFieldShowOutputTokens] = global.Enabled && global.IncludeDetailed && global.ShowOutputTokens
	capabilities.GlobalCaps[UserOverrideFieldShowTotalTokens] = global.Enabled && global.IncludeDetailed && global.ShowTotalTokens
	capabilities.GlobalCaps[UserOverrideFieldShowCacheHitTokens] = global.Enabled && global.IncludeDetailed && global.ShowCacheHitTokens
	capabilities.GlobalCaps[UserOverrideFieldShowCacheHitRate] = global.Enabled && global.IncludeDetailed && global.ShowCacheHitRate
	capabilities.GlobalCaps[UserOverrideFieldShowReasoningTokens] = global.Enabled && global.IncludeDetailed && global.ShowReasoningTokens
	capabilities.GlobalCaps[UserOverrideFieldShowLatency] = global.Enabled && global.IncludeDetailed && global.ShowLatency
	capabilities.GlobalCaps[UserOverrideFieldShowTTFT] = global.Enabled && global.IncludeDetailed && global.ShowTTFT
	capabilities.GlobalCaps[UserOverrideFieldShowRequestID] = global.Enabled && global.IncludeDetailed && global.ShowRequestID
	capabilities.GlobalCaps[UserOverrideFieldShowRetryCount] = global.Enabled && global.IncludeDetailed && global.ShowRetryCount
	capabilities.GlobalCaps[UserOverrideFieldShowModel] = global.Enabled && global.IncludeDetailed && global.ShowModel
	capabilities.GlobalCaps[UserOverrideFieldShowBillingSource] = global.Enabled && global.IncludeDetailed && global.ShowBillingSource
	capabilities.GlobalCaps[UserOverrideFieldShowUnavailableFields] = global.Enabled && global.IncludeDetailed && global.ShowUnavailableFields
	capabilities.GlobalCaps[UserOverrideFieldDetailTemplate] = global.Enabled && global.IncludeDetailed
	capabilities.GlobalCaps[UserOverrideFieldCustomQuotaBalance] = global.Enabled && global.BalanceSource == BalanceSourceCustom
	capabilities.GlobalCaps[UserOverrideFieldCustomDetailedQuotaBalance] = global.Enabled && global.IncludeDetailed && global.BalanceSource == BalanceSourceCustom
	capabilities.GlobalValues[UserOverrideFieldEnabled] = global.Enabled
	capabilities.GlobalValues[UserOverrideFieldActivationMode] = global.ActivationMode
	capabilities.GlobalValues[UserOverrideFieldBalanceSource] = global.BalanceSource
	capabilities.GlobalValues[UserOverrideFieldIncludeDetailed] = global.IncludeDetailed
	capabilities.GlobalValues[UserOverrideFieldDuplicatePolicy] = global.DuplicatePolicy
	capabilities.GlobalValues[UserOverrideFieldExposeExtra] = global.ExposeExtra
	capabilities.GlobalValues[UserOverrideFieldTokenDisplayMode] = global.TokenDisplayMode
	capabilities.GlobalValues[UserOverrideFieldCacheHitRateScope] = global.CacheHitRateScope
	capabilities.GlobalValues[UserOverrideFieldCacheHitRateDays] = global.CacheHitRateDays

	if !global.Enabled || !global.AllowUserDisplayOverride {
		return capabilities
	}

	capabilities.AllowedFields[UserOverrideFieldEnabled] = capabilities.GlobalCaps[UserOverrideFieldEnabled] &&
		(global.AllowUserEnabled || global.AllowUserDisplayOverride)
	capabilities.AllowedFields[UserOverrideFieldActivationMode] = global.AllowUserActivationMode && global.Enabled
	capabilities.AllowedFields[UserOverrideFieldBalanceSource] = global.AllowUserBalanceSource && global.Enabled
	capabilities.AllowedFields[UserOverrideFieldIncludeDetailed] = global.AllowUserIncludeDetailed && capabilities.GlobalCaps[UserOverrideFieldIncludeDetailed]
	capabilities.AllowedFields[UserOverrideFieldDuplicatePolicy] = global.AllowUserDuplicatePolicy && global.Enabled
	capabilities.AllowedFields[UserOverrideFieldExposeExtra] = global.AllowUserExposeExtra && capabilities.GlobalCaps[UserOverrideFieldExposeExtra]
	capabilities.AllowedFields[UserOverrideFieldTokenDisplayMode] = global.AllowUserTokenDisplayMode && global.Enabled
	capabilities.AllowedFields[UserOverrideFieldCacheHitRateScope] = global.AllowUserCacheHitRateScope && global.Enabled
	capabilities.AllowedFields[UserOverrideFieldCacheHitRateDays] = global.AllowUserCacheHitRateDays && global.Enabled && global.CacheHitRateScope == CacheHitRateScopeRecentDays

	capabilities.AllowedFields[UserOverrideFieldShowBalance] = global.AllowUserShowBalance && capabilities.GlobalCaps[UserOverrideFieldShowBalance]
	capabilities.AllowedFields[UserOverrideFieldShowRequestQuota] = global.AllowUserShowRequestQuota && capabilities.GlobalCaps[UserOverrideFieldShowRequestQuota]
	capabilities.AllowedFields[UserOverrideFieldShowTodayQuota] = global.AllowUserShowTodayQuota && capabilities.GlobalCaps[UserOverrideFieldShowTodayQuota]
	capabilities.AllowedFields[UserOverrideFieldShowTodayTokens] = global.AllowUserShowTodayTokens && capabilities.GlobalCaps[UserOverrideFieldShowTodayTokens]
	capabilities.AllowedFields[UserOverrideFieldShowMonthQuota] = global.AllowUserShowMonthQuota && capabilities.GlobalCaps[UserOverrideFieldShowMonthQuota]
	capabilities.AllowedFields[UserOverrideFieldShowMonthTokens] = global.AllowUserShowMonthTokens && capabilities.GlobalCaps[UserOverrideFieldShowMonthTokens]
	capabilities.AllowedFields[UserOverrideFieldShowTotalQuota] = global.AllowUserShowTotalQuota && capabilities.GlobalCaps[UserOverrideFieldShowTotalQuota]
	capabilities.AllowedFields[UserOverrideFieldShowUsedQuota] = global.AllowUserShowUsedQuota && capabilities.GlobalCaps[UserOverrideFieldShowUsedQuota]
	capabilities.AllowedFields[UserOverrideFieldShowInputTokens] = global.AllowUserShowInputTokens && capabilities.GlobalCaps[UserOverrideFieldShowInputTokens]
	capabilities.AllowedFields[UserOverrideFieldShowOutputTokens] = global.AllowUserShowOutputTokens && capabilities.GlobalCaps[UserOverrideFieldShowOutputTokens]
	capabilities.AllowedFields[UserOverrideFieldShowTotalTokens] = global.AllowUserShowTotalTokens && capabilities.GlobalCaps[UserOverrideFieldShowTotalTokens]
	capabilities.AllowedFields[UserOverrideFieldShowCacheHitTokens] = global.AllowUserShowCacheHitTokens && capabilities.GlobalCaps[UserOverrideFieldShowCacheHitTokens]
	capabilities.AllowedFields[UserOverrideFieldShowCacheHitRate] = global.AllowUserShowCacheHitRate && capabilities.GlobalCaps[UserOverrideFieldShowCacheHitRate]
	capabilities.AllowedFields[UserOverrideFieldShowReasoningTokens] = global.AllowUserShowReasoningTokens && capabilities.GlobalCaps[UserOverrideFieldShowReasoningTokens]
	capabilities.AllowedFields[UserOverrideFieldShowLatency] = global.AllowUserShowLatency && capabilities.GlobalCaps[UserOverrideFieldShowLatency]
	capabilities.AllowedFields[UserOverrideFieldShowTTFT] = global.AllowUserShowTTFT && capabilities.GlobalCaps[UserOverrideFieldShowTTFT]
	capabilities.AllowedFields[UserOverrideFieldShowRequestID] = global.AllowUserShowRequestID && capabilities.GlobalCaps[UserOverrideFieldShowRequestID]
	capabilities.AllowedFields[UserOverrideFieldShowRetryCount] = global.AllowUserShowRetryCount && capabilities.GlobalCaps[UserOverrideFieldShowRetryCount]
	capabilities.AllowedFields[UserOverrideFieldShowModel] = global.AllowUserShowModel && capabilities.GlobalCaps[UserOverrideFieldShowModel]
	capabilities.AllowedFields[UserOverrideFieldShowBillingSource] = global.AllowUserShowBillingSource && capabilities.GlobalCaps[UserOverrideFieldShowBillingSource]
	capabilities.AllowedFields[UserOverrideFieldShowUnavailableFields] = global.AllowUserShowUnavailableFields && capabilities.GlobalCaps[UserOverrideFieldShowUnavailableFields]
	capabilities.AllowedFields[UserOverrideFieldDetailTemplate] = global.AllowUserDetailTemplate && capabilities.GlobalCaps[UserOverrideFieldDetailTemplate]
	capabilities.AllowedFields[UserOverrideFieldCustomQuotaBalance] = global.AllowUserCustomQuotaBalance && capabilities.GlobalCaps[UserOverrideFieldCustomQuotaBalance]
	capabilities.AllowedFields[UserOverrideFieldCustomDetailedQuotaBalance] = global.AllowUserCustomDetailedQuotaBalance && capabilities.GlobalCaps[UserOverrideFieldCustomDetailedQuotaBalance]

	capabilities.ActivationModes = []string{
		ActivationModeNever,
		ActivationModeHeaderOnly,
		ActivationModeUserAgentOnly,
		ActivationModeHeaderOrUserAgent,
		ActivationModeAlways,
	}
	capabilities.BalanceSources = []string{
		BalanceSourceEffective,
		BalanceSourceUserQuota,
		BalanceSourceTokenQuota,
	}
	if global.BalanceSource == BalanceSourceCustom {
		capabilities.BalanceSources = append(capabilities.BalanceSources, BalanceSourceCustom)
	}
	capabilities.CacheHitRateScopes = []string{
		CacheHitRateScopeRequest,
		CacheHitRateScopeToday,
		CacheHitRateScopeRecentDays,
	}
	for _, allowed := range capabilities.AllowedFields {
		if allowed {
			capabilities.Visible = true
			break
		}
	}

	return capabilities
}
