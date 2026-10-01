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
 * web/src/features/dashboard/components/models/cache-hit-rate-card.tsx
 * 页面：模型调用分析
 * 职责：
 * - 按当前看板时间与用户筛选展示 Token 缓存统计
 * - 展示输入、输出、缓存命中、缓存写入和总 Token
 * - 展示缓存命中率及加载、无数据、不可用状态
 */
import { Database, Gauge } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getCacheHitRateSummary } from '@/features/dashboard/api'
import { buildQueryParams, getDefaultDays } from '@/features/dashboard/lib'
import type {
  CacheHitRateSummary,
  DashboardFilters,
} from '@/features/dashboard/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatPercent } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { computeTimeRange } from '@/lib/time'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

interface CacheHitRateCardProps {
  filters?: DashboardFilters
}

function formatTokenValue(value: number, locale: Intl.LocalesArgument): string {
  if (!Number.isFinite(value)) return '--'
  return Intl.NumberFormat(locale, {
    notation: 'compact',
    maximumFractionDigits: 2,
  }).format(value)
}

function formatTokenFullValue(
  value: number,
  locale: Intl.LocalesArgument
): string {
  if (!Number.isFinite(value)) return '--'
  return Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(value)
}

function MetricItem(props: {
  label: string
  value?: number
  colorClassName: string
  locale: Intl.LocalesArgument
  loading?: boolean
}) {
  const value =
    props.value == null ? '--' : formatTokenValue(props.value, props.locale)
  const fullValue =
    props.value == null ? '--' : formatTokenFullValue(props.value, props.locale)

  return (
    <div className='min-w-0 px-3 py-2.5 sm:px-5 sm:py-3'>
      <div className='text-muted-foreground truncate text-[11px] leading-4 font-medium tracking-wide uppercase sm:text-xs sm:tracking-wider'>
        {props.label}
      </div>
      <div
        className={cn(
          'mt-1 truncate font-mono text-base leading-tight font-bold tracking-tight tabular-nums sm:text-xl sm:leading-normal',
          props.colorClassName
        )}
        title={fullValue}
      >
        {props.loading ? (
          <Skeleton className='h-6 w-20 sm:h-7 sm:w-24' />
        ) : (
          value
        )}
      </div>
    </div>
  )
}

export function CacheHitRateCard({ filters }: CacheHitRateCardProps) {
  const { t, i18n } = useTranslation()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)
  const [summary, setSummary] = useState<CacheHitRateSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError(false)

    const timeRange = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )

    void getCacheHitRateSummary(buildQueryParams(timeRange, filters), isAdmin)
      .then((res) => {
        if (!active) return
        if (!res?.success) {
          setSummary(null)
          setError(true)
          return
        }
        setSummary(res.data ?? null)
      })
      .catch(() => {
        if (!active) return
        setSummary(null)
        setError(true)
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [filters, isAdmin])

  const hasTokens = (summary?.total_tokens ?? 0) > 0
  const canShowRate = Boolean(summary?.available && summary.complete)
  const totalTokens = summary?.total_tokens ?? 0
  const cacheHitTokens = summary?.cache_hit_tokens ?? 0
  const cacheInputTokens = summary?.cache_input_tokens ?? 0
  const totalTokenDisplay = hasTokens
    ? formatTokenValue(totalTokens, locale)
    : '--'
  const cacheHitRateDisplay = canShowRate
    ? formatPercent(summary?.cache_hit_rate ?? 0)
    : '--'
  const cacheRatioDisplay = canShowRate
    ? `${formatTokenValue(cacheHitTokens, locale)} / ${formatTokenValue(cacheInputTokens, locale)}`
    : '-- / --'

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-col gap-3 border-b px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:px-5 sm:py-4'>
        <div className='min-w-0'>
          <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium tracking-wide uppercase'>
            <IconBadge tone='chart-2' size='xs'>
              <Database />
            </IconBadge>
            {t('Total Tokens')}
          </div>
          <div className='text-foreground mt-1 font-mono text-2xl leading-tight font-bold tracking-tight tabular-nums sm:text-3xl'>
            {loading ? (
              <Skeleton className='h-8 w-28 sm:h-9 sm:w-36' />
            ) : (
              totalTokenDisplay
            )}
          </div>
          {!loading && error && (
            <div className='text-muted-foreground mt-1 text-xs'>
              {t('Cache statistics unavailable')}
            </div>
          )}
          {!loading && !error && !hasTokens && (
            <div className='text-muted-foreground mt-1 text-xs'>
              {t('No cache data available')}
            </div>
          )}
        </div>

        <div className='bg-chart-2/10 text-chart-2 rounded-lg px-3 py-2 sm:min-w-36'>
          <div className='flex items-center gap-1.5 text-xs font-semibold'>
            <Gauge className='size-3.5' />
            {t('Cache Hit Rate')}
            <span className='font-mono tabular-nums'>
              {loading ? '--' : cacheHitRateDisplay}
            </span>
          </div>
          <div className='text-chart-2/80 mt-0.5 font-mono text-[11px] tabular-nums'>
            {loading ? '-- / --' : cacheRatioDisplay}
          </div>
        </div>
      </div>

      <div className='divide-border/60 grid grid-cols-2 divide-x divide-y sm:grid-cols-4 sm:divide-y-0'>
        <MetricItem
          label={t('Input Tokens')}
          value={summary?.input_tokens}
          colorClassName='text-chart-1'
          locale={locale}
          loading={loading}
        />
        <MetricItem
          label={t('Output Tokens')}
          value={summary?.output_tokens}
          colorClassName='text-chart-4'
          locale={locale}
          loading={loading}
        />
        <MetricItem
          label={t('Cache Hit')}
          value={summary?.cache_hit_tokens}
          colorClassName='text-chart-2'
          locale={locale}
          loading={loading}
        />
        <MetricItem
          label={t('Cache Write')}
          value={summary?.cache_write_tokens}
          colorClassName='text-warning'
          locale={locale}
          loading={loading}
        />
      </div>
    </div>
  )
}
