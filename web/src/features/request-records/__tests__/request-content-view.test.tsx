// web/src/features/request-records/__tests__/request-content-view.test.tsx
// 结构化请求内容视图：验证默认不读取完整区块，展开或主动加载时才请求正文，
// 并区分加密区块与按保留策略未保存的区块。

import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

import { requestContentConciseViewStorageKey } from '../lib/concise-view'
import type { RequestContentAuditView } from '../types'

const bunTestModule = ['bun', 'test'].join(':')
const { mock } = await import(bunTestModule)

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'localStorage',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'customElements',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: () => ({
    matches: false,
    media: '',
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  }),
})

const sectionRequests: string[] = []
// 这个文件验证的是完整视图（工具结果、推理区块都要在场），所以先明确关掉简洁模式，
// 不依赖同一进程里别的测试文件留下的开关状态。
localStorage.setItem(requestContentConciseViewStorageKey, 'false')

// 只替换区块正文接口，其余导出保持真实实现，避免同一进程里的其它测试文件拿不到导出。
const actualApi = await import('@/features/request-records/api')
mock.module('@/features/request-records/api', () => ({
  ...actualApi,
  getRequestContentViewSection: async (auditId: number, sectionId: string) => {
    sectionRequests.push(`${auditId}:${sectionId}`)
    return {
      id: sectionId,
      kind: sectionId === 'input-1' ? 'tool_output' : 'message',
      title: sectionId === 'input-1' ? 'Skill' : 'user',
      content: JSON.stringify({ loaded: true, sectionId }),
      content_format: 'json',
      content_size: 128,
      opaque: false,
      expandable: true,
      truncated: false,
    }
  },
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { RequestContentView } =
  await import('../components/request-content-view')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {},
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  mock.restore()
  domWindow.close()
})

const baseSummary = {
  input_item_count: 1,
  message_count: 0,
  sections_truncated: false,
  omitted_section_count: 0,
  tool_call_count: 0,
  tool_output_count: 0,
  reasoning_count: 0,
  advanced_field_count: 0,
  opaque_bytes: 0,
  dropped_bytes: 0,
  asset_count: 0,
}

function createView(
  overrides: Partial<RequestContentAuditView> = {}
): RequestContentAuditView {
  return {
    request_id: 'req-1',
    relay_format: 'openai_responses',
    kind: 'json',
    source_size: 130 * 1024,
    stored_size: 80 * 1024,
    projection_available: true,
    summary: baseSummary,
    sections: [],
    ...overrides,
  }
}

async function renderView(view: RequestContentAuditView) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <RequestContentView auditId={42} view={view} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  return {
    container,
    cleanup: async () => {
      await act(async () => root.unmount())
      container.remove()
      queryClient.clear()
    },
  }
}

test('renders classified sections without loading full content by default', async () => {
  sectionRequests.length = 0
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <RequestContentView
            auditId={42}
            view={{
              request_id: 'req-1',
              relay_format: 'openai_responses',
              kind: 'json',
              source_size: 130 * 1024,
              stored_size: 80 * 1024,
              projection_available: true,
              summary: {
                input_item_count: 3,
                message_count: 1,
                sections_truncated: false,
                omitted_section_count: 0,
                tool_call_count: 1,
                tool_output_count: 1,
                reasoning_count: 0,
                advanced_field_count: 1,
                opaque_bytes: 0,
                dropped_bytes: 0,
                asset_count: 0,
              },
              sections: [
                {
                  id: 'input-0',
                  kind: 'message',
                  role: 'user',
                  title: 'user',
                  preview: '生成图片',
                  content_size: 32,
                  opaque: false,
                  expandable: true,
                  truncated: false,
                },
                {
                  id: 'input-1',
                  kind: 'tool_output',
                  type: 'function_call_output',
                  title: 'Function result',
                  preview: 'skill docs preview',
                  content_size: 100 * 1024,
                  opaque: false,
                  expandable: true,
                  truncated: false,
                },
                {
                  id: 'input-2',
                  kind: 'reasoning',
                  type: 'reasoning',
                  title: 'Encrypted reasoning',
                  preview: 'Encrypted reasoning content is hidden',
                  content_size: 70 * 1024,
                  opaque: true,
                  expandable: false,
                  truncated: false,
                },
              ],
            }}
          />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  assert.equal(container.textContent?.includes('生成图片'), true)
  assert.equal(container.textContent?.includes('skill docs preview'), false)
  assert.equal(
    container.textContent?.includes('Encrypted content is hidden'),
    true
  )
  assert.deepEqual(sectionRequests, [])

  const loadButton = [...container.querySelectorAll('button')].find(
    (button) => button.textContent?.trim() === 'Load Content'
  )
  assert.ok(loadButton)
  await act(async () => {
    loadButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
    await Promise.resolve()
  })
  assert.deepEqual(sectionRequests, ['42:input-0'])

  const toolOutputTrigger = [...container.querySelectorAll('button')].find(
    (button) => button.textContent?.includes('Function Result')
  )
  assert.ok(toolOutputTrigger)
  await act(async () => {
    toolOutputTrigger?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
    await Promise.resolve()
  })
  assert.deepEqual(sectionRequests, ['42:input-0', '42:input-1'])

  await act(async () => root.unmount())
  container.remove()
})

test('explains the retention policy instead of showing the encrypted placeholder for dropped sections', async () => {
  sectionRequests.length = 0
  const { container, cleanup } = await renderView(
    createView({
      summary: {
        ...baseSummary,
        reasoning_count: 1,
        opaque_bytes: 280 * 1024,
        dropped_bytes: 280 * 1024,
      },
      sections: [
        {
          id: 'input-0',
          kind: 'reasoning',
          type: 'reasoning',
          title: 'Encrypted reasoning',
          preview: '{"audit_dropped":true,"audit_kind":"reasoning_opaque"}',
          content_size: 280 * 1024,
          opaque_bytes: 280 * 1024,
          opaque: true,
          dropped: true,
          dropped_kind: 'reasoning_opaque',
          dropped_bytes: 280 * 1024,
          expandable: false,
          truncated: false,
        },
      ],
    })
  )

  const notice = container.querySelector(
    '[data-slot="request-content-dropped-notice"]'
  )
  assert.ok(notice)
  assert.equal(
    notice.textContent?.includes('Content not kept by retention policy'),
    true
  )
  assert.equal(notice.textContent?.includes('Encrypted Reasoning'), true)
  assert.equal(notice.textContent?.includes('280.0 KB'), true)
  assert.equal(
    container.textContent?.includes('Encrypted content is hidden'),
    false
  )

  const droppedBadge = [
    ...container.querySelectorAll('[data-slot="status-badge"]'),
  ].find((element) => element.textContent?.includes('Not Kept'))
  assert.ok(droppedBadge)
  assert.equal(droppedBadge.textContent, '280.0 KB Not Kept')

  await cleanup()
})

test('keeps a dropped section expandable so the remaining content still loads', async () => {
  sectionRequests.length = 0
  const { container, cleanup } = await renderView(
    createView({
      summary: { ...baseSummary, tool_call_count: 1, dropped_bytes: 5600 },
      sections: [
        {
          id: 'input-1',
          kind: 'tool_call',
          type: 'function_call',
          call_id: 'call_1',
          title: 'get_weather',
          preview: '{"audit_dropped":true,"audit_kind":"tool_call"}',
          content_size: 6 * 1024,
          opaque: false,
          dropped: true,
          dropped_kind: 'tool_call',
          dropped_bytes: 5600,
          expandable: true,
          truncated: false,
        },
      ],
    })
  )

  const trigger = [...container.querySelectorAll('button')].find((button) =>
    button.textContent?.includes('get_weather')
  )
  assert.ok(trigger)
  assert.deepEqual(sectionRequests, [])

  await act(async () => {
    trigger.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
    await Promise.resolve()
  })

  assert.deepEqual(sectionRequests, ['42:input-1'])
  const notice = container.querySelector(
    '[data-slot="request-content-dropped-notice"]'
  )
  assert.ok(notice)
  assert.equal(notice.textContent?.includes('Tool Call Arguments'), true)
  assert.equal(notice.textContent?.includes('5.5 KB'), true)

  await cleanup()
})

test('lists the retention breakdown in the overview when the record carries retention stats', async () => {
  sectionRequests.length = 0
  const originalHash = `3f2b${'a'.repeat(60)}`
  const { container, cleanup } = await renderView(
    createView({
      retention: {
        schema: 1,
        kept_bytes: 15_234,
        dropped_bytes: 983_643,
        original_sha256: originalHash,
        original_size: 998_877,
        dropped: [
          { kind: 'tool_result', count: 12, bytes: 650_000 },
          { kind: 'reasoning_opaque', count: 8, bytes: 280_000 },
        ],
      },
      summary: { ...baseSummary, dropped_bytes: 983_643 },
    })
  )

  const retentionRow = container.querySelector(
    '[data-slot="request-content-retention"]'
  )
  assert.ok(retentionRow)
  const retentionText = retentionRow.textContent ?? ''
  assert.equal(retentionText.includes('Retention Policy'), true)
  assert.equal(retentionText.includes('Original Size: 975.5 KB'), true)
  assert.equal(retentionText.includes('Kept: 14.9 KB'), true)
  assert.equal(retentionText.includes('Not Kept: 960.6 KB'), true)
  assert.equal(
    retentionText.includes('Tool Execution Results · 12 Items · 634.8 KB'),
    true
  )
  assert.equal(
    retentionText.includes('Encrypted Reasoning · 8 Items · 273.4 KB'),
    true
  )

  const hashBadge = [
    ...retentionRow.querySelectorAll('[data-slot="status-badge"]'),
  ].find((element) => element.textContent?.includes('Original SHA-256'))
  assert.ok(hashBadge)
  assert.equal(hashBadge.textContent, 'Original SHA-256: 3f2baaaaaaaa')
  assert.equal(
    hashBadge.getAttribute('title'),
    `Click to copy: ${originalHash}`
  )

  await cleanup()
})

test('hides the retention overview for older records without retention stats', async () => {
  sectionRequests.length = 0
  const { container, cleanup } = await renderView(createView())

  assert.equal(
    container.querySelector('[data-slot="request-content-retention"]'),
    null
  )
  assert.equal(container.textContent?.includes('Retention Policy'), false)

  await cleanup()
})

test('keeps the encrypted placeholder for opaque sections that were not dropped', async () => {
  sectionRequests.length = 0
  const { container, cleanup } = await renderView(
    createView({
      summary: {
        ...baseSummary,
        reasoning_count: 1,
        opaque_bytes: 70 * 1024,
      },
      sections: [
        {
          id: 'input-0',
          kind: 'reasoning',
          type: 'reasoning',
          title: 'Encrypted reasoning',
          preview: 'Encrypted reasoning content is hidden',
          content_size: 70 * 1024,
          opaque_bytes: 70 * 1024,
          opaque: true,
          expandable: false,
          truncated: false,
        },
      ],
    })
  )

  assert.equal(
    container.textContent?.includes('Encrypted content is hidden'),
    true
  )
  assert.equal(
    container.querySelector('[data-slot="request-content-dropped-notice"]'),
    null
  )
  const badges = [...container.querySelectorAll('[data-slot="status-badge"]')]
  assert.equal(
    badges.some((element) => element.textContent?.includes('Not Kept')),
    false
  )
  assert.equal(
    badges.some((element) => element.textContent === '70.0 KB'),
    true
  )

  await cleanup()
})
