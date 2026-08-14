// web/src/features/request-records/index.tsx
// 请求记录独立页面：分页展示元数据，点击后流式读取正文并懒加载多模态缩略图。

import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, ExternalLink, RefreshCw, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

import {
  getRequestContentAudit,
  getRequestContentAudits,
  streamRequestContent,
} from './api'
import { RequestContentAssetPreview } from './components/asset-preview'
import type { RequestContentAuditDetail, RequestContentAuditSummary } from './types'

const route = getRouteApi('/_authenticated/request-records/')
const EMPTY_REQUEST_RECORDS: RequestContentAuditSummary[] = []

export function RequestRecords() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = route.useSearch()
  const [requestIdInput, setRequestIdInput] = useState(search.requestId ?? '')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [contentOpen, setContentOpen] = useState(false)
  const [highlightRequestId, setHighlightRequestId] = useState<string | null>(null)

  useEffect(() => {
    setRequestIdInput(search.requestId ?? '')
  }, [search.requestId])

  const listQuery = useQuery({
    queryKey: ['request-content-audits', search.page, search.requestId],
    queryFn: () =>
      getRequestContentAudits({
        page: search.page,
        request_id: search.requestId,
      }),
  })

  const detailQuery = useQuery({
    queryKey: ['request-content-audit', selectedId],
    queryFn: () => getRequestContentAudit(selectedId as number),
    enabled: selectedId != null,
  })

  const items = listQuery.data?.items ?? EMPTY_REQUEST_RECORDS
  const totalPages = Math.max(
    1,
    Math.ceil((listQuery.data?.total ?? 0) / (listQuery.data?.page_size || 20))
  )
  const currentPage = search.page ?? 1

  useEffect(() => {
    const requestId = search.requestId ?? ''
    if (!requestId) {
      setHighlightRequestId(null)
      return
    }
    setHighlightRequestId(requestId)
    const matching = items.find((item) => item.request_id === requestId)
    if (matching) setSelectedId(matching.id)
    const timeout = window.setTimeout(() => setHighlightRequestId(null), 5000)
    return () => window.clearTimeout(timeout)
  }, [items, search.requestId])

  const submitRequestId = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void navigate({
      to: '/request-records',
      search: {
        requestId: requestIdInput.trim() || undefined,
        page: 1,
      },
    })
  }

  const selectRecord = (item: RequestContentAuditSummary) => {
    setSelectedId(item.id)
    setContentOpen(true)
  }

  const goToUsageLog = (requestId: string) => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: { requestId },
    })
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Request Records')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void listQuery.refetch()}
          disabled={listQuery.isFetching}
        >
          <RefreshCw className={cn('mr-2 size-4', listQuery.isFetching && 'animate-spin')} />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          <Card>
            <CardContent className='pt-4'>
              <form className='flex flex-col gap-2 sm:flex-row' onSubmit={submitRequestId}>
                <div className='relative min-w-0 flex-1'>
                  <Search className='text-muted-foreground absolute top-2.5 left-3 size-4' />
                  <Input
                    value={requestIdInput}
                    onChange={(event) => setRequestIdInput(event.target.value)}
                    placeholder={t('Filter by request ID')}
                    className='pl-9'
                  />
                </div>
                <Button type='submit'>{t('Search')}</Button>
                {search.requestId && (
                  <Button
                    type='button'
                    variant='ghost'
                    onClick={() => {
                      setRequestIdInput('')
                      void navigate({
                        to: '/request-records',
                        search: { page: 1, requestId: undefined },
                      })
                    }}
                  >
                    {t('Clear')}
                  </Button>
                )}
              </form>
            </CardContent>
          </Card>

          <div className='min-h-0 flex-1 overflow-auto rounded-lg border'>
            <table className='w-full min-w-[900px] text-sm'>
              <thead className='bg-muted/50 sticky top-0 z-10 border-b'>
                <tr className='text-muted-foreground text-left text-xs'>
                  <th className='px-3 py-2 font-medium'>{t('Time')}</th>
                  <th className='px-3 py-2 font-medium'>{t('User')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Model')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Request Type')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Size')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Status')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Actions')}</th>
                </tr>
              </thead>
              <tbody>
                <RequestRecordsTableBody
                  isLoading={listQuery.isLoading}
                  items={items}
                  highlightRequestId={highlightRequestId}
                  onSelect={selectRecord}
                  onUsageLog={goToUsageLog}
                />
              </tbody>
            </table>
          </div>

          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground text-xs'>
              {t('Total')}: {listQuery.data?.total ?? 0}
            </span>
            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={currentPage <= 1 || listQuery.isFetching}
                onClick={() =>
                  void navigate({
                    to: '/request-records',
                    search: { ...search, page: currentPage - 1 },
                  })
                }
              >
                <ArrowLeft className='mr-1 size-4' />
                {t('Previous')}
              </Button>
              <span className='text-muted-foreground text-xs'>
                {currentPage} / {totalPages}
              </span>
              <Button
                variant='outline'
                size='sm'
                disabled={currentPage >= totalPages || listQuery.isFetching}
                onClick={() =>
                  void navigate({
                    to: '/request-records',
                    search: { ...search, page: currentPage + 1 },
                  })
                }
              >
                {t('Next')}
                <ArrowRight className='ml-1 size-4' />
              </Button>
            </div>
          </div>
        </div>
      </SectionPageLayout.Content>

      <RequestContentDialog
        audit={detailQuery.data ?? null}
        open={contentOpen}
        onOpenChange={setContentOpen}
        isLoading={detailQuery.isLoading}
        onOpenUsageLog={goToUsageLog}
      />
    </SectionPageLayout>
  )
}

function RequestRecordsTableBody(props: {
  isLoading: boolean
  items: RequestContentAuditSummary[]
  highlightRequestId: string | null
  onSelect: (item: RequestContentAuditSummary) => void
  onUsageLog: (requestId: string) => void
}) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return (
      <tr>
        <td colSpan={7} className='text-muted-foreground px-3 py-12 text-center'>
          {t('Loading...')}
        </td>
      </tr>
    )
  }
  if (props.items.length === 0) {
    return (
      <tr>
        <td colSpan={7} className='text-muted-foreground px-3 py-12 text-center'>
          {t('No request records')}
        </td>
      </tr>
    )
  }
  return props.items.map((item) => {
    let statusLabel = t('Available')
    let statusVariant: 'outline' | 'destructive' = 'outline'
    if (item.capture_status !== 'complete' || !item.content_available) {
      statusLabel = t('Unavailable')
      statusVariant = 'destructive'
    } else if (item.is_redacted) {
      statusLabel = t('Redacted')
    }
    return (
      <tr
      key={item.id}
      className={cn(
        'hover:bg-muted/30 cursor-pointer border-b last:border-0',
        props.highlightRequestId === item.request_id &&
          'bg-primary/5 animate-pulse ring-2 ring-inset ring-primary/40'
      )}
      onClick={() => props.onSelect(item)}
    >
      <td className='px-3 py-2.5 whitespace-nowrap'>{formatDate(item.created_at)}</td>
      <td className='px-3 py-2.5'>
        <div className='font-medium'>{item.username || `#${item.user_id}`}</div>
        <div className='text-muted-foreground font-mono text-[11px]'>{item.request_id}</div>
      </td>
      <td className='px-3 py-2.5'>
        <div className='font-medium'>{item.model_name || '-'}</div>
        <div className='text-muted-foreground truncate text-xs'>{item.endpoint_path}</div>
      </td>
      <td className='px-3 py-2.5'>
        <Badge variant='secondary'>{item.request_type || item.relay_format}</Badge>
      </td>
      <td className='px-3 py-2.5 font-mono text-xs'>
        {formatBytes(item.content_size)}
        {item.asset_count > 0 && ` · ${item.asset_count} ${t('assets')}`}
      </td>
      <td className='px-3 py-2.5'>
        <Badge variant={statusVariant}>{statusLabel}</Badge>
      </td>
      <td className='px-3 py-2.5 text-right'>
        <div className='flex justify-end gap-1'>
          <Button
            size='sm'
            variant='ghost'
            onClick={(event) => {
              event.stopPropagation()
              props.onSelect(item)
            }}
          >
            {t('View')}
          </Button>
          <Button
            size='sm'
            variant='ghost'
            onClick={(event) => {
              event.stopPropagation()
              props.onUsageLog(item.request_id)
            }}
          >
            <ExternalLink className='mr-1 size-3.5' />
            {t('Usage Log')}
          </Button>
        </div>
      </td>
    </tr>
    )
  })
}

function RequestContentDialog(props: {
  audit: RequestContentAuditDetail | null
  open: boolean
  onOpenChange: (open: boolean) => void
  isLoading: boolean
  onOpenUsageLog: (requestId: string) => void
}) {
  const { t } = useTranslation()
  const [content, setContent] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [streamTruncated, setStreamTruncated] = useState(false)
  const [streamError, setStreamError] = useState('')

  useEffect(() => {
    if (!props.open || !props.audit?.id || !props.audit.content_available) return
    const controller = new AbortController()
    setContent('')
    setStreamError('')
    setStreamTruncated(false)
    setStreaming(true)
    void streamRequestContent(
      props.audit.id,
      (next) => setContent(next),
      controller.signal,
      (truncated) => setStreamTruncated(truncated)
    )
      .catch(() => {
        if (!controller.signal.aborted) {
          setStreamError(t('Request failed'))
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setStreaming(false)
      })
    return () => controller.abort()
  }, [props.audit?.content_available, props.audit?.id, props.open, t])

  const displayContent = useMemo(() => formatContent(content), [content])

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Request Content')}
      description={props.audit?.request_id}
      contentClassName='sm:max-w-5xl'
      contentHeight='min(80dvh, 760px)'
      footer={
        props.audit ? (
          <div className='flex w-full items-center justify-between gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => props.onOpenUsageLog(props.audit?.request_id ?? '')}
            >
              <ExternalLink className='mr-2 size-4' />
              {t('Go to Usage Log')}
            </Button>
            <Button variant='outline' size='sm' onClick={() => props.onOpenChange(false)}>
              {t('Close')}
            </Button>
          </div>
        ) : null
      }
    >
      {props.isLoading || !props.audit ? (
        <div className='text-muted-foreground py-12 text-center'>{t('Loading...')}</div>
      ) : (
        <div className='space-y-4'>
          <div className='grid gap-2 text-xs sm:grid-cols-4'>
            <MetaCell label={t('Request ID')} value={props.audit.request_id} mono />
            <MetaCell label={t('Model')} value={props.audit.model_name || '-'} />
            <MetaCell label={t('Content Type')} value={props.audit.content_type || '-'} />
            <MetaCell label={t('Stored Size')} value={formatBytes(props.audit.stored_size)} />
          </div>
          <Separator />
          {!props.audit.content_available ? (
            <div className='text-muted-foreground rounded-md border p-4 text-sm'>
              {t('Request content is unavailable')}
            </div>
          ) : (
            <pre className='bg-muted/30 max-h-[48dvh] min-h-48 overflow-auto rounded-md border p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all'>
              {streamError || displayContent || (streaming ? t('Loading...') : '')}
              {streamTruncated && `\n\n${t('Content view is truncated')}`}
            </pre>
          )}
          {props.audit.assets.length > 0 && (
            <div className='space-y-2'>
              <div className='text-sm font-semibold'>{t('Multimodal Assets')}</div>
              <div className='flex flex-wrap gap-2'>
                {props.audit.assets.map((asset) => (
                  <RequestContentAssetPreview
                    key={asset.asset_key}
                    auditId={props.audit?.id ?? 0}
                    asset={asset}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </Dialog>
  )
}

function MetaCell(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground'>{props.label}</div>
      <div className={cn('truncate', props.mono && 'font-mono')} title={props.value}>
        {props.value}
      </div>
    </div>
  )
}

function formatContent(content: string): string {
  if (!content) return ''
  try {
    return JSON.stringify(JSON.parse(content), null, 2)
  } catch {
    return content
  }
}

function formatDate(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
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
