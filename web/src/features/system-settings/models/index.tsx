/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// web/src/features/system-settings/models/index.tsx
// 模型设置页入口：声明各配置项默认值（默认值类型同时决定 option map 的解析方式），并挂载分区渲染。
import { SettingsPage } from '../components/settings-page'
import type { ModelSettings } from '../types'
import {
  MODELS_DEFAULT_SECTION,
  getModelsSectionContent,
  getModelsSectionMeta,
} from './section-registry.tsx'

const defaultModelSettings: ModelSettings = {
  'request_content_audit.enabled': false,
  'request_content_audit.allow_user_view': false,
  'request_content_audit.admin_allowlist': [],
  'request_content_audit.retention_days': 30,
  'request_content_audit.storage_path': 'request-content-audit',
  'request_content_audit.max_record_bytes': 128 * 1024 * 1024,
  'request_content_audit.max_asset_bytes': 32 * 1024 * 1024,
  'request_content_audit.chunk_size_bytes': 64 * 1024,
  'narrafork_setting.enabled': false,
  'narrafork_setting.allow_user_display_override': false,
  'narrafork_setting.allow_user_enabled': false,
  'narrafork_setting.allow_user_activation_mode': false,
  'narrafork_setting.allow_user_balance_source': false,
  'narrafork_setting.allow_user_include_detailed': false,
  'narrafork_setting.allow_user_duplicate_policy': false,
  'narrafork_setting.allow_user_expose_extra': false,
  'narrafork_setting.allow_user_token_display_mode': false,
  'narrafork_setting.allow_user_cache_hit_rate_scope': false,
  'narrafork_setting.allow_user_cache_hit_rate_days': false,
  'narrafork_setting.allow_user_show_balance': false,
  'narrafork_setting.allow_user_show_request_quota': false,
  'narrafork_setting.allow_user_show_today_quota': false,
  'narrafork_setting.allow_user_show_today_tokens': false,
  'narrafork_setting.allow_user_show_month_quota': false,
  'narrafork_setting.allow_user_show_month_tokens': false,
  'narrafork_setting.allow_user_show_total_quota': false,
  'narrafork_setting.allow_user_show_used_quota': false,
  'narrafork_setting.allow_user_show_input_tokens': false,
  'narrafork_setting.allow_user_show_output_tokens': false,
  'narrafork_setting.allow_user_show_total_tokens': false,
  'narrafork_setting.allow_user_show_cache_hit_tokens': false,
  'narrafork_setting.allow_user_show_cache_hit_rate': false,
  'narrafork_setting.allow_user_show_reasoning_tokens': false,
  'narrafork_setting.allow_user_show_latency': false,
  'narrafork_setting.allow_user_show_ttft': false,
  'narrafork_setting.allow_user_show_request_id': false,
  'narrafork_setting.allow_user_show_retry_count': false,
  'narrafork_setting.allow_user_show_model': false,
  'narrafork_setting.allow_user_show_billing_source': false,
  'narrafork_setting.allow_user_show_unavailable_fields': false,
  'narrafork_setting.allow_user_detail_template': false,
  'narrafork_setting.allow_user_custom_quota_balance': false,
  'narrafork_setting.allow_user_custom_detailed_quota_balance': false,
  'narrafork_setting.activation_mode': 'header_or_user_agent',
  'narrafork_setting.balance_source': 'effective',
  'narrafork_setting.include_detailed': true,
  'narrafork_setting.duplicate_policy': 'skip',
  'narrafork_setting.expose_extra': false,
  'narrafork_setting.token_display_mode': 'exact',
  'narrafork_setting.cache_hit_rate_scope': 'request',
  'narrafork_setting.cache_hit_rate_days': 7,
  'narrafork_setting.show_balance': true,
  'narrafork_setting.show_request_quota': true,
  'narrafork_setting.show_today_quota': true,
  'narrafork_setting.show_today_tokens': true,
  'narrafork_setting.show_month_quota': true,
  'narrafork_setting.show_month_tokens': true,
  'narrafork_setting.show_total_quota': true,
  'narrafork_setting.show_used_quota': true,
  'narrafork_setting.show_input_tokens': true,
  'narrafork_setting.show_output_tokens': true,
  'narrafork_setting.show_total_tokens': true,
  'narrafork_setting.show_cache_hit_tokens': true,
  'narrafork_setting.show_cache_hit_rate': true,
  'narrafork_setting.show_reasoning_tokens': true,
  'narrafork_setting.show_latency': true,
  'narrafork_setting.show_ttft': true,
  'narrafork_setting.show_request_id': true,
  'narrafork_setting.show_retry_count': true,
  'narrafork_setting.show_model': true,
  'narrafork_setting.show_billing_source': true,
  'narrafork_setting.show_unavailable_fields': false,
  'narrafork_setting.detail_template': '',
  'narrafork_setting.custom_quota_balance': '',
  'narrafork_setting.custom_detailed_quota_balance': '',
  'global.pass_through_request_enabled': false,
  'global.thinking_model_blacklist': '[]',
  'global.chat_completions_to_responses_policy': '{}',
  'general_setting.ping_interval_enabled': false,
  'general_setting.ping_interval_seconds': 60,
  'gemini.safety_settings': '',
  'gemini.version_settings': '',
  'gemini.supported_imagine_models': '',
  'gemini.thinking_adapter_enabled': false,
  'gemini.thinking_adapter_budget_tokens_percentage': 0.6,
  'gemini.function_call_thought_signature_enabled': true,
  'gemini.remove_function_response_id_enabled': true,
  'claude.model_headers_settings': '',
  'claude.default_max_tokens': '',
  'claude.thinking_adapter_enabled': true,
  'claude.thinking_adapter_budget_tokens_percentage': 0.8,
  'grok.violation_deduction_enabled': true,
  'grok.violation_deduction_amount': 0.05,
  ModelPrice: '',
  ModelRatio: '',
  CacheRatio: '',
  CreateCacheRatio: '',
  CompletionRatio: '',
  ImageRatio: '',
  AudioRatio: '',
  AudioCompletionRatio: '',
  ExposeRatioEnabled: false,
  'billing_setting.billing_mode': '{}',
  'billing_setting.billing_expr': '{}',
  'tool_price_setting.prices': '{}',
  TopupGroupRatio: '',
  GroupRatio: '',
  UserUsableGroups: '',
  GroupGroupRatio: '',
  AutoGroups: '',
  MaxTokenAutoGroups: 5,
  DefaultUseAutoGroup: false,
  'group_ratio_setting.group_special_usable_group': '{}',
  RetryTimes: 0,
  ChannelDisableThreshold: '',
  AutomaticDisableChannelEnabled: false,
  AutomaticEnableChannelEnabled: false,
  AutomaticDisableKeywords: '',
  AutomaticDisableStatusCodes: '401',
  AutomaticRetryStatusCodes:
    '100-199,300-399,401-407,409-499,500-503,505-523,525-599',
  // 静默切换渠道：布尔按 'true'/'1' 解析，文案留空时后端回落内置默认值，
  // 尝试上限 0 表示跟随 RetryTimes（后端硬上限 32）。
  SilentChannelSwitchEnabled: false,
  SilentChannelSwitchMessage: '',
  SilentChannelSwitchMaxAttempts: 0,
  // 流式接力续写：默认关闭，开启后会重复计费已输出内容的 prompt token。
  StreamHandoffEnabled: false,
  StreamHandoffMaxAttempts: 2,
  'monitor_setting.auto_test_channel_enabled': false,
  'monitor_setting.auto_test_channel_minutes': 10,
  'monitor_setting.channel_test_mode': 'scheduled_all',
  'channel_affinity_setting.enabled': false,
  'channel_affinity_setting.switch_on_success': true,
  'channel_affinity_setting.keep_on_channel_disabled': false,
  'channel_affinity_setting.max_entries': 100000,
  'channel_affinity_setting.default_ttl_seconds': 3600,
  'channel_affinity_setting.rules': '[]',
  'model_deployment.ionet.api_key': '',
  'model_deployment.ionet.enabled': false,
}

export function ModelSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/models/$section'
      defaultSettings={defaultModelSettings}
      defaultSection={MODELS_DEFAULT_SECTION}
      getSectionContent={getModelsSectionContent}
      getSectionMeta={getModelsSectionMeta}
    />
  )
}
