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
/*
web/src/features/system-settings/models/__tests__/narrafork-select.test.tsx
测试：NarraFork 全局设置下拉框
职责：
- 验证关闭状态直接显示当前语言的选项标签
- 验证打开时使用正常锚定，避免弹层错位并保留动画状态
*/
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { NarraForkSettingsCard } = await import('../narrafork-settings-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'zhCN',
  fallbackLng: 'en',
  resources: {
    zhCN: {
      translation: {
        'NarraFork activation mode: header_or_user_agent':
          'Header 或 User-Agent',
        'NarraFork balance source: effective': '实际结算后余额',
        'NarraFork duplicate policy: skip': '保留上游事件',
        'NarraFork token display mode: exact': '精确数值',
        'NarraFork cache hit rate scope: request': '当次请求',
      },
    },
    en: { translation: {} },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const defaultValues = {
  'narrafork_setting.enabled': true,
  'narrafork_setting.allow_user_display_override': false,
  'narrafork_setting.allow_user_enabled': false,
  'narrafork_setting.allow_user_activation_mode': false,
  'narrafork_setting.allow_user_balance_source': false,
  'narrafork_setting.allow_user_include_detailed': false,
  'narrafork_setting.allow_user_duplicate_policy': false,
  'narrafork_setting.allow_user_expose_extra': false,
  'narrafork_setting.allow_user_token_display_mode': false,
  'narrafork_setting.allow_user_cache_hit_rate_scope': false,
  'narrafork_setting.allow_user_cache_hit_rate_days': false,
  'narrafork_setting.allow_user_show_balance': false,
  'narrafork_setting.allow_user_show_request_quota': false,
  'narrafork_setting.allow_user_show_today_quota': false,
  'narrafork_setting.allow_user_show_today_tokens': false,
  'narrafork_setting.allow_user_show_month_quota': false,
  'narrafork_setting.allow_user_show_month_tokens': false,
  'narrafork_setting.allow_user_show_total_quota': false,
  'narrafork_setting.allow_user_show_used_quota': false,
  'narrafork_setting.allow_user_show_input_tokens': false,
  'narrafork_setting.allow_user_show_output_tokens': false,
  'narrafork_setting.allow_user_show_total_tokens': false,
  'narrafork_setting.allow_user_show_cache_hit_tokens': false,
  'narrafork_setting.allow_user_show_cache_hit_rate': false,
  'narrafork_setting.allow_user_show_reasoning_tokens': false,
  'narrafork_setting.allow_user_show_latency': false,
  'narrafork_setting.allow_user_show_ttft': false,
  'narrafork_setting.allow_user_show_request_id': false,
  'narrafork_setting.allow_user_show_retry_count': false,
  'narrafork_setting.allow_user_show_model': false,
  'narrafork_setting.allow_user_show_billing_source': false,
  'narrafork_setting.allow_user_show_unavailable_fields': false,
  'narrafork_setting.allow_user_detail_template': false,
  'narrafork_setting.allow_user_custom_quota_balance': false,
  'narrafork_setting.allow_user_custom_detailed_quota_balance': false,
  'narrafork_setting.activation_mode': 'header_or_user_agent' as const,
  'narrafork_setting.balance_source': 'effective' as const,
  'narrafork_setting.include_detailed': true,
  'narrafork_setting.duplicate_policy': 'skip' as const,
  'narrafork_setting.expose_extra': false,
  'narrafork_setting.token_display_mode': 'exact' as const,
  'narrafork_setting.cache_hit_rate_scope': 'request' as const,
  'narrafork_setting.cache_hit_rate_days': 7,
  'narrafork_setting.show_balance': true,
  'narrafork_setting.show_request_quota': true,
  'narrafork_setting.show_today_quota': true,
  'narrafork_setting.show_today_tokens': true,
  'narrafork_setting.show_month_quota': true,
  'narrafork_setting.show_month_tokens': true,
  'narrafork_setting.show_total_quota': true,
  'narrafork_setting.show_used_quota': true,
  'narrafork_setting.show_input_tokens': true,
  'narrafork_setting.show_output_tokens': true,
  'narrafork_setting.show_total_tokens': true,
  'narrafork_setting.show_cache_hit_tokens': true,
  'narrafork_setting.show_cache_hit_rate': true,
  'narrafork_setting.show_reasoning_tokens': true,
  'narrafork_setting.show_latency': true,
  'narrafork_setting.show_ttft': true,
  'narrafork_setting.show_request_id': true,
  'narrafork_setting.show_retry_count': true,
  'narrafork_setting.show_model': true,
  'narrafork_setting.show_billing_source': true,
  'narrafork_setting.show_unavailable_fields': false,
  'narrafork_setting.detail_template': '',
  'narrafork_setting.custom_quota_balance': '',
  'narrafork_setting.custom_detailed_quota_balance': '',
}

describe('NarraFork global settings selects', () => {
  after(() => {
    domWindow.close()
  })

  test('shows localized selected labels before opening and uses normal popup alignment', async () => {
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
        createElement(
          QueryClientProvider,
          { client: queryClient },
          createElement(
            I18nextProvider,
            { i18n },
            createElement(NarraForkSettingsCard, { defaultValues })
          )
        )
      )
    })

    const triggers = [
      ...container.querySelectorAll<HTMLButtonElement>(
        'button[role="combobox"]'
      ),
    ]
    assert.equal(triggers.length, 6)
    assert.match(triggers[0].textContent ?? '', /Header 或 User-Agent/)
    assert.match(triggers[1].textContent ?? '', /实际结算后余额/)
    assert.match(triggers[2].textContent ?? '', /保留上游事件/)
    assert.match(triggers[3].textContent ?? '', /精确数值/)
    assert.match(triggers[4].textContent ?? '', /当次请求/)
    assert.match(triggers[5].textContent ?? '', /group|用户组/i)
    assert.equal(
      triggers[0].textContent?.includes('header_or_user_agent'),
      false
    )

    assert.match(container.textContent ?? '', /Show balance line/)
    assert.match(container.textContent ?? '', /Show reasoning Tokens/)

    await act(async () => triggers[0].click())

    const content = document.body.querySelector<HTMLElement>(
      '[data-slot="select-content"]'
    )
    assert.ok(content)
    assert.equal(content.dataset.alignTrigger, 'false')

    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  })
})
