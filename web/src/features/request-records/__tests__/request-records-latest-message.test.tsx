// web/src/features/request-records/__tests__/request-records-latest-message.test.tsx
// 列表页最新用户消息列：可用时每行显示摘要并带完整 title，正文不可用的记录不发请求，
// 摘要接口失败时整张表照常渲染。

import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

import type {
  RequestContentAuditHighlight,
  RequestContentAuditSummary,
} from '../types'

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

const latestPreview =
  '帮我看下这个 429 的问题，Codex 每轮都把整段对话重发一遍，日志里翻不到'

function createSummary(
  overrides: Partial<RequestContentAuditSummary> & {
    id: number
    request_id: string
  }
): RequestContentAuditSummary {
  return {
    user_id: 7,
    username: 'XIMOYA',
    model_name: 'gpt-5.6-luna',
    request_type: 'openai_responses',
    relay_format: 'openai_responses',
    endpoint_path: '/v1/responses',
    content_type: 'application/json',
    created_at: 1_760_000_000,
    expires_at: 1_760_100_000,
    content_size: 364 * 1024,
    stored_size: 364 * 1024,
    message_count: 525,
    asset_count: 0,
    capture_status: 'complete',
    normalization_version: 1,
    content_available: true,
    can_view_full_content: true,
    is_redacted: false,
    ...overrides,
  }
}

const page = {
  page: 1,
  page_size: 20,
  total: 3,
  items: [
    createSummary({ id: 42, request_id: 'req-with-highlight' }),
    createSummary({ id: 43, request_id: 'req-without-highlight' }),
    createSummary({
      id: 44,
      request_id: 'req-content-unavailable',
      content_available: false,
    }),
  ],
}

const highlightCalls: number[] = []
let respondToHighlight: (
  id: number
) => Promise<RequestContentAuditHighlight> = async (id) => ({
  request_id: String(id),
  available: false,
})

const actualRouter = await import('@tanstack/react-router')
mock.module('@tanstack/react-router', () => ({
  ...actualRouter,
  getRouteApi: () => ({
    useSearch: () => ({ page: 1, requestId: undefined }),
  }),
  useNavigate: () => () => undefined,
}))

const actualApi = await import('@/features/request-records/api')
mock.module('@/features/request-records/api', () => ({
  ...actualApi,
  getRequestContentAudits: async () => page,
  getRequestContentHighlight: async (id: number) => {
    highlightCalls.push(id)
    return respondToHighlight(id)
  },
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { RequestRecords } = await import('../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  mock.restore()
  domWindow.close()
})

async function renderList() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['request-content-audits', 1, undefined], page)
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <RequestRecords />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  return {
    container,
    queryClient,
    cleanup: async () => {
      await act(async () => root.unmount())
      container.remove()
      queryClient.clear()
    },
  }
}

// 每行摘要是独立查询，等到界面或缓存进入预期状态再断言，不靠固定等待时间。
async function waitFor(condition: () => boolean) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (condition()) return
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
  }
  throw new Error('timed out waiting for the expected state')
}

function latestMessageCells(container: Element) {
  return [
    ...container.querySelectorAll(
      '[data-slot="request-record-latest-message"]'
    ),
  ]
}

test('shows the latest user message only for rows the endpoint can answer', async () => {
  highlightCalls.length = 0
  respondToHighlight = async (id) => {
    if (id !== 42) {
      return { request_id: 'req-without-highlight', available: false }
    }
    return {
      request_id: 'req-with-highlight',
      available: true,
      section_id: 'list-bWVzc2FnZXM-516',
      role: 'user',
      preview: latestPreview,
      message_count: 525,
      truncated: true,
    }
  }
  const { container, cleanup } = await renderList()
  await waitFor(() => latestMessageCells(container).length > 0)

  const cells = latestMessageCells(container)
  assert.equal(cells.length, 1)
  assert.equal(cells[0]?.textContent, latestPreview)
  assert.equal(cells[0]?.getAttribute('title'), latestPreview)
  // 正文不可用的记录压根不该发请求。
  assert.deepEqual(
    [...highlightCalls].sort((a, b) => a - b),
    [42, 43]
  )
  assert.equal(container.querySelectorAll('tbody tr').length, 3)

  await cleanup()
})

test('keeps the table intact when the highlight request fails', async () => {
  highlightCalls.length = 0
  respondToHighlight = async () => {
    throw new Error('highlight endpoint exploded')
  }
  const { container, queryClient, cleanup } = await renderList()
  await waitFor(
    () => highlightCalls.length === 2 && queryClient.isFetching() === 0
  )

  assert.deepEqual(latestMessageCells(container), [])
  assert.equal(container.querySelectorAll('tbody tr').length, 3)
  assert.equal(container.textContent?.includes('req-with-highlight'), true)
  assert.equal(container.textContent?.includes('gpt-5.6-luna'), true)

  await cleanup()
})
