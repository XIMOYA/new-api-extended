/*
web/src/features/channels/components/drawers/sections/narrafork-quota-event-section.tsx
组件：渠道级 NarraFork 额度事件覆盖设置
职责：
- 为渠道配置全局设置继承、显式开启/关闭和字段覆盖
- 保存 quotaBalanceEvent 的激活、额度来源、详细信息与重复策略
*/
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { ChannelFormValues } from '../../../lib/channel-form'

type Props = {
  disabled?: boolean
}

const booleanOverrides = ['inherit', 'true', 'false'] as const
const activationModes = [
  'inherit',
  'never',
  'header_only',
  'user_agent_only',
  'header_or_user_agent',
  'always',
] as const
const balanceSources = [
  'inherit',
  'effective',
  'user_quota',
  'token_quota',
  'custom',
] as const
const duplicatePolicies = ['inherit', 'skip', 'replace', 'always'] as const

function OverrideSelect(props: {
  value: string
  onChange: (value: string) => void
  options: readonly string[]
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const items = props.options.map((option) => ({
    value: option,
    label: t(`NarraFork channel option: ${option}`),
  }))
  return (
    <Select
      items={items}
      value={props.value}
      onValueChange={(value) => {
        if (value !== null) props.onChange(value)
      }}
      disabled={props.disabled}
    >
      <SelectTrigger className='w-full'>
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        {props.options.map((option) => (
          <SelectItem key={option} value={option}>
            {t(`NarraFork channel option: ${option}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export function NarraForkQuotaEventSection({ disabled = false }: Props) {
  const { t } = useTranslation()
  const form = useFormContext<ChannelFormValues>()

  return (
    <div className='space-y-4'>
      <div className='bg-muted/20 text-muted-foreground rounded-md border p-3 text-sm'>
        {t('Leave values as inherit to use the global NarraFork settings.')}
      </div>

      <FormField
        control={form.control}
        name='narrafork_enabled'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('NarraFork channel enabled override')}</FormLabel>
            <FormControl>
              <OverrideSelect
                value={field.value}
                onChange={field.onChange}
                options={booleanOverrides}
                disabled={disabled}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className='grid gap-4 md:grid-cols-2'>
        <FormField
          control={form.control}
          name='narrafork_activation_mode'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('NarraFork activation override')}</FormLabel>
              <FormControl>
                <OverrideSelect
                  value={field.value}
                  onChange={field.onChange}
                  options={activationModes}
                  disabled={disabled}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Header trigger is X-NarraFork-Quota-Event: true; User-Agent matching is case-insensitive.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='narrafork_balance_source'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('NarraFork balance source override')}</FormLabel>
              <FormControl>
                <OverrideSelect
                  value={field.value}
                  onChange={field.onChange}
                  options={balanceSources}
                  disabled={disabled}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='narrafork_include_detailed'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('NarraFork detailed balance override')}</FormLabel>
              <FormControl>
                <OverrideSelect
                  value={field.value}
                  onChange={field.onChange}
                  options={booleanOverrides}
                  disabled={disabled}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='narrafork_duplicate_policy'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('NarraFork duplicate policy override')}</FormLabel>
              <FormControl>
                <OverrideSelect
                  value={field.value}
                  onChange={field.onChange}
                  options={duplicatePolicies}
                  disabled={disabled}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name='narrafork_expose_extra'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('NarraFork extra fields override')}</FormLabel>
            <FormControl>
              <OverrideSelect
                value={field.value}
                onChange={field.onChange}
                options={booleanOverrides}
                disabled={disabled}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className='grid gap-4 md:grid-cols-2'>
        <FormField
          control={form.control}
          name='narrafork_custom_quota_balance'
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                {t('NarraFork custom quota balance override')}
              </FormLabel>
              <FormControl>
                <Input
                  {...field}
                  disabled={disabled}
                  placeholder={t('Leave empty to inherit')}
                />
              </FormControl>
              <FormDescription>
                {t('Used when the effective balance source is custom.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='narrafork_custom_detailed_quota_balance'
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                {t('NarraFork custom detailed balance override')}
              </FormLabel>
              <FormControl>
                <Input
                  {...field}
                  disabled={disabled}
                  placeholder={t('Leave empty to inherit')}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name='narrafork_policy_json'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('NarraFork advanced display overrides')}</FormLabel>
            <FormControl>
              <Textarea
                {...field}
                disabled={disabled}
                className='min-h-32 font-mono text-xs'
                placeholder={t('Example: {"cache_hit_rate_scope":"recent_days","cache_hit_rate_days":7}')}
              />
            </FormControl>
            <FormDescription>
              {t('Use JSON to override any NarraFork display field. Empty or omitted fields inherit from global settings.')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}
