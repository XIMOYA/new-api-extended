// web/src/features/request-records/components/request-content-omitted-sections.tsx
// 区块列表里的断档提示：后端投影只返回了序列的头尾两段，这里说明中间跳过了多少个区块。
// 和简洁模式的「已隐藏 N 个区块」不是一回事——这些区块压根没返回，前端展不开。

import { Ellipsis } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export function RequestContentOmittedSectionsNotice(props: {
  omitted: number
}) {
  const { t } = useTranslation()

  return (
    <div
      className='border-border/70 text-muted-foreground flex flex-wrap items-center justify-center gap-x-2 gap-y-0.5 rounded-md border border-dashed px-3 py-2 text-center text-xs'
      data-slot='request-content-omitted-sections'
    >
      <Ellipsis aria-hidden='true' className='size-4 shrink-0' />
      <span>
        {t('{{count}} sections in between are not shown', {
          count: props.omitted,
        })}
      </span>
      <span className='text-[11px] opacity-80'>
        {t('Only the newest sections are kept')}
      </span>
    </div>
  )
}
