/*
web/src/features/channels/components/drawers/sections/__tests__/narrafork-select.test.tsx
测试：渠道级 NarraFork 下拉框
职责：
- 验证渠道覆盖项的当前值使用中文 label 展示
- 验证渠道下拉弹层使用正常锚定，避免错位并保留动画状态
*/
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { ChannelFormValues } from '../../../../lib/channel-form'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
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

const mediaQuery = domWindow.matchMedia('(max-width: 640px)')
Object.defineProperty(domWindow, 'matchMedia', {
  configurable: true,
  value: () => mediaQuery,
})

const { act, createElement } = await import('react')
const { createRoot } = await import('react-dom/client')
const { FormProvider, useForm } = await import('react-hook-form')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { NarraForkQuotaEventSection } =
  await import('../narrafork-quota-event-section')
const { CHANNEL_FORM_DEFAULT_VALUES } = await import('../../../../lib/channel-form')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'zhCN',
  fallbackLng: 'en',
  resources: {
    zhCN: {
      translation: {
        'NarraFork channel option: inherit': '继承全局',
      },
    },
    en: { translation: {} },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function Harness() {
  const form = useForm<ChannelFormValues>({
    defaultValues: CHANNEL_FORM_DEFAULT_VALUES,
  })

  return (
    <I18nextProvider i18n={i18n}>
      <FormProvider {...form}>
        <NarraForkQuotaEventSection />
      </FormProvider>
    </I18nextProvider>
  )
}

describe('NarraFork channel override selects', () => {
  after(() => {
    domWindow.close()
  })

  test('shows localized inherited values before opening and uses normal popup alignment', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => root.render(createElement(Harness)))

    const triggers = [
      ...container.querySelectorAll<HTMLButtonElement>('button[role="combobox"]'),
    ]
    assert.equal(triggers.length, 6)
    for (const trigger of triggers) {
      assert.match(trigger.textContent ?? '', /继承全局/)
    }

    await act(async () => triggers[0].click())

    const content = document.body.querySelector<HTMLElement>(
      '[data-slot="select-content"]'
    )
    assert.ok(content)
    assert.equal(content.dataset.alignTrigger, 'false')

    await act(async () => root.unmount())
    container.remove()
  })
})
