// web/src/features/request-records/lib/request-content-dialog-state.ts
// 规范化请求记录 ID，并统一详情弹窗的加载、成功和失败状态。

import type {
  RequestContentAuditDetail,
  RequestContentAuditSummary,
} from '../types'

export type RequestContentAuditDialogAudit =
  | RequestContentAuditSummary
  | RequestContentAuditDetail

export type RequestContentAuditDialogState = {
  audit: RequestContentAuditDialogAudit | null
  phase: 'idle' | 'loading' | 'error' | 'ready'
}

export function normalizeRequestContentAuditId(value: unknown): number | null {
  const id = typeof value === 'number' ? value : Number(value)
  if (!Number.isSafeInteger(id) || id <= 0) return null
  return id
}

export function resolveRequestContentAuditDialogState(
  selectedAudit: RequestContentAuditSummary | null,
  detail: RequestContentAuditDetail | null | undefined,
  isPending: boolean,
  isError: boolean
): RequestContentAuditDialogState {
  if (!selectedAudit && !detail) {
    return { audit: null, phase: 'idle' }
  }
  if (detail) {
    return { audit: detail, phase: 'ready' }
  }
  if (isError) {
    return { audit: selectedAudit, phase: 'error' }
  }
  if (isPending) {
    return { audit: selectedAudit, phase: 'loading' }
  }
  return { audit: selectedAudit, phase: 'loading' }
}
