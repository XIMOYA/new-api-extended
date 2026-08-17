// web/src/features/request-records/types.ts
// 请求记录页面使用的元数据、资源和流式内容类型。

export type RequestContentAuditSummary = {
  id: number
  request_id: string
  upstream_request_id?: string
  user_id: number
  username?: string
  token_id?: number
  channel_id?: number
  model_name: string
  request_type: string
  relay_format: string
  endpoint_path: string
  content_type: string
  created_at: number
  expires_at: number
  content_size: number
  stored_size: number
  message_count: number
  asset_count: number
  capture_status: string
  normalization_version: number
  content_available: boolean
  can_view_full_content: boolean
  is_redacted: boolean
}

export type RequestContentAuditAsset = {
  asset_key: string
  file_name?: string
  asset_type: string
  mime_type: string
  original_size: number
  width?: number
  height?: number
  thumbnail_available: boolean
  original_available: boolean
}

export type RequestContentAuditDetail = RequestContentAuditSummary & {
  assets: RequestContentAuditAsset[]
}

export type RequestContentAuditViewSummary = {
  input_item_count: number
  message_count: number
  sections_truncated: boolean
  // 投影只保留序列头尾两段时中间跳过的条数，没跳过时为 0。
  omitted_section_count: number
  tool_call_count: number
  tool_output_count: number
  reasoning_count: number
  advanced_field_count: number
  opaque_bytes: number
  dropped_bytes: number
  asset_count: number
}

// 按保留策略丢弃的一个分类，kind 取值见 lib/retention.ts。
export type RequestContentAuditViewRetentionCategory = {
  kind: string
  count: number
  bytes: number
}

export type RequestContentAuditViewRetention = {
  schema: number
  kept_bytes: number
  dropped_bytes: number
  original_sha256?: string
  original_size?: number
  dropped?: RequestContentAuditViewRetentionCategory[]
}

export type RequestContentAuditViewSection = {
  id: string
  kind: string
  type?: string
  role?: string
  call_id?: string
  output_type?: string
  title: string
  preview?: string
  content?: string
  content_format?: 'text' | 'json'
  content_size: number
  opaque_bytes?: number
  opaque_hash?: string
  opaque: boolean
  dropped?: boolean
  dropped_kind?: string
  dropped_bytes?: number
  expandable: boolean
  truncated: boolean
  asset_keys?: string[]
}

export type RequestContentAuditView = {
  request_id: string
  schema_version?: number
  relay_format: string
  endpoint_path?: string
  kind: string
  source_size: number
  stored_size: number
  projection_available: boolean
  projection_message?: string
  retention?: RequestContentAuditViewRetention
  summary: RequestContentAuditViewSummary
  sections: RequestContentAuditViewSection[]
}

export type RequestContentAuditPage = {
  page: number
  page_size: number
  total: number
  items: RequestContentAuditSummary[]
}

// 对应 service/request_content_audit_highlight.go：这条记录里最后一条用户消息的定位信息。
// available 为 false 时只有 request_id 和 message 可用（正文不可用、raw/multipart 记录等）。
export type RequestContentAuditHighlight = {
  request_id: string
  available: boolean
  section_id?: string
  section_index?: number
  role?: string
  preview?: string
  content_size?: number
  message_count?: number
  section_count?: number
  truncated?: boolean
  message?: string
}

export type RequestContentAuditPreview = {
  request_id: string
  content: string
  truncated: boolean
  redacted: boolean
}

export type RequestContentAuditListParams = {
  page?: number
  page_size?: number
  request_id?: string
  model_name?: string
  request_type?: string
  start_timestamp?: number
  end_timestamp?: number
}
