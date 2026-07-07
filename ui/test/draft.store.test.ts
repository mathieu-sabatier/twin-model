// Tests for app/stores/draft.ts — the read-side Pinia draft store.
//
// Client mock strategy (per Task 3 brief, DI alternative):
//   mockNuxtImport('useApi', ...) requires useApi to be a Nuxt auto-import,
//   which it is NOT (it's a manual export from ~/api). Instead we use the
//   equally-acceptable DI approach: the store factory accepts an optional client
//   argument. In tests we pass a fakeClient backed by real fixtures; in the app
//   the store calls useApi() internally when no client is injected.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { equipmentModel, diagnostics as fixtureDiags, seedFile, repoInfoSample } from '~/mocks/fixtures'
import { ProposeConflictError } from '~/api'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'

// ── Fake client (backed by real fixtures) ────────────────────────────────────

const FAKE_DRAFT_ID = 'draftdeadbeef'

function makeFakeClient(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    getSchema: vi.fn().mockResolvedValue({}),
    getModel: vi.fn().mockResolvedValue({ file: seedFile, model: equipmentModel, diagnostics: [] }),
    createDraft: vi.fn().mockResolvedValue({
      id: FAKE_DRAFT_ID,
      baseRef: 'main',
      files: [seedFile],
    } satisfies CreateDraftResponse),
    getDraftModel: vi.fn().mockResolvedValue({
      file: seedFile,
      model: equipmentModel,
      diagnostics: fixtureDiags,
    }),
    getDraftFile: vi.fn().mockResolvedValue('model: Demo\n'),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({
      file: seedFile,
      diagnostics: fixtureDiags,
    } satisfies ValidateResponse),
    previewModelDesign: vi.fn().mockResolvedValue('<ModelDesign/>'),
    previewDiagram: vi.fn().mockResolvedValue('classDiagram\n  A <|-- B'),
    diff: vi.fn().mockResolvedValue({ changes: [], text: "" }),
    resolved: vi.fn().mockResolvedValue({ type: 'FurnaceType', members: [] }),
    propose: vi.fn().mockResolvedValue({ url: 'https://github.com/org/repo/pull/1' }),
    getUnits: vi.fn().mockResolvedValue([]),
    getCatalog: vi.fn().mockResolvedValue([]),
    getCatalogTypes: vi.fn().mockResolvedValue([]),
    getCatalogType: vi.fn().mockResolvedValue({ alias: '', uri: '', name: '', nodeClass: 'ObjectType', abstract: false, baseChain: [], members: [] }),
    searchCatalog: vi.fn().mockResolvedValue([]),
    getRepo: vi.fn().mockResolvedValue({
      host: 'github', owner: 'mathieu-sabatier', repo: 'twin-model',
      url: 'https://github.com/mathieu-sabatier/twin-model', defaultBranch: 'main',
      commitName: 'twinmodel-bot', commitEmail: 'bot@twinmodel',
      proposeEnabled: true, proposeReason: '',
    }),
    listPRs: vi.fn().mockResolvedValue([]),
    listBranches: vi.fn().mockResolvedValue({ branches: ['main'], defaultBranch: 'main' }),
    ...overrides,
  }
}

// Import store factory — DI approach: pass fakeClient to useDraftStore(client).
const { useDraftStore } = await import('~/stores/draft')

// ── Helpers ──────────────────────────────────────────────────────────────────

/** Returns the YAML string from the last putFiles call (the only file in the payload). */
function lastPutYaml(c: ApiClient): string {
  const calls = (c.putFiles as any).mock.calls
  return Object.values(calls.at(-1)[1])[0] as string
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('useDraftStore — initial state', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
  })

  it('starts empty (no draftId, no model, not loading)', () => {
    const store = useDraftStore(fakeClient)
    expect(store.draftId).toBeNull()
    expect(store.model).toBeNull()
    expect(store.diagnostics).toEqual([])
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
    expect(store.frozen).toBe(false)
  })

  it('diagnosticIndex is available on an empty store (returns empty arrays)', () => {
    const store = useDraftStore(fakeClient)
    expect(store.diagnosticIndex.forType('FurnaceType')).toEqual([])
    expect(store.diagnosticIndex.forModelField('name')).toEqual([])
  })

  it('errorCount and hasErrors are 0/false on empty store', () => {
    const store = useDraftStore(fakeClient)
    expect(store.errorCount).toBe(0)
    expect(store.hasErrors).toBe(false)
  })
})

describe('useDraftStore — createDraft()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
  })

  it('sets draftId and file, loads model from fixtures', async () => {
    const store = useDraftStore(fakeClient)
    const id = await store.createDraft()
    expect(id).toBe(FAKE_DRAFT_ID)
    expect(store.draftId).toBe(FAKE_DRAFT_ID)
    expect(store.file).toBe(seedFile)
    // model is populated from the fixture (getDraftModel resolves with equipmentModel)
    expect(store.model?.name).toBe('AcmeEquipment')
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('populates diagnostics with fixture diagnostics', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    expect(store.diagnostics).toHaveLength(fixtureDiags.length)
    expect(store.diagnostics[0]!.code).toBe(fixtureDiags[0]!.code)
  })

  it('diagnosticIndex reflects the fixture diagnostics after createDraft', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    // Fixture diag[0]: path 'object_types/FurnaceType/members/Efficiency/unit'
    const hits = store.diagnosticIndex.forMemberField('FurnaceType', 'Efficiency', 'unit')
    expect(hits).toHaveLength(1)
    expect(hits[0]!.code).toBe('unit-on-non-numeric')
  })

  it('errorCount reflects error-severity diagnostics only', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    // Fixture has 1 error + 1 warning
    const errCount = fixtureDiags.filter((d) => d.severity === 'error').length
    expect(store.errorCount).toBe(errCount)
  })

  it('hasErrors is true when there are error-severity diagnostics', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    expect(store.hasErrors).toBe(true)
  })
})

describe('useDraftStore — loadDraft()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
  })

  it('restores draftId + file from route params and loads model', async () => {
    const store = useDraftStore(fakeClient)
    await store.loadDraft(FAKE_DRAFT_ID, seedFile)
    expect(store.draftId).toBe(FAKE_DRAFT_ID)
    expect(store.file).toBe(seedFile)
    expect(store.model?.name).toBe('AcmeEquipment')
  })

  it('no localStorage — draftId comes in as a param', async () => {
    const store = useDraftStore(fakeClient)
    // Simulate page refresh: route provides the id, not localStorage.
    await store.loadDraft('some-other-id')
    expect(store.draftId).toBe('some-other-id')
  })
})

describe('useDraftStore — computed getters', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
  })

  it('objectTypes convenience getter', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    expect(Array.isArray(store.objectTypes)).toBe(true)
    expect(store.objectTypes.length).toBeGreaterThan(0)
  })

  it('instances convenience getter', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    expect(Array.isArray(store.instances)).toBe(true)
    expect(store.instances.length).toBeGreaterThan(0)
  })

  it('enums convenience getter', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    expect(Array.isArray(store.enums)).toBe(true)
    expect(store.enums.length).toBeGreaterThan(0)
  })

  it('imports convenience getter', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    expect(Array.isArray(store.imports)).toBe(true)
  })
})

describe('useDraftStore — loadYaml() and loadDiagram()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
  })

  it('loadYaml and loadDiagram populate their state from the client', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft('main')
    await store.loadYaml()
    await store.loadDiagram('types')
    expect(store.yaml).toContain('model')
    expect(store.diagramSrc).toContain('classDiagram')
  })

  it('loadDiagram failure does NOT raise the global error banner', async () => {
    const fake = makeFakeClient({
      previewDiagram: vi.fn().mockRejectedValue(new Error('[GET] "/api/…/preview/diagram": 422 Unprocessable Entity')),
    })
    const store = useDraftStore(fake)
    store.draftId = 'd1'; store.file = 'equipment.yaml'
    await store.loadDiagram()
    expect(store.error).toBeNull()      // preview failure is not a blocking, banner-worthy error
    expect(store.diagramSrc).toBeNull()
  })
})

describe('useDraftStore — validateNow() and scheduleValidate()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('validateNow() calls validate on the client and updates diagnostics', async () => {
    const store = useDraftStore(fakeClient)
    await store.loadDraft(FAKE_DRAFT_ID, seedFile)
    vi.mocked(fakeClient.validate).mockClear()
    await store.validateNow()
    expect(fakeClient.validate).toHaveBeenCalledWith(FAKE_DRAFT_ID, seedFile)
    expect(store.diagnostics).toHaveLength(fixtureDiags.length)
  })

  it('scheduleValidate debounces: multiple rapid calls → ONE validate call', async () => {
    const store = useDraftStore(fakeClient)
    // loadDraft without fake timers to avoid issues, then set up timers
    vi.useRealTimers()
    await store.loadDraft(FAKE_DRAFT_ID, seedFile)
    vi.useFakeTimers()

    // Reset call count after setup
    vi.mocked(fakeClient.validate).mockClear()

    // Rapid-fire 5 calls — should be debounced to 1
    store.scheduleValidate()
    store.scheduleValidate()
    store.scheduleValidate()
    store.scheduleValidate()
    store.scheduleValidate()

    // Before debounce window: validate should NOT have been called yet
    expect(fakeClient.validate).not.toHaveBeenCalled()

    // Advance past the debounce window (≈300ms)
    await vi.runAllTimersAsync()

    // Exactly ONE call despite 5 rapid invocations
    expect(fakeClient.validate).toHaveBeenCalledTimes(1)
  })
})

describe('useDraftStore — saveModel()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient()
    setActivePinia(createPinia())
  })

  it('saveModel emits YAML, PUTs it, and re-fetches the canonical model', async () => {
    // Use a distinct "reloaded" model to verify the store uses the server's AST
    const reloadedModel = { ...equipmentModel, name: 'ReloadedModel' }
    const putFilesSpy = vi.fn().mockResolvedValue(undefined)
    const getDraftModelSpy = vi.fn().mockResolvedValue({
      file: seedFile,
      model: reloadedModel,
      diagnostics: [],
    })
    fakeClient = makeFakeClient({
      putFiles: putFilesSpy,
      getDraftModel: getDraftModelSpy,
    })
    const store = useDraftStore(fakeClient)
    await store.createDraft()

    // Reset call counts after setup
    putFilesSpy.mockClear()
    getDraftModelSpy.mockClear()

    await store.saveModel(store.model!)

    // putFiles called once with { [file]: <yaml containing 'model:'> }
    expect(putFilesSpy).toHaveBeenCalledTimes(1)
    const [calledDraftId, calledFiles] = putFilesSpy.mock.calls[0] as [string, Record<string, string>]
    expect(calledDraftId).toBe(FAKE_DRAFT_ID)
    expect(typeof calledFiles[seedFile]).toBe('string')
    expect(calledFiles[seedFile]).toMatch(/^model:/m)

    // getDraftModel re-fetched and store.model is the reloaded server model
    expect(getDraftModelSpy).toHaveBeenCalledTimes(1)
    expect(store.model?.name).toBe('ReloadedModel')
  })

  it('saveModel refuses when the draft is frozen', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    store.frozen = true

    await expect(store.saveModel(store.model!)).rejects.toThrow(/frozen/)
    expect(fakeClient.putFiles).not.toHaveBeenCalled()
  })

  it('saving defaults to false on a fresh store', () => {
    const store = useDraftStore(fakeClient)
    expect(store.saving).toBe(false)
  })

  it('saveModel: rejected putFiles sets store.error, leaves saving=false, and rejects', async () => {
    const putErr = new Error('network error')
    const failingPutFiles = vi.fn().mockRejectedValue(putErr)
    fakeClient = makeFakeClient({ putFiles: failingPutFiles })
    const store = useDraftStore(fakeClient)
    await store.createDraft()

    const promise = store.saveModel(store.model!)
    // Promise should reject
    await expect(promise).rejects.toThrow('network error')
    // store.error is set to the error message
    expect(store.error).toBe('network error')
    // finally ran: saving is back to false
    expect(store.saving).toBe(false)
  })
})

describe('useDraftStore — loadUnits()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient({
      getUnits: vi.fn().mockResolvedValue([
        { symbol: '°C', displayName: '°C', description: 'degree Celsius' },
        { symbol: 'bar', displayName: 'bar', description: 'bar' },
      ]),
    })
    setActivePinia(createPinia())
  })

  it('starts with an empty units array', () => {
    const store = useDraftStore(fakeClient)
    expect(store.units).toEqual([])
  })

  it('loadUnits() populates units from the client', async () => {
    const store = useDraftStore(fakeClient)
    await store.loadUnits()
    expect(store.units).toHaveLength(2)
    expect(store.units[0]!.symbol).toBe('°C')
    expect(store.units[1]!.symbol).toBe('bar')
  })

  it('loadUnits() is idempotent — second call does not re-fetch', async () => {
    const store = useDraftStore(fakeClient)
    await store.loadUnits()
    await store.loadUnits()
    expect(fakeClient.getUnits).toHaveBeenCalledTimes(1)
  })
})

describe('useDraftStore — resolvedFor()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient({
      resolved: vi.fn().mockResolvedValue({ type: 'FurnaceType', members: [{ name: 'DoorClosed', kind: 'variable', rule: 'mandatory', pos: { file: '', line: 0, col: 0 }, declaredIn: 'FurnaceType' }] }),
    })
    setActivePinia(createPinia())
  })

  it('calls api.resolved once and caches — second call does not re-fetch', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()

    const first = await store.resolvedFor('FurnaceType')
    const second = await store.resolvedFor('FurnaceType')

    expect(fakeClient.resolved).toHaveBeenCalledTimes(1)
    expect(first).toHaveLength(1)
    expect(first[0]!.name).toBe('DoorClosed')
    // Same array contents — deep equality (spread creates a new object wrapper but
    // the array reference inside resolvedCache.value[type] is the same).
    expect(second).toStrictEqual(first)
  })

  it('returns [] when no draftId is set', async () => {
    const store = useDraftStore(fakeClient)
    // draftId is null initially — no loadDraft/createDraft
    const result = await store.resolvedFor('FurnaceType')
    expect(result).toEqual([])
    expect(fakeClient.resolved).not.toHaveBeenCalled()
  })
})

describe('useDraftStore — createInstance()', () => {
  let fakeClient: ApiClient
  let putFilesSpy: ReturnType<typeof vi.fn>
  let getDraftModelSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    putFilesSpy = vi.fn().mockResolvedValue(undefined)
    getDraftModelSpy = vi.fn().mockResolvedValue({
      file: seedFile,
      model: equipmentModel,
      diagnostics: [],
    })
    fakeClient = makeFakeClient({
      putFiles: putFilesSpy,
      getDraftModel: getDraftModelSpy,
    })
    setActivePinia(createPinia())
  })

  it('calls putFiles once and the PUT body YAML contains the new instance name and type', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()

    // Reset spies after setup
    putFilesSpy.mockClear()
    getDraftModelSpy.mockClear()

    await store.createInstance({ name: 'Furnace09', type: 'FurnaceType', under: 'OpcUa:ObjectsFolder' })

    expect(putFilesSpy).toHaveBeenCalledTimes(1)
    const [calledDraftId, calledFiles] = putFilesSpy.mock.calls[0] as [string, Record<string, string>]
    expect(calledDraftId).toBe(FAKE_DRAFT_ID)
    const yamlBody = calledFiles[seedFile]!
    expect(typeof yamlBody).toBe('string')
    expect(yamlBody).toMatch(/Furnace09:/)
    expect(yamlBody).toMatch(/type: FurnaceType/)
  })

  it('rejects with /frozen/ when store.frozen is true', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    store.frozen = true

    await expect(
      store.createInstance({ name: 'Furnace09', type: 'FurnaceType', under: 'OpcUa:ObjectsFolder' })
    ).rejects.toThrow(/frozen/)
    expect(putFilesSpy).not.toHaveBeenCalled()
  })
})

describe('useDraftStore — loadDiff()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    fakeClient = makeFakeClient({
      diff: vi.fn().mockResolvedValue({
        changes: [{ kind: 'MemberAdded', type: 'FurnaceType', member: 'DoorClosed', text: 'FurnaceType: added member DoorClosed' }],
        text: 'FurnaceType: added member DoorClosed',
      }),
    })
    setActivePinia(createPinia())
  })

  it('populates changes and diffText from the API response', async () => {
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    await store.loadDiff()
    expect(store.changes).toHaveLength(1)
    expect(store.changes[0]!.kind).toBe('MemberAdded')
    expect(store.diffText).toBe('FurnaceType: added member DoorClosed')
  })

  it('is a no-op when there is no draftId', async () => {
    const store = useDraftStore(fakeClient)
    await store.loadDiff()
    expect(fakeClient.diff).not.toHaveBeenCalled()
    expect(store.changes).toEqual([])
    expect(store.diffText).toBe('')
  })
})

describe('useDraftStore — files + setFile (Task 2)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('createDraft populates files and setFile switches the active file + reloads', async () => {
    const getDraftModel = vi.fn().mockResolvedValue({ file: 'a.yaml', model: { pos: { file: '', line: 0, col: 0 } }, diagnostics: [] })
    const fake = makeFakeClient({
      createDraft: vi.fn().mockResolvedValue({ id: 'd1', baseRef: 'main', files: ['a.yaml', 'b.yaml'] }),
      getDraftModel,
    })
    const store = useDraftStore(fake)
    await store.createDraft('main')
    expect(store.files).toEqual(['a.yaml', 'b.yaml'])
    expect(store.file).toBe('a.yaml')

    getDraftModel.mockClear()
    await store.setFile('b.yaml')
    expect(store.file).toBe('b.yaml')
    expect(getDraftModel).toHaveBeenCalledWith('d1', 'b.yaml')
  })
})

describe('useDraftStore — propose()', () => {
  let fakeClient: ApiClient

  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('returns the PR url and sets frozen=true on success', async () => {
    fakeClient = makeFakeClient({
      propose: vi.fn().mockResolvedValue({ url: 'https://github.com/org/repo/pull/7' }),
    })
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    const url = await store.propose({ branch: 'model/update-model', title: 'Update model', message: 'Fix' })
    expect(url).toBe('https://github.com/org/repo/pull/7')
    expect(store.frozen).toBe(true)
  })

  it('on ProposeConflictError: frozen stays false, diagnostics populated, error rethrown', async () => {
    const conflictDiags = [fixtureDiags[0]!]
    fakeClient = makeFakeClient({
      propose: vi.fn().mockRejectedValue(
        new ProposeConflictError('draft has lint errors', conflictDiags),
      ),
    })
    const store = useDraftStore(fakeClient)
    await store.createDraft()
    // Clear diagnostics to verify they are re-populated by the conflict error
    store.diagnostics = []

    await expect(
      store.propose({ branch: 'model/update-model', title: 'Update model', message: 'Fix' })
    ).rejects.toBeInstanceOf(ProposeConflictError)

    expect(store.frozen).toBe(false)
    expect(store.diagnostics).toHaveLength(1)
    expect(store.diagnostics[0]!.code).toBe(fixtureDiags[0]!.code)
  })
})

describe('useDraftStore — loadDiff null-guard (H1)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('coerces a null semantic diff to an empty changes array (no crash)', async () => {
    const client = makeFakeClient({
      // Server sends this when only doc/comment/formatting changed.
      diff: vi.fn().mockResolvedValue({ changes: null, text: '_No semantic changes._' } as never),
    })
    const store = useDraftStore(client)
    await store.createDraft()
    await store.loadDiff()
    expect(Array.isArray(store.changes)).toBe(true)
    expect(store.changes).toHaveLength(0)
    expect(store.diffText).toBe('_No semantic changes._')
  })
})

describe('useDraftStore — recoverOrCreate (H3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reuses the id when the draft still exists', async () => {
    const client = makeFakeClient()
    const store = useDraftStore(client)
    const id = await store.recoverOrCreate('existing-1')
    expect(id).toBe('existing-1')
    expect(store.draftId).toBe('existing-1')
    expect(store.file).toBe(seedFile)
    expect(client.createDraft).not.toHaveBeenCalled()
    expect(store.error).toBeNull()
  })

  it('mints a fresh draft when the stored id 404s, without surfacing an error', async () => {
    const client = makeFakeClient({
      getDraftModel: vi.fn().mockImplementation(async (id: string) => {
        if (id === 'stale-xyz') throw { status: 404 }
        return { file: seedFile, model: equipmentModel, diagnostics: fixtureDiags }
      }),
    })
    const store = useDraftStore(client)
    const id = await store.recoverOrCreate('stale-xyz')
    expect(id).toBe(FAKE_DRAFT_ID) // createDraft's fresh id
    expect(client.createDraft).toHaveBeenCalledTimes(1)
    expect(store.draftId).toBe(FAKE_DRAFT_ID)
    expect(store.file).toBe(seedFile)
    expect(store.error).toBeNull()
  })

  it('does NOT recreate the draft on a file-not-found 404 — it surfaces the error', async () => {
    const client = makeFakeClient({
      getDraftModel: vi.fn().mockRejectedValue({ status: 404, data: { error: 'file not found in draft' } }),
    })
    const store = useDraftStore(client)
    await expect(store.recoverOrCreate('draft-1')).rejects.toBeTruthy()
    expect(client.createDraft).not.toHaveBeenCalled() // no loop
    expect(store.error).toBeTruthy()
  })

  it('recreates the draft only when the draft itself is not found', async () => {
    const client = makeFakeClient({
      getDraftModel: vi.fn().mockImplementation(async (id: string) => {
        if (id === 'gone') throw { status: 404, data: { error: 'draft not found' } }
        return { file: seedFile, model: equipmentModel, diagnostics: fixtureDiags }
      }),
    })
    const store = useDraftStore(client)
    const id = await store.recoverOrCreate('gone')
    expect(id).toBe(FAKE_DRAFT_ID)
    expect(client.createDraft).toHaveBeenCalledTimes(1)
    expect(store.error).toBeNull()
  })
})

describe('useDraftStore — nameTaken / deleteInstance / renameInstance', () => {
  let client: ApiClient
  let putFilesSpy: ReturnType<typeof vi.fn>
  let getDraftModelSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    putFilesSpy = vi.fn().mockResolvedValue(undefined)
    getDraftModelSpy = vi.fn().mockResolvedValue({
      file: seedFile,
      model: equipmentModel,
      diagnostics: [],
    })
    client = makeFakeClient({
      putFiles: putFilesSpy,
      getDraftModel: getDraftModelSpy,
    })
    setActivePinia(createPinia())
  })

  it('nameTaken reports existing instance names', () => {
    const store = useDraftStore(client)
    store.model = equipmentModel
    expect(store.nameTaken('Furnace01')).toBe(true)
    expect(store.nameTaken('Nonexistent')).toBe(false)
  })

  it('deleteInstance removes the instance and saves', async () => {
    const store = useDraftStore(client)
    store.model = equipmentModel
    store.draftId = FAKE_DRAFT_ID
    store.file = seedFile
    await store.deleteInstance('Furnace01')
    const yaml = lastPutYaml(client)
    expect(yaml).not.toMatch(/^\s*Furnace01:/m)
    expect(client.putFiles).toHaveBeenCalled()
  })

  it('renameInstance rewrites the key and saves', async () => {
    const store = useDraftStore(client)
    store.model = equipmentModel
    store.draftId = FAKE_DRAFT_ID
    store.file = seedFile
    await store.renameInstance('Furnace01', 'FurnaceX')
    const yaml = lastPutYaml(client)
    expect(yaml).toMatch(/^\s*FurnaceX:/m)
    expect(yaml).not.toMatch(/^\s*Furnace01:/m)
  })
})

describe('useDraftStore — preview fetch de-dup (L3)', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loadYaml fetches once for repeated calls with the same model, again after model changes', async () => {
    const getDraftFile = vi.fn().mockResolvedValue('yaml: text')
    const store = useDraftStore(makeFakeClient({ getDraftFile }))
    await store.createDraft('main') // sets draftId + file + model
    getDraftFile.mockClear()

    await store.loadYaml()
    await store.loadYaml()
    await store.loadYaml()
    expect(getDraftFile).toHaveBeenCalledTimes(1) // de-duped

    store.model = { ...(store.model as object) } as never // new reference = an edit landed
    await store.loadYaml()
    expect(getDraftFile).toHaveBeenCalledTimes(2) // refreshed
  })

  it('loadYaml retries after a failed fetch (a failure does not lock out the same key)', async () => {
    const getDraftFile = vi
      .fn()
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValue('yaml: text')
    const store = useDraftStore(makeFakeClient({ getDraftFile }))
    await store.createDraft('main')
    getDraftFile.mockClear()

    await store.loadYaml() // fails → key cleared
    expect(getDraftFile).toHaveBeenCalledTimes(1)
    await store.loadYaml() // same (draft,file,model) but retry is allowed
    expect(getDraftFile).toHaveBeenCalledTimes(2)
  })

  it('loadDiagram de-dups per (draft,file,view,model) and refreshes on view change', async () => {
    const previewDiagram = vi.fn().mockResolvedValue('classDiagram')
    const store = useDraftStore(makeFakeClient({ previewDiagram }))
    await store.createDraft('main')
    previewDiagram.mockClear()

    await store.loadDiagram()
    await store.loadDiagram()
    expect(previewDiagram).toHaveBeenCalledTimes(1) // de-duped

    await store.loadDiagram('instances') // different view → refetch
    expect(previewDiagram).toHaveBeenCalledTimes(2)
  })
})

describe('draft store — repo context & branch switching', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loadRepo populates repo from the client', async () => {
    const store = useDraftStore(makeFakeClient({
      getRepo: vi.fn().mockResolvedValue(repoInfoSample),
    }))
    await store.loadRepo()
    expect(store.repo?.owner).toBe('mathieu-sabatier')
    expect(store.repo?.proposeEnabled).toBe(true)
  })

  it('loadRepo degrades silently when the client throws', async () => {
    const store = useDraftStore(makeFakeClient({
      getRepo: vi.fn().mockRejectedValue(new Error('boom')),
    }))
    await store.loadRepo()
    expect(store.repo).toBeNull()
    expect(store.error).toBeNull() // no user-facing error; chip just hides
  })

  it('loadBranchOptions builds options from listBranches (default first, base included, deduped)', async () => {
    const store = useDraftStore(makeFakeClient({
      createDraft: vi.fn().mockResolvedValue({ id: 'draftmain01', baseRef: 'main', files: [seedFile] }),
      listBranches: vi.fn().mockResolvedValue({
        branches: ['main', 'model/furnace-zones', 'model/press-curve'],
        defaultBranch: 'main',
      }),
    }))
    await store.createDraft('main')
    await store.loadBranchOptions()
    expect(store.branchOptions).toContain('main')
    expect(store.branchOptions).toContain('model/furnace-zones')
    expect(store.branchOptions).toContain('model/press-curve')
    expect(store.branchOptions[0]).toBe('main')                 // default first
    expect(new Set(store.branchOptions).size).toBe(store.branchOptions.length) // deduped
  })

  it('loadBranchOptions degrades to default + base when listBranches rejects', async () => {
    const store = useDraftStore(makeFakeClient({
      createDraft: vi.fn().mockResolvedValue({ id: 'draftdev01', baseRef: 'dev', files: [seedFile] }),
      listBranches: vi.fn().mockRejectedValue(new Error('offline')),
    }))
    await store.createDraft('dev')
    await store.loadBranchOptions()
    expect(store.branchOptions).toContain('main')   // default fallback
    expect(store.branchOptions).toContain('dev')    // current base preserved
    expect(store.error).toBeNull()                  // silent degrade, no user error
  })

  it('switchBranch creates a fresh draft off the new base and returns its id', async () => {
    const createDraft = vi.fn().mockResolvedValue({ id: 'draftbeef01', baseRef: 'model/press-curve', files: [seedFile] })
    const store = useDraftStore(makeFakeClient({ createDraft }))
    const id = await store.switchBranch('model/press-curve')
    expect(id).toBe('draftbeef01')
    expect(createDraft).toHaveBeenCalledWith('model/press-curve')
    expect(store.baseRef).toBe('model/press-curve')
  })

  it('switchBranch on a missing branch keeps the current draft and restores the branch', async () => {
    // createDraft resolves for the initial 'main' draft, then rejects for the bad
    // branch with a 404-shaped error body (as ofetch surfaces it on `.data`).
    const createDraft = vi.fn()
      .mockResolvedValueOnce({ id: 'draftmain01', baseRef: 'main', files: [seedFile] })
      .mockRejectedValueOnce(Object.assign(new Error('404'), { data: { error: 'branch "nope" not found' } }))
    const store = useDraftStore(makeFakeClient({ createDraft }))

    await store.createDraft('main')
    expect(store.draftId).toBe('draftmain01')

    const id = await store.switchBranch('nope')
    expect(id).toBeNull()
    expect(store.baseRef).toBe('main')                // restored
    expect(store.draftId).toBe('draftmain01')         // current draft kept
    expect(store.error).toContain('branch "nope" not found')
  })
})
