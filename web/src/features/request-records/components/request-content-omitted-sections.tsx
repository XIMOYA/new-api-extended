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
