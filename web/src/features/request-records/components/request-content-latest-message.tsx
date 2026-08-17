// web/src/features/request-records/components/request-content-latest-message.tsx
// 详情页顶部的"本轮最新用户消息"定位卡：显示这条记录里最后一条用户输入的预览和序号，
// 点一下跳到对应区块并展开。数据全部来自已加载的 sections，不额外请求接口。

import { CornerDownRight, MessageSquare } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { RequestContentLatestUserMessage } from '../lib/concise-view'

export function RequestContentLatestMessageCard(props: {
  latest: RequestContentLatestUserMessage
  onFocusSection: (sectionId: string) => void
}) {
  const { t } = useTranslation()
  const preview = props.latest.section.preview?.trim() || t('No content')

  return (
    <button
      className='border-border bg-muted/10 hover:bg-muted/30 focus-visible:ring-ring/50 flex w-full items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors focus-visible:ring-2 focus-visible:outline-none'
      data-slot='request-content-latest-message'
      onClick={() => props.onFocusSection(props.latest.section.id)}
      type='button'
    >
      <MessageSquare className='text-muted-foreground mt-0.5 size-4 shrink-0' />
      <div className='min-w-0 flex-1'>
        <div className='flex flex-wrap items-center gap-x-2 gap-y-0.5'>
          <span className='text-sm font-medium'>
            {t('Latest User Message')}
          </span>
          <span className='text-muted-foreground font-mono text-[11px] tabular-nums'>
            {t('Section {{index}} of {{total}}', {
              index: props.latest.index,
              total: props.latest.total,
            })}
          </span>
        </div>
        <div className='text-muted-foreground mt-0.5 line-clamp-2 text-xs break-words whitespace-pre-wrap'>
          {preview}
        </div>
      </div>
      <CornerDownRight
        aria-hidden='true'
        className='text-muted-foreground mt-0.5 size-4 shrink-0'
      />
    </button>
  )
}
