// web/src/features/request-records/__tests__/request-content-concise.test.tsx
// 简洁模式与定位卡的界面行为：默认只展示用户与 AI 消息、显示全部后区块回归、
// 开关状态跟随 localStorage，以及点击定位卡展开并滚动到最后一条用户消息。

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

// 记录滚动目标，happy-dom 的 scrollIntoView 本身是空实现，看不出被谁调用过。
const scrollTargets: string[] = []
const originalScrollIntoView = domWindow.Element.prototype.scrollIntoView
domWindow.Element.prototype.scrollIntoView = function scrollIntoViewSpy(
  this: Element
) {
  scrollTargets.push(this.id)
}

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
// 带插值的两条词条按 en.json 的原文提供，隐藏计数和区块序号是这里要保护的契约。
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Hidden sections: {{count}}': 'Hidden sections: {{count}}',
        'Section {{index}} of {{total}}': 'Section {{index}} of {{total}}',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  domWindow.Element.prototype.scrollIntoView = originalScrollIntoView
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

// 一条 agent 记录的典型构成：系统提示、工具定义、往返的工具调用，最后才是刚发的那句。
const agentSections: RequestContentAuditViewSection[] = [
  createSection({
    id: 'field-instructions',
    kind: 'instructions',
    title: 'instructions',
    preview: 'You are Codex',
    content_size: 2048,
  }),
  createSection({
    id: 'input-0',
    kind: 'message',
    role: 'system',
    preview: '系统提示正文',
  }),
  createSection({
    id: 'input-1',
    kind: 'message',
    role: 'user',
    preview: '第一轮的问题',
  }),
  createSection({
    id: 'input-2',
    kind: 'reasoning',
    type: 'reasoning',
    title: 'Encrypted reasoning',
    preview: 'Encrypted reasoning content is hidden',
    content_size: 70 * 1024,
    opaque_bytes: 70 * 1024,
    opaque: true,
    expandable: false,
  }),
  createSection({
    id: 'input-3',
    kind: 'tool_call',
    type: 'function_call',
    call_id: 'call_1',
    title: 'get_weather',
    preview: '{"audit_dropped":true,"audit_kind":"tool_call","size":5600}',
    dropped: true,
    dropped_kind: 'tool_call',
    dropped_bytes: 5600,
    content_size: 6 * 1024,
  }),
  createSection({
    id: 'input-4',
    kind: 'tool_output',
    type: 'function_call_output',
    title: 'Function result',
    preview: 'skill docs preview',
    content_size: 100 * 1024,
  }),
  createSection({
    id: 'input-5',
    kind: 'message',
    role: 'assistant',
    preview: '我看到 429 了',
  }),
  createSection({
    id: 'input-6',
    kind: 'message',
    role: 'user',
    preview: '帮我看下这个 429 的问题',
  }),
]

function createView(
  sections: RequestContentAuditViewSection[]
): RequestContentAuditView {
  // input_item_count 只数序列条目，field 区块不算，和后端投影一致。
  const inputItemCount = sections.filter((section) =>
    section.id.startsWith('input-')
  ).length
  return {
    request_id: 'req-1',
    relay_format: 'openai_responses',
    kind: 'json',
    source_size: 640 * 1024,
    stored_size: 120 * 1024,
    projection_available: true,
    summary: {
      input_item_count: inputItemCount,
      message_count: 4,
      sections_truncated: false,
      omitted_section_count: 0,
      tool_call_count: 1,
      tool_output_count: 1,
      reasoning_count: 1,
      advanced_field_count: 1,
      opaque_bytes: 70 * 1024,
      dropped_bytes: 5600,
      asset_count: 0,
    },
    sections,
  }
}

async function renderView(sections: RequestContentAuditViewSection[]) {
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
          <RequestContentView auditId={42} view={createView(sections)} />
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

function sectionIds(container: Element): string[] {
  return [
    ...container.querySelectorAll('[id^="request-content-section-"]'),
  ].map((element) => element.id.replace('request-content-section-', ''))
}

function findButton(container: Element, text: string) {
  return [...container.querySelectorAll('button')].find((button) =>
    button.textContent?.includes(text)
  )
}

test('hides every non-conversation section by default and reports the breakdown', async () => {
  localStorage.clear()
  const { container, cleanup } = await renderView(agentSections)

  assert.deepEqual(sectionIds(container), ['input-1', 'input-5', 'input-6'])
  const text = container.textContent ?? ''
  assert.equal(text.includes('帮我看下这个 429 的问题'), true)
  assert.equal(text.includes('我看到 429 了'), true)
  assert.equal(text.includes('第一轮的问题'), true)
  assert.equal(text.includes('系统提示正文'), false)
  assert.equal(text.includes('Encrypted content is hidden'), false)
  assert.equal(findButton(container, 'Function Result'), undefined)
  assert.equal(findButton(container, 'get_weather'), undefined)

  const summary = container.querySelector(
    '[data-slot="request-content-hidden-summary"]'
  )
  assert.ok(summary)
  const summaryText = summary.textContent ?? ''
  assert.equal(summaryText.includes('Hidden sections: 5'), true)
  assert.equal(summaryText.includes('Reasoning 1'), true)
  assert.equal(summaryText.includes('Tool Calls 1'), true)
  assert.equal(summaryText.includes('Tool Results 1'), true)
  assert.equal(summaryText.includes('Other 2'), true)

  await cleanup()
})

test('brings every section back and remembers the choice after Show All', async () => {
  localStorage.clear()
  const { container, cleanup } = await renderView(agentSections)

  const showAll = findButton(container, 'Show All')
  assert.ok(showAll)
  await act(async () => {
    showAll.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
  })

  assert.deepEqual(sectionIds(container), [
    'field-instructions',
    'input-0',
    'input-1',
    'input-2',
    'input-3',
    'input-4',
    'input-5',
    'input-6',
  ])
  const text = container.textContent ?? ''
  assert.equal(text.includes('系统提示正文'), true)
  assert.equal(text.includes('Encrypted content is hidden'), true)
  assert.ok(findButton(container, 'Function Result'))
  assert.ok(findButton(container, 'get_weather'))
  assert.ok(findButton(container, 'Instructions'))
  assert.equal(
    container.querySelector('[data-slot="request-content-hidden-summary"]'),
    null
  )
  assert.equal(
    localStorage.getItem(requestContentConciseViewStorageKey),
    'false'
  )

  await cleanup()
})

test('follows the stored preference instead of the default on mount', async () => {
  localStorage.clear()
  localStorage.setItem(requestContentConciseViewStorageKey, 'false')
  const stored = await renderView(agentSections)

  assert.equal(sectionIds(stored.container).length, 8)
  assert.equal(
    stored.container.querySelector(
      '[data-slot="request-content-hidden-summary"]'
    ),
    null
  )
  await stored.cleanup()
})

test('keeps the concise default when the stored preference is not a boolean', async () => {
  localStorage.clear()
  localStorage.setItem(requestContentConciseViewStorageKey, 'maybe')
  const { container, cleanup } = await renderView(agentSections)

  assert.deepEqual(sectionIds(container), ['input-1', 'input-5', 'input-6'])

  await cleanup()
})

test('expands and scrolls to the last user message when the locator card is clicked', async () => {
  localStorage.clear()
  scrollTargets.length = 0
  const { container, cleanup } = await renderView(agentSections)

  const card = container.querySelector(
    '[data-slot="request-content-latest-message"]'
  )
  assert.ok(card)
  const cardText = card.textContent ?? ''
  assert.equal(cardText.includes('帮我看下这个 429 的问题'), true)
  // input-6 是这条记录的第 7 个序列条目，序号按 section id 的真实下标算。
  assert.equal(cardText.includes('Section 7 of 7'), true)

  const target = container.querySelector('#request-content-section-input-6')
  assert.ok(target)
  const trigger = target.querySelector('[data-slot="collapsible-trigger"]')
  assert.ok(trigger)

  // 消息区块默认展开，先收起来才能验证定位卡真的把它重新打开了。
  await act(async () => {
    trigger.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
  })
  assert.equal(trigger.getAttribute('aria-expanded'), 'false')

  await act(async () => {
    card.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
  })

  assert.equal(trigger.getAttribute('aria-expanded'), 'true')
  assert.deepEqual(scrollTargets, ['request-content-section-input-6'])

  await cleanup()
})

test('renders no locator card for a record without any user message', async () => {
  localStorage.clear()
  const { container, cleanup } = await renderView([
    createSection({
      id: 'input-0',
      kind: 'message',
      role: 'assistant',
      preview: '只有模型输出',
    }),
    createSection({
      id: 'input-1',
      kind: 'tool_output',
      title: 'Function result',
    }),
  ])

  assert.equal(
    container.querySelector('[data-slot="request-content-latest-message"]'),
    null
  )
  assert.equal(container.textContent?.includes('只有模型输出'), true)

  await cleanup()
})

test('shows the full section list when the record carries no conversation section', async () => {
  localStorage.clear()
  const { container, cleanup } = await renderView([
    createSection({
      id: 'field-prompt',
      kind: 'field',
      title: 'prompt',
      preview: 'a cat riding a bike',
    }),
    createSection({
      id: 'field-size',
      kind: 'field',
      title: 'size',
      preview: '1024x1024',
    }),
  ])

  assert.deepEqual(sectionIds(container), ['field-prompt', 'field-size'])
  assert.equal(
    container.querySelector('[data-slot="request-content-hidden-summary"]'),
    null
  )
  assert.equal(
    container
      .querySelector('[data-slot="switch"]')
      ?.getAttribute('aria-checked'),
    'true'
  )

  await cleanup()
})

test('turns the concise view off from the switch and persists it', async () => {
  localStorage.clear()
  const { container, cleanup } = await renderView(agentSections)

  const conciseSwitch = container.querySelector('[data-slot="switch"]')
  assert.ok(conciseSwitch)
  assert.equal(conciseSwitch.getAttribute('aria-checked'), 'true')

  // 可见文案要真的绑在开关控件上，否则读屏用户听不到这个开关是干什么的。
  const label = [...container.querySelectorAll('label')].find(
    (element) => element.textContent === 'Concise View'
  )
  assert.ok(label)
  assert.equal(conciseSwitch.getAttribute('aria-labelledby'), label.id)
  assert.equal(conciseSwitch.getAttribute('role'), 'switch')
  assert.equal(conciseSwitch.getAttribute('tabindex'), '0')

  await act(async () => {
    conciseSwitch.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
  })

  assert.equal(conciseSwitch.getAttribute('aria-checked'), 'false')
  assert.equal(sectionIds(container).length, 8)
  assert.equal(
    localStorage.getItem(requestContentConciseViewStorageKey),
    'false'
  )

  await cleanup()
})
