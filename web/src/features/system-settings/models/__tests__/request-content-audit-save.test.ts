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
// web/src/features/system-settings/models/__tests__/request-content-audit-save.test.ts
// 验证请求内容审计设置保存遇到业务失败时会停止后续提交。

import { deepStrictEqual, strictEqual } from 'node:assert/strict'
import { test } from 'node:test'

import { saveRequestContentAuditOptions } from '../request-content-audit-save'

test('stops submitting options after the first business failure', async () => {
  const calls: string[] = []
  const result = await saveRequestContentAuditOptions(
    [
      { key: 'request_content_audit.enabled', value: 'true' },
      { key: 'request_content_audit.allow_user_view', value: 'true' },
    ],
    async (entry) => {
      calls.push(entry.key)
      return { success: false }
    }
  )

  strictEqual(result, false)
  deepStrictEqual(calls, ['request_content_audit.enabled'])
})

test('submits all options when every update succeeds', async () => {
  const calls: string[] = []
  const result = await saveRequestContentAuditOptions(
    [
      { key: 'request_content_audit.enabled', value: 'false' },
      { key: 'request_content_audit.allow_user_view', value: 'false' },
    ],
    async (entry) => {
      calls.push(entry.key)
      return { success: true }
    }
  )

  strictEqual(result, true)
  deepStrictEqual(calls, [
    'request_content_audit.enabled',
    'request_content_audit.allow_user_view',
  ])
})
