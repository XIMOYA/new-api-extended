// web/src/features/request-records/__tests__/section-gaps.test.ts
// 区块下标解析与断档定位：识别两种 section id 格式、算出中间跳过的条数，
// 并把断档提示排到过滤后仍然渲染的那个区块前面。

import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  buildRequestContentSectionRows,
  findRequestContentSectionGaps,
  parseRequestContentSectionPosition,
} from '../lib/section-gaps'
import type { RequestContentAuditViewSection } from '../types'

function createSection(
  overrides: Partial<RequestContentAuditViewSection> & { id: string }
): RequestContentAuditViewSection {
  return {
    kind: 'message',
    role: 'user',
    content_size: 64,
    expandable: true,
    opaque: false,
    title: 'user',
    truncated: false,
    ...overrides,
  }
}

function createInputRange(
  from: number,
  to: number
): RequestContentAuditViewSection[] {
  const sections: RequestContentAuditViewSection[] = []
  for (let index = from; index < to; index += 1) {
    sections.push(createSection({ id: `input-${index}` }))
  }
  return sections
}

test('parses the real index out of both section id formats', () => {
  assert.deepEqual(parseRequestContentSectionPosition('input-0'), {
    sequence: 'input',
    index: 0,
  })
  assert.deepEqual(parseRequestContentSectionPosition('input-599'), {
    sequence: 'input',
    index: 599,
  })
  assert.deepEqual(parseRequestContentSectionPosition('list-bWVzc2FnZXM-516'), {
    sequence: 'list-bWVzc2FnZXM',
    index: 516,
  })
  // base64url 的 key 里可能带 -，下标只看最后一段。
  assert.deepEqual(parseRequestContentSectionPosition('list-YS1i-7'), {
    sequence: 'list-YS1i',
    index: 7,
  })
})

test('refuses ids that carry no sequence index', () => {
  assert.equal(parseRequestContentSectionPosition('payload'), null)
  assert.equal(parseRequestContentSectionPosition('field-dG9vbHM'), null)
  assert.equal(parseRequestContentSectionPosition('input-'), null)
  assert.equal(parseRequestContentSectionPosition('input-abc'), null)
  assert.equal(parseRequestContentSectionPosition('input-1e3'), null)
  assert.equal(parseRequestContentSectionPosition('list-bWVzc2FnZXM'), null)
})

test('reports the gap between the head and tail windows of a long record', () => {
  const sections = [...createInputRange(0, 32), ...createInputRange(376, 600)]

  const gaps = findRequestContentSectionGaps(sections)

  assert.deepEqual(gaps, [
    { afterSectionId: 'input-31', beforeSectionId: 'input-376', omitted: 344 },
  ])
  // 后端投影是头 32 条 + 尾 224 条，600 条记录正好跳过 344 条。
  assert.equal(gaps[0]?.omitted, 600 - 224 - 32)
})

test('reports no gap when the projection returned every item', () => {
  assert.deepEqual(findRequestContentSectionGaps(createInputRange(0, 12)), [])
  assert.deepEqual(
    findRequestContentSectionGaps([
      createSection({ id: 'list-bWVzc2FnZXM-0' }),
      createSection({ id: 'list-bWVzc2FnZXM-1' }),
      createSection({ id: 'field-dG9vbHM', kind: 'tools' }),
    ]),
    []
  )
})

test('never reports a gap across different sequences or unparsable ids', () => {
  assert.deepEqual(
    findRequestContentSectionGaps([
      createSection({ id: 'input-31' }),
      createSection({ id: 'list-bWVzc2FnZXM-40' }),
      createSection({ id: 'list-Y29udGVudHM-0' }),
    ]),
    []
  )
  assert.deepEqual(
    findRequestContentSectionGaps([
      createSection({ id: 'input-0' }),
      createSection({ id: 'field-dG9vbHM', kind: 'tools' }),
      createSection({ id: 'input-40' }),
    ]),
    []
  )
})

test('inserts one gap row in front of the section that follows the gap', () => {
  const sections = [
    createSection({ id: 'input-0' }),
    createSection({ id: 'input-1' }),
    createSection({ id: 'input-40' }),
  ]

  const rows = buildRequestContentSectionRows(sections, sections)

  assert.deepEqual(
    rows.map((row) =>
      row.kind === 'gap' ? `gap:${row.omitted}` : row.section.id
    ),
    ['input-0', 'input-1', 'gap:38', 'input-40']
  )
})

test('moves the gap row to the next rendered section when the following one is filtered out', () => {
  const sections = [
    createSection({ id: 'input-0' }),
    createSection({ id: 'input-40', kind: 'tool_output', role: undefined }),
    createSection({ id: 'input-41', role: 'assistant' }),
  ]
  const visible = [sections[0], sections[2]]

  const rows = buildRequestContentSectionRows(sections, visible)

  assert.deepEqual(
    rows.map((row) =>
      row.kind === 'gap' ? `gap:${row.omitted}` : row.section.id
    ),
    ['input-0', 'gap:39', 'input-41']
  )
})

test('keeps only section rows when the projection has no gap', () => {
  const sections = createInputRange(0, 3)

  const rows = buildRequestContentSectionRows(sections, sections)

  assert.deepEqual(
    rows.map((row) => row.kind),
    ['section', 'section', 'section']
  )
})

test('keeps the gap row at the end when every section after the gap is filtered out', () => {
  const sections = [
    createSection({ id: 'input-0' }),
    createSection({ id: 'input-40', kind: 'tool_output', role: undefined }),
  ]

  const rows = buildRequestContentSectionRows(sections, [sections[0]])

  assert.deepEqual(rows, [
    { kind: 'section', section: sections[0] },
    { kind: 'gap', beforeSectionId: null, omitted: 39 },
  ])
})

test('adds up several gaps that fall between the same pair of rendered sections', () => {
  const sections = [
    createSection({ id: 'input-0' }),
    createSection({ id: 'input-10', kind: 'tool_output', role: undefined }),
    createSection({ id: 'input-20', kind: 'reasoning', role: undefined }),
    createSection({ id: 'input-30' }),
  ]
  const visible = [sections[0], sections[3]]

  const rows = buildRequestContentSectionRows(sections, visible)

  assert.deepEqual(
    rows.map((row) =>
      row.kind === 'gap' ? `gap:${row.omitted}` : row.section.id
    ),
    ['input-0', 'gap:27', 'input-30']
  )
})
