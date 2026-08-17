// web/src/features/request-records/__tests__/retention.test.ts
// 保留策略辅助逻辑：分类名映射、区块与详情裁剪信息的合并规则，以及字节数展示格式。

import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  formatRequestContentBytes,
  getRequestContentDroppedKindLabel,
  resolveRequestContentDropped,
} from '../lib/retention'

const identity = (key: string) => key

test('maps every backend dropped kind to its own translation key', () => {
  assert.equal(
    getRequestContentDroppedKindLabel('tools_schema', identity),
    'Tool Definitions'
  )
  assert.equal(
    getRequestContentDroppedKindLabel('tool_call', identity),
    'Tool Call Arguments'
  )
  assert.equal(
    getRequestContentDroppedKindLabel('tool_result', identity),
    'Tool Execution Results'
  )
  assert.equal(
    getRequestContentDroppedKindLabel('reasoning_opaque', identity),
    'Encrypted Reasoning'
  )
  assert.equal(
    getRequestContentDroppedKindLabel('signature', identity),
    'Signature'
  )
})

test('falls back to the raw kind for unknown categories and to Content when missing', () => {
  assert.equal(
    getRequestContentDroppedKindLabel('audio_blob', identity),
    'audio_blob'
  )
  assert.equal(
    getRequestContentDroppedKindLabel(undefined, identity),
    'Content'
  )
})

test('prefers the section detail response over the list payload for dropped stats', () => {
  const state = resolveRequestContentDropped(
    { dropped: true, dropped_kind: 'tool_call', dropped_bytes: 5600 },
    { dropped: true, dropped_kind: 'tool_result', dropped_bytes: 65_536 }
  )

  assert.deepEqual(state, {
    dropped: true,
    kind: 'tool_result',
    bytes: 65_536,
  })
})

test('keeps the list payload stats when the detail response omits them', () => {
  const state = resolveRequestContentDropped(
    { dropped: true, dropped_kind: 'tool_result', dropped_bytes: 65_536 },
    {}
  )

  assert.deepEqual(state, {
    dropped: true,
    kind: 'tool_result',
    bytes: 65_536,
  })
})

test('treats a positive dropped size as dropped even without the flag', () => {
  const state = resolveRequestContentDropped({ dropped_bytes: 400 })

  assert.equal(state.dropped, true)
  assert.equal(state.bytes, 400)
  assert.equal(state.kind, undefined)
})

test('reports sections without retention stats as not dropped', () => {
  assert.deepEqual(resolveRequestContentDropped({}), {
    dropped: false,
    kind: undefined,
    bytes: 0,
  })
})

test('formats dropped sizes with the same units as the rest of the view', () => {
  assert.equal(formatRequestContentBytes(0), '0 B')
  assert.equal(formatRequestContentBytes(-1), '0 B')
  assert.equal(formatRequestContentBytes(Number.NaN), '0 B')
  assert.equal(formatRequestContentBytes(999), '999 B')
  assert.equal(formatRequestContentBytes(1024), '1.0 KB')
  assert.equal(formatRequestContentBytes(280_000), '273.4 KB')
  assert.equal(formatRequestContentBytes(5 * 1024 ** 3), '5.0 GB')
  assert.equal(formatRequestContentBytes(1024 ** 5), '1048576.0 GB')
})
