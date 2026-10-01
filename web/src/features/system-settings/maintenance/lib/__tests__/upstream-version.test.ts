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
// web/src/features/system-settings/maintenance/lib/__tests__/upstream-version.test.ts
// 系统维护：验证 XIMOYA 二开版本在上游更新检查时能还原为对应的上游版本。

import assert from 'node:assert/strict'

import { getUpstreamVersion } from '../upstream-version'

const bunTestModule = 'bun:test'
const { describe, test } = (await import(bunTestModule)) as {
  describe: typeof import('node:test').describe
  test: typeof import('node:test').test
}

describe('getUpstreamVersion', () => {
  test('strips the XIMOYA release suffix', () => {
    assert.equal(getUpstreamVersion('v1.0.0-rc.24-ximoya.1'), 'v1.0.0-rc.24')
  })

  test('strips XIMOYA suffix and build metadata', () => {
    assert.equal(
      getUpstreamVersion('v1.0.0-rc.24-ximoya.1+g03b2f727'),
      'v1.0.0-rc.24'
    )
  })

  test('strips git describe commit distance and dirty marker', () => {
    assert.equal(
      getUpstreamVersion('v1.0.0-rc.24-ximoya.1-5-g03b2f727-dirty'),
      'v1.0.0-rc.24'
    )
  })

  test('preserves a normal upstream version', () => {
    assert.equal(getUpstreamVersion('v1.0.0-rc.24'), 'v1.0.0-rc.24')
  })
})
