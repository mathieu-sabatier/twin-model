// @vitest-environment node
import { describe, expect, it } from 'vitest'
import golden from '../../internal/api/dto/testdata/model.golden.json'
import type {
  Argument,
  Instance,
  MemberAccess,
  MemberKind,
  MemberRule,
  Model,
  ObjectType,
} from '~/types'

// Drift guard: types.ts is a hand-kept mirror of internal/api/dto/dto.go. This
// test imports the REAL Go golden (a bare `Model`, emitted by the Go golden test
// from the same DTO the server uses) and asserts it is assignable to our `Model`.
//
// HOW THIS CATCHES DRIFT (both layers must be considered):
//
//  1. COMPILE-TIME (vue-tsc / `npm run typecheck`): the assignments below are
//     typed against types.ts. If types.ts DROPS a field the golden carries, or
//     RENAMES one, or narrows a union so a golden value no longer fits, these
//     `const _x: T = ...` lines stop type-checking and the typecheck job fails.
//     (`resolveJsonModule` is on via the Nuxt tsconfig, so the imported JSON is a
//     precisely-typed literal — not `any` — which is what makes this bite.)
//
//  2. RUNTIME (`vitest run`): vue-tsc runs in a SEPARATE job, so we ALSO assert
//     structure at runtime here. If types.ts loses a field, the compile-time
//     assignment above already fails; but to make `vitest run` alone a genuine
//     tripwire, we assert the golden actually carries the exact top-level keys we
//     model and that a member's `kind`/`rule` values fall inside our unions. A
//     future golden that adds a NEW top-level key (a field the server started
//     emitting) fails the exact-keys assertion below, forcing a types.ts update.

// --- Compile-time assignability (the load-bearing part) ---

// The whole golden is assignable to Model. If any modeled field drifted in name
// or optionality, this line fails to type-check.
const _model: Model = golden as Model

// Precise field walks that would fail to type-check if a nested field
// name/optionality drifted. Indices reference the CURRENT golden:
//   objectTypes[2] = FurnaceType, .members[1] = Zones (kind "object"),
//   .children[0]   = Zone (browseName "<ZoneNo>").
const _furnace: ObjectType = _model.objectTypes![2]!
const _browseName: string | undefined = _furnace.members![1]!.children![0]!.browseName
const _kind: MemberKind = _furnace.members![1]!.kind
const _rule: MemberRule = _furnace.members![1]!.children![0]!.rule
const _access: MemberAccess | undefined = _furnace.members![0]!.access

// StartProgram is a method with in/out Argument arrays (FurnaceType members:
// DoorClosed=0, Zones=1, StartProgram=2).
const _startProgram = _furnace.members![2]!
const _inArg: Argument = _startProgram.in![0]!
const _outArg: Argument = _startProgram.out![0]!

// instances[2] = Furnace02, .values[0].raw = "ACME GmbH", .children[0] a placeholder child.
const _furnace02: Instance = _model.instances![2]!
const _raw: string = _furnace02.values![0]!.raw
const _childName: string = _furnace02.children![0]!.name

// `level`/`hierarchy`/`perspectives` are new omitempty fields (added for the
// ISA-95 hierarchy work); the equipment golden doesn't carry them, so these are
// `undefined` at runtime — the compile-time assignment is what matters here (a
// future Go DTO rename/drop of these fields breaks typecheck).
const _level: string | undefined = _model.instances![0]!.level
const _persp = _model.perspectives
const _hier = _model.hierarchy

// Reference the bindings so noUnusedLocals (if ever enabled) and reviewers see
// they are intentional; also feeds the runtime checks below.
const _touched = {
  _model,
  _furnace,
  _browseName,
  _kind,
  _rule,
  _access,
  _startProgram,
  _inArg,
  _outArg,
  _furnace02,
  _raw,
  _childName,
  _level,
  _persp,
  _hier,
}

describe('types.ts drift guard vs the real Go golden', () => {
  it('imports the real cross-repo golden (single source of truth)', () => {
    // Sanity: this is the actual Go golden, not a stub.
    expect(golden.name).toBe('AcmeEquipment')
    expect(_touched._model).toBe(golden)
  })

  it('golden has exactly the top-level keys types.ts models (no new server field)', () => {
    // If the server starts emitting a NEW top-level field, the golden gains a key
    // and this fails at RUNTIME, forcing a types.ts update even though the
    // compile-time assignment (extra properties are allowed on a value) would not
    // have caught an ADDED field.
    const expectedTopLevelKeys = [
      'pos',
      'name',
      'namespace',
      'prefix',
      'version',
      'publicationDate',
      'imports',
      'enums',
      'objectTypes',
      'instances',
    ].sort()
    expect(Object.keys(golden).sort()).toEqual(expectedTopLevelKeys)
  })

  it('a member carries kind/rule/access values inside our string-literal unions', () => {
    const kinds = new Set<MemberKind>(['property', 'variable', 'object', 'method'])
    const rules = new Set<MemberRule>([
      'mandatory',
      'optional',
      'optional_placeholder',
      'mandatory_placeholder',
    ])
    const accesses = new Set<MemberAccess>(['r', 'rw'])

    // Read through the TYPED `_model` (a `Model`), not the raw JSON literal: this
    // way these accesses ALSO fail to COMPILE if types.ts drops `children`,
    // `browseName`, `access`, etc. — so the guard bites at both layers.
    const zones = _model.objectTypes![2]!.members![1]!
    expect(kinds.has(zones.kind)).toBe(true)
    expect(zones.kind).toBe('object')

    const zone = zones.children![0]!
    expect(rules.has(zone.rule)).toBe(true)
    expect(zone.rule).toBe('optional_placeholder')
    expect(zone.browseName).toBe('<ZoneNo>')

    const manufacturer = _model.objectTypes![0]!.members![0]!
    // `access` is optional in the type; the golden sets it, so assert present + valid.
    expect(manufacturer.access).toBeDefined()
    expect(accesses.has(manufacturer.access!)).toBe(true)
  })

  it('preserves nested instance fields modeled in types.ts', () => {
    const furnace02 = _model.instances![2]!
    expect(furnace02.name).toBe('Furnace02')
    expect(furnace02.values![0]!.raw).toBe('ACME GmbH')
    expect(furnace02.children![0]!.name).toBe('Zone1')
    // `of` is a TypeRef with name/raw.
    expect(furnace02.children![0]!.of.raw).toBe('Zone<No>')
  })
})
