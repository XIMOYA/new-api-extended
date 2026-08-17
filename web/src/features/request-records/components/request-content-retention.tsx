// web/src/features/request-records/components/request-content-retention.tsx
// 保留策略展示：概览行汇总原始/已保留/未保留大小与分类明细，区块内提示这段正文按策略没有落盘。

import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { cn } from '@/lib/utils'

import {
  formatRequestContentBytes,
  getRequestContentDroppedKindLabel,
} from '../lib/retention'
import type { RequestContentAuditViewRetention } from '../types'

const droppedToneClassName =
  'border-amber-500/30 bg-amber-500/5 text-amber-700 dark:text-amber-300'
const droppedBadgeClassName = cn(
  'h-6 shrink-0 rounded-md border px-2 font-mono text-[11px]',
  droppedToneClassName
)

export function RequestContentRetentionSummary(props: {
  retention: RequestContentAuditViewRetention
}) {
  const { t } = useTranslation()
  const sizeItems: { label: string; value: number }[] = []
  if (props.retention.original_size && props.retention.original_size > 0) {
    sizeItems.push({
      label: t('Original Size'),
      value: props.retention.original_size,
    })
  }
  sizeItems.push({ label: t('Kept'), value: props.retention.kept_bytes })
  sizeItems.push({ label: t('Not Kept'), value: props.retention.dropped_bytes })
  const originalHash = props.retention.original_sha256
  const categories = props.retention.dropped ?? []

  return (
    <div
      className='border-border bg-muted/10 space-y-1.5 rounded-md border px-3 py-2 text-xs'
      data-slot='request-content-retention'
    >
      <div className='flex flex-wrap items-center gap-x-3 gap-y-1'>
        <span className='font-medium'>{t('Retention Policy')}</span>
        {sizeItems.map((item) => (
          <span className='text-muted-foreground' key={item.label}>
            {item.label}:{' '}
            <span className='font-mono tabular-nums'>
              {formatRequestContentBytes(item.value)}
            </span>
          </span>
        ))}
        {originalHash && (
          <StatusBadge
            label={`${t('Original SHA-256')}: ${originalHash.slice(0, 12)}`}
            copyText={originalHash}
            showDot={false}
            className='border-border/60 bg-muted/30 h-6 shrink-0 rounded-md border px-2 font-mono text-[11px]'
          />
        )}
      </div>
      {categories.length > 0 && (
        <div className='flex flex-wrap gap-1.5'>
          {categories.map((category) => (
            <StatusBadge
              className={droppedBadgeClassName}
              copyable={false}
              key={category.kind}
              label={`${getRequestContentDroppedKindLabel(category.kind, t)} · ${category.count} ${t('Items')} · ${formatRequestContentBytes(category.bytes)}`}
              showDot={false}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export function RequestContentDroppedNotice(props: {
  bytes: number
  kind?: string
  className?: string
}) {
  const { t } = useTranslation()
  const details = [getRequestContentDroppedKindLabel(props.kind, t)]
  if (props.bytes > 0) details.push(formatRequestContentBytes(props.bytes))

  return (
    <div
      className={cn(
        'rounded-md border px-3 py-2 text-xs',
        droppedToneClassName,
        props.className
      )}
      data-slot='request-content-dropped-notice'
    >
      <div>{t('Content not kept by retention policy')}</div>
      <div className='mt-0.5 font-mono text-[11px] opacity-80'>
        {details.join(' · ')}
      </div>
    </div>
  )
}

export function RequestContentDroppedBadge(props: { bytes: number }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      className={droppedBadgeClassName}
      copyable={false}
      label={`${formatRequestContentBytes(props.bytes)} ${t('Not Kept')}`}
      showDot={false}
    />
  )
}
