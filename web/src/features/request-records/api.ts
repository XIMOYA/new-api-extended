// web/src/features/request-records/api.ts
// 请求记录列表、详情、预览、最新用户消息摘要、流式正文和二进制资源请求。

import { api, getFreshAuthHeaders } from '@/lib/api'

import type {
  RequestContentAuditDetail,
  RequestContentAuditHighlight,
  RequestContentAuditListParams,
  RequestContentAuditPage,
  RequestContentAuditPreview,
  RequestContentAuditView,
  RequestContentAuditViewSection,
} from './types'

const requestRecordsPath = '/api/request-records'
const requestContentDisplayLimit = 8 * 1024 * 1024

function buildQuery(params: RequestContentAuditListParams): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    search.set(key, String(value))
  }
  return search.toString()
}

export async function getRequestContentAudits(
  params: RequestContentAuditListParams = {}
): Promise<RequestContentAuditPage> {
  const { page = 1, ...rest } = params
  const queryParams = new URLSearchParams(
    buildQuery({ page_size: 20, ...rest })
  )
  queryParams.set('p', String(page))
  const response = await api.get(
    `${requestRecordsPath}/?${queryParams.toString()}`
  )
  return response.data.data as RequestContentAuditPage
}

export async function getRequestContentAudit(
  id: number
): Promise<RequestContentAuditDetail> {
  const response = await api.get(`${requestRecordsPath}/${id}`)
  return response.data.data as RequestContentAuditDetail
}

export async function getRequestContentAuditByRequestId(
  requestId: string
): Promise<RequestContentAuditDetail> {
  const response = await api.get(
    `${requestRecordsPath}/by-request-id/${encodeURIComponent(requestId)}`
  )
  return response.data.data as RequestContentAuditDetail
}

export async function getRequestContentView(
  id: number
): Promise<RequestContentAuditView> {
  const response = await api.get(`${requestRecordsPath}/${id}/view`)
  return response.data.data as RequestContentAuditView
}

export async function getRequestContentViewSection(
  id: number,
  sectionId: string
): Promise<RequestContentAuditViewSection> {
  const response = await api.get(
    `${requestRecordsPath}/${id}/view/sections/${encodeURIComponent(sectionId)}`
  )
  return response.data.data as RequestContentAuditViewSection
}

export async function getRequestContentPreview(
  id: number
): Promise<RequestContentAuditPreview> {
  const response = await api.get(`${requestRecordsPath}/${id}/preview`)
  return response.data.data as RequestContentAuditPreview
}

// 列表页每一行都会调它，属于装饰性信息：接口报错、业务失败或正文不可读时统一降级成
// "没有可展示的用户消息"，不弹 toast 也不让整页的错误处理介入。
export async function getRequestContentHighlight(
  id: number
): Promise<RequestContentAuditHighlight> {
  try {
    const response = await api.get(`${requestRecordsPath}/${id}/highlight`, {
      skipBusinessError: true,
      skipErrorHandler: true,
    })
    const highlight = response.data?.data as
      | RequestContentAuditHighlight
      | undefined
    if (!highlight || response.data?.success === false) {
      return { request_id: '', available: false }
    }
    return highlight
  } catch {
    return { request_id: '', available: false }
  }
}

export async function getRequestContentPreviewByRequestId(
  requestId: string
): Promise<RequestContentAuditPreview> {
  const response = await api.get(
    `${requestRecordsPath}/by-request-id/${encodeURIComponent(requestId)}/preview`
  )
  return response.data.data as RequestContentAuditPreview
}

export async function streamRequestContent(
  id: number,
  onChunk: (content: string) => void,
  signal?: AbortSignal,
  onTruncated?: (truncated: boolean) => void
): Promise<string> {
  const response = await fetch(`${requestRecordsPath}/${id}/content`, {
    credentials: 'include',
    headers: await getFreshAuthHeaders(),
    signal,
  })
  if (!response.ok) {
    throw new Error(`Request content stream failed: ${response.status}`)
  }

  const appendChunk = (content: string, chunk: string) => {
    if (!chunk) return content
    if (content.length >= requestContentDisplayLimit) {
      onTruncated?.(true)
      return content
    }
    const remaining = requestContentDisplayLimit - content.length
    const next = content + chunk.slice(0, remaining)
    if (chunk.length > remaining) onTruncated?.(true)
    onChunk(next)
    return next
  }

  if (!response.body) {
    return appendChunk('', await response.text())
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let content = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    content = appendChunk(content, decoder.decode(value, { stream: true }))
  }
  content = appendChunk(content, decoder.decode())
  return content
}

export async function getRequestContentAssetBlob(
  id: number,
  assetKey: string,
  variant: 'thumbnail' | 'original'
): Promise<Blob> {
  const response = await api.get(
    `${requestRecordsPath}/${id}/assets/${encodeURIComponent(assetKey)}/${variant}`,
    { responseType: 'blob' }
  )
  return response.data as Blob
}
