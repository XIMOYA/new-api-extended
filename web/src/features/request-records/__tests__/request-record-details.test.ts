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
// web/src/features/request-records/__tests__/request-record-details.test.ts
// 验证请求记录详情打开时的 ID 规范化和错误状态降级行为。

import { deepStrictEqual, strictEqual } from 'node:assert/strict'
import { test } from 'node:test'

import {
  normalizeRequestContentAuditId,
  resolveRequestContentAuditDialogState,
} from '../lib/request-content-dialog-state'
import type {
  RequestContentAuditDetail,
  RequestContentAuditSummary,
} from '../types'

const summary: RequestContentAuditSummary = {
  id: 42,
  request_id: 'req_42',
  user_id: 7,
  model_name: 'gpt-5.6-luna',
  request_type: 'openai_responses',
  relay_format: 'openai_responses',
  endpoint_path: '/v1/responses',
  content_type: 'application/json',
  created_at: 1,
  expires_at: 2,
  content_size: 128,
  stored_size: 256,
  message_count: 1,
  asset_count: 0,
  capture_status: 'complete',
  normalization_version: 1,
  content_available: true,
  can_view_full_content: true,
  is_redacted: false,
}

test('normalizes numeric request record IDs before detail lookup', () => {
  strictEqual(normalizeRequestContentAuditId('42'), 42)
  strictEqual(normalizeRequestContentAuditId(42), 42)
  strictEqual(normalizeRequestContentAuditId('not-a-number'), null)
  strictEqual(normalizeRequestContentAuditId(0), null)
})

test('keeps the selected summary visible when the detail request fails', () => {
  const state = resolveRequestContentAuditDialogState(
    summary,
    null,
    false,
    true
  )

  strictEqual(state.phase, 'error')
  deepStrictEqual(state.audit, summary)
})

test('uses the detail response after it loads successfully', () => {
  const detail: RequestContentAuditDetail = {
    ...summary,
    assets: [],
  }
  const state = resolveRequestContentAuditDialogState(
    summary,
    detail,
    false,
    false
  )

  strictEqual(state.phase, 'ready')
  deepStrictEqual(state.audit, detail)
})
