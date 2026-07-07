// @vitest-environment node
//
// Pure unit tests for app/lib/diagnosticPath.ts — the Path→field registry.
// No Vue, no Nuxt runtime needed. Tests every path grammar from API-CONTRACT.md.
import { describe, expect, it } from 'vitest'
import { parsePath, indexDiagnostics } from '~/lib/diagnosticPath'
import type { Diagnostic } from '~/types'
import { diagnostics as fixtureDiags } from '~/mocks/fixtures'

// ── parsePath ─────────────────────────────────────────────────────────────────

describe('parsePath — model scope', () => {
  it('model/name', () => {
    expect(parsePath('model/name')).toEqual({ scope: 'model', field: 'name' })
  })

  it('model/namespace', () => {
    expect(parsePath('model/namespace')).toEqual({ scope: 'model', field: 'namespace' })
  })

  it('model/version', () => {
    expect(parsePath('model/version')).toEqual({ scope: 'model', field: 'version' })
  })

  it('model/publication_date', () => {
    expect(parsePath('model/publication_date')).toEqual({
      scope: 'model',
      field: 'publication_date',
    })
  })
})

describe('parsePath — enum scope', () => {
  it('enums/EquipmentState (enum row, no value)', () => {
    expect(parsePath('enums/EquipmentState')).toEqual({
      scope: 'enum',
      enum: 'EquipmentState',
    })
  })

  it('enums/EquipmentState/values/Idle (enum value)', () => {
    expect(parsePath('enums/EquipmentState/values/Idle')).toEqual({
      scope: 'enum',
      enum: 'EquipmentState',
      value: 'Idle',
    })
  })
})

describe('parsePath — type scope', () => {
  it('object_types/FurnaceType (type row)', () => {
    expect(parsePath('object_types/FurnaceType')).toEqual({
      scope: 'type',
      type: 'FurnaceType',
    })
  })

  it('object_types/FurnaceType/base', () => {
    expect(parsePath('object_types/FurnaceType/base')).toEqual({
      scope: 'type',
      type: 'FurnaceType',
      field: 'base',
    })
  })
})

describe('parsePath — member scope', () => {
  it('object_types/FurnaceType/members/Setpoint (member row)', () => {
    expect(parsePath('object_types/FurnaceType/members/Setpoint')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'Setpoint',
    })
  })

  it('object_types/FurnaceType/members/Setpoint/kind', () => {
    expect(parsePath('object_types/FurnaceType/members/Setpoint/kind')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'Setpoint',
      field: 'kind',
    })
  })

  it('object_types/FurnaceType/members/Setpoint/rule', () => {
    expect(parsePath('object_types/FurnaceType/members/Setpoint/rule')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'Setpoint',
      field: 'rule',
    })
  })

  it('object_types/FurnaceType/members/Setpoint/access', () => {
    expect(parsePath('object_types/FurnaceType/members/Setpoint/access')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'Setpoint',
      field: 'access',
    })
  })

  it('object_types/FurnaceType/members/Setpoint/type', () => {
    expect(parsePath('object_types/FurnaceType/members/Setpoint/type')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'Setpoint',
      field: 'type',
    })
  })

  it('object_types/FurnaceType/members/Efficiency/unit', () => {
    expect(parsePath('object_types/FurnaceType/members/Efficiency/unit')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'Efficiency',
      field: 'unit',
    })
  })
})

describe('parsePath — memberChild scope', () => {
  it('object_types/FurnaceType/members/Zones/children/Zone', () => {
    expect(parsePath('object_types/FurnaceType/members/Zones/children/Zone')).toEqual({
      scope: 'memberChild',
      type: 'FurnaceType',
      member: 'Zones',
      child: 'Zone',
    })
  })
})

describe('parsePath — method arg scope (member with argDir/argIndex)', () => {
  it('object_types/FurnaceType/members/StartProgram/in/0/type', () => {
    expect(parsePath('object_types/FurnaceType/members/StartProgram/in/0/type')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'StartProgram',
      field: 'type',
      argDir: 'in',
      argIndex: 0,
    })
  })

  it('object_types/FurnaceType/members/StartProgram/out/0/type', () => {
    expect(parsePath('object_types/FurnaceType/members/StartProgram/out/0/type')).toEqual({
      scope: 'member',
      type: 'FurnaceType',
      member: 'StartProgram',
      field: 'type',
      argDir: 'out',
      argIndex: 0,
    })
  })
})

describe('parsePath — instance scope', () => {
  it('instances/Furnace02 (instance row)', () => {
    expect(parsePath('instances/Furnace02')).toEqual({
      scope: 'instance',
      instance: 'Furnace02',
    })
  })

  it('instances/Furnace02/type', () => {
    expect(parsePath('instances/Furnace02/type')).toEqual({
      scope: 'instance',
      instance: 'Furnace02',
      field: 'type',
    })
  })

  it('instances/Furnace02/under', () => {
    expect(parsePath('instances/Furnace02/under')).toEqual({
      scope: 'instance',
      instance: 'Furnace02',
      field: 'under',
    })
  })
})

describe('parsePath — instanceValue scope', () => {
  it('instances/Furnace02/values/SerialNumber', () => {
    expect(parsePath('instances/Furnace02/values/SerialNumber')).toEqual({
      scope: 'instanceValue',
      instance: 'Furnace02',
      member: 'SerialNumber',
    })
  })
})

describe('parsePath — instanceChild scope', () => {
  it('instances/Furnace02/children/Zone1', () => {
    expect(parsePath('instances/Furnace02/children/Zone1')).toEqual({
      scope: 'instanceChild',
      instance: 'Furnace02',
      child: 'Zone1',
    })
  })
})

describe('parsePath — instanceLevel scope', () => {
  it('instances/Site1/level', () => {
    expect(parsePath('instances/Site1/level')).toEqual({
      scope: 'instanceLevel',
      instance: 'Site1',
    })
  })
})

describe('parsePath — perspectiveMember scope', () => {
  it('perspectives/zones/nodes/n/members/A', () => {
    expect(parsePath('perspectives/zones/nodes/n/members/A')).toEqual({
      scope: 'perspectiveMember',
      perspective: 'zones',
      node: 'n',
      member: 'A',
    })
  })
})

describe('parsePath — unknown fallback', () => {
  it('completely unknown path returns { scope: "unknown", raw }', () => {
    expect(parsePath('something/weird/path')).toEqual({
      scope: 'unknown',
      raw: 'something/weird/path',
    })
  })

  it('empty string returns unknown', () => {
    expect(parsePath('')).toEqual({ scope: 'unknown', raw: '' })
  })
})

// ── indexDiagnostics ──────────────────────────────────────────────────────────

describe('indexDiagnostics — lookups with fixture diagnostics', () => {
  // Fixture:
  //   [0] code: 'unit-on-non-numeric', path: 'object_types/FurnaceType/members/Efficiency/unit'
  //   [1] code: 'optional',            path: 'object_types/EquipmentType/members/CycleCount'

  const idx = indexDiagnostics(fixtureDiags)

  it('forMemberField returns the right diagnostic', () => {
    const result = idx.forMemberField('FurnaceType', 'Efficiency', 'unit')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('unit-on-non-numeric')
  })

  it('forMember returns the right diagnostic (member row, no field)', () => {
    const result = idx.forMember('EquipmentType', 'CycleCount')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('optional')
  })

  it('forMember for the field diag also hits (field diag is a subset of member)', () => {
    // A diagnostic on members/Efficiency/unit is also "for" the member Efficiency.
    const result = idx.forMember('FurnaceType', 'Efficiency')
    expect(result.length).toBeGreaterThan(0)
  })

  it('forType returns diagnostics for a type (member diags included)', () => {
    const result = idx.forType('FurnaceType')
    expect(result.length).toBeGreaterThan(0)
  })

  it('forType returns empty for an unknown type', () => {
    expect(idx.forType('NoSuchType')).toHaveLength(0)
  })

  it('forMemberField returns empty for the wrong member', () => {
    expect(idx.forMemberField('FurnaceType', 'Setpoint', 'unit')).toHaveLength(0)
  })

  it('forModelField returns empty when no model-scope diagnostics', () => {
    expect(idx.forModelField('namespace')).toHaveLength(0)
  })

  it('forInstance returns empty when no instance diagnostics', () => {
    expect(idx.forInstance('Furnace02')).toHaveLength(0)
  })

  it('forInstanceValue returns empty when no instance-value diagnostics', () => {
    expect(idx.forInstanceValue('Furnace02', 'SerialNumber')).toHaveLength(0)
  })
})

describe('indexDiagnostics — enum diagnostics (Fix 1)', () => {
  const enumDiags: Diagnostic[] = [
    {
      code: 'empty-enum',
      severity: 'error',
      file: 'equipment.yaml',
      line: 5,
      col: 1,
      path: 'enums/EquipmentState',
      message: 'enum has no values',
    },
    {
      code: 'duplicate-enum-value',
      severity: 'error',
      file: 'equipment.yaml',
      line: 8,
      col: 1,
      path: 'enums/EquipmentState/values/Idle',
      message: 'duplicate enum value name',
    },
  ]

  const idx = indexDiagnostics(enumDiags)

  it('forEnum returns enum-row diagnostic by enum name', () => {
    const result = idx.forEnum('EquipmentState')
    expect(result).toHaveLength(2) // enum-row + value diagnostic (broaden)
    const codes = result.map((d) => d.code)
    expect(codes).toContain('empty-enum')
  })

  it('forEnum with value returns only the value diagnostic', () => {
    const result = idx.forEnum('EquipmentState', 'Idle')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('duplicate-enum-value')
  })

  it('value diagnostic is broadened to forEnum (enum sees its values errors)', () => {
    const result = idx.forEnum('EquipmentState')
    const codes = result.map((d) => d.code)
    expect(codes).toContain('duplicate-enum-value')
  })

  it('forEnum returns empty for a different enum name', () => {
    expect(idx.forEnum('SomeOtherEnum')).toHaveLength(0)
  })

  it('forEnum with wrong value returns empty', () => {
    expect(idx.forEnum('EquipmentState', 'Running')).toHaveLength(0)
  })
})

describe('indexDiagnostics — with inline instance-value and model-field diagnostics', () => {
  const inline: Diagnostic[] = [
    {
      code: 'missing-name',
      severity: 'error',
      file: 'equipment.yaml',
      line: 1,
      col: 1,
      path: 'model/name',
      message: 'model name is required',
    },
    {
      code: 'unknown-value-member',
      severity: 'error',
      file: 'equipment.yaml',
      line: 10,
      col: 1,
      path: 'instances/Furnace02/values/SerialNumber',
      message: 'unknown value member',
    },
    {
      code: 'unknown-placeholder',
      severity: 'warning',
      file: 'equipment.yaml',
      line: 20,
      col: 1,
      path: 'instances/Furnace02/children/Zone1',
      message: 'unknown placeholder child',
    },
  ]

  const idx = indexDiagnostics(inline)

  it('forModelField returns model-scope diagnostic', () => {
    const result = idx.forModelField('name')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('missing-name')
  })

  it('forInstance returns diagnostics anchored to the instance', () => {
    // Both instanceValue and instanceChild belong to Furnace02
    const result = idx.forInstance('Furnace02')
    expect(result.length).toBe(2)
  })

  it('forInstanceValue returns only the value diagnostic', () => {
    const result = idx.forInstanceValue('Furnace02', 'SerialNumber')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('unknown-value-member')
  })
})

describe('indexDiagnostics — instanceLevel and perspectiveMember diagnostics', () => {
  const levelAndMemberDiags: Diagnostic[] = [
    {
      code: 'level-out-of-range',
      severity: 'error',
      file: 'equipment.yaml',
      line: 3,
      col: 1,
      path: 'instances/Site1/level',
      message: 'level is out of range',
    },
    {
      code: 'unknown-perspective-member',
      severity: 'error',
      file: 'equipment.yaml',
      line: 7,
      col: 1,
      path: 'perspectives/zones/nodes/n/members/A',
      message: 'unknown perspective member',
    },
  ]

  const idx = indexDiagnostics(levelAndMemberDiags)

  it('forInstanceLevel returns the level diagnostic', () => {
    const result = idx.forInstanceLevel('Site1')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('level-out-of-range')
  })

  it('forInstanceLevel returns empty for a different instance', () => {
    expect(idx.forInstanceLevel('Furnace02')).toHaveLength(0)
  })

  it('forPerspectiveMember returns the member diagnostic', () => {
    const result = idx.forPerspectiveMember('zones', 'n', 'A')
    expect(result).toHaveLength(1)
    expect(result[0]!.code).toBe('unknown-perspective-member')
  })

  it('forPerspectiveMember returns empty for a different member', () => {
    expect(idx.forPerspectiveMember('zones', 'n', 'B')).toHaveLength(0)
  })
})
