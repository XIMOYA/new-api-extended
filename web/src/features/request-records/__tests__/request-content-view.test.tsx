// web/src/features/request-records/__tests__/request-content-view.test.tsx
// 结构化请求内容视图：验证默认不读取完整区块，展开或主动加载时才请求正文。

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

const sectionRequests: string[] = []
mock.module('@/features/request-records/api', () => ({
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
                tool_call_count: 1,
                tool_output_count: 1,
                reasoning_count: 0,
                advanced_field_count: 1,
                opaque_bytes: 0,
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
