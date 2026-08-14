// web/src/features/request-records/components/asset-preview.tsx
// 请求记录图片缩略图的懒加载与原件打开，不把二进制内容放进页面状态。

import { Download, ImageOff, Loader2 } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useQuery } from '@tanstack/react-query'

import { getRequestContentAssetBlob } from '../api'
import type { RequestContentAuditAsset } from '../types'

export function RequestContentAssetPreview(props: {
  auditId: number
  asset: RequestContentAuditAsset
}) {
  const { t } = useTranslation()
  const [visible, setVisible] = useState(false)
  const [imageUrl, setImageUrl] = useState<string | null>(null)
  const assetQuery = useQuery({
    queryKey: ['request-content-asset', props.auditId, props.asset.asset_key],
    queryFn: () =>
      getRequestContentAssetBlob(
        props.auditId,
        props.asset.asset_key,
        'thumbnail'
      ),
    enabled: visible && props.asset.thumbnail_available,
    staleTime: 10 * 60 * 1000,
  })

  useEffect(() => {
    if (!assetQuery.data) return
    const url = URL.createObjectURL(assetQuery.data)
    setImageUrl(url)
    return () => URL.revokeObjectURL(url)
  }, [assetQuery.data])

  let assetVisual: ReactNode = (
    <ImageOff className='text-muted-foreground size-4' />
  )
  if (imageUrl) {
    assetVisual = (
      <img
        src={imageUrl}
        alt={props.asset.file_name || props.asset.asset_key}
        className='size-full object-cover'
      />
    )
  } else if (assetQuery.isLoading) {
    assetVisual = <Loader2 className='text-muted-foreground size-4 animate-spin' />
  }

  return (
    <div
      className='border-border bg-muted/20 flex min-w-36 items-center gap-3 rounded-md border p-2'
      ref={(node) => {
        if (!node || visible) return
        const observer = new IntersectionObserver(
          ([entry]) => {
            if (entry?.isIntersecting) {
              setVisible(true)
              observer.disconnect()
            }
          },
          { rootMargin: '160px' }
        )
        observer.observe(node)
        return () => observer.disconnect()
      }}
    >
      <div className='bg-muted flex size-16 shrink-0 items-center justify-center overflow-hidden rounded'>
        {assetVisual}
      </div>
      <div className='min-w-0 flex-1'>
        <div className='truncate text-xs font-medium'>
          {props.asset.file_name || props.asset.asset_key}
        </div>
        <div className='text-muted-foreground truncate text-[11px]'>
          {props.asset.mime_type} · {formatBytes(props.asset.original_size)}
        </div>
        {props.asset.original_available && (
          <Button
            variant='ghost'
            size='sm'
            className='mt-1 h-6 px-1.5 text-[11px]'
            onClick={() => {
              void openOriginal(
                props.auditId,
                props.asset.asset_key,
                t('Unable to open original')
              )
            }}
          >
            <Download className='mr-1 size-3' />
            {t('Open original')}
          </Button>
        )}
      </div>
    </div>
  )
}

async function openOriginal(
  auditId: number,
  assetKey: string,
  errorMessage: string
) {
  try {
    const blob = await getRequestContentAssetBlob(auditId, assetKey, 'original')
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'request-content-asset'
    link.rel = 'noopener'
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
  } catch {
    // The shared HTTP client already reports authenticated request failures.
    void errorMessage
  }
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(value) / Math.log(1024))
  )
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
