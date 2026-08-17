// web/src/features/request-records/index.tsx
// 请求记录独立页面：分页展示元数据，每行带本轮最新用户消息摘要，点击后流式读取正文并懒加载多模态缩略图。

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft,
  ArrowRight,
  ExternalLink,
  RefreshCw,
  Search,
} from 'lucide-react'
import {
  useEffect,
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { ModelBadge } from '@/features/usage-logs/components/model-badge'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  getRequestContentAuditByRequestId,
  getRequestContentAudits,
  getRequestContentHighlight,
  getRequestContentView,
  streamRequestContent,
} from './api'
import { RequestContentAssetPreview } from './components/asset-preview'
import { RequestContentView } from './components/request-content-view'
import {
  normalizeRequestContentAuditId,
  resolveRequestContentAuditDialogState,
  type RequestContentAuditDialogAudit,
} from './lib/request-content-dialog-state'
import type { RequestContentAuditSummary } from './types'

const route = getRouteApi('/_authenticated/request-records/')
const EMPTY_REQUEST_RECORDS: RequestContentAuditSummary[] = []

export function RequestRecords() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = route.useSearch()
  const [requestIdInput, setRequestIdInput] = useState(search.requestId ?? '')
  const [selectedAudit, setSelectedAudit] =
    useState<RequestContentAuditSummary | null>(null)
  const [contentOpen, setContentOpen] = useState(false)
  const [highlightRequestId, setHighlightRequestId] = useState<string | null>(
    null
  )

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

  const selectedRequestId = selectedAudit?.request_id ?? null
  const detailQuery = useQuery({
    queryKey: ['request-content-audit', selectedRequestId],
    queryFn: () =>
      getRequestContentAuditByRequestId(selectedRequestId as string),
    enabled: contentOpen && selectedRequestId != null,
  })
  const dialogState = resolveRequestContentAuditDialogState(
    selectedAudit,
    detailQuery.data,
    detailQuery.isPending,
    detailQuery.isError
  )

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
    if (matching) setSelectedAudit(matching)
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
    setSelectedAudit(item)
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
          type='button'
          variant='outline'
          size='sm'
          onClick={() => void listQuery.refetch()}
          disabled={listQuery.isFetching}
        >
          <RefreshCw
            className={cn(
              'mr-2 size-4',
              listQuery.isFetching && 'animate-spin'
            )}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          <Card>
            <CardContent className='pt-4'>
              <form
                className='flex flex-col gap-2 sm:flex-row'
                onSubmit={submitRequestId}
              >
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
            <table className='w-full min-w-[1100px] text-[13px]'>
              <thead className='bg-muted/50 sticky top-0 z-10 border-b'>
                <tr className='text-muted-foreground text-left text-xs'>
                  <th className='px-3 py-2 font-medium'>{t('Time')}</th>
                  <th className='px-3 py-2 font-medium'>{t('User')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Model')}</th>
                  <th className='px-3 py-2 font-medium'>
                    {t('Latest User Message')}
                  </th>
                  <th className='px-3 py-2 font-medium'>{t('Request Type')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Size')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Status')}</th>
                  <th className='px-3 py-2 text-right font-medium'>
                    {t('Actions')}
                  </th>
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
                type='button'
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
                type='button'
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
        <RequestContentDialog
          audit={dialogState.audit}
          open={contentOpen}
          onOpenChange={setContentOpen}
          isLoading={dialogState.phase === 'loading'}
          isError={dialogState.phase === 'error'}
          onRetry={() => void detailQuery.refetch()}
          onOpenUsageLog={goToUsageLog}
        />
      </SectionPageLayout.Content>
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
        <td
          colSpan={8}
          className='text-muted-foreground px-3 py-12 text-center'
        >
          {t('Loading...')}
        </td>
      </tr>
    )
  }
  if (props.items.length === 0) {
    return (
      <tr>
        <td
          colSpan={8}
          className='text-muted-foreground px-3 py-12 text-center'
        >
          {t('No request records')}
        </td>
      </tr>
    )
  }
  return props.items.map((item) => (
    <RequestRecordsTableRow
      highlighted={props.highlightRequestId === item.request_id}
      item={item}
      key={item.id}
      onSelect={props.onSelect}
      onUsageLog={props.onUsageLog}
    />
  ))
}

function RequestRecordsTableRow(props: {
  item: RequestContentAuditSummary
  highlighted: boolean
  onSelect: (item: RequestContentAuditSummary) => void
  onUsageLog: (requestId: string) => void
}) {
  const { t } = useTranslation()
  const auditId = normalizeRequestContentAuditId(props.item.id)
  const contentReady =
    props.item.capture_status === 'complete' && props.item.content_available
  // 只给当前页渲染出来的行取摘要，正文不可用的记录直接跳过请求。
  const highlightQuery = useQuery({
    queryKey: ['request-content-highlight', props.item.id],
    queryFn: () => getRequestContentHighlight(auditId as number),
    enabled: auditId != null && contentReady,
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const latestMessage = highlightQuery.data?.available
    ? highlightQuery.data.preview?.trim() || ''
    : ''

  let statusLabel = t('Available')
  let statusVariant: 'success' | 'warning' | 'danger' = 'success'
  if (!contentReady) {
    statusLabel = t('Unavailable')
    statusVariant = 'danger'
  } else if (props.item.is_redacted) {
    statusLabel = t('Redacted')
    statusVariant = 'warning'
  }
  const displayUsername = props.item.username || `#${props.item.user_id}`
  const requestType = props.item.request_type || props.item.relay_format || '-'

  return (
    <tr
      className={cn(
        'hover:bg-muted/30 cursor-pointer border-b last:border-0',
        props.highlighted &&
          'bg-primary/5 animate-pulse ring-2 ring-inset ring-primary/40'
      )}
      onClick={() => props.onSelect(props.item)}
    >
      <td className='px-3 py-2.5 whitespace-nowrap'>
        <span className='font-mono text-xs tabular-nums'>
          {formatTimestampToDate(props.item.created_at)}
        </span>
      </td>
      <td className='px-3 py-2.5'>
        <div className='flex min-w-0 items-center gap-1.5'>
          <Avatar className='ring-border/60 size-6 shrink-0 ring-1'>
            <AvatarFallback
              className='text-[11px] font-semibold'
              style={getUserAvatarStyle(displayUsername)}
            >
              {getUserAvatarFallback(displayUsername)}
            </AvatarFallback>
          </Avatar>
          <div className='min-w-0'>
            <div
              className='max-w-[150px] truncate font-medium'
              title={displayUsername}
            >
              {displayUsername}
            </div>
            <div className='text-muted-foreground font-mono text-[11px]'>
              {props.item.request_id}
            </div>
          </div>
        </div>
      </td>
      <td className='px-3 py-2.5'>
        <div className='flex min-w-0 flex-col gap-0.5'>
          <ModelBadge
            modelName={props.item.model_name || '-'}
            className='max-w-[220px]'
          />
          <div
            className='text-muted-foreground max-w-[220px] truncate text-xs'
            title={props.item.endpoint_path}
          >
            {props.item.endpoint_path}
          </div>
        </div>
      </td>
      <td className='px-3 py-2.5'>
        {latestMessage && (
          <div
            className='text-muted-foreground max-w-[280px] min-w-[140px] truncate text-xs'
            data-slot='request-record-latest-message'
            title={latestMessage}
          >
            {latestMessage}
          </div>
        )}
      </td>
      <td className='px-3 py-2.5'>
        <StatusBadge
          label={requestType}
          autoColor={requestType}
          showDot={false}
          copyable={false}
          className='border-border/60 bg-muted/30 h-6 rounded-md border px-2 [font-family:var(--font-body)]'
        />
      </td>
      <td className='px-3 py-2.5 font-mono text-xs'>
        {formatBytes(props.item.content_size)}
        {props.item.asset_count > 0 &&
          ` · ${props.item.asset_count} ${t('assets')}`}
      </td>
      <td className='px-3 py-2.5'>
        <StatusBadge
          label={statusLabel}
          variant={statusVariant}
          showDot
          copyable={false}
        />
      </td>
      <td className='px-3 py-2.5 text-right'>
        <div className='flex justify-end gap-1'>
          <Button
            type='button'
            size='sm'
            variant='ghost'
            onClick={(event) => {
              event.stopPropagation()
              props.onSelect(props.item)
            }}
          >
            {t('View')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='ghost'
            onClick={(event) => {
              event.stopPropagation()
              props.onUsageLog(props.item.request_id)
            }}
          >
            <ExternalLink className='mr-1 size-3.5' />
            {t('Usage Log')}
          </Button>
        </div>
      </td>
    </tr>
  )
}

function RequestContentDialog(props: {
  audit: RequestContentAuditDialogAudit | null
  open: boolean
  onOpenChange: (open: boolean) => void
  isLoading: boolean
  isError: boolean
  onRetry: () => void
  onOpenUsageLog: (requestId: string) => void
}) {
  const { t } = useTranslation()
  const [content, setContent] = useState('')
  const [rawOpen, setRawOpen] = useState(false)
  const [streaming, setStreaming] = useState(false)
  const [streamTruncated, setStreamTruncated] = useState(false)
  const [streamError, setStreamError] = useState('')
  const detailAudit =
    props.audit && 'assets' in props.audit ? props.audit : null
  const auditId = detailAudit
    ? normalizeRequestContentAuditId(detailAudit.id)
    : null
  const contentAvailable = detailAudit?.content_available === true
  const viewQuery = useQuery({
    queryKey: ['request-content-audit-view', auditId],
    queryFn: () => getRequestContentView(auditId as number),
    enabled:
      props.open && auditId != null && contentAvailable && !props.isError,
    staleTime: 10 * 60 * 1000,
  })

  useEffect(() => {
    setRawOpen(false)
    setContent('')
    setStreamError('')
    setStreamTruncated(false)
    setStreaming(false)
  }, [auditId, props.open])

  useEffect(() => {
    if (props.open && viewQuery.isError && !rawOpen) {
      setRawOpen(true)
    }
  }, [props.open, rawOpen, viewQuery.isError])

  useEffect(() => {
    if (
      !props.open ||
      !rawOpen ||
      auditId == null ||
      !contentAvailable ||
      props.isError
    ) {
      return
    }
    const controller = new AbortController()
    setContent('')
    setStreamError('')
    setStreamTruncated(false)
    setStreaming(true)
    void streamRequestContent(
      auditId,
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
  }, [auditId, contentAvailable, props.isError, props.open, rawOpen, t])

  const displayContent = useMemo(() => formatContent(content), [content])
  const assets = detailAudit?.assets ?? []

  let dialogBody: ReactNode
  if (!props.audit) {
    dialogBody = (
      <div className='text-muted-foreground py-12 text-center'>
        {t('Loading...')}
      </div>
    )
  } else {
    const metadata = (
      <div className='grid gap-2 text-xs sm:grid-cols-4'>
        <MetaCell label={t('Request ID')} value={props.audit.request_id} mono />
        <MetaCell label={t('Model')} value={props.audit.model_name || '-'} />
        <MetaCell
          label={t('Content Type')}
          value={props.audit.content_type || '-'}
        />
        <MetaCell
          label={t('Stored Size')}
          value={formatBytes(props.audit.stored_size)}
        />
      </div>
    )

    if (props.isError) {
      dialogBody = (
        <div className='space-y-4'>
          {metadata}
          <Separator />
          <div className='border-destructive/30 bg-destructive/5 flex min-h-32 flex-col items-center justify-center gap-3 rounded-md border p-6 text-center'>
            <p className='text-destructive text-sm'>{t('Request failed')}</p>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={props.onRetry}
            >
              <RefreshCw className='mr-2 size-4' />
              {t('Refresh')}
            </Button>
          </div>
        </div>
      )
    } else {
      let contentBody: ReactNode
      if (props.isLoading && !detailAudit) {
        contentBody = (
          <div className='text-muted-foreground py-12 text-center'>
            {t('Loading...')}
          </div>
        )
      } else if (!props.audit.content_available || auditId == null) {
        contentBody = (
          <div className='text-muted-foreground rounded-md border p-4 text-sm'>
            {t('Request content is unavailable')}
          </div>
        )
      } else {
        const rawContentBody = (
          <pre className='bg-muted/30 max-h-[48dvh] min-h-48 overflow-auto rounded-md border p-3 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap'>
            {streamError ||
              displayContent ||
              (streaming ? t('Loading...') : '')}
            {streamTruncated && `\n\n${t('Content view is truncated')}`}
          </pre>
        )
        let structuredContentBody: ReactNode
        if (viewQuery.isPending) {
          structuredContentBody = (
            <div className='text-muted-foreground flex min-h-48 items-center justify-center rounded-md border p-6 text-sm'>
              {t('Loading...')}
            </div>
          )
        } else if (viewQuery.isError) {
          structuredContentBody = (
            <div className='border-destructive/30 bg-destructive/5 flex min-h-48 flex-col items-center justify-center gap-3 rounded-md border p-6 text-center'>
              <p className='text-destructive text-sm'>{t('Request failed')}</p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => void viewQuery.refetch()}
              >
                <RefreshCw className='mr-2 size-4' />
                {t('Refresh')}
              </Button>
            </div>
          )
        } else if (viewQuery.data) {
          structuredContentBody = (
            <RequestContentView auditId={auditId} view={viewQuery.data} />
          )
        } else {
          structuredContentBody = null
        }

        contentBody = (
          <div className='space-y-3'>
            <div className='flex flex-wrap gap-2'>
              <Button
                type='button'
                size='sm'
                variant={rawOpen ? 'outline' : 'secondary'}
                onClick={() => setRawOpen(false)}
              >
                {t('Structured View')}
              </Button>
              <Button
                type='button'
                size='sm'
                variant={rawOpen ? 'secondary' : 'outline'}
                onClick={() => setRawOpen(true)}
              >
                {t('Raw Content')}
              </Button>
            </div>
            {rawOpen ? rawContentBody : structuredContentBody}
          </div>
        )
      }

      dialogBody = (
        <div className='space-y-4'>
          {metadata}
          <Separator />
          {contentBody}
          {assets.length > 0 && auditId != null && (
            <div className='space-y-2'>
              <div className='text-sm font-semibold'>
                {t('Multimodal Assets')}
              </div>
              <div className='flex flex-wrap gap-2'>
                {assets.map((asset) => (
                  <RequestContentAssetPreview
                    key={asset.asset_key}
                    auditId={auditId}
                    asset={asset}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      )
    }
  }

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
              type='button'
              variant='outline'
              size='sm'
              onClick={() =>
                props.onOpenUsageLog(props.audit?.request_id ?? '')
              }
            >
              <ExternalLink className='mr-2 size-4' />
              {t('Go to Usage Log')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => props.onOpenChange(false)}
            >
              {t('Close')}
            </Button>
          </div>
        ) : null
      }
    >
      {dialogBody}
    </Dialog>
  )
}

function MetaCell(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground'>{props.label}</div>
      <div
        className={cn('truncate', props.mono && 'font-mono')}
        title={props.value}
      >
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

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(value) / Math.log(1024))
  )
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
