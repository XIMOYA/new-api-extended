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
// web/src/features/system-settings/models/__tests__/silent-channel-switch-fields.test.tsx
// 模型设置 - 路由可靠性：验证静默切换渠道三个字段的默认值回填、关闭时置灰、尝试上限校验与保存载荷类型。

import assert from 'node:assert/strict'

import { Window } from 'happy-dom'

// 整个测试套件一起跑时，node:test 的注册方式会被 Bun 判成嵌套 test 而直接报错，
// 所以这里统一从 bun:test 取注册函数，保持和 update-checker-section 测试一致。
const bunTestModule = ['bun', 'test'].join(':')
const { afterAll, describe, mock, test } = (await import(bunTestModule)) as {
  afterAll: typeof import('node:test').after
  describe: typeof import('node:test').describe
  mock: { module: (specifier: string, factory: () => unknown) => void }
  test: typeof import('node:test').test
}

type OptionCall = { key: string; value: string | number | boolean }
const optionCalls: OptionCall[] = []

mock.module('@/features/system-settings/api', () => ({
  updateSystemOption: async (request: OptionCall) => {
    optionCalls.push(request)
    return { success: true, message: '' }
  },
  updateNarraForkSettings: async () => ({ success: true, message: '' }),
}))

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLInputElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'KeyboardEvent',
  'MouseEvent',
  'PointerEvent',
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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance, default: globalI18next } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { RoutingReliabilitySection } =
  await import('../routing-reliability-section')

const testResources = {
  'Silent channel switch': 'Silent channel switch',
  'Unified failure message': 'Unified failure message',
  'Max channels per request': 'Max channels per request',
  'Enter a whole number between 0 and 32':
    'Enter a whole number between 0 and 32',
  'Setting updated successfully': 'Setting updated successfully',
}

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: testResources } },
})

// useUpdateOption 的成功提示直接用全局 i18next 实例，未初始化会在 mutation 回调里炸掉。
await globalI18next.init({
  lng: 'en',
  resources: { en: { translation: testResources } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const baseDefaults = {
  RetryTimes: 3,
  ChannelDisableThreshold: '10',
  AutomaticDisableChannelEnabled: false,
  AutomaticEnableChannelEnabled: false,
  AutomaticDisableKeywords: '',
  AutomaticDisableStatusCodes: '401',
  AutomaticRetryStatusCodes: '500-599',
  SilentChannelSwitchEnabled: true,
  SilentChannelSwitchMessage: '上游忙，稍后再来',
  SilentChannelSwitchMaxAttempts: 8,
  'monitor_setting.auto_test_channel_enabled': false,
  'monitor_setting.auto_test_channel_minutes': 10,
  'monitor_setting.channel_test_mode': 'scheduled_all' as const,
}

type SectionDefaults = typeof baseDefaults

async function renderSection(defaults: SectionDefaults) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <RoutingReliabilitySection defaultValues={defaults} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  const cleanup = async () => {
    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  }

  return { container, cleanup }
}

// FormControl 把 id 放在控件上，FormLabel 的 for 指向同一个 id，靠标签文案定位最稳。
function controlByLabel(container: HTMLElement, labelText: string) {
  const label = [...container.querySelectorAll('label')].find(
    (item) => item.textContent === labelText
  )
  assert.ok(label, `label not found: ${labelText}`)
  const controlId = label.getAttribute('for')
  assert.ok(controlId, `label has no for: ${labelText}`)
  // React 的 useId 可能带 CSS 不友好的字符，用属性选择器绕开转义问题。
  const control = container.querySelector<HTMLElement>(`[id="${controlId}"]`)
  assert.ok(control, `control not found: ${labelText}`)
  return control
}

function inputByLabel(container: HTMLElement, labelText: string) {
  const control = controlByLabel(container, labelText)
  assert.equal(control.tagName, 'INPUT')
  return control as HTMLInputElement
}

// Switch 的 for/id 指向那个视觉隐藏的 checkbox，开关状态挂在同一组里的 role=switch 上。
function switchStateByLabel(container: HTMLElement, labelText: string) {
  const item = controlByLabel(container, labelText).closest(
    '[data-slot="form-item"]'
  )
  assert.ok(item, `form item not found: ${labelText}`)
  const switchRoot = item.querySelector('[role="switch"]')
  assert.ok(switchRoot, `switch root not found: ${labelText}`)
  return switchRoot
}

function changeInputValue(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(input, value)
  input.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

async function submitSection(container: HTMLElement) {
  const form = container.querySelector('form')
  assert.ok(form)
  await act(async () => {
    form.dispatchEvent(
      new domWindow.Event('submit', {
        bubbles: true,
        cancelable: true,
      }) as unknown as Event
    )
  })
}

function fieldError(container: HTMLElement, labelText: string) {
  const control = controlByLabel(container, labelText)
  return (
    control
      .closest('[data-slot="form-item"]')
      ?.querySelector('[data-slot="form-message"]')?.textContent ?? ''
  )
}

describe('silent channel switch fields', () => {
  afterAll(() => {
    domWindow.close()
  })

  test('restores the saved boolean, string and number values', async () => {
    optionCalls.length = 0
    const { container, cleanup } = await renderSection(baseDefaults)

    const silentSwitch = switchStateByLabel(container, 'Silent channel switch')
    assert.equal(silentSwitch.getAttribute('aria-checked'), 'true')
    assert.equal(
      inputByLabel(container, 'Unified failure message').value,
      '上游忙，稍后再来'
    )
    assert.equal(inputByLabel(container, 'Max channels per request').value, '8')

    await cleanup()
  })

  test('greys out the message and attempts fields while the switch is off without clearing them', async () => {
    optionCalls.length = 0
    const { container, cleanup } = await renderSection({
      ...baseDefaults,
      SilentChannelSwitchEnabled: false,
      SilentChannelSwitchMessage: '保留已保存的文案',
      SilentChannelSwitchMaxAttempts: 12,
    })

    const message = inputByLabel(container, 'Unified failure message')
    const attempts = inputByLabel(container, 'Max channels per request')
    assert.equal(
      switchStateByLabel(container, 'Silent channel switch').getAttribute(
        'aria-checked'
      ),
      'false'
    )
    assert.equal(message.disabled, true)
    assert.equal(attempts.disabled, true)
    assert.equal(message.value, '保留已保存的文案')
    assert.equal(attempts.value, '12')

    await act(async () => {
      controlByLabel(container, 'Silent channel switch').click()
    })

    assert.equal(
      switchStateByLabel(container, 'Silent channel switch').getAttribute(
        'aria-checked'
      ),
      'true'
    )
    assert.equal(message.disabled, false)
    assert.equal(attempts.disabled, false)
    assert.equal(message.value, '保留已保存的文案')
    assert.equal(attempts.value, '12')

    await cleanup()
  })

  test('rejects 33 attempts and accepts both 0 and 32', async () => {
    optionCalls.length = 0
    const { container, cleanup } = await renderSection(baseDefaults)
    const attempts = inputByLabel(container, 'Max channels per request')

    await act(async () => changeInputValue(attempts, '33'))
    await submitSection(container)
    assert.equal(
      fieldError(container, 'Max channels per request'),
      'Enter a whole number between 0 and 32'
    )
    assert.deepEqual(optionCalls, [])

    await act(async () => changeInputValue(attempts, '32'))
    await submitSection(container)
    assert.equal(fieldError(container, 'Max channels per request'), '')
    assert.deepEqual(optionCalls, [
      { key: 'SilentChannelSwitchMaxAttempts', value: 32 },
    ])

    optionCalls.length = 0
    await act(async () => changeInputValue(attempts, '0'))
    await submitSection(container)
    assert.deepEqual(optionCalls, [
      { key: 'SilentChannelSwitchMaxAttempts', value: 0 },
    ])

    await cleanup()
  })

  test('saves all three keys with the value types the option API expects', async () => {
    optionCalls.length = 0
    const { container, cleanup } = await renderSection({
      ...baseDefaults,
      SilentChannelSwitchEnabled: false,
      SilentChannelSwitchMessage: '',
      SilentChannelSwitchMaxAttempts: 0,
    })

    await act(async () => {
      controlByLabel(container, 'Silent channel switch').click()
    })
    await act(async () =>
      changeInputValue(
        inputByLabel(container, 'Unified failure message'),
        '上游服务忙，请稍后再试'
      )
    )
    await act(async () =>
      changeInputValue(inputByLabel(container, 'Max channels per request'), '5')
    )
    await submitSection(container)

    assert.equal(optionCalls.length, 3)
    const payload = new Map(optionCalls.map((call) => [call.key, call.value]))
    assert.equal(payload.get('SilentChannelSwitchEnabled'), true)
    assert.equal(
      payload.get('SilentChannelSwitchMessage'),
      '上游服务忙，请稍后再试'
    )
    assert.equal(payload.get('SilentChannelSwitchMaxAttempts'), 5)
    assert.equal(typeof payload.get('SilentChannelSwitchEnabled'), 'boolean')
    assert.equal(typeof payload.get('SilentChannelSwitchMessage'), 'string')
    assert.equal(typeof payload.get('SilentChannelSwitchMaxAttempts'), 'number')

    await cleanup()
  })
})
