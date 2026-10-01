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
// web/src/features/system-settings/maintenance/lib/upstream-version.ts
// 系统维护：从 XIMOYA 二开版本中提取上游版本，用于兼容上游更新检查。

const XIMOYA_VERSION_SUFFIX =
  /-ximoya\.\d+(?:-\d+-g[0-9a-f]+)?(?:-dirty)?(?:\+[^\s]+)?$/i

export function getUpstreamVersion(
  version?: string | null
): string | undefined {
  const normalizedVersion = version?.trim()
  if (!normalizedVersion) return undefined

  return normalizedVersion.replace(XIMOYA_VERSION_SUFFIX, '')
}
