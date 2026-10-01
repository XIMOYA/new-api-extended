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
// web/src/features/request-records/lib/concise-view.ts
// 请求内容简洁模式的纯逻辑：只留用户与 AI 的对话区块、统计被隐藏区块的分类，
// 定位本轮最后一条用户消息，以及开关状态的 localStorage 读写。
// agent 客户端每轮重发整段对话，一条记录动辄几百个区块，站长只关心自己刚发的那句。

import type { RequestContentAuditViewSection } from '../types'
import { parseRequestContentSectionPosition } from './section-gaps'

export const requestContentConciseViewStorageKey =
  'request-records.concise-view'
// 默认开启：绝大多数记录都是 agent 重发的长对话，先给干净视图。
const conciseViewDefault = true

type ConciseViewStorage = Pick<Storage, 'getItem' | 'setItem'>

export type RequestContentHiddenSectionCounts = {
  total: number
  reasoning: number
  toolCall: number
  toolOutput: number
  other: number
}

export type RequestContentConciseSelection = {
  sections: RequestContentAuditViewSection[]
  hidden: RequestContentHiddenSectionCounts
}

export type RequestContentLatestUserMessage = {
  section: RequestContentAuditViewSection
  // 从 1 开始，与后端 highlight 接口的 section_index 对齐。
  index: number
  total: number
}

// 裁剪占位对象里的固定字段名，见 service/request_content_audit_prune.go。
const droppedPlaceholderFlag = '"audit_dropped"'

// 只有用户输入和 AI 输出算对话正文；整段被裁掉、只剩占位对象的区块没有阅读价值。
// 注意消息里嵌套的签名被裁剪时 dropped 也会是 true，这种消息正文还在，必须留着。
function isConversationSection(
  section: RequestContentAuditViewSection
): boolean {
  if (section.kind !== 'message') return false
  if (section.role !== 'user' && section.role !== 'assistant') return false
  if (!section.dropped) return true
  const preview = section.preview?.trim() ?? ''
  return preview !== '' && !preview.includes(droppedPlaceholderFlag)
}

export function selectRequestContentConciseSections(
  sections: RequestContentAuditViewSection[]
): RequestContentConciseSelection {
  const kept: RequestContentAuditViewSection[] = []
  const hidden: RequestContentHiddenSectionCounts = {
    total: 0,
    reasoning: 0,
    toolCall: 0,
    toolOutput: 0,
    other: 0,
  }

  for (const section of sections) {
    if (isConversationSection(section)) {
      kept.push(section)
      continue
    }
    hidden.total += 1
    if (section.kind === 'reasoning') hidden.reasoning += 1
    else if (section.kind === 'tool_call') hidden.toolCall += 1
    else if (section.kind === 'tool_output') hidden.toolOutput += 1
    else hidden.other += 1
  }

  return { sections: kept, hidden }
}

// 倒着找最后一条用户消息。序号优先取 section id 里的真实下标：投影只返回序列头尾两段时，
// 数组位置和真实位置差得很远，这里要和后端 highlight 的 section_index 对上。
// itemCount 传 summary.input_item_count，没有时退回按返回的区块数计算。
export function findRequestContentLatestUserMessage(
  sections: RequestContentAuditViewSection[],
  itemCount = 0
): RequestContentLatestUserMessage | null {
  for (let cursor = sections.length - 1; cursor >= 0; cursor -= 1) {
    const section = sections[cursor]
    if (section.kind !== 'message' || section.role !== 'user') continue
    const position = parseRequestContentSectionPosition(section.id)
    const index = position ? position.index + 1 : cursor + 1
    const total =
      itemCount > 0
        ? Math.max(itemCount, index)
        : Math.max(sections.length, index)
    return { section, index, total }
  }
  return null
}

function resolveStorage(
  storage?: ConciseViewStorage | null
): ConciseViewStorage | null {
  if (storage) return storage
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

export function readRequestContentConciseView(
  storage?: ConciseViewStorage | null
): boolean {
  const target = resolveStorage(storage)
  if (!target) return conciseViewDefault
  try {
    const raw = target.getItem(requestContentConciseViewStorageKey)
    if (raw === 'true') return true
    if (raw === 'false') return false
    return conciseViewDefault
  } catch {
    return conciseViewDefault
  }
}

export function writeRequestContentConciseView(
  value: boolean,
  storage?: ConciseViewStorage | null
): void {
  const target = resolveStorage(storage)
  if (!target) return
  try {
    target.setItem(
      requestContentConciseViewStorageKey,
      value ? 'true' : 'false'
    )
  } catch {
    // 隐私模式下写不进去就算了，开关本轮仍然生效。
  }
}
