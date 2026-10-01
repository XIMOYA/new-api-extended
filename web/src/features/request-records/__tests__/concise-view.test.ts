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
// web/src/features/request-records/__tests__/concise-view.test.ts
// 简洁模式纯逻辑：区块过滤与隐藏计数、最后一条用户消息的定位，以及开关的 localStorage 读写容错。

import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  findRequestContentLatestUserMessage,
  readRequestContentConciseView,
  requestContentConciseViewStorageKey,
  selectRequestContentConciseSections,
  writeRequestContentConciseView,
} from '../lib/concise-view'
import type { RequestContentAuditViewSection } from '../types'

function createSection(
  overrides: Partial<RequestContentAuditViewSection> & {
    id: string
    kind: string
  }
): RequestContentAuditViewSection {
  return {
    content_size: 64,
    expandable: true,
    opaque: false,
    title: overrides.kind,
    truncated: false,
    ...overrides,
  }
}

function createStorage(initial?: string | null) {
  const values = new Map<string, string>()
  if (initial != null) values.set(requestContentConciseViewStorageKey, initial)
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value)
    },
    values,
  }
}

test('keeps only user and assistant messages and counts hidden sections per kind', () => {
  const selection = selectRequestContentConciseSections([
    createSection({ id: 'field-tools', kind: 'tools', title: 'tools' }),
    createSection({ id: 'input-0', kind: 'message', role: 'system' }),
    createSection({ id: 'input-1', kind: 'message', role: 'user' }),
    createSection({ id: 'input-2', kind: 'reasoning' }),
    createSection({ id: 'input-3', kind: 'tool_call' }),
    createSection({ id: 'input-4', kind: 'tool_output' }),
    createSection({ id: 'input-5', kind: 'tool_output' }),
    createSection({ id: 'input-6', kind: 'message', role: 'assistant' }),
    createSection({ id: 'input-7', kind: 'item', role: '' }),
  ])

  assert.deepEqual(
    selection.sections.map((section) => section.id),
    ['input-1', 'input-6']
  )
  assert.deepEqual(selection.hidden, {
    total: 7,
    reasoning: 1,
    toolCall: 1,
    toolOutput: 2,
    other: 3,
  })
})

test('keeps a message whose nested signature was pruned because the text is still readable', () => {
  const selection = selectRequestContentConciseSections([
    createSection({
      id: 'input-0',
      kind: 'message',
      role: 'assistant',
      preview: '好的，我先看下这个 429 的问题',
      dropped: true,
      dropped_kind: 'signature',
      dropped_bytes: 320,
    }),
  ])

  assert.equal(selection.sections.length, 1)
  assert.equal(selection.hidden.total, 0)
})

test('hides a message that is left as a retention placeholder only', () => {
  const selection = selectRequestContentConciseSections([
    createSection({
      id: 'input-0',
      kind: 'message',
      role: 'user',
      preview: '{"audit_dropped":true,"audit_kind":"tool_result","size":6144}',
      dropped: true,
      dropped_kind: 'tool_result',
      dropped_bytes: 6144,
    }),
  ])

  assert.deepEqual(selection.sections, [])
  assert.equal(selection.hidden.total, 1)
  assert.equal(selection.hidden.other, 1)
})

test('reports an empty selection without hidden sections for an empty view', () => {
  const selection = selectRequestContentConciseSections([])

  assert.deepEqual(selection.sections, [])
  assert.deepEqual(selection.hidden, {
    total: 0,
    reasoning: 0,
    toolCall: 0,
    toolOutput: 0,
    other: 0,
  })
})

test('locates the last user message with a one-based index over all sections', () => {
  const latest = findRequestContentLatestUserMessage([
    createSection({ id: 'input-0', kind: 'message', role: 'user' }),
    createSection({ id: 'input-1', kind: 'message', role: 'assistant' }),
    createSection({
      id: 'input-2',
      kind: 'message',
      role: 'user',
      preview: '帮我看下这个 429 的问题',
    }),
    createSection({ id: 'input-3', kind: 'tool_call' }),
  ])

  assert.equal(latest?.section.id, 'input-2')
  assert.equal(latest?.index, 3)
  assert.equal(latest?.total, 4)
  assert.equal(latest?.section.preview, '帮我看下这个 429 的问题')
})

test('returns no locator when the record carries no user message', () => {
  assert.equal(
    findRequestContentLatestUserMessage([
      createSection({ id: 'payload', kind: 'field', title: 'payload' }),
      createSection({ id: 'input-0', kind: 'message', role: 'assistant' }),
    ]),
    null
  )
  assert.equal(findRequestContentLatestUserMessage([]), null)
})

test('numbers the latest user message by its real index when the middle is skipped', () => {
  const latest = findRequestContentLatestUserMessage(
    [
      createSection({ id: 'input-0', kind: 'message', role: 'user' }),
      createSection({ id: 'input-597', kind: 'message', role: 'assistant' }),
      createSection({
        id: 'input-598',
        kind: 'message',
        role: 'user',
        preview: '帮我看下这个 429 的问题',
      }),
    ],
    600
  )

  assert.equal(latest?.section.id, 'input-598')
  assert.equal(latest?.index, 599)
  assert.equal(latest?.total, 600)
})

test('numbers list-style section ids by their real index too', () => {
  const latest = findRequestContentLatestUserMessage(
    [
      createSection({
        id: 'list-bWVzc2FnZXM-516',
        kind: 'message',
        role: 'user',
      }),
    ],
    525
  )

  assert.equal(latest?.index, 517)
  assert.equal(latest?.total, 525)
})

test('falls back to the rendered position when the section id has no index', () => {
  const latest = findRequestContentLatestUserMessage(
    [
      createSection({ id: 'field-dG9vbHM', kind: 'tools' }),
      createSection({ id: 'payload', kind: 'message', role: 'user' }),
    ],
    9
  )

  assert.equal(latest?.index, 2)
  assert.equal(latest?.total, 9)
})

test('defaults the concise switch to on when nothing is stored', () => {
  assert.equal(readRequestContentConciseView(createStorage()), true)
})

test('prefers the stored concise switch value over the default', () => {
  assert.equal(readRequestContentConciseView(createStorage('false')), false)
  assert.equal(readRequestContentConciseView(createStorage('true')), true)
})

test('falls back to the default for values that are not a stored boolean', () => {
  assert.equal(readRequestContentConciseView(createStorage('')), true)
  assert.equal(readRequestContentConciseView(createStorage('0')), true)
  assert.equal(readRequestContentConciseView(createStorage('nope')), true)
  assert.equal(readRequestContentConciseView(createStorage('{"a":1}')), true)
})

test('falls back to the default when storage access throws', () => {
  const blockedStorage = {
    getItem: () => {
      throw new Error('storage disabled')
    },
    setItem: () => {
      throw new Error('storage disabled')
    },
  }

  assert.equal(readRequestContentConciseView(blockedStorage), true)
  assert.doesNotThrow(() =>
    writeRequestContentConciseView(false, blockedStorage)
  )
})

test('stores the concise switch as a plain boolean string', () => {
  const storage = createStorage()

  writeRequestContentConciseView(false, storage)
  assert.equal(storage.values.get(requestContentConciseViewStorageKey), 'false')

  writeRequestContentConciseView(true, storage)
  assert.equal(storage.values.get(requestContentConciseViewStorageKey), 'true')
})
