import { describe, expect, it } from 'vitest'
import { computeBaseOptions } from '~/lib/baseOptions'

const zero = { file: '', line: 0, col: 0 }
const ref = (name: string) => ({ raw: name, name, pos: zero })

// A: base of B and (via B) of C; D is unrelated.
const types = [
  { name: 'A', base: undefined },
  { name: 'B', base: ref('A') },
  { name: 'C', base: ref('B') },
  { name: 'D', base: undefined },
]

describe('computeBaseOptions', () => {
  it('excludes the type itself and its transitive descendants', () => {
    const opts = computeBaseOptions(types, 'A')
    expect(opts).not.toContain('A') // self
    expect(opts).not.toContain('B') // direct descendant
    expect(opts).not.toContain('C') // transitive descendant
    expect(opts).toContain('D') // unrelated → valid base
  })

  it('keeps a non-local current base ref (import alias) as an option', () => {
    const opts = computeBaseOptions(types, 'B', 'OpcUa:BaseObjectType')
    expect(opts[0]).toBe('OpcUa:BaseObjectType')
    expect(opts).toContain('A') // B may still base on its ancestor's sibling
    expect(opts).not.toContain('B')
    expect(opts).not.toContain('C') // C derives from B
  })
})
