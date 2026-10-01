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
web/src/features/profile/components/tabs/narrafork-settings-tab.tsx
页面：用户 NarraFork 设置
职责：
- 展示管理员开放给当前用户的 NarraFork 覆盖项
- 保存当前用户自己的事件、额度和详情显示偏好
- 通过服务端能力描述隐藏未开放或全局关闭的选项
*/
import { FileText, Loader2, Settings2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { updateNarraForkSettings } from '../../api'
import type {
  NarraForkActivationMode,
  NarraForkBalanceSource,
  NarraForkCacheHitRateScope,
  NarraForkDuplicatePolicy,
  NarraForkTokenDisplayMode,
  NarraForkUserConfig,
  NarraForkUserOverrideKey,
  NarraForkUserSettings,
  UserProfile,
} from '../../types'

interface NarraForkSettingsTabProps {
  profile: UserProfile | null
  onUpdate: () => void
}

type BooleanSettingKey =
  | 'enabled'
  | 'include_detailed'
  | 'expose_extra'
  | 'show_balance'
  | 'show_request_quota'
  | 'show_today_quota'
  | 'show_today_tokens'
  | 'show_month_quota'
  | 'show_month_tokens'
  | 'show_total_quota'
  | 'show_used_quota'
  | 'show_input_tokens'
  | 'show_output_tokens'
  | 'show_total_tokens'
  | 'show_cache_hit_tokens'
  | 'show_cache_hit_rate'
  | 'show_reasoning_tokens'
  | 'show_latency'
  | 'show_ttft'
  | 'show_request_id'
  | 'show_retry_count'
  | 'show_model'
  | 'show_billing_source'
  | 'show_unavailable_fields'

const detailFields: Array<{
  key: Exclude<
    BooleanSettingKey,
    'enabled' | 'include_detailed' | 'expose_extra'
  >
  label: string
  description: string
}> = [
  {
    key: 'show_balance',
    label: 'Show balance line',
    description: 'Show the current quota balance.',
  },
  {
    key: 'show_request_quota',
    label: 'Show request cost line',
    description: 'Show the quota consumed by this request.',
  },
  {
    key: 'show_today_quota',
    label: "Show today's quota",
    description: "Show today's quota consumption amount.",
  },
  {
    key: 'show_today_tokens',
    label: "Show today's Tokens",
    description: "Show today's aggregated Token count.",
  },
  {
    key: 'show_month_quota',
    label: "Show this month's quota",
    description: "Show this month's quota consumption amount.",
  },
  {
    key: 'show_month_tokens',
    label: "Show this month's Tokens",
    description: "Show this month's aggregated Token count.",
  },
  {
    key: 'show_total_quota',
    label: 'Show total quota',
    description: 'Show the reliable total quota when available.',
  },
  {
    key: 'show_used_quota',
    label: 'Show accumulated usage',
    description: 'Show the accumulated quota used so far.',
  },
  {
    key: 'show_input_tokens',
    label: 'Show input Tokens',
    description: 'Show input Token count from the upstream usage.',
  },
  {
    key: 'show_output_tokens',
    label: 'Show output Tokens',
    description: 'Show output Token count from the upstream usage.',
  },
  {
    key: 'show_total_tokens',
    label: 'Show total Tokens',
    description: 'Show the total Token count from the upstream usage.',
  },
  {
    key: 'show_cache_hit_tokens',
    label: 'Show cache-hit Tokens',
    description: 'Show the number of cached input Tokens.',
  },
  {
    key: 'show_cache_hit_rate',
    label: 'Show cache-hit rate',
    description: 'Show the cache-hit percentage.',
  },
  {
    key: 'show_reasoning_tokens',
    label: 'Show reasoning Tokens',
    description: 'Show reasoning Tokens when the upstream reports them.',
  },
  {
    key: 'show_latency',
    label: 'Show request duration',
    description: 'Show total request duration in NarraFork details.',
  },
  {
    key: 'show_ttft',
    label: 'Show first Token latency',
    description: 'Show time to first Token when a streamed response has one.',
  },
  {
    key: 'show_request_id',
    label: 'Show request ID',
    description: 'Show the stable request ID for tracing and support.',
  },
  {
    key: 'show_retry_count',
    label: 'Show retry count',
    description: 'Show how many channel retries were used for this request.',
  },
  {
    key: 'show_model',
    label: 'Show model name',
    description: 'Show the upstream model name.',
  },
  {
    key: 'show_billing_source',
    label: 'Show billing source',
    description: 'Show whether the request used wallet or subscription quota.',
  },
  {
    key: 'show_unavailable_fields',
    label: 'Show unavailable fields',
    description: 'Show 未提供 when a selected field has no reliable data.',
  },
]

function normalizeConfig(
  profile: UserProfile | null
): NarraForkUserConfig | null {
  return profile?.narrafork_user_config ?? null
}

function getInitialSettings(
  config: NarraForkUserConfig | null
): NarraForkUserSettings {
  const values = { ...config?.values }
  if (values.enabled === undefined) {
    if (config?.legacy_display_mode === 'hide') values.enabled = false
    if (config?.legacy_display_mode === 'show') values.enabled = true
  }
  return values
}

function isAllowed(
  config: NarraForkUserConfig | null,
  key: NarraForkUserOverrideKey
) {
  return config?.allowed_fields?.[key] === true
}

function isGlobalCapEnabled(
  config: NarraForkUserConfig | null,
  key: NarraForkUserOverrideKey
) {
  return config?.global_caps?.[key] !== false
}

function toTriState(value: boolean | undefined) {
  if (value === undefined) return 'inherit'
  return value ? 'true' : 'false'
}

function fromTriState(value: string | null): boolean | undefined {
  if (!value || value === 'inherit') return undefined
  return value === 'true'
}

function NarraForkBooleanSelect({
  value,
  onChange,
  label,
  description,
}: {
  value: boolean | undefined
  onChange: (value: boolean | undefined) => void
  label: string
  description: string
}) {
  const { t } = useTranslation()
  return (
    <div className='space-y-1.5 rounded-lg border p-3 sm:p-4'>
      <Label>{t(label)}</Label>
      <Select
        items={[
          { value: 'inherit', label: t('Follow global setting') },
          { value: 'true', label: t('Enable for my requests') },
          { value: 'false', label: t('Disable for my requests') },
        ]}
        value={toTriState(value)}
        onValueChange={(next) => onChange(fromTriState(next))}
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectItem value='inherit'>{t('Follow global setting')}</SelectItem>
          <SelectItem value='true'>{t('Enable for my requests')}</SelectItem>
          <SelectItem value='false'>{t('Disable for my requests')}</SelectItem>
        </SelectContent>
      </Select>
      <p className='text-muted-foreground text-xs sm:text-sm'>
        {t(description)}
      </p>
    </div>
  )
}

function NarraForkSelectField({
  value,
  onChange,
  label,
  description,
  items,
}: {
  value: string | undefined
  onChange: (value: string | undefined) => void
  label: string
  description: string
  items: Array<{ value: string; label: string }>
}) {
  const { t } = useTranslation()
  const allItems = [
    { value: 'inherit', label: t('Follow global setting') },
    ...items,
  ]
  return (
    <div className='space-y-1.5 rounded-lg border p-3 sm:p-4'>
      <Label>{t(label)}</Label>
      <Select
        items={allItems}
        value={value ?? 'inherit'}
        onValueChange={(next) =>
          onChange(next === 'inherit' ? undefined : (next ?? undefined))
        }
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          {allItems.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className='text-muted-foreground text-xs sm:text-sm'>
        {t(description)}
      </p>
    </div>
  )
}

export function NarraForkSettingsTab({
  profile,
  onUpdate,
}: NarraForkSettingsTabProps) {
  const { t } = useTranslation()
  const config = normalizeConfig(profile)
  const [settings, setSettings] = useState<NarraForkUserSettings>(() =>
    getInitialSettings(config)
  )
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setSettings(getInitialSettings(config))
  }, [config])

  const updateField = useCallback(
    <K extends keyof NarraForkUserSettings>(
      field: K,
      value: NarraForkUserSettings[K]
    ) => {
      setSettings((current) => ({ ...current, [field]: value }))
    },
    []
  )

  const visibleDetailFields = useMemo(
    () =>
      detailFields.filter(
        ({ key }) => isAllowed(config, key) && isGlobalCapEnabled(config, key)
      ),
    [config]
  )

  const handleSave = async () => {
    if (!config?.visible) return
    try {
      setLoading(true)
      const payload = Object.fromEntries(
        Object.entries(settings).filter(([key]) =>
          isAllowed(config, key as NarraForkUserOverrideKey)
        )
      ) as NarraForkUserSettings
      const response = await updateNarraForkSettings({ narrafork: payload })
      if (!response.success) {
        toast.error(response.message || t('Failed to update settings'))
        return
      }
      toast.success(t('Settings updated successfully'))
      onUpdate()
    } catch {
      toast.error(t('Failed to update settings'))
    } finally {
      setLoading(false)
    }
  }

  if (!config?.visible) return null

  const activationModes = config.activation_modes.map((value) => ({
    value,
    label: t(`NarraFork activation mode: ${value}`),
  }))
  const balanceSources = config.balance_sources.map((value) => ({
    value,
    label: t(`NarraFork balance source: ${value}`),
  }))
  const duplicatePolicies: Array<{
    value: NarraForkDuplicatePolicy
    label: string
  }> = [
    { value: 'skip', label: t('NarraFork duplicate policy: skip') },
    { value: 'replace', label: t('NarraFork duplicate policy: replace') },
    { value: 'always', label: t('NarraFork duplicate policy: always') },
  ]
  const tokenDisplayModes: Array<{
    value: NarraForkTokenDisplayMode
    label: string
  }> = [
    { value: 'exact', label: t('NarraFork token display mode: exact') },
    { value: 'compact', label: t('NarraFork token display mode: compact') },
  ]
  const cacheHitRateScopes = config.cache_hit_rate_scopes.map((value) => ({
    value,
    label: t(`NarraFork cache hit rate scope: ${value}`),
  }))

  const globalValues = config.global_values ?? {}
  const includeDetailed =
    settings.include_detailed ?? globalValues.include_detailed === true
  const balanceSource =
    settings.balance_source ??
    (typeof globalValues.balance_source === 'string'
      ? (globalValues.balance_source as NarraForkBalanceSource)
      : undefined)
  const cacheHitRateScope =
    settings.cache_hit_rate_scope ??
    (typeof globalValues.cache_hit_rate_scope === 'string'
      ? (globalValues.cache_hit_rate_scope as NarraForkCacheHitRateScope)
      : undefined)

  return (
    <div className='space-y-4 sm:space-y-6'>
      <div className='bg-muted/30 flex items-start gap-3 rounded-lg border p-3 sm:p-4'>
        <Settings2 className='text-primary mt-0.5 h-4 w-4 shrink-0' />
        <div className='space-y-1'>
          <h4 className='text-sm font-medium'>
            {t('NarraFork user settings')}
          </h4>
          <p className='text-muted-foreground text-xs sm:text-sm'>
            {t(
              'Only options opened by the administrator are shown here. Global settings remain the upper limit.'
            )}
          </p>
        </div>
      </div>

      {(isAllowed(config, 'enabled') ||
        isAllowed(config, 'activation_mode') ||
        isAllowed(config, 'duplicate_policy')) && (
        <section className='space-y-3'>
          <div>
            <h4 className='text-sm font-medium'>
              {t('NarraFork event control')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs sm:text-sm'>
              {t('Choose how NarraFork events behave for your own requests.')}
            </p>
          </div>
          <div className='grid gap-3 lg:grid-cols-2'>
            {isAllowed(config, 'enabled') &&
              isGlobalCapEnabled(config, 'enabled') && (
                <NarraForkBooleanSelect
                  value={settings.enabled}
                  onChange={(value) => updateField('enabled', value)}
                  label='Receive NarraFork events'
                  description='Choose whether your requests receive NarraFork quota events.'
                />
              )}
            {isAllowed(config, 'activation_mode') && (
              <NarraForkSelectField
                value={settings.activation_mode}
                onChange={(value) =>
                  updateField(
                    'activation_mode',
                    value as NarraForkActivationMode | undefined
                  )
                }
                label='NarraFork event activation mode'
                description='Choose how your requests activate NarraFork events.'
                items={activationModes}
              />
            )}
            {isAllowed(config, 'duplicate_policy') && (
              <NarraForkSelectField
                value={settings.duplicate_policy}
                onChange={(value) =>
                  updateField(
                    'duplicate_policy',
                    value as NarraForkDuplicatePolicy | undefined
                  )
                }
                label='NarraFork duplicate event policy'
                description='Choose how duplicate quota events are handled for your requests.'
                items={duplicatePolicies}
              />
            )}
          </div>
        </section>
      )}

      {(isAllowed(config, 'balance_source') ||
        isAllowed(config, 'include_detailed') ||
        isAllowed(config, 'expose_extra') ||
        isAllowed(config, 'token_display_mode') ||
        isAllowed(config, 'cache_hit_rate_scope') ||
        isAllowed(config, 'cache_hit_rate_days')) && (
        <section className='space-y-3'>
          <div>
            <h4 className='text-sm font-medium'>
              {t('NarraFork quota and data')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs sm:text-sm'>
              {t(
                'Choose which quota source and machine-readable details are used for your requests.'
              )}
            </p>
          </div>
          <div className='grid gap-3 lg:grid-cols-2'>
            {isAllowed(config, 'balance_source') && (
              <NarraForkSelectField
                value={settings.balance_source}
                onChange={(value) =>
                  updateField(
                    'balance_source',
                    value as NarraForkBalanceSource | undefined
                  )
                }
                label='NarraFork quota balance source'
                description='Choose which quota balance source is shown for your requests.'
                items={balanceSources}
              />
            )}
            {isAllowed(config, 'include_detailed') &&
              isGlobalCapEnabled(config, 'include_detailed') && (
                <NarraForkBooleanSelect
                  value={settings.include_detailed}
                  onChange={(value) => updateField('include_detailed', value)}
                  label='Include detailed quota balance'
                  description='Choose whether human-readable quota details are included.'
                />
              )}
            {isAllowed(config, 'expose_extra') &&
              isGlobalCapEnabled(config, 'expose_extra') && (
                <NarraForkBooleanSelect
                  value={settings.expose_extra}
                  onChange={(value) => updateField('expose_extra', value)}
                  label='Expose NarraFork event extra fields'
                  description='Choose whether safe machine-readable billing fields are exposed.'
                />
              )}
            {isAllowed(config, 'token_display_mode') && (
              <NarraForkSelectField
                value={settings.token_display_mode}
                onChange={(value) =>
                  updateField(
                    'token_display_mode',
                    value as NarraForkTokenDisplayMode | undefined
                  )
                }
                label='NarraFork token display format'
                description='Choose exact Token counts or compact K/M/B units.'
                items={tokenDisplayModes}
              />
            )}
            {isAllowed(config, 'cache_hit_rate_scope') && (
              <NarraForkSelectField
                value={settings.cache_hit_rate_scope}
                onChange={(value) =>
                  updateField(
                    'cache_hit_rate_scope',
                    value as NarraForkCacheHitRateScope | undefined
                  )
                }
                label='NarraFork cache hit rate window'
                description='Choose the cache-hit rate aggregation window.'
                items={cacheHitRateScopes}
              />
            )}
            {isAllowed(config, 'cache_hit_rate_days') &&
              cacheHitRateScope === 'recent_days' && (
                <div className='space-y-1.5 rounded-lg border p-3 sm:p-4'>
                  <Label htmlFor='narrafork-cache-hit-rate-days'>
                    {t('NarraFork cache hit rate recent days')}
                  </Label>
                  <Input
                    id='narrafork-cache-hit-rate-days'
                    type='number'
                    min={1}
                    max={config.max_cache_hit_rate_days}
                    value={
                      settings.cache_hit_rate_days ??
                      config.max_cache_hit_rate_days
                    }
                    onChange={(event) =>
                      updateField(
                        'cache_hit_rate_days',
                        Number(event.target.value)
                      )
                    }
                  />
                  <p className='text-muted-foreground text-xs sm:text-sm'>
                    {t(
                      'Used when the cache-hit rate window is set to recent N days; allowed range is 1–30.'
                    )}
                  </p>
                </div>
              )}
          </div>
        </section>
      )}

      {includeDetailed && visibleDetailFields.length > 0 && (
        <section className='space-y-3'>
          <div>
            <h4 className='text-sm font-medium'>
              {t('NarraFork detail display fields')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs sm:text-sm'>
              {t(
                'Choose which human-readable lines appear in your detailed quota balance.'
              )}
            </p>
          </div>
          <div className='grid gap-3 lg:grid-cols-2'>
            {visibleDetailFields.map(({ key, label, description }) => (
              <NarraForkBooleanSelect
                key={key}
                value={settings[key]}
                onChange={(value) => updateField(key, value)}
                label={label}
                description={description}
              />
            ))}
          </div>
        </section>
      )}

      {(isAllowed(config, 'detail_template') ||
        isAllowed(config, 'custom_quota_balance') ||
        isAllowed(config, 'custom_detailed_quota_balance')) && (
        <section className='space-y-3'>
          <div>
            <h4 className='flex items-center gap-2 text-sm font-medium'>
              <FileText className='h-4 w-4' />
              {t('NarraFork custom display')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs sm:text-sm'>
              {t(
                'Customize the text shown in your own NarraFork quota events.'
              )}
            </p>
          </div>
          <div className='space-y-3'>
            {isAllowed(config, 'detail_template') && includeDetailed && (
              <div className='space-y-1.5 rounded-lg border p-3 sm:p-4'>
                <Label htmlFor='narrafork-detail-template'>
                  {t('NarraFork detail template')}
                </Label>
                <Textarea
                  id='narrafork-detail-template'
                  value={settings.detail_template ?? ''}
                  onChange={(event) =>
                    updateField('detail_template', event.target.value)
                  }
                  className='min-h-32 font-mono text-xs'
                  placeholder={t(
                    'Example: Balance: {{balance}} / Latency: {{latency_ms}} ms'
                  )}
                />
                <p className='text-muted-foreground text-xs sm:text-sm'>
                  {t('Only allowlisted placeholders are supported.')}
                </p>
              </div>
            )}
            {isAllowed(config, 'custom_quota_balance') &&
              balanceSource === 'custom' && (
                <div className='space-y-1.5 rounded-lg border p-3 sm:p-4'>
                  <Label htmlFor='narrafork-custom-quota-balance'>
                    {t('Custom quota balance')}
                  </Label>
                  <Input
                    id='narrafork-custom-quota-balance'
                    value={settings.custom_quota_balance ?? ''}
                    onChange={(event) =>
                      updateField('custom_quota_balance', event.target.value)
                    }
                    placeholder={t('Example: $12.34')}
                  />
                </div>
              )}
            {isAllowed(config, 'custom_detailed_quota_balance') &&
              includeDetailed &&
              balanceSource === 'custom' && (
                <div className='space-y-1.5 rounded-lg border p-3 sm:p-4'>
                  <Label htmlFor='narrafork-custom-detailed-quota-balance'>
                    {t('Custom detailed quota balance')}
                  </Label>
                  <Input
                    id='narrafork-custom-detailed-quota-balance'
                    value={settings.custom_detailed_quota_balance ?? ''}
                    onChange={(event) =>
                      updateField(
                        'custom_detailed_quota_balance',
                        event.target.value
                      )
                    }
                  />
                </div>
              )}
          </div>
        </section>
      )}

      <div className='flex justify-end'>
        <Button type='button' onClick={handleSave} disabled={loading}>
          {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
          {t('Save settings')}
        </Button>
      </div>
    </div>
  )
}
