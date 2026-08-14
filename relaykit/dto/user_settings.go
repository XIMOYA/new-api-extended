// relaykit/dto/user_settings.go
// 用户个人设置 DTO，承载通知、界面偏好和 NarraFork 显示偏好。
package dto

type NarraForkUserSettings struct {
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

type UserSetting struct {
	NotifyType                       string                 `json:"notify_type,omitempty"`                          // QuotaWarningType 额度预警类型
	QuotaWarningThreshold            float64                `json:"quota_warning_threshold,omitempty"`              // QuotaWarningThreshold 额度预警阈值
	WebhookUrl                       string                 `json:"webhook_url,omitempty"`                          // WebhookUrl webhook地址
	WebhookSecret                    string                 `json:"webhook_secret,omitempty"`                       // WebhookSecret webhook密钥
	NotificationEmail                string                 `json:"notification_email,omitempty"`                   // NotificationEmail 通知邮箱地址
	BarkUrl                          string                 `json:"bark_url,omitempty"`                             // BarkUrl Bark推送URL
	GotifyUrl                        string                 `json:"gotify_url,omitempty"`                           // GotifyUrl Gotify服务器地址
	GotifyToken                      string                 `json:"gotify_token,omitempty"`                         // GotifyToken Gotify应用令牌
	GotifyPriority                   int                    `json:"gotify_priority"`                                // GotifyPriority Gotify消息优先级
	UpstreamModelUpdateNotifyEnabled bool                   `json:"upstream_model_update_notify_enabled,omitempty"` // 是否接收上游模型更新定时检测通知（仅管理员）
	AcceptUnsetRatioModel            bool                   `json:"accept_unset_model_ratio_model,omitempty"`       // AcceptUnsetRatioModel 是否接受未设置价格的模型
	RecordIpLog                      bool                   `json:"record_ip_log,omitempty"`                        // 是否记录请求和错误日志IP
	SidebarModules                   string                 `json:"sidebar_modules,omitempty"`                      // SidebarModules 左侧边栏模块配置
	BillingPreference                string                 `json:"billing_preference,omitempty"`                   // BillingPreference 扣费策略（订阅/钱包）
	Language                         string                 `json:"language,omitempty"`                             // Language 用户语言偏好 (zh, en)
	NarraForkDisplayMode             string                 `json:"narrafork_display_mode,omitempty"`               // NarraFork 信息显示偏好（inherit/show/hide）
	NarraFork                        *NarraForkUserSettings `json:"narrafork,omitempty"`                            // NarraFork 用户级覆盖设置
}

var (
	NotifyTypeEmail   = "email"   // Email 邮件
	NotifyTypeWebhook = "webhook" // Webhook
	NotifyTypeBark    = "bark"    // Bark 推送
	NotifyTypeGotify  = "gotify"  // Gotify 推送
)
