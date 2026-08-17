// web/src/features/request-records/__tests__/request-content-omitted-sections.test.tsx
// 断档提示的界面行为：投影跳过中间区块时插入一行说明且数量正确，下标连续时不插，
// 简洁模式下「后端未显示」和「前端已隐藏」两条提示同时在场且各算各的数。

import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

import { requestContentConciseViewStorageKey } from '../lib/concise-view'
import type {
  RequestContentAuditView,
  RequestContentAuditViewSection,
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

const actualApi = await import('@/features/request-records/api')
mock.module('@/features/request-records/api', () => ({
  ...actualApi,
  getRequestContentViewSection: async (
    _auditId: number,
    sectionId: string
  ) => ({
    id: sectionId,
    kind: 'message',
    title: 'user',
    content: '完整正文',
    content_format: 'text',
    content_size: 128,
    opaque: false,
    expandable: true,
    truncated: false,
  }),
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
// 两条提示的原文按 en.json 提供，数量和措辞的区分本身就是这里要保护的契约。
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Hidden sections: {{count}}': 'Hidden sections: {{count}}',
        '{{count}} sections in between are not shown':
          '{{count}} sections in between are not shown',
        'Only the newest sections are kept':
          'Only the newest sections are kept',
      },
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

function createSection(
  overrides: Partial<RequestContentAuditViewSection> & {
    id: string
    kind: string
  }
): RequestContentAuditViewSection {
  return {
    content_size: 128,
    expandable: true,
    opaque: false,
    title: overrides.kind,
    truncated: false,
    ...overrides,
  }
}

// 投影只返回了序列开头两条和结尾三条，中间 38 条（下标 2~39）被跳过。
const windowedSections: RequestContentAuditViewSection[] = [
  createSection({
    id: 'input-0',
    kind: 'message',
    role: 'user',
    preview: '第一轮的问题',
  }),
  createSection({
    id: 'input-1',
    kind: 'message',
    role: 'system',
    preview: '系统提示正文',
  }),
  createSection({
    id: 'input-40',
    kind: 'tool_output',
    title: 'Function result',
    preview: 'skill docs preview',
  }),
  createSection({
    id: 'input-41',
    kind: 'message',
    role: 'assistant',
    preview: '我看到 429 了',
  }),
  createSection({
    id: 'input-42',
    kind: 'message',
    role: 'user',
    preview: '帮我看下这个 429 的问题',
  }),
]

function createView(
  sections: RequestContentAuditViewSection[],
  omittedSectionCount: number
): RequestContentAuditView {
  return {
    request_id: 'req-1',
    relay_format: 'openai_responses',
    kind: 'json',
    source_size: 640 * 1024,
    stored_size: 120 * 1024,
    projection_available: true,
    summary: {
      input_item_count: sections.length + omittedSectionCount,
      message_count: 4,
      sections_truncated: omittedSectionCount > 0,
      omitted_section_count: omittedSectionCount,
      tool_call_count: 0,
      tool_output_count: 1,
      reasoning_count: 0,
      advanced_field_count: 0,
      opaque_bytes: 0,
      dropped_bytes: 0,
      asset_count: 0,
    },
    sections,
  }
}

async function renderView(
  sections: RequestContentAuditViewSection[],
  omittedSectionCount: number
) {
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
            view={createView(sections, omittedSectionCount)}
          />
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

function omittedNotices(container: Element) {
  return [
    ...container.querySelectorAll(
      '[data-slot="request-content-omitted-sections"]'
    ),
  ]
}

test('explains how many sections the projection skipped at the gap position', async () => {
  localStorage.clear()
  localStorage.setItem(requestContentConciseViewStorageKey, 'false')
  const { container, cleanup } = await renderView(windowedSections, 38)

  const notices = omittedNotices(container)
  assert.equal(notices.length, 1)
  const noticeText = notices[0]?.textContent ?? ''
  assert.equal(
    noticeText.includes('38 sections in between are not shown'),
    true
  )
  assert.equal(noticeText.includes('Only the newest sections are kept'), true)
  // 提示要挂在断档后的第一个区块前面，而不是列表开头或结尾。
  assert.equal(
    notices[0]?.nextElementSibling?.id,
    'request-content-section-input-40'
  )
  assert.equal(
    notices[0]?.previousElementSibling?.id,
    'request-content-section-input-1'
  )

  await cleanup()
})

test('adds no gap notice when the projection returned every section', async () => {
  localStorage.clear()
  localStorage.setItem(requestContentConciseViewStorageKey, 'false')
  const { container, cleanup } = await renderView(
    [
      createSection({
        id: 'input-0',
        kind: 'message',
        role: 'user',
        preview: '就一句话',
      }),
      createSection({
        id: 'input-1',
        kind: 'message',
        role: 'assistant',
        preview: '收到',
      }),
    ],
    0
  )

  assert.deepEqual(omittedNotices(container), [])
  assert.equal(
    container.textContent?.includes('sections in between are not shown'),
    false
  )

  await cleanup()
})

test('keeps the skipped and the hidden notices separate in the concise view', async () => {
  localStorage.clear()
  const { container, cleanup } = await renderView(windowedSections, 38)

  const notices = omittedNotices(container)
  assert.equal(notices.length, 1)
  const gapText = notices[0]?.textContent ?? ''
  assert.equal(gapText.includes('38 sections in between are not shown'), true)
  // 简洁模式把 input-40 藏了，提示顺延到下一个真正渲染出来的区块前面。
  assert.equal(
    notices[0]?.nextElementSibling?.id,
    'request-content-section-input-41'
  )

  const hiddenSummary = container.querySelector(
    '[data-slot="request-content-hidden-summary"]'
  )
  assert.ok(hiddenSummary)
  const hiddenText = hiddenSummary.textContent ?? ''
  // 前端隐藏的只有系统消息和工具结果两个，绝不能和后端跳过的 38 个加在一起。
  assert.equal(hiddenText.includes('Hidden sections: 2'), true)
  assert.equal(hiddenText.includes('Tool Results 1'), true)
  assert.equal(hiddenText.includes('Other 1'), true)
  assert.equal(hiddenText.includes('40'), false)
  assert.equal(hiddenText.includes('sections in between are not shown'), false)
  assert.equal(gapText.includes('Hidden sections'), false)
  assert.equal(gapText.includes('Show All'), false)

  await cleanup()
})
