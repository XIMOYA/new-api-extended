// web/src/features/request-records/components/request-content-concise.tsx
// 简洁模式的两块 UI：区块列表上方的开关，以及列表下方"已隐藏 N 个区块"的说明与一键显示全部。
// 过滤和计数逻辑都在 lib/concise-view.ts，这里只负责渲染。

import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'

import type { RequestContentHiddenSectionCounts } from '../lib/concise-view'

export function RequestContentConciseToggle(props: {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  const { t } = useTranslation()
  const switchId = useId()

  return (
    <div className='flex shrink-0 items-center gap-2'>
      <label
        className='text-muted-foreground cursor-pointer select-none'
        htmlFor={switchId}
      >
        {t('Concise View')}
      </label>
      <Switch
        checked={props.checked}
        id={switchId}
        onCheckedChange={props.onCheckedChange}
        size='sm'
      />
    </div>
  )
}

export function RequestContentHiddenSummary(props: {
  hidden: RequestContentHiddenSectionCounts
  onShowAll: () => void
}) {
  const { t } = useTranslation()
  const breakdown = [
    { label: t('Reasoning'), count: props.hidden.reasoning },
    { label: t('Tool Calls'), count: props.hidden.toolCall },
    { label: t('Tool Results'), count: props.hidden.toolOutput },
    { label: t('Other'), count: props.hidden.other },
  ]
  const details = breakdown
    .filter((item) => item.count > 0)
    .map((item) => `${item.label} ${item.count}`)
    .join(' · ')

  return (
    <div
      className='border-border bg-muted/10 flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2 text-xs'
      data-slot='request-content-hidden-summary'
    >
      <div className='text-muted-foreground min-w-0'>
        <span>
          {t('Hidden sections: {{count}}', { count: props.hidden.total })}
        </span>
        {details && <span className='ml-1'>({details})</span>}
      </div>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={props.onShowAll}
      >
        {t('Show All')}
      </Button>
    </div>
  )
}
