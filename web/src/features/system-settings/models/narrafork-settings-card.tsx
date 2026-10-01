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
/*
web/src/features/system-settings/models/narrafork-settings-card.tsx
页面：NarraFork 额度事件全局设置
职责：
- 控制 quotaBalanceEvent 是否启用以及请求激活方式
- 配置额度来源、详细余额、extra 字段和重复事件策略
- 配置 custom 额度展示字符串
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo, useRef } from 'react'
import { useForm, useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateNarraForkSettings } from '../hooks/use-update-option'
import type { NarraForkPolicyPatch } from '../types'
import { NarraForkPolicyPanel } from './narrafork-policy-panel'
import { NarraForkPreviewPanel } from './narrafork-preview-panel'

const activationModes = [
  'never',
  'header_only',
  'user_agent_only',
  'header_or_user_agent',
  'always',
] as const
const balanceSources = [
  'effective',
  'user_quota',
  'token_quota',
  'custom',
] as const
const duplicatePolicies = ['skip', 'replace', 'always'] as const
const tokenDisplayModes = ['exact', 'compact'] as const
const cacheHitRateScopes = ['request', 'today', 'recent_days'] as const

const narraforkSchema = z.object({
  narrafork_setting: z.object({
    enabled: z.boolean(),
    allow_user_display_override: z.boolean(),
    allow_user_enabled: z.boolean(),
    allow_user_activation_mode: z.boolean(),
    allow_user_balance_source: z.boolean(),
    allow_user_include_detailed: z.boolean(),
    allow_user_duplicate_policy: z.boolean(),
    allow_user_expose_extra: z.boolean(),
    allow_user_token_display_mode: z.boolean(),
    allow_user_cache_hit_rate_scope: z.boolean(),
    allow_user_cache_hit_rate_days: z.boolean(),
    allow_user_show_balance: z.boolean(),
    allow_user_show_request_quota: z.boolean(),
    allow_user_show_today_quota: z.boolean(),
    allow_user_show_today_tokens: z.boolean(),
    allow_user_show_month_quota: z.boolean(),
    allow_user_show_month_tokens: z.boolean(),
    allow_user_show_total_quota: z.boolean(),
    allow_user_show_used_quota: z.boolean(),
    allow_user_show_input_tokens: z.boolean(),
    allow_user_show_output_tokens: z.boolean(),
    allow_user_show_total_tokens: z.boolean(),
    allow_user_show_cache_hit_tokens: z.boolean(),
    allow_user_show_cache_hit_rate: z.boolean(),
    allow_user_show_reasoning_tokens: z.boolean(),
    allow_user_show_latency: z.boolean(),
    allow_user_show_ttft: z.boolean(),
    allow_user_show_request_id: z.boolean(),
    allow_user_show_retry_count: z.boolean(),
    allow_user_show_model: z.boolean(),
    allow_user_show_billing_source: z.boolean(),
    allow_user_show_unavailable_fields: z.boolean(),
    allow_user_detail_template: z.boolean(),
    allow_user_custom_quota_balance: z.boolean(),
    allow_user_custom_detailed_quota_balance: z.boolean(),
    activation_mode: z.enum(activationModes),
    balance_source: z.enum(balanceSources),
    include_detailed: z.boolean(),
    duplicate_policy: z.enum(duplicatePolicies),
    expose_extra: z.boolean(),
    token_display_mode: z.enum(tokenDisplayModes),
    cache_hit_rate_scope: z.enum(cacheHitRateScopes),
    cache_hit_rate_days: z.number().int().min(1).max(30),
    show_balance: z.boolean(),
    show_request_quota: z.boolean(),
    show_today_quota: z.boolean(),
    show_today_tokens: z.boolean(),
    show_month_quota: z.boolean(),
    show_month_tokens: z.boolean(),
    show_total_quota: z.boolean(),
    show_used_quota: z.boolean(),
    show_input_tokens: z.boolean(),
    show_output_tokens: z.boolean(),
    show_total_tokens: z.boolean(),
    show_cache_hit_tokens: z.boolean(),
    show_cache_hit_rate: z.boolean(),
    show_reasoning_tokens: z.boolean(),
    show_latency: z.boolean(),
    show_ttft: z.boolean(),
    show_request_id: z.boolean(),
    show_retry_count: z.boolean(),
    show_model: z.boolean(),
    show_billing_source: z.boolean(),
    show_unavailable_fields: z.boolean(),
    detail_template: z.string(),
    custom_quota_balance: z.string(),
    custom_detailed_quota_balance: z.string(),
  }),
})

type NarraForkFormInput = z.input<typeof narraforkSchema>
type NarraForkFormValues = z.output<typeof narraforkSchema>

type FlatNarraForkDefaults = {
  'narrafork_setting.enabled': boolean
  'narrafork_setting.allow_user_display_override': boolean
  'narrafork_setting.allow_user_enabled': boolean
  'narrafork_setting.allow_user_activation_mode': boolean
  'narrafork_setting.allow_user_balance_source': boolean
  'narrafork_setting.allow_user_include_detailed': boolean
  'narrafork_setting.allow_user_duplicate_policy': boolean
  'narrafork_setting.allow_user_expose_extra': boolean
  'narrafork_setting.allow_user_token_display_mode': boolean
  'narrafork_setting.allow_user_cache_hit_rate_scope': boolean
  'narrafork_setting.allow_user_cache_hit_rate_days': boolean
  'narrafork_setting.allow_user_show_balance': boolean
  'narrafork_setting.allow_user_show_request_quota': boolean
  'narrafork_setting.allow_user_show_today_quota': boolean
  'narrafork_setting.allow_user_show_today_tokens': boolean
  'narrafork_setting.allow_user_show_month_quota': boolean
  'narrafork_setting.allow_user_show_month_tokens': boolean
  'narrafork_setting.allow_user_show_total_quota': boolean
  'narrafork_setting.allow_user_show_used_quota': boolean
  'narrafork_setting.allow_user_show_input_tokens': boolean
  'narrafork_setting.allow_user_show_output_tokens': boolean
  'narrafork_setting.allow_user_show_total_tokens': boolean
  'narrafork_setting.allow_user_show_cache_hit_tokens': boolean
  'narrafork_setting.allow_user_show_cache_hit_rate': boolean
  'narrafork_setting.allow_user_show_reasoning_tokens': boolean
  'narrafork_setting.allow_user_show_latency': boolean
  'narrafork_setting.allow_user_show_ttft': boolean
  'narrafork_setting.allow_user_show_request_id': boolean
  'narrafork_setting.allow_user_show_retry_count': boolean
  'narrafork_setting.allow_user_show_model': boolean
  'narrafork_setting.allow_user_show_billing_source': boolean
  'narrafork_setting.allow_user_show_unavailable_fields': boolean
  'narrafork_setting.allow_user_detail_template': boolean
  'narrafork_setting.allow_user_custom_quota_balance': boolean
  'narrafork_setting.allow_user_custom_detailed_quota_balance': boolean
  'narrafork_setting.activation_mode': (typeof activationModes)[number]
  'narrafork_setting.balance_source': (typeof balanceSources)[number]
  'narrafork_setting.include_detailed': boolean
  'narrafork_setting.duplicate_policy': (typeof duplicatePolicies)[number]
  'narrafork_setting.expose_extra': boolean
  'narrafork_setting.token_display_mode': (typeof tokenDisplayModes)[number]
  'narrafork_setting.cache_hit_rate_scope': (typeof cacheHitRateScopes)[number]
  'narrafork_setting.cache_hit_rate_days': number
  'narrafork_setting.show_balance': boolean
  'narrafork_setting.show_request_quota': boolean
  'narrafork_setting.show_today_quota': boolean
  'narrafork_setting.show_today_tokens': boolean
  'narrafork_setting.show_month_quota': boolean
  'narrafork_setting.show_month_tokens': boolean
  'narrafork_setting.show_total_quota': boolean
  'narrafork_setting.show_used_quota': boolean
  'narrafork_setting.show_input_tokens': boolean
  'narrafork_setting.show_output_tokens': boolean
  'narrafork_setting.show_total_tokens': boolean
  'narrafork_setting.show_cache_hit_tokens': boolean
  'narrafork_setting.show_cache_hit_rate': boolean
  'narrafork_setting.show_reasoning_tokens': boolean
  'narrafork_setting.show_latency': boolean
  'narrafork_setting.show_ttft': boolean
  'narrafork_setting.show_request_id': boolean
  'narrafork_setting.show_retry_count': boolean
  'narrafork_setting.show_model': boolean
  'narrafork_setting.show_billing_source': boolean
  'narrafork_setting.show_unavailable_fields': boolean
  'narrafork_setting.detail_template': string
  'narrafork_setting.custom_quota_balance': string
  'narrafork_setting.custom_detailed_quota_balance': string
}

function buildFormDefaults(
  defaults: FlatNarraForkDefaults
): NarraForkFormInput {
  return {
    narrafork_setting: {
      enabled: defaults['narrafork_setting.enabled'],
      allow_user_display_override:
        defaults['narrafork_setting.allow_user_display_override'],
      allow_user_enabled: defaults['narrafork_setting.allow_user_enabled'],
      allow_user_activation_mode:
        defaults['narrafork_setting.allow_user_activation_mode'],
      allow_user_balance_source:
        defaults['narrafork_setting.allow_user_balance_source'],
      allow_user_include_detailed:
        defaults['narrafork_setting.allow_user_include_detailed'],
      allow_user_duplicate_policy:
        defaults['narrafork_setting.allow_user_duplicate_policy'],
      allow_user_expose_extra:
        defaults['narrafork_setting.allow_user_expose_extra'],
      allow_user_token_display_mode:
        defaults['narrafork_setting.allow_user_token_display_mode'],
      allow_user_cache_hit_rate_scope:
        defaults['narrafork_setting.allow_user_cache_hit_rate_scope'],
      allow_user_cache_hit_rate_days:
        defaults['narrafork_setting.allow_user_cache_hit_rate_days'],
      allow_user_show_balance:
        defaults['narrafork_setting.allow_user_show_balance'],
      allow_user_show_request_quota:
        defaults['narrafork_setting.allow_user_show_request_quota'],
      allow_user_show_today_quota:
        defaults['narrafork_setting.allow_user_show_today_quota'],
      allow_user_show_today_tokens:
        defaults['narrafork_setting.allow_user_show_today_tokens'],
      allow_user_show_month_quota:
        defaults['narrafork_setting.allow_user_show_month_quota'],
      allow_user_show_month_tokens:
        defaults['narrafork_setting.allow_user_show_month_tokens'],
      allow_user_show_total_quota:
        defaults['narrafork_setting.allow_user_show_total_quota'],
      allow_user_show_used_quota:
        defaults['narrafork_setting.allow_user_show_used_quota'],
      allow_user_show_input_tokens:
        defaults['narrafork_setting.allow_user_show_input_tokens'],
      allow_user_show_output_tokens:
        defaults['narrafork_setting.allow_user_show_output_tokens'],
      allow_user_show_total_tokens:
        defaults['narrafork_setting.allow_user_show_total_tokens'],
      allow_user_show_cache_hit_tokens:
        defaults['narrafork_setting.allow_user_show_cache_hit_tokens'],
      allow_user_show_cache_hit_rate:
        defaults['narrafork_setting.allow_user_show_cache_hit_rate'],
      allow_user_show_reasoning_tokens:
        defaults['narrafork_setting.allow_user_show_reasoning_tokens'],
      allow_user_show_latency:
        defaults['narrafork_setting.allow_user_show_latency'],
      allow_user_show_ttft: defaults['narrafork_setting.allow_user_show_ttft'],
      allow_user_show_request_id:
        defaults['narrafork_setting.allow_user_show_request_id'],
      allow_user_show_retry_count:
        defaults['narrafork_setting.allow_user_show_retry_count'],
      allow_user_show_model:
        defaults['narrafork_setting.allow_user_show_model'],
      allow_user_show_billing_source:
        defaults['narrafork_setting.allow_user_show_billing_source'],
      allow_user_show_unavailable_fields:
        defaults['narrafork_setting.allow_user_show_unavailable_fields'],
      allow_user_detail_template:
        defaults['narrafork_setting.allow_user_detail_template'],
      allow_user_custom_quota_balance:
        defaults['narrafork_setting.allow_user_custom_quota_balance'],
      allow_user_custom_detailed_quota_balance:
        defaults['narrafork_setting.allow_user_custom_detailed_quota_balance'],
      activation_mode: defaults['narrafork_setting.activation_mode'],
      balance_source: defaults['narrafork_setting.balance_source'],
      include_detailed: defaults['narrafork_setting.include_detailed'],
      duplicate_policy: defaults['narrafork_setting.duplicate_policy'],
      expose_extra: defaults['narrafork_setting.expose_extra'],
      token_display_mode: defaults['narrafork_setting.token_display_mode'],
      cache_hit_rate_scope: defaults['narrafork_setting.cache_hit_rate_scope'],
      cache_hit_rate_days: defaults['narrafork_setting.cache_hit_rate_days'],
      show_balance: defaults['narrafork_setting.show_balance'],
      show_request_quota: defaults['narrafork_setting.show_request_quota'],
      show_today_quota: defaults['narrafork_setting.show_today_quota'],
      show_today_tokens: defaults['narrafork_setting.show_today_tokens'],
      show_month_quota: defaults['narrafork_setting.show_month_quota'],
      show_month_tokens: defaults['narrafork_setting.show_month_tokens'],
      show_total_quota: defaults['narrafork_setting.show_total_quota'],
      show_used_quota: defaults['narrafork_setting.show_used_quota'],
      show_input_tokens: defaults['narrafork_setting.show_input_tokens'],
      show_output_tokens: defaults['narrafork_setting.show_output_tokens'],
      show_total_tokens: defaults['narrafork_setting.show_total_tokens'],
      show_cache_hit_tokens:
        defaults['narrafork_setting.show_cache_hit_tokens'],
      show_cache_hit_rate: defaults['narrafork_setting.show_cache_hit_rate'],
      show_reasoning_tokens:
        defaults['narrafork_setting.show_reasoning_tokens'],
      show_latency: defaults['narrafork_setting.show_latency'],
      show_ttft: defaults['narrafork_setting.show_ttft'],
      show_request_id: defaults['narrafork_setting.show_request_id'],
      show_retry_count: defaults['narrafork_setting.show_retry_count'],
      show_model: defaults['narrafork_setting.show_model'],
      show_billing_source: defaults['narrafork_setting.show_billing_source'],
      show_unavailable_fields:
        defaults['narrafork_setting.show_unavailable_fields'],
      detail_template: defaults['narrafork_setting.detail_template'],
      custom_quota_balance: defaults['narrafork_setting.custom_quota_balance'],
      custom_detailed_quota_balance:
        defaults['narrafork_setting.custom_detailed_quota_balance'],
    },
  }
}

function flattenFormValues(values: NarraForkFormValues): FlatNarraForkDefaults {
  return {
    'narrafork_setting.enabled': values.narrafork_setting.enabled,
    'narrafork_setting.allow_user_display_override':
      values.narrafork_setting.allow_user_display_override,
    'narrafork_setting.allow_user_enabled':
      values.narrafork_setting.allow_user_enabled,
    'narrafork_setting.allow_user_activation_mode':
      values.narrafork_setting.allow_user_activation_mode,
    'narrafork_setting.allow_user_balance_source':
      values.narrafork_setting.allow_user_balance_source,
    'narrafork_setting.allow_user_include_detailed':
      values.narrafork_setting.allow_user_include_detailed,
    'narrafork_setting.allow_user_duplicate_policy':
      values.narrafork_setting.allow_user_duplicate_policy,
    'narrafork_setting.allow_user_expose_extra':
      values.narrafork_setting.allow_user_expose_extra,
    'narrafork_setting.allow_user_token_display_mode':
      values.narrafork_setting.allow_user_token_display_mode,
    'narrafork_setting.allow_user_cache_hit_rate_scope':
      values.narrafork_setting.allow_user_cache_hit_rate_scope,
    'narrafork_setting.allow_user_cache_hit_rate_days':
      values.narrafork_setting.allow_user_cache_hit_rate_days,
    'narrafork_setting.allow_user_show_balance':
      values.narrafork_setting.allow_user_show_balance,
    'narrafork_setting.allow_user_show_request_quota':
      values.narrafork_setting.allow_user_show_request_quota,
    'narrafork_setting.allow_user_show_today_quota':
      values.narrafork_setting.allow_user_show_today_quota,
    'narrafork_setting.allow_user_show_today_tokens':
      values.narrafork_setting.allow_user_show_today_tokens,
    'narrafork_setting.allow_user_show_month_quota':
      values.narrafork_setting.allow_user_show_month_quota,
    'narrafork_setting.allow_user_show_month_tokens':
      values.narrafork_setting.allow_user_show_month_tokens,
    'narrafork_setting.allow_user_show_total_quota':
      values.narrafork_setting.allow_user_show_total_quota,
    'narrafork_setting.allow_user_show_used_quota':
      values.narrafork_setting.allow_user_show_used_quota,
    'narrafork_setting.allow_user_show_input_tokens':
      values.narrafork_setting.allow_user_show_input_tokens,
    'narrafork_setting.allow_user_show_output_tokens':
      values.narrafork_setting.allow_user_show_output_tokens,
    'narrafork_setting.allow_user_show_total_tokens':
      values.narrafork_setting.allow_user_show_total_tokens,
    'narrafork_setting.allow_user_show_cache_hit_tokens':
      values.narrafork_setting.allow_user_show_cache_hit_tokens,
    'narrafork_setting.allow_user_show_cache_hit_rate':
      values.narrafork_setting.allow_user_show_cache_hit_rate,
    'narrafork_setting.allow_user_show_reasoning_tokens':
      values.narrafork_setting.allow_user_show_reasoning_tokens,
    'narrafork_setting.allow_user_show_latency':
      values.narrafork_setting.allow_user_show_latency,
    'narrafork_setting.allow_user_show_ttft':
      values.narrafork_setting.allow_user_show_ttft,
    'narrafork_setting.allow_user_show_request_id':
      values.narrafork_setting.allow_user_show_request_id,
    'narrafork_setting.allow_user_show_retry_count':
      values.narrafork_setting.allow_user_show_retry_count,
    'narrafork_setting.allow_user_show_model':
      values.narrafork_setting.allow_user_show_model,
    'narrafork_setting.allow_user_show_billing_source':
      values.narrafork_setting.allow_user_show_billing_source,
    'narrafork_setting.allow_user_show_unavailable_fields':
      values.narrafork_setting.allow_user_show_unavailable_fields,
    'narrafork_setting.allow_user_detail_template':
      values.narrafork_setting.allow_user_detail_template,
    'narrafork_setting.allow_user_custom_quota_balance':
      values.narrafork_setting.allow_user_custom_quota_balance,
    'narrafork_setting.allow_user_custom_detailed_quota_balance':
      values.narrafork_setting.allow_user_custom_detailed_quota_balance,
    'narrafork_setting.activation_mode':
      values.narrafork_setting.activation_mode,
    'narrafork_setting.balance_source': values.narrafork_setting.balance_source,
    'narrafork_setting.include_detailed':
      values.narrafork_setting.include_detailed,
    'narrafork_setting.duplicate_policy':
      values.narrafork_setting.duplicate_policy,
    'narrafork_setting.expose_extra': values.narrafork_setting.expose_extra,
    'narrafork_setting.token_display_mode':
      values.narrafork_setting.token_display_mode,
    'narrafork_setting.cache_hit_rate_scope':
      values.narrafork_setting.cache_hit_rate_scope,
    'narrafork_setting.cache_hit_rate_days':
      values.narrafork_setting.cache_hit_rate_days,
    'narrafork_setting.show_balance': values.narrafork_setting.show_balance,
    'narrafork_setting.show_request_quota':
      values.narrafork_setting.show_request_quota,
    'narrafork_setting.show_today_quota':
      values.narrafork_setting.show_today_quota,
    'narrafork_setting.show_today_tokens':
      values.narrafork_setting.show_today_tokens,
    'narrafork_setting.show_month_quota':
      values.narrafork_setting.show_month_quota,
    'narrafork_setting.show_month_tokens':
      values.narrafork_setting.show_month_tokens,
    'narrafork_setting.show_total_quota':
      values.narrafork_setting.show_total_quota,
    'narrafork_setting.show_used_quota':
      values.narrafork_setting.show_used_quota,
    'narrafork_setting.show_input_tokens':
      values.narrafork_setting.show_input_tokens,
    'narrafork_setting.show_output_tokens':
      values.narrafork_setting.show_output_tokens,
    'narrafork_setting.show_total_tokens':
      values.narrafork_setting.show_total_tokens,
    'narrafork_setting.show_cache_hit_tokens':
      values.narrafork_setting.show_cache_hit_tokens,
    'narrafork_setting.show_cache_hit_rate':
      values.narrafork_setting.show_cache_hit_rate,
    'narrafork_setting.show_reasoning_tokens':
      values.narrafork_setting.show_reasoning_tokens,
    'narrafork_setting.show_latency': values.narrafork_setting.show_latency,
    'narrafork_setting.show_ttft': values.narrafork_setting.show_ttft,
    'narrafork_setting.show_request_id':
      values.narrafork_setting.show_request_id,
    'narrafork_setting.show_retry_count':
      values.narrafork_setting.show_retry_count,
    'narrafork_setting.show_model': values.narrafork_setting.show_model,
    'narrafork_setting.show_billing_source':
      values.narrafork_setting.show_billing_source,
    'narrafork_setting.show_unavailable_fields':
      values.narrafork_setting.show_unavailable_fields,
    'narrafork_setting.detail_template':
      values.narrafork_setting.detail_template,
    'narrafork_setting.custom_quota_balance':
      values.narrafork_setting.custom_quota_balance,
    'narrafork_setting.custom_detailed_quota_balance':
      values.narrafork_setting.custom_detailed_quota_balance,
  }
}

interface Props {
  defaultValues: FlatNarraForkDefaults
}

type NarraForkDisplayOptionName =
  | 'narrafork_setting.show_balance'
  | 'narrafork_setting.show_request_quota'
  | 'narrafork_setting.show_today_quota'
  | 'narrafork_setting.show_today_tokens'
  | 'narrafork_setting.show_month_quota'
  | 'narrafork_setting.show_month_tokens'
  | 'narrafork_setting.show_total_quota'
  | 'narrafork_setting.show_used_quota'
  | 'narrafork_setting.show_input_tokens'
  | 'narrafork_setting.show_output_tokens'
  | 'narrafork_setting.show_total_tokens'
  | 'narrafork_setting.show_cache_hit_tokens'
  | 'narrafork_setting.show_cache_hit_rate'
  | 'narrafork_setting.show_reasoning_tokens'
  | 'narrafork_setting.show_latency'
  | 'narrafork_setting.show_ttft'
  | 'narrafork_setting.show_request_id'
  | 'narrafork_setting.show_retry_count'
  | 'narrafork_setting.show_model'
  | 'narrafork_setting.show_billing_source'
  | 'narrafork_setting.show_unavailable_fields'

type NarraForkUserPermissionName =
  | 'narrafork_setting.allow_user_enabled'
  | 'narrafork_setting.allow_user_activation_mode'
  | 'narrafork_setting.allow_user_balance_source'
  | 'narrafork_setting.allow_user_include_detailed'
  | 'narrafork_setting.allow_user_duplicate_policy'
  | 'narrafork_setting.allow_user_expose_extra'
  | 'narrafork_setting.allow_user_token_display_mode'
  | 'narrafork_setting.allow_user_cache_hit_rate_scope'
  | 'narrafork_setting.allow_user_cache_hit_rate_days'
  | 'narrafork_setting.allow_user_show_balance'
  | 'narrafork_setting.allow_user_show_request_quota'
  | 'narrafork_setting.allow_user_show_today_quota'
  | 'narrafork_setting.allow_user_show_today_tokens'
  | 'narrafork_setting.allow_user_show_month_quota'
  | 'narrafork_setting.allow_user_show_month_tokens'
  | 'narrafork_setting.allow_user_show_total_quota'
  | 'narrafork_setting.allow_user_show_used_quota'
  | 'narrafork_setting.allow_user_show_input_tokens'
  | 'narrafork_setting.allow_user_show_output_tokens'
  | 'narrafork_setting.allow_user_show_total_tokens'
  | 'narrafork_setting.allow_user_show_cache_hit_tokens'
  | 'narrafork_setting.allow_user_show_cache_hit_rate'
  | 'narrafork_setting.allow_user_show_reasoning_tokens'
  | 'narrafork_setting.allow_user_show_latency'
  | 'narrafork_setting.allow_user_show_ttft'
  | 'narrafork_setting.allow_user_show_request_id'
  | 'narrafork_setting.allow_user_show_retry_count'
  | 'narrafork_setting.allow_user_show_model'
  | 'narrafork_setting.allow_user_show_billing_source'
  | 'narrafork_setting.allow_user_show_unavailable_fields'
  | 'narrafork_setting.allow_user_detail_template'
  | 'narrafork_setting.allow_user_custom_quota_balance'
  | 'narrafork_setting.allow_user_custom_detailed_quota_balance'

function NarraForkUserPermissionSwitch({
  name,
  label,
  description,
  disabled,
}: {
  name: NarraForkUserPermissionName
  label: string
  description: string
  disabled: boolean
}) {
  const { t } = useTranslation()
  const form = useFormContext<NarraForkFormInput>()

  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <SettingsSwitchItem>
          <SettingsSwitchContent>
            <FormLabel>{t(label)}</FormLabel>
            <FormDescription>{t(description)}</FormDescription>
          </SettingsSwitchContent>
          <FormControl>
            <Switch
              checked={Boolean(field.value)}
              onCheckedChange={field.onChange}
              disabled={disabled}
            />
          </FormControl>
        </SettingsSwitchItem>
      )}
    />
  )
}

function NarraForkDisplaySwitch({
  name,
  label,
  description,
  disabled,
}: {
  name: NarraForkDisplayOptionName
  label: string
  description: string
  disabled: boolean
}) {
  const { t } = useTranslation()
  const form = useFormContext<NarraForkFormInput>()

  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <SettingsSwitchItem>
          <SettingsSwitchContent>
            <FormLabel>{t(label)}</FormLabel>
            <FormDescription>{t(description)}</FormDescription>
          </SettingsSwitchContent>
          <FormControl>
            <Switch
              checked={Boolean(field.value)}
              onCheckedChange={field.onChange}
              disabled={disabled}
            />
          </FormControl>
        </SettingsSwitchItem>
      )}
    />
  )
}

function buildPreviewPatch(values: NarraForkFormValues): NarraForkPolicyPatch {
  const settings = values.narrafork_setting
  return {
    enabled: settings.enabled,
    activation_mode: settings.activation_mode,
    balance_source: settings.balance_source,
    include_detailed: settings.include_detailed,
    duplicate_policy: settings.duplicate_policy,
    expose_extra: settings.expose_extra,
    token_display_mode: settings.token_display_mode,
    cache_hit_rate_scope: settings.cache_hit_rate_scope,
    cache_hit_rate_days: settings.cache_hit_rate_days,
    show_balance: settings.show_balance,
    show_request_quota: settings.show_request_quota,
    show_today_quota: settings.show_today_quota,
    show_today_tokens: settings.show_today_tokens,
    show_month_quota: settings.show_month_quota,
    show_month_tokens: settings.show_month_tokens,
    show_total_quota: settings.show_total_quota,
    show_used_quota: settings.show_used_quota,
    show_input_tokens: settings.show_input_tokens,
    show_output_tokens: settings.show_output_tokens,
    show_total_tokens: settings.show_total_tokens,
    show_cache_hit_tokens: settings.show_cache_hit_tokens,
    show_cache_hit_rate: settings.show_cache_hit_rate,
    show_reasoning_tokens: settings.show_reasoning_tokens,
    show_latency: settings.show_latency,
    show_ttft: settings.show_ttft,
    show_request_id: settings.show_request_id,
    show_retry_count: settings.show_retry_count,
    show_model: settings.show_model,
    show_billing_source: settings.show_billing_source,
    show_unavailable_fields: settings.show_unavailable_fields,
    detail_template: settings.detail_template,
    custom_quota_balance: settings.custom_quota_balance,
    custom_detailed_quota_balance: settings.custom_detailed_quota_balance,
  }
}

export function NarraForkSettingsCard({ defaultValues }: Props) {
  const { t } = useTranslation()
  const updateNarraForkSettingsMutation = useUpdateNarraForkSettings()
  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )
  const form = useForm<NarraForkFormInput, unknown, NarraForkFormValues>({
    resolver: zodResolver(narraforkSchema),
    defaultValues: formDefaults,
  })
  const baselineRef = useRef(defaultValues)
  const baselineSerializedRef = useRef(JSON.stringify(defaultValues))

  useEffect(() => {
    const serialized = JSON.stringify(defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: NarraForkFormValues) => {
    const normalized = flattenFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatNarraForkDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    const changedValues = Object.fromEntries(
      changedKeys.map((key) => [key, String(normalized[key])])
    )
    try {
      const response =
        await updateNarraForkSettingsMutation.mutateAsync(changedValues)
      if (!response.success) return
    } catch {
      return
    }
    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
  }

  const enabled = form.watch('narrafork_setting.enabled')
  const allowUserOverride = form.watch(
    'narrafork_setting.allow_user_display_override'
  )
  const balanceSource = form.watch('narrafork_setting.balance_source')
  const includeDetailed = form.watch('narrafork_setting.include_detailed')
  const exposeExtra = form.watch('narrafork_setting.expose_extra')
  const cacheHitRateScope = form.watch('narrafork_setting.cache_hit_rate_scope')
  const showBalance = form.watch('narrafork_setting.show_balance')
  const showRequestQuota = form.watch('narrafork_setting.show_request_quota')
  const showTodayQuota = form.watch('narrafork_setting.show_today_quota')
  const showTodayTokens = form.watch('narrafork_setting.show_today_tokens')
  const showMonthQuota = form.watch('narrafork_setting.show_month_quota')
  const showMonthTokens = form.watch('narrafork_setting.show_month_tokens')
  const showTotalQuota = form.watch('narrafork_setting.show_total_quota')
  const showUsedQuota = form.watch('narrafork_setting.show_used_quota')
  const showInputTokens = form.watch('narrafork_setting.show_input_tokens')
  const showOutputTokens = form.watch('narrafork_setting.show_output_tokens')
  const showTotalTokens = form.watch('narrafork_setting.show_total_tokens')
  const showCacheHitTokens = form.watch(
    'narrafork_setting.show_cache_hit_tokens'
  )
  const showCacheHitRate = form.watch('narrafork_setting.show_cache_hit_rate')
  const showReasoningTokens = form.watch(
    'narrafork_setting.show_reasoning_tokens'
  )
  const showLatency = form.watch('narrafork_setting.show_latency')
  const showTTFT = form.watch('narrafork_setting.show_ttft')
  const showRequestID = form.watch('narrafork_setting.show_request_id')
  const showRetryCount = form.watch('narrafork_setting.show_retry_count')
  const showModel = form.watch('narrafork_setting.show_model')
  const showBillingSource = form.watch('narrafork_setting.show_billing_source')
  const showUnavailableFields = form.watch(
    'narrafork_setting.show_unavailable_fields'
  )
  const previewSettings = form.watch('narrafork_setting')
  const previewConfig = useMemo(
    () => buildPreviewPatch({ narrafork_setting: previewSettings }),
    [previewSettings]
  )
  const activationModeItems = activationModes.map((mode) => ({
    value: mode,
    label: t(`NarraFork activation mode: ${mode}`),
  }))
  const balanceSourceItems = balanceSources.map((source) => ({
    value: source,
    label: t(`NarraFork balance source: ${source}`),
  }))
  const duplicatePolicyItems = duplicatePolicies.map((policy) => ({
    value: policy,
    label: t(`NarraFork duplicate policy: ${policy}`),
  }))
  const tokenDisplayModeItems = tokenDisplayModes.map((mode) => ({
    value: mode,
    label: t(`NarraFork token display mode: ${mode}`),
  }))
  const cacheHitRateScopeItems = cacheHitRateScopes.map((scope) => ({
    value: scope,
    label: t(`NarraFork cache hit rate scope: ${scope}`),
  }))

  return (
    <SettingsSection title={t('NarraFork Gateway Events')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateNarraForkSettingsMutation.isPending}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable NarraFork quota events')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Disabled by default so ordinary OpenAI and Anthropic SDK clients receive no unknown SSE events.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.allow_user_display_override'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Allow users to customize NarraFork display')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Users can choose whether to show or hide NarraFork quota information for their own requests.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <SettingsControlGroup>
            <SettingsSwitchContent>
              <FormLabel>
                {t('NarraFork user customization permissions')}
              </FormLabel>
              <FormDescription>
                {t(
                  'Choose which NarraFork options users may customize for their own requests. Global settings remain the upper limit.'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <div className='grid gap-x-5 lg:grid-cols-2'>
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_enabled'
                label='Allow users to disable NarraFork events'
                description='Allow users to hide or receive their own NarraFork events.'
                disabled={!allowUserOverride || !enabled}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_activation_mode'
                label='Allow users to customize activation mode'
                description='Allow users to choose how their requests activate NarraFork events.'
                disabled={!allowUserOverride || !enabled}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_balance_source'
                label='Allow users to customize balance source'
                description='Allow users to choose the balance source shown for their requests.'
                disabled={!allowUserOverride || !enabled}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_include_detailed'
                label='Allow users to customize detailed output'
                description='Allow users to choose whether detailed quota information is included.'
                disabled={!allowUserOverride || !enabled || !includeDetailed}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_duplicate_policy'
                label='Allow users to customize duplicate policy'
                description='Allow users to choose how duplicate quota events are handled.'
                disabled={!allowUserOverride || !enabled}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_expose_extra'
                label='Allow users to customize extra fields'
                description='Allow users to choose whether safe machine-readable fields are exposed.'
                disabled={!allowUserOverride || !enabled || !exposeExtra}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_token_display_mode'
                label='Allow users to customize Token format'
                description='Allow users to choose exact or compact Token formatting.'
                disabled={!allowUserOverride || !enabled}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_cache_hit_rate_scope'
                label='Allow users to customize cache-hit window'
                description='Allow users to choose their cache-hit rate aggregation window.'
                disabled={!allowUserOverride || !enabled}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_cache_hit_rate_days'
                label='Allow users to customize cache-hit days'
                description='Allow users to choose the recent-days window within the global limit.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  cacheHitRateScope !== 'recent_days'
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_balance'
                label='Allow users to customize balance display'
                description='Allow users to show or hide the balance line.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showBalance
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_request_quota'
                label='Allow users to customize request cost display'
                description='Allow users to show or hide the request cost line.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showRequestQuota
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_today_quota'
                label='Allow users to customize today quota display'
                description='Allow users to show or hide today quota consumption.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showTodayQuota
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_today_tokens'
                label='Allow users to customize today Token display'
                description='Allow users to show or hide today Token totals.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showTodayTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_month_quota'
                label='Allow users to customize monthly quota display'
                description='Allow users to show or hide monthly quota consumption.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showMonthQuota
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_month_tokens'
                label='Allow users to customize monthly Token display'
                description='Allow users to show or hide monthly Token totals.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showMonthTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_total_quota'
                label='Allow users to customize total quota display'
                description='Allow users to show or hide total quota.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showTotalQuota
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_used_quota'
                label='Allow users to customize accumulated usage display'
                description='Allow users to show or hide accumulated quota usage.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showUsedQuota
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_input_tokens'
                label='Allow users to customize input Token display'
                description='Allow users to show or hide input Token totals.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showInputTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_output_tokens'
                label='Allow users to customize output Token display'
                description='Allow users to show or hide output Token totals.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showOutputTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_total_tokens'
                label='Allow users to customize total Token display'
                description='Allow users to show or hide total Token totals.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showTotalTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_cache_hit_tokens'
                label='Allow users to customize cache-hit Token display'
                description='Allow users to show or hide cache-hit Token totals.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showCacheHitTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_cache_hit_rate'
                label='Allow users to customize cache-hit rate display'
                description='Allow users to show or hide the cache-hit percentage.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showCacheHitRate
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_reasoning_tokens'
                label='Allow users to customize reasoning Token display'
                description='Allow users to show or hide reported reasoning Tokens.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showReasoningTokens
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_latency'
                label='Allow users to customize request duration display'
                description='Allow users to show or hide request duration.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showLatency
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_ttft'
                label='Allow users to customize first Token latency display'
                description='Allow users to show or hide time to first Token.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showTTFT
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_request_id'
                label='Allow users to customize request ID display'
                description='Allow users to show or hide the request ID.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showRequestID
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_retry_count'
                label='Allow users to customize retry count display'
                description='Allow users to show or hide retry counts.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showRetryCount
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_model'
                label='Allow users to customize model display'
                description='Allow users to show or hide the upstream model name.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showModel
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_billing_source'
                label='Allow users to customize billing source display'
                description='Allow users to show or hide the billing source.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showBillingSource
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_show_unavailable_fields'
                label='Allow users to customize unavailable fields'
                description='Allow users to choose whether unavailable fields are shown as 未提供.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  !showUnavailableFields
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_detail_template'
                label='Allow users to customize detail template'
                description='Allow users to customize their detailed quota template.'
                disabled={!allowUserOverride || !enabled || !includeDetailed}
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_custom_quota_balance'
                label='Allow users to customize quota balance text'
                description='Allow users to customize the quota balance text when custom balance is enabled.'
                disabled={
                  !allowUserOverride || !enabled || balanceSource !== 'custom'
                }
              />
              <NarraForkUserPermissionSwitch
                name='narrafork_setting.allow_user_custom_detailed_quota_balance'
                label='Allow users to customize detailed quota text'
                description='Allow users to customize detailed quota text when custom balance is enabled.'
                disabled={
                  !allowUserOverride ||
                  !enabled ||
                  !includeDetailed ||
                  balanceSource !== 'custom'
                }
              />
            </div>
          </SettingsControlGroup>

          <FormField
            control={form.control}
            name='narrafork_setting.activation_mode'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('NarraFork event activation mode')}</FormLabel>
                <Select
                  items={activationModeItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== null) field.onChange(value)
                  }}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {activationModes.map((mode) => (
                      <SelectItem key={mode} value={mode}>
                        {t(`NarraFork activation mode: ${mode}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Use the explicit header or NarraFork User-Agent to avoid affecting strict SSE clients.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.balance_source'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('NarraFork quota balance source')}</FormLabel>
                <Select
                  items={balanceSourceItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== null) field.onChange(value)
                  }}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {balanceSources.map((source) => (
                      <SelectItem key={source} value={source}>
                        {t(`NarraFork balance source: ${source}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Effective uses the balance source that will actually settle this request.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.include_detailed'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Include detailed quota balance')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Adds a human-readable balance and request cost summary.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!enabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.duplicate_policy'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('NarraFork duplicate event policy')}</FormLabel>
                <Select
                  items={duplicatePolicyItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== null) field.onChange(value)
                  }}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {duplicatePolicies.map((policy) => (
                      <SelectItem key={policy} value={policy}>
                        {t(`NarraFork duplicate policy: ${policy}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.expose_extra'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Expose NarraFork event extra fields')}
                  </FormLabel>
                  <FormDescription>
                    {t('Only safe billing metadata is included when enabled.')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!enabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.token_display_mode'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('NarraFork token display format')}</FormLabel>
                <Select
                  items={tokenDisplayModeItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== null) field.onChange(value)
                  }}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {tokenDisplayModes.map((mode) => (
                      <SelectItem key={mode} value={mode}>
                        {t(`NarraFork token display mode: ${mode}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Choose exact Token counts or compact K/M/B units for quota details.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.cache_hit_rate_scope'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('NarraFork cache hit rate window')}</FormLabel>
                <Select
                  items={cacheHitRateScopeItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== null) field.onChange(value)
                  }}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {cacheHitRateScopes.map((scope) => (
                      <SelectItem key={scope} value={scope}>
                        {t(`NarraFork cache hit rate scope: ${scope}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Cache-hit rate is cached input Tokens divided by input Tokens.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='narrafork_setting.cache_hit_rate_days'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>
                  {t('NarraFork cache hit rate recent days')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={30}
                    value={field.value}
                    onChange={(event) => {
                      const value = event.target.value
                      field.onChange(value === '' ? 7 : Number(value))
                    }}
                    disabled={!enabled || cacheHitRateScope !== 'recent_days'}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Used when the cache-hit rate window is set to recent N days; allowed range is 1–30.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <SettingsControlGroup>
            <SettingsSwitchContent>
              <FormLabel>{t('NarraFork detail display fields')}</FormLabel>
              <FormDescription>
                {t(
                  'Choose which human-readable lines appear in detailedQuotaBalance.'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <div className='grid gap-x-5 lg:grid-cols-2'>
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_balance'
                label='Show balance line'
                description='Show the current quota balance.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_request_quota'
                label='Show request cost line'
                description='Show the quota consumed by this request.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_today_quota'
                label="Show today's quota"
                description="Show today's quota consumption amount."
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_today_tokens'
                label="Show today's Tokens"
                description="Show today's aggregated Token count."
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_month_quota'
                label="Show this month's quota"
                description="Show this month's quota consumption amount."
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_month_tokens'
                label="Show this month's Tokens"
                description="Show this month's aggregated Token count."
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_total_quota'
                label='Show total quota'
                description='Show the reliable total quota when available.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_used_quota'
                label='Show accumulated usage'
                description='Show the accumulated quota used so far.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_input_tokens'
                label='Show input Tokens'
                description='Show input Token count from the upstream usage.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_output_tokens'
                label='Show output Tokens'
                description='Show output Token count from the upstream usage.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_total_tokens'
                label='Show total Tokens'
                description='Show the total Token count from the upstream usage.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_cache_hit_tokens'
                label='Show cache-hit Tokens'
                description='Show the number of cached input Tokens.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_cache_hit_rate'
                label='Show cache-hit rate'
                description='Show the cache-hit percentage.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_reasoning_tokens'
                label='Show reasoning Tokens'
                description='Show reasoning Tokens when the upstream reports them.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_latency'
                label='Show request duration'
                description='Show total request duration in NarraFork details.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_ttft'
                label='Show first Token latency'
                description='Show time to first Token when a streamed response has one.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_request_id'
                label='Show request ID'
                description='Show the stable request ID for tracing and support.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_retry_count'
                label='Show retry count'
                description='Show how many channel retries were used for this request.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_model'
                label='Show model name'
                description='Show the upstream model name.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_billing_source'
                label='Show billing source'
                description='Show whether the request used wallet or subscription quota.'
                disabled={!enabled}
              />
              <NarraForkDisplaySwitch
                name='narrafork_setting.show_unavailable_fields'
                label='Show unavailable fields'
                description='Show 未提供 when a selected field has no reliable data.'
                disabled={!enabled}
              />
            </div>
          </SettingsControlGroup>

          <FormField
            control={form.control}
            name='narrafork_setting.detail_template'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('NarraFork detail template')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    disabled={!enabled}
                    className='min-h-32 font-mono text-xs'
                    placeholder={t(
                      'Example: Balance: {{balance}} / Latency: {{latency_ms}} ms'
                    )}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional template for detailedQuotaBalance. Only allowlisted placeholders are supported.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {balanceSource === 'custom' && (
            <>
              <FormField
                control={form.control}
                name='narrafork_setting.custom_quota_balance'
                render={({ field }) => (
                  <FormItem className='max-w-xl'>
                    <FormLabel>{t('Custom quota balance')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        disabled={!enabled}
                        placeholder={t('Example: $12.34')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='narrafork_setting.custom_detailed_quota_balance'
                render={({ field }) => (
                  <FormItem className='max-w-xl'>
                    <FormLabel>{t('Custom detailed quota balance')}</FormLabel>
                    <FormControl>
                      <Input {...field} disabled={!enabled} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          )}

          <NarraForkPreviewPanel config={previewConfig} disabled={!enabled} />

          <SettingsControlGroup>
            <SettingsSwitchContent>
              <FormLabel>{t('NarraFork scoped policies')}</FormLabel>
              <FormDescription>
                {t(
                  'Override display fields for a user group or a specific user.'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <NarraForkPolicyPanel disabled={!enabled} />
          </SettingsControlGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
