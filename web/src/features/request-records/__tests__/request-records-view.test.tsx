// web/src/features/request-records/__tests__/request-records-view.test.tsx
// 请求记录页面：验证查看按钮、详情弹窗挂载以及使用日志同款视觉标记。

import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

const bunTestModule = ['bun', 'test'].join(':')
const { mock } = await import(bunTestModule)

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
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

const summary = {
  id: 0,
  request_id: '20260815091115356562042008268d9d6wwLRGfe',
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
  message_count: 1,
  asset_count: 0,
  capture_status: 'complete',
  normalization_version: 1,
  content_available: false,
  can_view_full_content: false,
  is_redacted: false,
}

const page = {
  page: 1,
  page_size: 20,
  total: 1,
  items: [summary],
}

const detail = {
  ...summary,
  id: 42,
  assets: [],
}

let lookedUpRequestId = ''

const actualRouter = await import('@tanstack/react-router')
mock.module('@tanstack/react-router', () => ({
  ...actualRouter,
  getRouteApi: () => ({
    useSearch: () => ({ page: 1, requestId: undefined }),
  }),
  useNavigate: () => () => undefined,
}))

mock.module('@/features/request-records/api', () => ({
  getRequestContentAuditByRequestId: async (requestId: string) => {
    lookedUpRequestId = requestId
    return detail
  },
  getRequestContentAudits: async () => page,
  getRequestContentAssetBlob: async () => new Blob(),
  getRequestContentPreview: async () => ({
    content: '',
    redacted: false,
    request_id: summary.request_id,
    truncated: false,
  }),
  getRequestContentPreviewByRequestId: async () => ({
    content: '',
    redacted: false,
    request_id: summary.request_id,
    truncated: false,
  }),
  getRequestContentView: async () => ({
    request_id: summary.request_id,
    relay_format: summary.relay_format,
    kind: 'json',
    source_size: summary.content_size,
    stored_size: summary.stored_size,
    projection_available: true,
    summary: {
      input_item_count: 0,
      message_count: 0,
      sections_truncated: false,
      tool_call_count: 0,
      tool_output_count: 0,
      reasoning_count: 0,
      advanced_field_count: 0,
      opaque_bytes: 0,
      asset_count: 0,
    },
    sections: [],
  }),
  getRequestContentViewSection: async () => ({
    id: 'input-0',
    kind: 'message',
    title: 'user',
    content_size: 0,
    opaque: false,
    expandable: false,
    truncated: false,
  }),
  streamRequestContent: async () => '',
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

test('opens the request dialog from View and resolves details by request ID', async () => {
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

  const modelBadge = [
    ...container.querySelectorAll('[data-slot="status-badge"]'),
  ].find((element) => element.textContent?.includes(summary.model_name))
  assert.ok(modelBadge)
  assert.ok(modelBadge.querySelector('svg'))

  const avatarFallback = container.querySelector(
    '[data-slot="avatar-fallback"]'
  )
  assert.ok(avatarFallback)
  assert.equal(avatarFallback.textContent, 'X')

  const viewButton = [...container.querySelectorAll('button')].find(
    (button) => button.textContent?.trim() === 'View'
  )
  assert.ok(viewButton)

  await act(async () => {
    viewButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
    await Promise.resolve()
  })

  assert.equal(document.body.textContent?.includes('Request Content'), true)
  assert.equal(document.body.textContent?.includes(summary.request_id), true)
  assert.equal(lookedUpRequestId, summary.request_id)

  await act(async () => root.unmount())
  container.remove()
})
