// web/src/features/request-records/components/request-content-view.tsx
// 请求内容结构化视图：按消息、工具调用、工具结果和高级参数分组，展开区块时才读取受限正文，
// 并区分「加密内容」与「按保留策略未保存的内容」。
// 另外提供简洁模式（只看用户输入与 AI 输出）和本轮最新用户消息的定位卡。

import { useQuery } from '@tanstack/react-query'
import {
  Brain,
  ChevronDown,
  FileJson,
  Info,
  Loader2,
  MessageSquare,
  Wrench,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { CodeBlock } from '@/components/ai-elements/code-block'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { getRequestContentViewSection } from '@/features/request-records/api'

import {
  findRequestContentLatestUserMessage,
  readRequestContentConciseView,
  selectRequestContentConciseSections,
  writeRequestContentConciseView,
} from '../lib/concise-view'
import {
  formatRequestContentBytes as formatBytes,
  resolveRequestContentDropped,
} from '../lib/retention'
import { buildRequestContentSectionRows } from '../lib/section-gaps'
import type {
  RequestContentAuditView,
  RequestContentAuditViewSection,
} from '../types'
import {
  RequestContentConciseToggle,
  RequestContentHiddenSummary,
} from './request-content-concise'
import { RequestContentLatestMessageCard } from './request-content-latest-message'
import { RequestContentOmittedSectionsNotice } from './request-content-omitted-sections'
import {
  RequestContentDroppedBadge,
  RequestContentDroppedNotice,
  RequestContentRetentionSummary,
} from './request-content-retention'

export function RequestContentView(props: {
  auditId: number
  view: RequestContentAuditView
}) {
  const { t } = useTranslation()
  const [concise, setConcise] = useState(readRequestContentConciseView)
  // token 每次点击都自增，同一个区块反复定位也能重新滚动。
  const [focus, setFocus] = useState<{
    sectionId: string
    token: number
  } | null>(null)
  const conciseSelection = useMemo(
    () => selectRequestContentConciseSections(props.view.sections),
    [props.view.sections]
  )
  const latestUserMessage = useMemo(
    () =>
      findRequestContentLatestUserMessage(
        props.view.sections,
        props.view.summary.input_item_count
      ),
    [props.view.sections, props.view.summary.input_item_count]
  )
  const summaryItems = [
    [t('Input Items'), props.view.summary.input_item_count],
    [t('Messages'), props.view.summary.message_count],
    [t('Tool Calls'), props.view.summary.tool_call_count],
    [t('Tool Results'), props.view.summary.tool_output_count],
    [t('Reasoning'), props.view.summary.reasoning_count],
    [t('Advanced'), props.view.summary.advanced_field_count],
  ] as const

  if (!props.view.projection_available) {
    return (
      <div className='border-border bg-muted/20 flex min-h-40 flex-col items-center justify-center gap-2 rounded-md border p-6 text-center'>
        <Info className='text-muted-foreground size-5' />
        <p className='text-muted-foreground text-sm'>
          {t('No structured view available')}
        </p>
        {props.view.projection_message && (
          <p className='text-muted-foreground max-w-xl text-xs'>
            {getProjectionMessage(props.view.projection_message, t)}
          </p>
        )}
      </div>
    )
  }

  // 图片生成、raw/multipart 这类记录压根没有对话区块，简洁模式不该把它们清空。
  const conciseApplied = concise && conciseSelection.sections.length > 0
  const visibleSections = conciseApplied
    ? conciseSelection.sections
    : props.view.sections
  const sectionRows = buildRequestContentSectionRows(
    props.view.sections,
    visibleSections
  )
  const changeConcise = (next: boolean) => {
    setConcise(next)
    writeRequestContentConciseView(next)
  }

  return (
    <div className='space-y-3'>
      <div className='grid gap-2 sm:grid-cols-3 lg:grid-cols-6'>
        {summaryItems.map(([label, value]) => (
          <div
            className='border-border bg-muted/15 min-w-0 rounded-md border px-3 py-2'
            key={label}
          >
            <div className='text-muted-foreground truncate text-[11px]'>
              {label}
            </div>
            <div className='font-mono text-sm tabular-nums'>{value}</div>
          </div>
        ))}
      </div>

      {props.view.retention && (
        <RequestContentRetentionSummary retention={props.view.retention} />
      )}

      {props.view.summary.sections_truncated && (
        <div className='rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-700 dark:text-amber-300'>
          {t('Content view is truncated')}
        </div>
      )}

      {latestUserMessage && (
        <RequestContentLatestMessageCard
          latest={latestUserMessage}
          onFocusSection={(sectionId) =>
            setFocus((current) => ({
              sectionId,
              token: (current?.token ?? 0) + 1,
            }))
          }
        />
      )}

      <div className='border-border bg-muted/10 flex flex-wrap items-center justify-between gap-x-3 gap-y-2 rounded-md border px-3 py-2 text-xs'>
        <div className='flex min-w-0 items-center gap-2'>
          <FileJson className='text-muted-foreground size-4 shrink-0' />
          <span className='text-muted-foreground truncate'>
            {[
              props.view.relay_format || props.view.kind || t('Content'),
              props.view.endpoint_path,
            ]
              .filter(Boolean)
              .join(' · ')}
          </span>
        </div>
        <div className='flex items-center gap-3'>
          <RequestContentConciseToggle
            checked={concise}
            onCheckedChange={changeConcise}
          />
          <span className='text-muted-foreground shrink-0 font-mono'>
            {formatBytes(props.view.source_size)}
          </span>
        </div>
      </div>

      <div className='space-y-2'>
        {sectionRows.map((row) =>
          row.kind === 'gap' ? (
            <RequestContentOmittedSectionsNotice
              key={`gap-${row.beforeSectionId ?? 'tail'}`}
              omitted={row.omitted}
            />
          ) : (
            <RequestContentViewSection
              auditId={props.auditId}
              focusToken={focus?.sectionId === row.section.id ? focus.token : 0}
              key={row.section.id}
              section={row.section}
            />
          )
        )}
      </div>

      {conciseApplied && conciseSelection.hidden.total > 0 && (
        <RequestContentHiddenSummary
          hidden={conciseSelection.hidden}
          onShowAll={() => changeConcise(false)}
        />
      )}
    </div>
  )
}

function RequestContentViewSection(props: {
  auditId: number
  focusToken: number
  section: RequestContentAuditViewSection
}) {
  const { t } = useTranslation()
  const defaultOpen = props.section.kind === 'message'
  const [open, setOpen] = useState(defaultOpen)
  const [requested, setRequested] = useState(false)
  const sectionQuery = useQuery({
    queryKey: ['request-content-view-section', props.auditId, props.section.id],
    queryFn: () =>
      getRequestContentViewSection(props.auditId, props.section.id),
    enabled: open && requested && props.section.expandable,
    staleTime: 10 * 60 * 1000,
  })
  const domId = `request-content-section-${props.section.id}`
  const rootRef = useRef<HTMLDivElement | null>(null)

  // 定位卡点过来时展开区块并滚到视口中间，等同于用户自己点开。
  useEffect(() => {
    if (!props.focusToken) return
    setOpen(true)
    setRequested(true)
    rootRef.current?.scrollIntoView({ block: 'center' })
  }, [props.focusToken])

  const label = getSectionLabel(props.section, t)
  const hasLoadedContent = Boolean(sectionQuery.data?.content)
  const preview = props.section.preview || t('No content')
  const opaque = sectionQuery.data?.opaque || props.section.opaque
  const dropped = resolveRequestContentDropped(props.section, sectionQuery.data)
  // 裁剪掉的内容不是加密内容，提示交给保留策略说明，正文位置继续展示还留着的部分。
  const content =
    opaque && !dropped.dropped
      ? t('Encrypted content is hidden')
      : sectionQuery.data?.content || preview
  const contentFormat = sectionQuery.data?.content_format || 'text'
  const icon = getSectionIcon(props.section.kind)

  if (!props.section.expandable) {
    return (
      <div
        className='border-border bg-muted/10 rounded-md border'
        id={domId}
        ref={rootRef}
      >
        <div className='flex items-center gap-2 px-3 py-2'>
          {icon}
          <span className='min-w-0 flex-1 truncate text-sm font-medium'>
            {label}
          </span>
          {dropped.bytes > 0 && (
            <RequestContentDroppedBadge bytes={dropped.bytes} />
          )}
          <StatusBadge
            label={formatBytes(
              props.section.opaque_bytes || props.section.content_size
            )}
            showDot={false}
            copyable={false}
            className='border-border/60 bg-muted/30 h-6 rounded-md border px-2 font-mono text-[11px]'
          />
        </div>
        <div className='border-border/70 text-muted-foreground border-t px-3 py-3 text-xs'>
          {dropped.dropped ? (
            <RequestContentDroppedNotice
              bytes={dropped.bytes}
              kind={dropped.kind}
            />
          ) : (
            t('Encrypted content is hidden')
          )}
        </div>
      </div>
    )
  }

  let sectionBody: ReactNode
  if (sectionQuery.isFetching && !hasLoadedContent) {
    sectionBody = (
      <div className='text-muted-foreground flex items-center gap-2 py-3 text-xs'>
        <Loader2 className='size-4 animate-spin' />
        {t('Loading...')}
      </div>
    )
  } else if (sectionQuery.isError) {
    sectionBody = (
      <div className='text-destructive py-2 text-xs'>{t('Request failed')}</div>
    )
  } else if (contentFormat === 'json' && hasLoadedContent) {
    sectionBody = (
      <CodeBlock
        code={content}
        defaultCollapsed
        language='json'
        maxExpandedLines={40}
        showToolbar={false}
      />
    )
  } else {
    sectionBody = (
      <>
        <pre className='bg-muted/30 max-h-64 overflow-auto rounded-md border p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap'>
          {content}
        </pre>
        {!requested && !props.section.opaque && (
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='mt-2'
            onClick={(event) => {
              event.stopPropagation()
              setRequested(true)
            }}
          >
            {t('Load Content')}
          </Button>
        )}
      </>
    )
  }

  return (
    <Collapsible
      className='border-border bg-muted/10 rounded-md border'
      id={domId}
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (nextOpen) setRequested(true)
      }}
      ref={rootRef}
    >
      <CollapsibleTrigger className='group flex w-full items-center gap-2 px-3 py-2 text-left'>
        {icon}
        <div className='min-w-0 flex-1'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='truncate text-sm font-medium'>{label}</span>
            {props.section.type && (
              <span className='text-muted-foreground shrink-0 font-mono text-[11px]'>
                {props.section.type}
              </span>
            )}
            {props.section.output_type && (
              <span className='text-muted-foreground shrink-0 font-mono text-[11px]'>
                {props.section.output_type}
              </span>
            )}
          </div>
          <div className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px]'>
            {props.section.role && (
              <span>{getRoleLabel(props.section.role, t)}</span>
            )}
            {props.section.call_id && (
              <span
                className='max-w-48 truncate font-mono'
                title={props.section.call_id}
              >
                {props.section.call_id}
              </span>
            )}
            {props.section.asset_keys?.length ? (
              <span title={props.section.asset_keys.join(', ')}>
                {props.section.asset_keys.length} {t('Assets')}
              </span>
            ) : null}
            <span>{formatBytes(props.section.content_size)}</span>
            {props.section.opaque && (
              <span>
                {formatBytes(props.section.opaque_bytes || 0)} {t('Opaque')}
              </span>
            )}
            {dropped.bytes > 0 && (
              <span className='text-amber-700 dark:text-amber-300'>
                {formatBytes(dropped.bytes)} {t('Not Kept')}
              </span>
            )}
          </div>
        </div>
        <ChevronDown className='text-muted-foreground size-4 shrink-0 transition-transform group-data-[panel-open]:rotate-180' />
      </CollapsibleTrigger>
      <CollapsibleContent className='border-border/70 border-t px-3 py-3'>
        {dropped.dropped && (
          <RequestContentDroppedNotice
            bytes={dropped.bytes}
            className='mb-2'
            kind={dropped.kind}
          />
        )}
        {sectionBody}
        {sectionQuery.data?.truncated && (
          <div className='text-muted-foreground mt-2 text-[11px]'>
            {t('Content view is truncated')}
          </div>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}

function getSectionLabel(
  section: RequestContentAuditViewSection,
  t: (key: string) => string
): string {
  switch (section.kind) {
    case 'message':
      return getRoleLabel(section.role, t)
    case 'tool_call':
      return section.title
        ? `${t('Function Call')}: ${section.title}`
        : t('Function Call')
    case 'tool_output':
      return t('Function Result')
    case 'reasoning':
      return t('Reasoning')
    case 'tools':
      return t('Tools')
    case 'metadata':
      return t('Metadata')
    case 'instructions':
      return t('Instructions')
    case 'item':
      return t('Input')
    case 'field':
      return section.title || t('Other Parameters')
    default:
      return section.title || t('Other Parameters')
  }
}

function getRoleLabel(role: string | undefined, t: (key: string) => string) {
  switch (role) {
    case 'user':
      return t('User')
    case 'assistant':
      return t('Assistant')
    case 'system':
      return t('System')
    case 'developer':
      return t('Developer')
    case 'tool':
      return t('Tool')
    default:
      return role || t('Message')
  }
}

function getProjectionMessage(
  message: string,
  t: (key: string) => string
): string {
  switch (message) {
    case 'source_too_large':
      return t('Structured view unavailable for large content')
    case 'content_unavailable':
      return t('Request content is unavailable')
    default:
      return t('No structured view available')
  }
}

function getSectionIcon(kind: string) {
  switch (kind) {
    case 'message':
      return <MessageSquare className='text-muted-foreground size-4 shrink-0' />
    case 'tool_call':
    case 'tool_output':
    case 'tools':
      return <Wrench className='text-muted-foreground size-4 shrink-0' />
    case 'reasoning':
      return <Brain className='text-muted-foreground size-4 shrink-0' />
    default:
      return <FileJson className='text-muted-foreground size-4 shrink-0' />
  }
}
