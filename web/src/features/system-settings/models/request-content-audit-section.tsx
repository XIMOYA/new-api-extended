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
// web/src/features/system-settings/models/request-content-audit-section.tsx
// Root 用户配置请求内容审计的记录开关、查看权限、白名单和存储参数。

import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { searchUsers } from '@/features/users/api'

import {
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { saveRequestContentAuditOptions } from './request-content-audit-save'

export type RequestContentAuditSettingsForm = {
  enabled: boolean
  allow_user_view: boolean
  admin_allowlist: number[]
  retention_days: number
  storage_path: string
  max_record_bytes: number
  max_asset_bytes: number
  chunk_size_bytes: number
}

export function RequestContentAuditSection(props: {
  defaultValues: RequestContentAuditSettingsForm
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [values, setValues] = useState(props.defaultValues)
  const [userKeyword, setUserKeyword] = useState('')
  const [selectedUser, setSelectedUser] = useState('')

  useEffect(() => {
    setValues({
      ...props.defaultValues,
      admin_allowlist: [...props.defaultValues.admin_allowlist],
    })
  }, [props.defaultValues])

  const usersQuery = useQuery({
    queryKey: ['request-content-audit-users', userKeyword],
    queryFn: () => searchUsers({ keyword: userKeyword, p: 1, page_size: 20 }),
    enabled: userKeyword.trim().length > 0,
  })
  const userOptions = useMemo(
    () =>
      (usersQuery.data?.data?.items ?? [])
        .filter((user) => user.role >= 10)
        .map((user) => ({
          value: String(user.id),
          label: `${user.username} (#${user.id})`,
        })),
    [usersQuery.data?.data?.items]
  )

  const update = <K extends keyof RequestContentAuditSettingsForm>(
    key: K,
    value: RequestContentAuditSettingsForm[K]
  ) => setValues((current) => ({ ...current, [key]: value }))

  const save = async () => {
    const entries = [
      { key: 'request_content_audit.enabled', value: String(values.enabled) },
      {
        key: 'request_content_audit.allow_user_view',
        value: String(values.allow_user_view),
      },
      {
        key: 'request_content_audit.admin_allowlist',
        value: JSON.stringify(values.admin_allowlist),
      },
      {
        key: 'request_content_audit.retention_days',
        value: String(values.retention_days),
      },
      {
        key: 'request_content_audit.storage_path',
        value: values.storage_path.trim(),
      },
      {
        key: 'request_content_audit.max_record_bytes',
        value: String(values.max_record_bytes),
      },
      {
        key: 'request_content_audit.max_asset_bytes',
        value: String(values.max_asset_bytes),
      },
      {
        key: 'request_content_audit.chunk_size_bytes',
        value: String(values.chunk_size_bytes),
      },
    ]
    await saveRequestContentAuditOptions(entries, (entry) =>
      updateOption.mutateAsync(entry)
    )
  }

  const addSelectedUser = (value: string | null) => {
    if (!value) return
    const userId = Number(value)
    if (!Number.isInteger(userId) || userId <= 0) return
    if (!values.admin_allowlist.includes(userId)) {
      update(
        'admin_allowlist',
        [...values.admin_allowlist, userId].sort((a, b) => a - b)
      )
    }
    setSelectedUser('')
  }

  return (
    <SettingsSection title={t('Request Content Audit')}>
      <SettingsPageFormActions
        onSave={() => void save()}
        isSaving={updateOption.isPending}
        saveLabel='Save request audit settings'
      />
      <SettingsForm>
        <SettingsSwitchField
          checked={values.enabled}
          onCheckedChange={(checked) => update('enabled', checked)}
          label={t('Enable request content audit')}
          description={t(
            'Record only requests actually submitted to New API after validation. Draft text and key events are never recorded.'
          )}
        />
        <SettingsSwitchField
          checked={values.allow_user_view}
          onCheckedChange={(checked) => update('allow_user_view', checked)}
          label={t('Allow users to view their own request records')}
          description={t(
            'Root users can always view all records. Non-Root administrators need the whitelist to view other users.'
          )}
        />

        <div className='space-y-3 lg:col-span-2'>
          <div>
            <div className='text-sm font-medium'>
              {t('Administrator view whitelist')}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Whitelisted administrators can view other users records in redacted form.'
              )}
            </p>
          </div>
          <Combobox
            options={userOptions}
            value={selectedUser}
            onValueChange={addSelectedUser}
            placeholder={t('Search and select an administrator')}
            searchPlaceholder={t('Search users...')}
            emptyText={t('No users found')}
            openOnFocus
          />
          <Input
            value={userKeyword}
            onChange={(event) => setUserKeyword(event.target.value)}
            placeholder={t('Type a username or user ID to search')}
          />
          <div className='flex flex-wrap gap-2'>
            {values.admin_allowlist.length === 0 ? (
              <span className='text-muted-foreground text-xs'>
                {t('No administrators selected')}
              </span>
            ) : (
              values.admin_allowlist.map((userId) => (
                <Button
                  key={userId}
                  type='button'
                  variant='secondary'
                  size='sm'
                  onClick={() =>
                    update(
                      'admin_allowlist',
                      values.admin_allowlist.filter((id) => id !== userId)
                    )
                  }
                >
                  #{userId} ×
                </Button>
              ))
            )}
          </div>
        </div>

        <div className='grid gap-4 sm:grid-cols-3 lg:col-span-2'>
          <SettingNumberField
            label={t('Retention days')}
            value={values.retention_days}
            min={1}
            max={3650}
            onChange={(value) => update('retention_days', value)}
          />
          <SettingNumberField
            label={t('Max record size (MiB)')}
            value={Math.round(values.max_record_bytes / (1 << 20))}
            min={1}
            max={128}
            onChange={(value) => update('max_record_bytes', value * (1 << 20))}
          />
          <SettingNumberField
            label={t('Max asset size (MiB)')}
            value={Math.round(values.max_asset_bytes / (1 << 20))}
            min={1}
            max={128}
            onChange={(value) => update('max_asset_bytes', value * (1 << 20))}
          />
        </div>

        <div className='grid gap-4 sm:grid-cols-2 lg:col-span-2'>
          <SettingNumberField
            label={t('Streaming chunk size (KiB)')}
            value={Math.round(values.chunk_size_bytes / 1024)}
            min={4}
            max={16384}
            onChange={(value) => update('chunk_size_bytes', value * 1024)}
          />
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Storage path')}</span>
            <Input
              value={values.storage_path}
              onChange={(event) => update('storage_path', event.target.value)}
            />
            <span className='text-muted-foreground block text-xs'>
              {t(
                'Relative paths are stored under the configured log directory.'
              )}
            </span>
          </label>
        </div>
      </SettingsForm>
    </SettingsSection>
  )
}

function SettingNumberField(props: {
  label: string
  value: number
  min: number
  max: number
  onChange: (value: number) => void
}) {
  return (
    <label className='space-y-2 text-sm'>
      <span className='font-medium'>{props.label}</span>
      <Input
        type='number'
        min={props.min}
        max={props.max}
        step={1}
        value={props.value}
        onChange={(event) =>
          props.onChange(Number(event.target.value) || props.min)
        }
      />
    </label>
  )
}
