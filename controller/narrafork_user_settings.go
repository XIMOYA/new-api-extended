// controller/narrafork_user_settings.go
// 用户端 NarraFork 覆盖设置的鉴权、校验与保存接口。
package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/gin-gonic/gin"
)

type UpdateNarraForkUserSettingsRequest struct {
	NarraFork *dto.NarraForkUserSettings `json:"narrafork"`
}

func UpdateNarraForkUserSettings(c *gin.Context) {
	var req UpdateNarraForkUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, fmt.Errorf("invalid NarraFork settings: %w", err))
		return
	}

	global := narrafork_setting.GetSettings()
	capabilities := narrafork_setting.BuildNarraForkUserOverrideCapabilities(global)
	if !capabilities.Visible {
		common.ApiError(c, fmt.Errorf("NarraFork user settings are not available"))
		return
	}
	if err := validateNarraForkUserSettings(req.NarraFork, capabilities); err != nil {
		common.ApiError(c, err)
		return
	}

	userID := c.GetInt("id")
	if err := model.UpdateUserNarraForkSettings(userID, req.NarraFork); err != nil {
		common.ApiError(c, err)
		return
	}

	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildSelfUserData(user))
}

func validateNarraForkUserSettings(
	settings *dto.NarraForkUserSettings,
	capabilities narrafork_setting.NarraForkUserOverrideCapabilities,
) error {
	if settings == nil {
		return nil
	}

	if settings.Enabled != nil && !capabilities.Allows(narrafork_setting.UserOverrideFieldEnabled) {
		return fmt.Errorf("NarraFork setting is not allowed: enabled")
	}
	if settings.ActivationMode != "" {
		if !capabilities.Allows(narrafork_setting.UserOverrideFieldActivationMode) {
			return fmt.Errorf("NarraFork setting is not allowed: activation_mode")
		}
		if !containsString(capabilities.ActivationModes, strings.ToLower(strings.TrimSpace(settings.ActivationMode))) {
			return fmt.Errorf("invalid NarraFork activation mode: %s", settings.ActivationMode)
		}
	}
	if settings.BalanceSource != "" {
		if !capabilities.Allows(narrafork_setting.UserOverrideFieldBalanceSource) {
			return fmt.Errorf("NarraFork setting is not allowed: balance_source")
		}
		if !containsString(capabilities.BalanceSources, strings.ToLower(strings.TrimSpace(settings.BalanceSource))) {
			return fmt.Errorf("invalid NarraFork balance source: %s", settings.BalanceSource)
		}
	}
	if settings.IncludeDetailed != nil && !capabilities.Allows(narrafork_setting.UserOverrideFieldIncludeDetailed) {
		return fmt.Errorf("NarraFork setting is not allowed: include_detailed")
	}
	if settings.DuplicatePolicy != "" && !capabilities.Allows(narrafork_setting.UserOverrideFieldDuplicatePolicy) {
		return fmt.Errorf("NarraFork setting is not allowed: duplicate_policy")
	}
	if settings.ExposeExtra != nil && !capabilities.Allows(narrafork_setting.UserOverrideFieldExposeExtra) {
		return fmt.Errorf("NarraFork setting is not allowed: expose_extra")
	}
	if settings.TokenDisplayMode != "" && !capabilities.Allows(narrafork_setting.UserOverrideFieldTokenDisplayMode) {
		return fmt.Errorf("NarraFork setting is not allowed: token_display_mode")
	}
	if settings.CacheHitRateScope != "" {
		if !capabilities.Allows(narrafork_setting.UserOverrideFieldCacheHitRateScope) {
			return fmt.Errorf("NarraFork setting is not allowed: cache_hit_rate_scope")
		}
		if !containsString(capabilities.CacheHitRateScopes, strings.ToLower(strings.TrimSpace(settings.CacheHitRateScope))) {
			return fmt.Errorf("invalid NarraFork cache hit rate scope: %s", settings.CacheHitRateScope)
		}
	}
	if settings.CacheHitRateDays != nil {
		if !capabilities.Allows(narrafork_setting.UserOverrideFieldCacheHitRateDays) {
			return fmt.Errorf("NarraFork setting is not allowed: cache_hit_rate_days")
		}
		if *settings.CacheHitRateDays < narrafork_setting.MinCacheHitRateDays || *settings.CacheHitRateDays > capabilities.MaxCacheHitRateDays {
			return fmt.Errorf("cache_hit_rate_days must be between %d and %d", narrafork_setting.MinCacheHitRateDays, capabilities.MaxCacheHitRateDays)
		}
	}

	boolFields := []struct {
		name  string
		value *bool
	}{
		{narrafork_setting.UserOverrideFieldShowBalance, settings.ShowBalance},
		{narrafork_setting.UserOverrideFieldShowRequestQuota, settings.ShowRequestQuota},
		{narrafork_setting.UserOverrideFieldShowTodayQuota, settings.ShowTodayQuota},
		{narrafork_setting.UserOverrideFieldShowTodayTokens, settings.ShowTodayTokens},
		{narrafork_setting.UserOverrideFieldShowMonthQuota, settings.ShowMonthQuota},
		{narrafork_setting.UserOverrideFieldShowMonthTokens, settings.ShowMonthTokens},
		{narrafork_setting.UserOverrideFieldShowTotalQuota, settings.ShowTotalQuota},
		{narrafork_setting.UserOverrideFieldShowUsedQuota, settings.ShowUsedQuota},
		{narrafork_setting.UserOverrideFieldShowInputTokens, settings.ShowInputTokens},
		{narrafork_setting.UserOverrideFieldShowOutputTokens, settings.ShowOutputTokens},
		{narrafork_setting.UserOverrideFieldShowTotalTokens, settings.ShowTotalTokens},
		{narrafork_setting.UserOverrideFieldShowCacheHitTokens, settings.ShowCacheHitTokens},
		{narrafork_setting.UserOverrideFieldShowCacheHitRate, settings.ShowCacheHitRate},
		{narrafork_setting.UserOverrideFieldShowReasoningTokens, settings.ShowReasoningTokens},
		{narrafork_setting.UserOverrideFieldShowLatency, settings.ShowLatency},
		{narrafork_setting.UserOverrideFieldShowTTFT, settings.ShowTTFT},
		{narrafork_setting.UserOverrideFieldShowRequestID, settings.ShowRequestID},
		{narrafork_setting.UserOverrideFieldShowRetryCount, settings.ShowRetryCount},
		{narrafork_setting.UserOverrideFieldShowModel, settings.ShowModel},
		{narrafork_setting.UserOverrideFieldShowBillingSource, settings.ShowBillingSource},
		{narrafork_setting.UserOverrideFieldShowUnavailableFields, settings.ShowUnavailableFields},
	}
	for _, field := range boolFields {
		if field.value != nil && !capabilities.Allows(field.name) {
			return fmt.Errorf("NarraFork setting is not allowed: %s", field.name)
		}
	}

	if settings.DetailTemplate != nil {
		if !capabilities.Allows(narrafork_setting.UserOverrideFieldDetailTemplate) {
			return fmt.Errorf("NarraFork setting is not allowed: detail_template")
		}
		if err := narrafork_setting.ValidateDetailTemplate(*settings.DetailTemplate); err != nil {
			return err
		}
		if err := validateNarraForkTemplateFields(*settings.DetailTemplate, capabilities.GlobalCaps); err != nil {
			return err
		}
	}
	if settings.CustomQuotaBalance != nil && !capabilities.Allows(narrafork_setting.UserOverrideFieldCustomQuotaBalance) {
		return fmt.Errorf("NarraFork setting is not allowed: custom_quota_balance")
	}
	if settings.CustomDetailedQuotaBalance != nil && !capabilities.Allows(narrafork_setting.UserOverrideFieldCustomDetailedQuotaBalance) {
		return fmt.Errorf("NarraFork setting is not allowed: custom_detailed_quota_balance")
	}

	patch := narraForkUserSettingsToPolicyPatch(settings)
	return patch.Validate()
}

func narraForkUserSettingsToPolicyPatch(settings *dto.NarraForkUserSettings) narrafork_setting.NarraForkPolicyPatch {
	if settings == nil {
		return narrafork_setting.NarraForkPolicyPatch{}
	}
	return narrafork_setting.NarraForkPolicyPatch{
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
}

func validateNarraForkTemplateFields(template string, globalCaps map[string]bool) error {
	placeholderFields := map[string]string{
		"balance":               narrafork_setting.UserOverrideFieldShowBalance,
		"request_quota":         narrafork_setting.UserOverrideFieldShowRequestQuota,
		"today_quota":           narrafork_setting.UserOverrideFieldShowTodayQuota,
		"today_tokens":          narrafork_setting.UserOverrideFieldShowTodayTokens,
		"month_quota":           narrafork_setting.UserOverrideFieldShowMonthQuota,
		"month_tokens":          narrafork_setting.UserOverrideFieldShowMonthTokens,
		"total_quota":           narrafork_setting.UserOverrideFieldShowTotalQuota,
		"used_quota":            narrafork_setting.UserOverrideFieldShowUsedQuota,
		"input_tokens":          narrafork_setting.UserOverrideFieldShowInputTokens,
		"output_tokens":         narrafork_setting.UserOverrideFieldShowOutputTokens,
		"total_tokens":          narrafork_setting.UserOverrideFieldShowTotalTokens,
		"cache_hit_tokens":      narrafork_setting.UserOverrideFieldShowCacheHitTokens,
		"cache_hit_rate":        narrafork_setting.UserOverrideFieldShowCacheHitRate,
		"cache_hit_rate_period": narrafork_setting.UserOverrideFieldShowCacheHitRate,
		"reasoning_tokens":      narrafork_setting.UserOverrideFieldShowReasoningTokens,
		"latency_ms":            narrafork_setting.UserOverrideFieldShowLatency,
		"ttft_ms":               narrafork_setting.UserOverrideFieldShowTTFT,
		"request_id":            narrafork_setting.UserOverrideFieldShowRequestID,
		"retry_count":           narrafork_setting.UserOverrideFieldShowRetryCount,
		"model":                 narrafork_setting.UserOverrideFieldShowModel,
		"billing_source":        narrafork_setting.UserOverrideFieldShowBillingSource,
	}
	remaining := template
	for {
		start := strings.Index(remaining, "{{")
		if start < 0 {
			return nil
		}
		end := strings.Index(remaining[start+2:], "}}")
		if end < 0 {
			return nil
		}
		end += start + 2
		placeholder := strings.TrimSpace(remaining[start+2 : end])
		if field, ok := placeholderFields[placeholder]; ok && !globalCaps[field] {
			return fmt.Errorf("NarraFork template uses a globally hidden field: %s", placeholder)
		}
		remaining = remaining[end+2:]
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
