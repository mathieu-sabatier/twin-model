// @vitest-environment node
import { describe, expect, it } from 'vitest'
import golden from '../../internal/api/dto/testdata/catalog.golden.json'
import type { CatalogTypeDetail, MemberKind } from '~/types'

// Drift guard: the catalog DTOs' JSON (Go golden) must stay assignable to the
// hand-kept types.ts interfaces. Compile-time assignability is the load-bearing
// check; the runtime assertions make `vitest run` alone a tripwire too.
const _detail: CatalogTypeDetail = golden as CatalogTypeDetail

describe('catalog types drift guard vs the Go golden', () => {
  it('golden is assignable to CatalogTypeDetail', () => {
    expect(_detail.name).toBe('DeviceType')
    expect(_detail.abstract).toBe(true)
  })

  it('has exactly the modeled top-level keys', () => {
    expect(Object.keys(golden).sort()).toEqual(
      ['abstract', 'alias', 'baseChain', 'members', 'name', 'nodeClass', 'uri'].sort(),
    )
  })

  it('members carry kind inside the MemberKind union', () => {
    const kinds = new Set<MemberKind>(['property', 'variable', 'object', 'method'])
    for (const m of _detail.members) {
      expect(kinds.has(m.kind)).toBe(true)
      expect(typeof m.placeholder).toBe('boolean')
    }
  })

  it('base-chain entries carry alias/name/uri', () => {
    expect(_detail.baseChain.length).toBeGreaterThan(0)
    for (const b of _detail.baseChain) {
      expect(typeof b.alias).toBe('string')
      expect(typeof b.name).toBe('string')
      expect(typeof b.uri).toBe('string')
    }
  })
})
