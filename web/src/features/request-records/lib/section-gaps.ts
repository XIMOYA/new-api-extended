// web/src/features/request-records/lib/section-gaps.ts
// 区块投影是「头 32 条 + 尾 224 条」，中间的真实下标会断档。这里负责从 section id 解析下标、
// 找出断档位置，并把断档提示排进渲染顺序：简洁模式过滤掉断档后的第一个区块时，
// 提示会往后挪到真正渲染出来的那个区块前面。id 格式见 service/request_content_audit_view.go。

import type { RequestContentAuditViewSection } from '../types'

export type RequestContentSectionPosition = {
  // 序列前缀，input-12 是 input，list-bWVzc2FnZXM-12 是 list-bWVzc2FnZXM。
  sequence: string
  index: number
}

export type RequestContentSectionGap = {
  afterSectionId: string
  beforeSectionId: string
  omitted: number
}

export type RequestContentSectionRow =
  | { kind: 'section'; section: RequestContentAuditViewSection }
  | { kind: 'gap'; beforeSectionId: string | null; omitted: number }

const sequenceSectionIdPrefixes = ['input-', 'list-']

export function parseRequestContentSectionPosition(
  id: string
): RequestContentSectionPosition | null {
  if (!sequenceSectionIdPrefixes.some((prefix) => id.startsWith(prefix))) {
    return null
  }
  const separator = id.lastIndexOf('-')
  if (separator <= 0) return null
  const rawIndex = id.slice(separator + 1)
  if (!/^\d+$/.test(rawIndex)) return null
  const index = Number(rawIndex)
  if (!Number.isSafeInteger(index)) return null
  return { sequence: id.slice(0, separator), index }
}

// 只有同一序列里下标不连续才算断档；解析不出下标或跨序列一律不算。
export function findRequestContentSectionGaps(
  sections: RequestContentAuditViewSection[]
): RequestContentSectionGap[] {
  const gaps: RequestContentSectionGap[] = []
  for (let cursor = 1; cursor < sections.length; cursor += 1) {
    const previous = sections[cursor - 1]
    const current = sections[cursor]
    const previousPosition = parseRequestContentSectionPosition(previous.id)
    const currentPosition = parseRequestContentSectionPosition(current.id)
    if (!previousPosition || !currentPosition) continue
    if (previousPosition.sequence !== currentPosition.sequence) continue
    const omitted = currentPosition.index - previousPosition.index - 1
    if (omitted <= 0) continue
    gaps.push({
      afterSectionId: previous.id,
      beforeSectionId: current.id,
      omitted,
    })
  }
  return gaps
}

export function buildRequestContentSectionRows(
  sections: RequestContentAuditViewSection[],
  visibleSections: RequestContentAuditViewSection[]
): RequestContentSectionRow[] {
  const gaps = findRequestContentSectionGaps(sections)
  if (gaps.length === 0) {
    return visibleSections.map((section) => ({
      kind: 'section' as const,
      section,
    }))
  }

  const orderById = new Map(
    sections.map((section, order) => [section.id, order])
  )
  const gapOrders = gaps.map((gap) => ({
    order: orderById.get(gap.beforeSectionId) ?? Number.MAX_SAFE_INTEGER,
    omitted: gap.omitted,
  }))
  const rows: RequestContentSectionRow[] = []
  let gapCursor = 0
  let pendingOmitted = 0

  for (const section of visibleSections) {
    const order = orderById.get(section.id) ?? Number.MAX_SAFE_INTEGER
    while (
      gapCursor < gapOrders.length &&
      gapOrders[gapCursor].order <= order
    ) {
      pendingOmitted += gapOrders[gapCursor].omitted
      gapCursor += 1
    }
    if (pendingOmitted > 0) {
      rows.push({
        kind: 'gap',
        beforeSectionId: section.id,
        omitted: pendingOmitted,
      })
      pendingOmitted = 0
    }
    rows.push({ kind: 'section', section })
  }

  // 断档之后的区块全被简洁模式隐藏时，提示挂在列表末尾，数量仍然只算后端没返回的部分。
  while (gapCursor < gapOrders.length) {
    pendingOmitted += gapOrders[gapCursor].omitted
    gapCursor += 1
  }
  if (pendingOmitted > 0) {
    rows.push({ kind: 'gap', beforeSectionId: null, omitted: pendingOmitted })
  }
  return rows
}
