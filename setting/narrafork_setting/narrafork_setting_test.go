// setting/narrafork_setting/narrafork_setting_test.go
// 验证 NarraFork 全局配置的安全默认值、枚举规范化与选项校验。
package narrafork_setting

import "testing"

func TestGetSettingsUsesSafeDefaults(t *testing.T) {
	settings := GetSettings()
	if settings.Enabled {
		t.Fatal("NarraFork quota events must be disabled by default")
	}
	if settings.ActivationMode != ActivationModeHeaderOrUserAgent {
		t.Fatalf("unexpected activation mode: %q", settings.ActivationMode)
	}
	if settings.BalanceSource != BalanceSourceEffective {
		t.Fatalf("unexpected balance source: %q", settings.BalanceSource)
	}
	if !settings.IncludeDetailed {
		t.Fatal("detailed quota balance should be enabled by default")
	}
	if settings.DuplicatePolicy != DuplicatePolicySkip {
		t.Fatalf("unexpected duplicate policy: %q", settings.DuplicatePolicy)
	}
	if settings.TokenDisplayMode != TokenDisplayModeExact {
		t.Fatalf("unexpected token display mode: %q", settings.TokenDisplayMode)
	}
	if settings.CacheHitRateScope != CacheHitRateScopeRequest {
		t.Fatalf("unexpected cache hit rate scope: %q", settings.CacheHitRateScope)
	}
	if settings.CacheHitRateDays != DefaultCacheHitRateDays {
		t.Fatalf("unexpected cache hit rate days: %d", settings.CacheHitRateDays)
	}
	for name, enabled := range map[string]bool{
		"show_balance":            settings.ShowBalance,
		"show_request_quota":      settings.ShowRequestQuota,
		"show_today_quota":        settings.ShowTodayQuota,
		"show_today_tokens":       settings.ShowTodayTokens,
		"show_month_quota":        settings.ShowMonthQuota,
		"show_month_tokens":       settings.ShowMonthTokens,
		"show_total_quota":        settings.ShowTotalQuota,
		"show_used_quota":         settings.ShowUsedQuota,
		"show_input_tokens":       settings.ShowInputTokens,
		"show_output_tokens":      settings.ShowOutputTokens,
		"show_total_tokens":       settings.ShowTotalTokens,
		"show_cache_hit_tokens":   settings.ShowCacheHitTokens,
		"show_cache_hit_rate":     settings.ShowCacheHitRate,
		"show_reasoning_tokens":   settings.ShowReasoningTokens,
		"show_model":              settings.ShowModel,
		"show_billing_source":     settings.ShowBillingSource,
		"show_unavailable_fields": settings.ShowUnavailableFields,
	} {
		if name == "show_unavailable_fields" && enabled {
			t.Fatalf("%s should be disabled by default", name)
		}
		if name != "show_unavailable_fields" && !enabled {
			t.Fatalf("%s should be enabled by default", name)
		}
	}
	if settings.ExposeExtra {
		t.Fatal("extra fields must be disabled by default")
	}
}

func TestNormalizeNarraforkSettingsFallsBackSafely(t *testing.T) {
	if got := NormalizeActivationMode("unknown"); got != ActivationModeHeaderOrUserAgent {
		t.Fatalf("unexpected activation fallback: %q", got)
	}
	if got := NormalizeBalanceSource("unknown"); got != BalanceSourceEffective {
		t.Fatalf("unexpected balance source fallback: %q", got)
	}
	if got := NormalizeDuplicatePolicy("unknown"); got != DuplicatePolicySkip {
		t.Fatalf("unexpected duplicate policy fallback: %q", got)
	}
	if got := NormalizeTokenDisplayMode("unknown"); got != TokenDisplayModeExact {
		t.Fatalf("unexpected token display mode fallback: %q", got)
	}
	if got := NormalizeCacheHitRateScope("unknown"); got != CacheHitRateScopeRequest {
		t.Fatalf("unexpected cache hit rate scope fallback: %q", got)
	}
	if got := NormalizeCacheHitRateDays(0); got != DefaultCacheHitRateDays {
		t.Fatalf("unexpected cache hit rate days fallback: %d", got)
	}
	if got := NormalizeCacheHitRateDays(MaxCacheHitRateDays + 1); got != DefaultCacheHitRateDays {
		t.Fatalf("unexpected cache hit rate max fallback: %d", got)
	}
}

func TestValidateOptionRejectsUnknownAndInvalidValues(t *testing.T) {
	valid := map[string]string{
		OptionPrefix + "enabled":                 "false",
		OptionPrefix + "activation_mode":         ActivationModeHeaderOnly,
		OptionPrefix + "balance_source":          BalanceSourceCustom,
		OptionPrefix + "duplicate_policy":        DuplicatePolicyReplace,
		OptionPrefix + "token_display_mode":      TokenDisplayModeCompact,
		OptionPrefix + "cache_hit_rate_scope":    CacheHitRateScopeRecentDays,
		OptionPrefix + "cache_hit_rate_days":     "14",
		OptionPrefix + "show_balance":            "false",
		OptionPrefix + "show_today_tokens":       "true",
		OptionPrefix + "show_reasoning_tokens":   "false",
		OptionPrefix + "show_unavailable_fields": "true",
		OptionPrefix + "custom_quota_balance":    "configured",
	}
	for key, value := range valid {
		if err := ValidateOption(key, value); err != nil {
			t.Errorf("ValidateOption(%q, %q) returned error: %v", key, value, err)
		}
	}

	invalid := map[string]string{
		OptionPrefix + "enabled":                 "not-a-bool",
		OptionPrefix + "activation_mode":         "unknown",
		OptionPrefix + "balance_source":          "unknown",
		OptionPrefix + "duplicate_policy":        "unknown",
		OptionPrefix + "cache_hit_rate_scope":    "unknown",
		OptionPrefix + "cache_hit_rate_days":     "31",
		OptionPrefix + "show_cache_hit_rate":     "not-a-bool",
		OptionPrefix + "show_unavailable_fields": "not-a-bool",
		OptionPrefix + "unknown":                 "value",
	}
	for key, value := range invalid {
		if err := ValidateOption(key, value); err == nil {
			t.Errorf("ValidateOption(%q, %q) accepted invalid value", key, value)
		}
	}
}
