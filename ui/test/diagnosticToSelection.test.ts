import { describe, it, expect } from 'vitest'
import { diagnosticToSelection } from '~/lib/diagnosticToSelection'
import type { Diagnostic } from '~/types'

const d = (path: string): Diagnostic => ({ code: 'x', severity: 'error', file: 'f', line: 0, col: 0, path, message: 'm' })

describe('diagnosticToSelection', () => {
  it('maps instance/member/type/enum paths to a Selection', () => {
    expect(diagnosticToSelection(d('instances/Furnace01/type'))).toEqual({ kind: 'instance', name: 'Furnace01' })
    expect(diagnosticToSelection(d('object_types/FurnaceType/members/DoorClosed/unit'))).toEqual({ kind: 'type', name: 'FurnaceType' })
    expect(diagnosticToSelection(d('enums/EquipmentState/values/Idle'))).toEqual({ kind: 'enum', name: 'EquipmentState' })
    expect(diagnosticToSelection(d('model/name'))).toBeNull()
  })
})
