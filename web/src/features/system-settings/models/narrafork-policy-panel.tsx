/*
web/src/features/system-settings/models/narrafork-policy-panel.tsx
组件：NarraFork 用户组/用户作用域策略管理
职责：
- 选择用户组或用户作用域
- 编辑可继承的 NarraFork 策略补丁 JSON
- 保存、删除作用域策略并刷新策略列表
*/
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

import {
  deleteNarraForkPolicy,
  getNarraForkPolicies,
  updateNarraForkPolicy,
} from '../api'
import type { NarraForkPolicy, NarraForkPolicyPatch } from '../types'

const scopeTypes = ['group', 'user'] as const

type Props = {
  disabled?: boolean
}

function prettyConfig(config: NarraForkPolicyPatch): string {
  return JSON.stringify(config, null, 2)
}

export function NarraForkPolicyPanel({ disabled = false }: Props) {
  const { t } = useTranslation()
  const [scopeType, setScopeType] = useState<(typeof scopeTypes)[number]>('group')
  const [scopeKey, setScopeKey] = useState('')
  const [configText, setConfigText] = useState('{}')
  const [policies, setPolicies] = useState<NarraForkPolicy[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  const scopedPolicies = useMemo(
    () => policies.filter((policy) => policy.scope_type === scopeType),
    [policies, scopeType]
  )

  const loadPolicies = useCallback(async () => {
    setIsLoading(true)
    try {
      const response = await getNarraForkPolicies()
      if (!response.success) throw new Error(response.message)
      setPolicies(response.data || [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Failed to load settings'))
    } finally {
      setIsLoading(false)
    }
  }, [t])

  useEffect(() => {
    void loadPolicies()
  }, [loadPolicies])

  useEffect(() => {
    const policy = policies.find(
      (item) => item.scope_type === scopeType && item.scope_key === scopeKey.trim()
    )
    setConfigText(policy ? prettyConfig(policy.config) : '{}')
  }, [policies, scopeKey, scopeType])

  const selectPolicy = (value: string | null) => {
    if (value === null) return
    const policy = scopedPolicies.find((item) => item.scope_key === value)
    if (!policy) return
    setScopeKey(policy.scope_key)
    setConfigText(prettyConfig(policy.config))
  }

  const save = async () => {
    const normalizedKey = scopeKey.trim()
    if (!normalizedKey) {
      toast.error(t('Enter a NarraFork policy scope key'))
      return
    }
    let config: NarraForkPolicyPatch
    try {
      const parsed = JSON.parse(configText)
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error(t('Policy config must be a JSON object'))
      }
      config = parsed as NarraForkPolicyPatch
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Invalid JSON'))
      return
    }
    setIsSaving(true)
    try {
      const response = await updateNarraForkPolicy(scopeType, normalizedKey, config)
      if (!response.success) throw new Error(response.message)
      toast.success(t('NarraFork policy saved'))
      await loadPolicies()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Failed to save setting'))
    } finally {
      setIsSaving(false)
    }
  }

  const remove = async () => {
    const normalizedKey = scopeKey.trim()
    if (!normalizedKey) return
    setIsSaving(true)
    try {
      const response = await deleteNarraForkPolicy(scopeType, normalizedKey)
      if (!response.success) throw new Error(response.message)
      toast.success(t('NarraFork policy reset to inherit'))
      setConfigText('{}')
      await loadPolicies()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Failed to update setting'))
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 md:grid-cols-2'>
        <div className='space-y-2'>
          <Label>{t('NarraFork policy scope')}</Label>
          <Select
            items={scopeTypes.map((value) => ({
              value,
              label: t(`NarraFork policy scope: ${value}`),
            }))}
            value={scopeType}
            onValueChange={(value) => {
              if (value === 'group' || value === 'user') {
                setScopeType(value)
                setScopeKey('')
              }
            }}
            disabled={disabled || isSaving}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              {scopeTypes.map((value) => (
                <SelectItem key={value} value={value}>
                  {t(`NarraFork policy scope: ${value}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='narrafork-policy-scope-key'>
            {t('NarraFork policy scope key')}
          </Label>
          <Input
            id='narrafork-policy-scope-key'
            value={scopeKey}
            onChange={(event) => setScopeKey(event.target.value)}
            placeholder={
              scopeType === 'group'
                ? t('Example: vip')
                : t('Example: 123')
            }
            disabled={disabled || isSaving}
          />
        </div>
      </div>

      {scopedPolicies.length > 0 && (
        <div className='space-y-2'>
          <Label>{t('Saved NarraFork policies')}</Label>
          <Select
            items={scopedPolicies.map((policy) => ({
              value: policy.scope_key,
              label: policy.scope_key,
            }))}
            value={scopeKey || null}
            onValueChange={selectPolicy}
            disabled={disabled || isLoading || isSaving}
          >
            <SelectTrigger>
              <SelectValue placeholder={t('Select an existing policy')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              {scopedPolicies.map((policy) => (
                <SelectItem key={policy.scope_key} value={policy.scope_key}>
                  {policy.scope_key}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      <div className='space-y-2'>
        <Label htmlFor='narrafork-policy-config'>
          {t('NarraFork policy patch JSON')}
        </Label>
        <Textarea
          id='narrafork-policy-config'
          value={configText}
          onChange={(event) => setConfigText(event.target.value)}
          className='min-h-48 font-mono text-xs'
          placeholder={t('Example: {"cache_hit_rate_scope":"recent_days","cache_hit_rate_days":7}')}
          disabled={disabled || isSaving}
        />
        <p className='text-muted-foreground text-xs'>
          {t('Empty fields inherit from the next broader NarraFork scope.')}
        </p>
      </div>

      <div className='flex flex-wrap gap-2'>
        <Button type='button' onClick={() => void save()} disabled={disabled || isSaving}>
          {t('Save NarraFork policy')}
        </Button>
        <Button
          type='button'
          variant='outline'
          onClick={() => void remove()}
          disabled={disabled || isSaving || !scopeKey.trim()}
        >
          {t('Reset NarraFork policy')}
        </Button>
        <Button
          type='button'
          variant='ghost'
          onClick={() => void loadPolicies()}
          disabled={disabled || isLoading || isSaving}
        >
          {t('Refresh policies')}
        </Button>
      </div>
    </div>
  )
}
