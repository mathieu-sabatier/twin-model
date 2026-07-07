import { describe, expect, it, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient } from '~/api'
import type { Model } from '~/types'

const zero = { file: '', line: 0, col: 0 }
function baseModel(): Model {
  return {
    pos: zero, name: 'M', namespace: 'urn:x', version: '1.0', publicationDate: '2026-01-01',
    imports: [{ pos: zero, alias: 'OpcUa', uri: 'http://opcfoundation.org/UA/' }],
    objectTypes: [{ pos: zero, name: 'EquipmentType', members: [] }],
    instances: [],
  }
}

function fakeApi(captured: { yaml?: string }): ApiClient {
  const model = baseModel()
  return {
    getDraftModel: async () => ({ file: 'm.yaml', model, diagnostics: [] }),
    putFiles: async (_id: string, files: Record<string, string>) => { captured.yaml = files['m.yaml'] },
  } as unknown as ApiClient
}

async function seeded(captured: { yaml?: string }) {
  const s = useDraftStore(fakeApi(captured))
  // Minimal seeding: set draftId/file/model directly via the read path.
  await s.loadDraft('d1', 'm.yaml')
  return s
}

describe('draft store — catalog authoring', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('ensureImport adds a missing import once (idempotent)', async () => {
    const cap = {}
    const s = await seeded(cap)
    const next = structuredClone(JSON.parse(JSON.stringify(baseModel()))) as Model
    s.ensureImport(next, 'DI', 'http://opcfoundation.org/UA/DI/')
    s.ensureImport(next, 'DI', 'http://opcfoundation.org/UA/DI/')
    expect(next.imports!.filter((i) => i.uri === 'http://opcfoundation.org/UA/DI/').length).toBe(1)
  })

  it('ensureImport reuses an existing alias for the same URI', () => {
    const s = useDraftStore(fakeApi({}))
    const next = baseModel()
    next.imports!.push({ pos: zero, alias: 'Devices', uri: 'http://opcfoundation.org/UA/DI/' })
    s.ensureImport(next, 'DI', 'http://opcfoundation.org/UA/DI/')
    expect(next.imports!.filter((i) => i.uri === 'http://opcfoundation.org/UA/DI/').length).toBe(1)
    expect(next.imports!.find((i) => i.uri === 'http://opcfoundation.org/UA/DI/')!.alias).toBe('Devices')
  })

  it('extendType adds the import and a based ObjectType, then PUTs YAML', async () => {
    const cap: { yaml?: string } = {}
    const s = await seeded(cap)
    await s.extendType({ name: 'MyDevice', baseAlias: 'DI', baseName: 'DeviceType', baseUri: 'http://opcfoundation.org/UA/DI/' })
    // emitModel quotes URIs (they contain ':'/'/'): `DI: "http://.../DI/"`.
    expect(cap.yaml).toContain('DI: "http://opcfoundation.org/UA/DI/"')
    expect(cap.yaml).toContain('MyDevice:')
    expect(cap.yaml).toContain('base: DI:DeviceType')
  })

  it('addCompanionInstance imports + scaffolds an instance', async () => {
    const cap: { yaml?: string } = {}
    const s = await seeded(cap)
    await s.addCompanionInstance({ name: 'Pump1', under: 'EquipmentType', typeAlias: 'Machinery', typeName: 'MachineType', typeUri: 'http://opcfoundation.org/UA/Machinery/' })
    // emitModel quotes URIs; type/under refs (no scheme) stay unquoted.
    expect(cap.yaml).toContain('Machinery: "http://opcfoundation.org/UA/Machinery/"')
    expect(cap.yaml).toContain('type: Machinery:MachineType')
    expect(cap.yaml).toContain('under: EquipmentType')
  })

  it('setMemberType imports + sets member type ref', async () => {
    const cap: { yaml?: string } = {}
    const s = await seeded(cap)
    // seed a member on EquipmentType
    ;(s.model!.objectTypes![0]!.members ??= []).push({ pos: zero, name: 'sensor', kind: 'object', rule: 'mandatory' })
    await s.setMemberType({ typeName: 'EquipmentType', member: 'sensor', refAlias: 'DI', refName: 'DeviceType', refUri: 'http://opcfoundation.org/UA/DI/' })
    expect(cap.yaml).toContain('type: DI:DeviceType')
  })
})
