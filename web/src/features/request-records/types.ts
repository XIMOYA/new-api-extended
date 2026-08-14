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

export type RequestContentAuditPage = {
  page: number
  page_size: number
  total: number
  items: RequestContentAuditSummary[]
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
