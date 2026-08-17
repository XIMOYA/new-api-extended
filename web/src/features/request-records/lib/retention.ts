// web/src/features/request-records/lib/retention.ts
// 请求内容保留策略：把后端裁剪分类映射成 i18n 词条，合并区块列表与区块详情里的裁剪信息，
// 并统一这一块的字节数展示格式。分类常量对应 service/request_content_audit_prune.go。

import type { RequestContentAuditViewSection } from '../types'

type RequestContentDroppedFields = Pick<
  RequestContentAuditViewSection,
  'dropped' | 'dropped_kind' | 'dropped_bytes'
>

export type RequestContentDroppedState = {
  dropped: boolean
  kind?: string
  bytes: number
}

export const requestContentDroppedKindLabelKeys: Record<string, string> = {
  tools_schema: 'Tool Definitions',
  tool_call: 'Tool Call Arguments',
  tool_result: 'Tool Execution Results',
  reasoning_opaque: 'Encrypted Reasoning',
  signature: 'Signature',
}

// 后端以后加了新分类也不至于显示空白，直接回落到原始 kind 字符串。
export function getRequestContentDroppedKindLabel(
  kind: string | undefined,
  t: (key: string) => string
): string {
  if (!kind) return t('Content')
  const labelKey = requestContentDroppedKindLabelKeys[kind]
  return labelKey ? t(labelKey) : kind
}

// 列表接口和单区块详情接口都可能带裁剪信息，详情优先，避免展开后提示消失。
export function resolveRequestContentDropped(
  section: RequestContentDroppedFields,
  detail?: RequestContentDroppedFields | null
): RequestContentDroppedState {
  const bytes = detail?.dropped_bytes || section.dropped_bytes || 0
  return {
    dropped: Boolean(detail?.dropped || section.dropped || bytes > 0),
    kind: detail?.dropped_kind || section.dropped_kind,
    bytes,
  }
}

export function formatRequestContentBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(value) / Math.log(1024))
  )
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
