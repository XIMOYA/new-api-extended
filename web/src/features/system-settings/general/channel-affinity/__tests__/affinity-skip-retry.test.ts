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
// web/src/features/system-settings/general/channel-affinity/__tests__/affinity-skip-retry.test.ts
// 渠道亲和「失败后不重试」：锁定内置模板默认值，并确认新增说明文案在全部语言包里齐备。
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import enLocale from '../../../../../i18n/locales/en.json'
import frLocale from '../../../../../i18n/locales/fr.json'
import jaLocale from '../../../../../i18n/locales/ja.json'
import ruLocale from '../../../../../i18n/locales/ru.json'
import viLocale from '../../../../../i18n/locales/vi.json'
import zhTWLocale from '../../../../../i18n/locales/zh-TW.json'
import zhLocale from '../../../../../i18n/locales/zh.json'
import { cloneTemplate, RULE_TEMPLATES } from '../constants'

const SKIP_RETRY_KEYS = [
  'Do not retry after failure',
  'A failed affinity channel may be replaced by another channel; the upstream prompt cache is lost when that happens.',
  'Locked to the affinity channel: when it fails the error goes straight to the user and silent channel switch does not apply to this rule.',
  'When enabled, requests matching this rule stay locked to the affinity channel: if that channel fails, the error is returned to the user and no other channel is tried. This keeps the upstream prompt cache warm and costs less. When disabled, a failed request may switch to another channel — combined with silent channel switch the user sees no upstream error, but the session moves to a new channel and the upstream cache is lost, so this round costs more. Note: this option takes precedence over silent channel switch, except when the upstream reports quota exhaustion or an invalid account, in which case the affinity entry is dropped and a switch is allowed anyway.',
  'Channel affinity rules with "Do not retry after failure" enabled keep their requests locked to one channel, so silent switch does not apply to them — adjust that option under Settings - General - Channel Affinity.',
] as const

const TRANSLATED_LOCALES: Record<string, Record<string, string>> = {
  zh: zhLocale.translation,
  'zh-TW': zhTWLocale.translation,
  ja: jaLocale.translation,
  fr: frLocale.translation,
  ru: ruLocale.translation,
  vi: viLocale.translation,
}

describe('channel affinity skip-retry defaults', () => {
  test('built-in rule templates allow retry so silent switch keeps working', () => {
    for (const [name, template] of Object.entries(RULE_TEMPLATES)) {
      assert.equal(
        template.skip_retry_on_failure,
        false,
        `内置模板 ${name} 默认应允许失败后换渠道`
      )
    }
  })

  test('cloning a template keeps the skip-retry field addressable', () => {
    const cloned = cloneTemplate(RULE_TEMPLATES.claudeCli)
    assert.equal(cloned.skip_retry_on_failure, false)

    cloned.skip_retry_on_failure = true
    assert.equal(
      RULE_TEMPLATES.claudeCli.skip_retry_on_failure,
      false,
      '克隆后的修改不应回写模板'
    )
  })
})

describe('channel affinity skip-retry copy', () => {
  const enTranslation = enLocale.translation as Record<string, string>

  test('english base keeps every new key as identity', () => {
    for (const key of SKIP_RETRY_KEYS) {
      assert.equal(enTranslation[key], key, `en.json 缺少或改写了词条: ${key}`)
    }
  })

  test('every other locale ships a real translation', () => {
    for (const [locale, translation] of Object.entries(TRANSLATED_LOCALES)) {
      for (const key of SKIP_RETRY_KEYS) {
        const value = translation[key]
        assert.ok(
          typeof value === 'string' && value.trim().length > 0,
          `${locale}.json 缺少词条: ${key}`
        )
        assert.notEqual(value, key, `${locale}.json 词条未翻译: ${key}`)
      }
    }
  })
})
