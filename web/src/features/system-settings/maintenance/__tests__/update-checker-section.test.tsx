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
// web/src/features/system-settings/maintenance/__tests__/update-checker-section.test.tsx
// 系统维护：验证版本卡片展示完整 NewAPI 版本与 XIMOYA 优化署名。

import assert from 'node:assert/strict'

import { Window } from 'happy-dom'

const bunTestModule = 'bun:test'
const { afterAll, test } = (await import(bunTestModule)) as {
  afterAll: typeof import('node:test').after
  test: typeof import('node:test').test
}

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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { UpdateCheckerSection } = await import('../update-checker-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Check for updates': 'Check for updates',
        'Current version': 'Current version',
        NewAPI: 'NewAPI',
        'Optimized by XIMOYA': 'Optimized by XIMOYA',
        'System maintenance': 'System maintenance',
        'Uptime since': 'Uptime since',
        Unknown: 'Unknown',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

afterAll(() => {
  domWindow.close()
})

test('shows the complete XIMOYA build version and optimizer attribution', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UpdateCheckerSection
          currentVersion='v1.0.0-rc.24-ximoya.1'
          startTime={1_700_000_000}
        />
      </I18nextProvider>
    )
  })

  assert.equal(
    container.textContent?.includes('NewAPI v1.0.0-rc.24-ximoya.1'),
    true
  )
  assert.equal(container.textContent?.includes('Optimized by XIMOYA'), true)

  await act(async () => root.unmount())
  container.remove()
})
