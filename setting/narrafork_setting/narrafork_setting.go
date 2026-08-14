// setting/narrafork_setting/narrafork_setting.go
// NarraFork 额度事件的全局配置、默认值与配置项校验。
package narrafork_setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const OptionPrefix = "narrafork_setting."

const (
	ActivationModeNever             = "never"
	ActivationModeHeaderOnly        = "header_only"
	ActivationModeUserAgentOnly     = "user_agent_only"
	ActivationModeHeaderOrUserAgent = "header_or_user_agent"
	ActivationModeAlways            = "always"

	BalanceSourceEffective  = "effective"
	BalanceSourceUserQuota  = "user_quota"
	BalanceSourceTokenQuota = "token_quota"
	BalanceSourceCustom     = "custom"

	DuplicatePolicySkip    = "skip"
	DuplicatePolicyReplace = "replace"
	DuplicatePolicyAlways  = "always"

	TokenDisplayModeExact   = "exact"
	TokenDisplayModeCompact = "compact"

	UserDisplayModeInherit = "inherit"
	UserDisplayModeShow    = "show"
	UserDisplayModeHide    = "hide"

	CacheHitRateScopeRequest    = "request"
	CacheHitRateScopeToday      = "today"
	CacheHitRateScopeRecentDays = "recent_days"
	DefaultCacheHitRateDays     = 7
	MinCacheHitRateDays         = 1
	MaxCacheHitRateDays         = 30
)

type NarraForkSetting struct {
	Enabled                             bool   `json:"enabled"`
	ActivationMode                      string `json:"activation_mode"`
	BalanceSource                       string `json:"balance_source"`
	IncludeDetailed                     bool   `json:"include_detailed"`
	DuplicatePolicy                     string `json:"duplicate_policy"`
	ExposeExtra                         bool   `json:"expose_extra"`
	TokenDisplayMode                    string `json:"token_display_mode"`
	AllowUserDisplayOverride            bool   `json:"allow_user_display_override"`
	AllowUserEnabled                    bool   `json:"allow_user_enabled"`
	AllowUserActivationMode             bool   `json:"allow_user_activation_mode"`
	AllowUserBalanceSource              bool   `json:"allow_user_balance_source"`
	AllowUserIncludeDetailed            bool   `json:"allow_user_include_detailed"`
	AllowUserDuplicatePolicy            bool   `json:"allow_user_duplicate_policy"`
	AllowUserExposeExtra                bool   `json:"allow_user_expose_extra"`
	AllowUserTokenDisplayMode           bool   `json:"allow_user_token_display_mode"`
	AllowUserCacheHitRateScope          bool   `json:"allow_user_cache_hit_rate_scope"`
	AllowUserCacheHitRateDays           bool   `json:"allow_user_cache_hit_rate_days"`
	CacheHitRateScope                   string `json:"cache_hit_rate_scope"`
	CacheHitRateDays                    int    `json:"cache_hit_rate_days"`
	AllowUserShowBalance                bool   `json:"allow_user_show_balance"`
	AllowUserShowRequestQuota           bool   `json:"allow_user_show_request_quota"`
	AllowUserShowTodayQuota             bool   `json:"allow_user_show_today_quota"`
	AllowUserShowTodayTokens            bool   `json:"allow_user_show_today_tokens"`
	AllowUserShowMonthQuota             bool   `json:"allow_user_show_month_quota"`
	AllowUserShowMonthTokens            bool   `json:"allow_user_show_month_tokens"`
	AllowUserShowTotalQuota             bool   `json:"allow_user_show_total_quota"`
	AllowUserShowUsedQuota              bool   `json:"allow_user_show_used_quota"`
	AllowUserShowInputTokens            bool   `json:"allow_user_show_input_tokens"`
	AllowUserShowOutputTokens           bool   `json:"allow_user_show_output_tokens"`
	AllowUserShowTotalTokens            bool   `json:"allow_user_show_total_tokens"`
	AllowUserShowCacheHitTokens         bool   `json:"allow_user_show_cache_hit_tokens"`
	AllowUserShowCacheHitRate           bool   `json:"allow_user_show_cache_hit_rate"`
	AllowUserShowReasoningTokens        bool   `json:"allow_user_show_reasoning_tokens"`
	AllowUserShowLatency                bool   `json:"allow_user_show_latency"`
	AllowUserShowTTFT                   bool   `json:"allow_user_show_ttft"`
	AllowUserShowRequestID              bool   `json:"allow_user_show_request_id"`
	AllowUserShowRetryCount             bool   `json:"allow_user_show_retry_count"`
	AllowUserShowModel                  bool   `json:"allow_user_show_model"`
	AllowUserShowBillingSource          bool   `json:"allow_user_show_billing_source"`
	AllowUserShowUnavailableFields      bool   `json:"allow_user_show_unavailable_fields"`
	AllowUserDetailTemplate             bool   `json:"allow_user_detail_template"`
	AllowUserCustomQuotaBalance         bool   `json:"allow_user_custom_quota_balance"`
	AllowUserCustomDetailedQuotaBalance bool   `json:"allow_user_custom_detailed_quota_balance"`
	ShowBalance                         bool   `json:"show_balance"`
	ShowRequestQuota                    bool   `json:"show_request_quota"`
	ShowTodayQuota                      bool   `json:"show_today_quota"`
	ShowTodayTokens                     bool   `json:"show_today_tokens"`
	ShowMonthQuota                      bool   `json:"show_month_quota"`
	ShowMonthTokens                     bool   `json:"show_month_tokens"`
	ShowTotalQuota                      bool   `json:"show_total_quota"`
	ShowUsedQuota                       bool   `json:"show_used_quota"`
	ShowInputTokens                     bool   `json:"show_input_tokens"`
	ShowOutputTokens                    bool   `json:"show_output_tokens"`
	ShowTotalTokens                     bool   `json:"show_total_tokens"`
	ShowCacheHitTokens                  bool   `json:"show_cache_hit_tokens"`
	ShowCacheHitRate                    bool   `json:"show_cache_hit_rate"`
	ShowReasoningTokens                 bool   `json:"show_reasoning_tokens"`
	ShowLatency                         bool   `json:"show_latency"`
	ShowTTFT                            bool   `json:"show_ttft"`
	ShowRequestID                       bool   `json:"show_request_id"`
	ShowRetryCount                      bool   `json:"show_retry_count"`
	ShowModel                           bool   `json:"show_model"`
	ShowBillingSource                   bool   `json:"show_billing_source"`
	ShowUnavailableFields               bool   `json:"show_unavailable_fields"`
	DetailTemplate                      string `json:"detail_template"`
	CustomQuotaBalance                  string `json:"custom_quota_balance"`
	CustomDetailedQuotaBalance          string `json:"custom_detailed_quota_balance"`
}

var narraForkSetting = NarraForkSetting{
	Enabled:                    false,
	ActivationMode:             ActivationModeHeaderOrUserAgent,
	BalanceSource:              BalanceSourceEffective,
	IncludeDetailed:            true,
	DuplicatePolicy:            DuplicatePolicySkip,
	ExposeExtra:                false,
	TokenDisplayMode:           TokenDisplayModeExact,
	AllowUserDisplayOverride:   false,
	AllowUserEnabled:           false,
	AllowUserActivationMode:    false,
	AllowUserBalanceSource:     false,
	AllowUserIncludeDetailed:   false,
	AllowUserDuplicatePolicy:   false,
	AllowUserExposeExtra:       false,
	AllowUserTokenDisplayMode:  false,
	AllowUserCacheHitRateScope: false,
	AllowUserCacheHitRateDays:  false,
	CacheHitRateScope:          CacheHitRateScopeRequest,
	CacheHitRateDays:           DefaultCacheHitRateDays,
	ShowBalance:                true,
	ShowRequestQuota:           true,
	ShowTodayQuota:             true,
	ShowTodayTokens:            true,
	ShowMonthQuota:             true,
	ShowMonthTokens:            true,
	ShowTotalQuota:             true,
	ShowUsedQuota:              true,
	ShowInputTokens:            true,
	ShowOutputTokens:           true,
	ShowTotalTokens:            true,
	ShowCacheHitTokens:         true,
	ShowCacheHitRate:           true,
	ShowReasoningTokens:        true,
	ShowLatency:                true,
	ShowTTFT:                   true,
	ShowRequestID:              true,
	ShowRetryCount:             true,
	ShowModel:                  true,
	ShowBillingSource:          true,
	ShowUnavailableFields:      false,
	DetailTemplate:             "",
	CustomQuotaBalance:         "",
	CustomDetailedQuotaBalance: "",
}

func init() {
	config.GlobalConfig.Register("narrafork_setting", &narraForkSetting)
}

func GetSettings() NarraForkSetting {
	settings := narraForkSetting
	settings.ActivationMode = NormalizeActivationMode(settings.ActivationMode)
	settings.BalanceSource = NormalizeBalanceSource(settings.BalanceSource)
	settings.DuplicatePolicy = NormalizeDuplicatePolicy(settings.DuplicatePolicy)
	settings.TokenDisplayMode = NormalizeTokenDisplayMode(settings.TokenDisplayMode)
	settings.CacheHitRateScope = NormalizeCacheHitRateScope(settings.CacheHitRateScope)
	settings.CacheHitRateDays = NormalizeCacheHitRateDays(settings.CacheHitRateDays)
	return settings
}

func NormalizeActivationMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ActivationModeNever, ActivationModeHeaderOnly, ActivationModeUserAgentOnly,
		ActivationModeHeaderOrUserAgent, ActivationModeAlways:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ActivationModeHeaderOrUserAgent
	}
}

func NormalizeBalanceSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BalanceSourceEffective, BalanceSourceUserQuota, BalanceSourceTokenQuota, BalanceSourceCustom:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return BalanceSourceEffective
	}
}

func NormalizeDuplicatePolicy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DuplicatePolicySkip, DuplicatePolicyReplace, DuplicatePolicyAlways:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return DuplicatePolicySkip
	}
}

func NormalizeTokenDisplayMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TokenDisplayModeExact, TokenDisplayModeCompact:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return TokenDisplayModeExact
	}
}

func NormalizeUserDisplayMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UserDisplayModeInherit, UserDisplayModeShow, UserDisplayModeHide:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return UserDisplayModeInherit
	}
}

func IsValidUserDisplayMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UserDisplayModeInherit, UserDisplayModeShow, UserDisplayModeHide:
		return true
	default:
		return false
	}
}

func NormalizeCacheHitRateScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CacheHitRateScopeRequest, CacheHitRateScopeToday, CacheHitRateScopeRecentDays:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return CacheHitRateScopeRequest
	}
}

func NormalizeCacheHitRateDays(value int) int {
	if value < MinCacheHitRateDays || value > MaxCacheHitRateDays {
		return DefaultCacheHitRateDays
	}
	return value
}

func IsOptionKey(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), OptionPrefix)
}

func ValidateOption(key string, value string) error {
	if !IsOptionKey(key) {
		return nil
	}

	field := strings.TrimPrefix(strings.TrimSpace(key), OptionPrefix)
	switch field {
	case "enabled", "allow_user_display_override", "allow_user_enabled", "allow_user_activation_mode",
		"allow_user_balance_source", "allow_user_include_detailed", "allow_user_duplicate_policy",
		"allow_user_expose_extra", "allow_user_token_display_mode", "allow_user_cache_hit_rate_scope",
		"allow_user_cache_hit_rate_days", "allow_user_show_balance", "allow_user_show_request_quota",
		"allow_user_show_today_quota", "allow_user_show_today_tokens", "allow_user_show_month_quota",
		"allow_user_show_month_tokens", "allow_user_show_total_quota", "allow_user_show_used_quota",
		"allow_user_show_input_tokens", "allow_user_show_output_tokens", "allow_user_show_total_tokens",
		"allow_user_show_cache_hit_tokens", "allow_user_show_cache_hit_rate", "allow_user_show_reasoning_tokens",
		"allow_user_show_latency", "allow_user_show_ttft", "allow_user_show_request_id",
		"allow_user_show_retry_count", "allow_user_show_model", "allow_user_show_billing_source",
		"allow_user_show_unavailable_fields", "allow_user_detail_template", "allow_user_custom_quota_balance",
		"allow_user_custom_detailed_quota_balance", "include_detailed", "expose_extra",
		"show_balance", "show_request_quota", "show_today_quota", "show_today_tokens",
		"show_month_quota", "show_month_tokens", "show_total_quota", "show_used_quota",
		"show_input_tokens", "show_output_tokens", "show_total_tokens", "show_cache_hit_tokens",
		"show_cache_hit_rate", "show_reasoning_tokens", "show_latency", "show_ttft",
		"show_request_id", "show_retry_count", "show_model", "show_billing_source",
		"show_unavailable_fields":
		if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
	case "activation_mode":
		if !isActivationMode(value) {
			return fmt.Errorf("invalid NarraFork activation mode: %s", value)
		}
	case "balance_source":
		if !isBalanceSource(value) {
			return fmt.Errorf("invalid NarraFork balance source: %s", value)
		}
	case "duplicate_policy":
		if !isDuplicatePolicy(value) {
			return fmt.Errorf("invalid NarraFork duplicate policy: %s", value)
		}
	case "token_display_mode":
		if !isTokenDisplayMode(value) {
			return fmt.Errorf("invalid NarraFork token display mode: %s", value)
		}
	case "cache_hit_rate_scope":
		if !isCacheHitRateScope(value) {
			return fmt.Errorf("invalid NarraFork cache hit rate scope: %s", value)
		}
	case "cache_hit_rate_days":
		days, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || days < MinCacheHitRateDays || days > MaxCacheHitRateDays {
			return fmt.Errorf("%s must be between %d and %d", key, MinCacheHitRateDays, MaxCacheHitRateDays)
		}
	case "detail_template":
		if err := ValidateDetailTemplate(value); err != nil {
			return err
		}
	case "custom_quota_balance", "custom_detailed_quota_balance":
		if len(value) > 4096 {
			return fmt.Errorf("%s is too long", key)
		}
	default:
		return fmt.Errorf("unknown NarraFork setting: %s", key)
	}
	return nil
}

func isActivationMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ActivationModeNever, ActivationModeHeaderOnly, ActivationModeUserAgentOnly,
		ActivationModeHeaderOrUserAgent, ActivationModeAlways:
		return true
	default:
		return false
	}
}

func isBalanceSource(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BalanceSourceEffective, BalanceSourceUserQuota, BalanceSourceTokenQuota, BalanceSourceCustom:
		return true
	default:
		return false
	}
}

func isDuplicatePolicy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DuplicatePolicySkip, DuplicatePolicyReplace, DuplicatePolicyAlways:
		return true
	default:
		return false
	}
}

func isTokenDisplayMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TokenDisplayModeExact, TokenDisplayModeCompact:
		return true
	default:
		return false
	}
}

func isCacheHitRateScope(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CacheHitRateScopeRequest, CacheHitRateScopeToday, CacheHitRateScopeRecentDays:
		return true
	default:
		return false
	}
}
