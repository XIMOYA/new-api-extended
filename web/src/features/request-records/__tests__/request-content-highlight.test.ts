// web/src/features/request-records/__tests__/request-content-highlight.test.ts
// 最新用户消息摘要接口封装：成功时原样返回，HTTP 失败或业务失败时静默降级成"没有摘要"，
// 并确认请求带上了跳过全局错误处理的配置（列表页每行都会调，绝不能弹 toast 或跳 500 页）。

import assert from 'node:assert/strict'
import { after, test } from 'node:test'

const { api } = await import('@/lib/api')
const { getRequestContentHighlight } = await import('../api')

type RecordedRequest = { url: string; config?: Record<string, unknown> }

const requests: RecordedRequest[] = []
const originalGet = api.get

after(() => {
  api.get = originalGet
})

function stubGet(handler: () => Promise<unknown>) {
  api.get = ((url: string, config?: Record<string, unknown>) => {
    requests.push({ url, config })
    return handler()
  }) as typeof api.get
}

test('returns the highlight payload and skips the global error handlers', async () => {
  requests.length = 0
  stubGet(async () => ({
    data: {
      success: true,
      data: {
        request_id: '20260817091115356562042008268d9d6wwLRGfe',
        available: true,
        section_id: 'list-bWVzc2FnZXM-516',
        section_index: 517,
        role: 'user',
        preview: '帮我看下这个 429 的问题',
        content_size: 1234,
        message_count: 525,
        section_count: 525,
        truncated: true,
      },
    },
  }))

  const highlight = await getRequestContentHighlight(42)

  assert.equal(highlight.available, true)
  assert.equal(highlight.preview, '帮我看下这个 429 的问题')
  assert.equal(highlight.section_id, 'list-bWVzc2FnZXM-516')
  assert.equal(requests.length, 1)
  assert.equal(requests[0]?.url, '/api/request-records/42/highlight')
  assert.equal(requests[0]?.config?.skipBusinessError, true)
  assert.equal(requests[0]?.config?.skipErrorHandler, true)
})

test('degrades to no highlight when the request fails', async () => {
  requests.length = 0
  stubGet(async () => {
    throw new Error('network down')
  })

  const highlight = await getRequestContentHighlight(42)

  assert.deepEqual(highlight, { request_id: '', available: false })
  assert.equal(requests.length, 1)
})

test('degrades to no highlight when the server reports a business failure', async () => {
  requests.length = 0
  stubGet(async () => ({
    data: { success: false, message: 'request record not found' },
  }))

  const highlight = await getRequestContentHighlight(7)

  assert.deepEqual(highlight, { request_id: '', available: false })
})
