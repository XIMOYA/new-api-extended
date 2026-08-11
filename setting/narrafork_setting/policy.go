// setting/narrafork_setting/policy.go
// NarraFork 额度事件的作用域策略补丁、模板占位符白名单与策略校验。
package narrafork_setting

import (
	"fmt"
	"strings"
)

const MaxDetailTemplateLength = 8192

type NarraForkPolicyPatch struct {
	Enabled                    *bool   `json:"enabled,omitempty"`
	ActivationMode             string  `json:"activation_mode,omitempty"`
	BalanceSource              string  `json:"balance_source,omitempty"`
	IncludeDetailed            *bool   `json:"include_detailed,omitempty"`
	DuplicatePolicy            string  `json:"duplicate_policy,omitempty"`
	ExposeExtra                *bool   `json:"expose_extra,omitempty"`
	TokenDisplayMode           string  `json:"token_display_mode,omitempty"`
	CacheHitRateScope          string  `json:"cache_hit_rate_scope,omitempty"`
	CacheHitRateDays           *int    `json:"cache_hit_rate_days,omitempty"`
	ShowBalance                *bool   `json:"show_balance,omitempty"`
	ShowRequestQuota           *bool   `json:"show_request_quota,omitempty"`
	ShowTodayQuota             *bool   `json:"show_today_quota,omitempty"`
	ShowTodayTokens            *bool   `json:"show_today_tokens,omitempty"`
	ShowMonthQuota             *bool   `json:"show_month_quota,omitempty"`
	ShowMonthTokens            *bool   `json:"show_month_tokens,omitempty"`
	ShowTotalQuota             *bool   `json:"show_total_quota,omitempty"`
	ShowUsedQuota              *bool   `json:"show_used_quota,omitempty"`
	ShowInputTokens            *bool   `json:"show_input_tokens,omitempty"`
	ShowOutputTokens           *bool   `json:"show_output_tokens,omitempty"`
	ShowTotalTokens            *bool   `json:"show_total_tokens,omitempty"`
	ShowCacheHitTokens         *bool   `json:"show_cache_hit_tokens,omitempty"`
	ShowCacheHitRate           *bool   `json:"show_cache_hit_rate,omitempty"`
	ShowReasoningTokens        *bool   `json:"show_reasoning_tokens,omitempty"`
	ShowLatency                *bool   `json:"show_latency,omitempty"`
	ShowTTFT                   *bool   `json:"show_ttft,omitempty"`
	ShowRequestID              *bool   `json:"show_request_id,omitempty"`
	ShowRetryCount             *bool   `json:"show_retry_count,omitempty"`
	ShowModel                  *bool   `json:"show_model,omitempty"`
	ShowBillingSource          *bool   `json:"show_billing_source,omitempty"`
	ShowUnavailableFields      *bool   `json:"show_unavailable_fields,omitempty"`
	DetailTemplate             *string `json:"detail_template,omitempty"`
	CustomQuotaBalance         *string `json:"custom_quota_balance,omitempty"`
	CustomDetailedQuotaBalance *string `json:"custom_detailed_quota_balance,omitempty"`
}

type NarraForkPolicyOverrides struct {
	Group *NarraForkPolicyPatch `json:"group,omitempty"`
	User  *NarraForkPolicyPatch `json:"user,omitempty"`
}

var detailTemplatePlaceholders = map[string]struct{}{
	"balance":               {},
	"request_quota":         {},
	"today_quota":           {},
	"today_tokens":          {},
	"month_quota":           {},
	"month_tokens":          {},
	"total_quota":           {},
	"used_quota":            {},
	"input_tokens":          {},
	"output_tokens":         {},
	"total_tokens":          {},
	"cache_hit_tokens":      {},
	"cache_hit_rate":        {},
	"cache_hit_rate_period": {},
	"reasoning_tokens":      {},
	"model":                 {},
	"billing_source":        {},
	"latency_ms":            {},
	"ttft_ms":               {},
	"request_id":            {},
	"retry_count":           {},
}

func ValidateDetailTemplate(value string) error {
	if len(value) > MaxDetailTemplateLength {
		return fmt.Errorf("detail_template is too long")
	}
	remaining := value
	for {
		start := strings.Index(remaining, "{{")
		end := strings.Index(remaining, "}}")
		if start < 0 {
			if end >= 0 {
				return fmt.Errorf("detail_template contains an unmatched closing delimiter")
			}
			return nil
		}
		if end < start+2 {
			return fmt.Errorf("detail_template contains an unmatched opening delimiter")
		}
		placeholder := strings.TrimSpace(remaining[start+2 : end])
		if _, ok := detailTemplatePlaceholders[placeholder]; !ok {
			return fmt.Errorf("unsupported detail_template placeholder: %s", placeholder)
		}
		remaining = remaining[end+2:]
	}
}

func (patch NarraForkPolicyPatch) Validate() error {
	if patch.ActivationMode != "" && !isActivationMode(patch.ActivationMode) {
		return fmt.Errorf("invalid NarraFork activation mode: %s", patch.ActivationMode)
	}
	if patch.BalanceSource != "" && !isBalanceSource(patch.BalanceSource) {
		return fmt.Errorf("invalid NarraFork balance source: %s", patch.BalanceSource)
	}
	if patch.DuplicatePolicy != "" && !isDuplicatePolicy(patch.DuplicatePolicy) {
		return fmt.Errorf("invalid NarraFork duplicate policy: %s", patch.DuplicatePolicy)
	}
	if patch.TokenDisplayMode != "" && !isTokenDisplayMode(patch.TokenDisplayMode) {
		return fmt.Errorf("invalid NarraFork token display mode: %s", patch.TokenDisplayMode)
	}
	if patch.CacheHitRateScope != "" && !isCacheHitRateScope(patch.CacheHitRateScope) {
		return fmt.Errorf("invalid NarraFork cache hit rate scope: %s", patch.CacheHitRateScope)
	}
	if patch.CacheHitRateDays != nil && (*patch.CacheHitRateDays < MinCacheHitRateDays || *patch.CacheHitRateDays > MaxCacheHitRateDays) {
		return fmt.Errorf("cache_hit_rate_days must be between %d and %d", MinCacheHitRateDays, MaxCacheHitRateDays)
	}
	if patch.DetailTemplate != nil {
		if err := ValidateDetailTemplate(*patch.DetailTemplate); err != nil {
			return err
		}
	}
	if patch.CustomQuotaBalance != nil && len(*patch.CustomQuotaBalance) > 4096 {
		return fmt.Errorf("custom_quota_balance is too long")
	}
	if patch.CustomDetailedQuotaBalance != nil && len(*patch.CustomDetailedQuotaBalance) > 4096 {
		return fmt.Errorf("custom_detailed_quota_balance is too long")
	}
	return nil
}
