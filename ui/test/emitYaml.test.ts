// @vitest-environment node
//
// Unit tests for app/lib/emitYaml.ts — the client-side YAML emitter.
// Pure module: no Nuxt runtime needed. Uses the real golden fixture to ensure
// the emitter is faithful to the Go parser/formatter's expected structure.
import { describe, it, expect } from 'vitest'
import { emitModel } from '~/lib/emitYaml'
import { equipmentModel } from '~/mocks/fixtures'
import type { Model } from '~/types'

describe('emitModel', () => {
  const y = emitModel(equipmentModel)

  it('uses snake_case DSL top-level keys', () => {
    expect(y).toMatch(/^model:/m)
    expect(y).toMatch(/^\s*publication_date:/m)
    expect(y).toMatch(/^object_types:/m)
    expect(y).not.toMatch(/publicationDate|objectTypes/)
  })

  it('emits a placeholder member key as Name<Suffix>, quoted', () => {
    expect(y).toContain('"Zone<No>"')
  })

  it('emits a method with in and out argument lists', () => {
    expect(y).toMatch(/in:/)
    expect(y).toMatch(/out:/)
    expect(y).toMatch(/name: ProgramId/)
  })

  it('emits an instance with values and children', () => {
    expect(y).toMatch(/Furnace02:/)
    expect(y).toMatch(/values:/)
    expect(y).toMatch(/children:/)
  })

  it('emits numeric and boolean instance values unquoted (H1)', () => {
    // Furnace02 in the golden has CycleCount: '42' (UInt32) and DoorClosed: 'true' (Boolean)
    expect(y).toMatch(/CycleCount: 42\b/)
    expect(y).not.toMatch(/CycleCount: "42"/)
    expect(y).toMatch(/DoorClosed: true\b/)
    expect(y).not.toMatch(/DoorClosed: "true"/)
  })

  it('is deterministic', () => {
    expect(emitModel(equipmentModel)).toBe(y)
  })
})

describe('emitModel — level, hierarchy, perspectives', () => {
  const zero = { file: '', line: 0, col: 0 }
  const ref = (raw: string) => ({
    raw,
    name: raw.includes(':') ? raw.split(':')[1]! : raw,
    pos: zero,
  })

  it('emits level, hierarchy and perspectives', () => {
    const m: Model = {
      pos: zero,
      name: 'M',
      namespace: 'https://x/',
      version: '1.0.0',
      publicationDate: '2026-07-06',
      hierarchy: { allowLevelSkip: true },
      instances: [
        {
          pos: zero,
          name: 'Site1',
          type: ref('ISA95:EquipmentType'),
          under: ref('OpcUa:ObjectsFolder'),
          level: 'Site',
        },
      ],
      perspectives: [
        {
          pos: zero,
          id: 'zones',
          label: 'Zones',
          membership: 'exclusive',
          export: false,
          nodes: [{ pos: zero, id: 'n', label: 'N', members: ['Site1'] }],
        },
      ],
    }
    const out = emitModel(m)
    expect(out).toContain('hierarchy: { allowLevelSkip: true }')
    expect(out).toContain('level: Site')
    expect(out).toContain('perspectives:')
    expect(out).toContain('members: [Site1]')
  })
})
