// Tests for the B6 draft store actions: reparentInstance + assignMembership.
//
// Store-level test (NOT mountSuspended): the store factory accepts an injected
// ApiClient (see app/stores/draft.ts DI note), so we can inject a fake client
// and capture the PUT body directly — unlike component .nuxt.test.ts tests,
// which hit the Nuxt-runtime Pinia and make injected fakes a no-op.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { Model, ValidateResponse } from '~/types'

const FAKE_DRAFT_ID = 'draftfeedface'
const SEED_FILE = 'model.yaml'
const zero = { file: '', line: 0, col: 0 }

/** Minimal model with an instance to reparent (A), a candidate new parent
 *  (Site1), and a perspective (zones) with one empty node (n) to assign into. */
const testModel: Model = {
  pos: zero,
  name: 'TestModel',
  namespace: 'urn:test',
  version: '1.0.0',
  publicationDate: '2024-01-01',
  instances: [
    {
      pos: zero,
      name: 'A',
      type: { raw: 'FooType', name: 'FooType', pos: zero },
      under: { raw: 'OpcUa:ObjectsFolder', name: 'ObjectsFolder', alias: 'OpcUa', pos: zero },
    },
    {
      pos: zero,
      name: 'Site1',
      type: { raw: 'SiteType', name: 'SiteType', pos: zero },
      under: { raw: 'OpcUa:ObjectsFolder', name: 'ObjectsFolder', alias: 'OpcUa', pos: zero },
    },
  ],
  perspectives: [
    {
      pos: zero,
      id: 'zones',
      label: 'Zones',
      nodes: [{ pos: zero, id: 'n', label: 'N', members: [] }],
    },
  ],
}

function makeFakeClient(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    getSchema: vi.fn().mockResolvedValue({}),
    getModel: vi.fn().mockResolvedValue({ file: SEED_FILE, model: testModel, diagnostics: [] }),
    createDraft: vi.fn().mockResolvedValue({
      id: FAKE_DRAFT_ID,
      baseRef: 'main',
      files: [SEED_FILE],
    } satisfies CreateDraftResponse),
    getDraftModel: vi.fn().mockResolvedValue({
      file: SEED_FILE,
      model: testModel,
      diagnostics: [],
    }),
    getDraftFile: vi.fn().mockResolvedValue('model: Demo\n'),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({
      file: SEED_FILE,
      diagnostics: [],
    } satisfies ValidateResponse),
    previewModelDesign: vi.fn().mockResolvedValue('<ModelDesign/>'),
    previewDiagram: vi.fn().mockResolvedValue('classDiagram\n  A <|-- B'),
    diff: vi.fn().mockResolvedValue({ changes: [], text: '' }),
    resolved: vi.fn().mockResolvedValue({ type: 'FooType', members: [] }),
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

const { useDraftStore } = await import('~/stores/draft')

/** Returns the YAML string from the last putFiles call (the only file in the payload). */
function putBody(put: ReturnType<typeof vi.fn>): string {
  const calls = put.mock.calls
  return Object.values(calls.at(-1)![1] as Record<string, string>)[0] as string
}

describe('useDraftStore — reparentInstance / assignMembership (B6)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reparentInstance sets under and saves', async () => {
    const put = vi.fn().mockResolvedValue(undefined)
    const client = makeFakeClient({ putFiles: put })
    const store = useDraftStore(client)
    await store.createDraft()

    await store.reparentInstance('A', 'Site1')

    expect(put).toHaveBeenCalledTimes(1)
    const yaml = putBody(put)
    expect(yaml).toContain('under: Site1')
  })

  it('assignMembership adds an instance to a perspective node', async () => {
    const put = vi.fn().mockResolvedValue(undefined)
    const client = makeFakeClient({ putFiles: put })
    const store = useDraftStore(client)
    await store.createDraft()

    await store.assignMembership({ perspective: 'zones', node: 'n', instance: 'A', mode: 'add' })

    expect(put).toHaveBeenCalledTimes(1)
    const yaml = putBody(put)
    expect(yaml).toContain('members: [A]')
  })

  it('reparentInstance rejects with /frozen/ when store.frozen is true', async () => {
    const put = vi.fn().mockResolvedValue(undefined)
    const client = makeFakeClient({ putFiles: put })
    const store = useDraftStore(client)
    await store.createDraft()
    store.frozen = true

    await expect(store.reparentInstance('A', 'Site1')).rejects.toThrow(/frozen/)
    expect(put).not.toHaveBeenCalled()
  })

  it('assignMembership mode "remove" removes the instance from the node', async () => {
    const put = vi.fn().mockResolvedValue(undefined)
    const client = makeFakeClient({ putFiles: put })
    const store = useDraftStore(client)
    await store.createDraft()

    await store.assignMembership({ perspective: 'zones', node: 'n', instance: 'A', mode: 'add' })
    put.mockClear()
    await store.assignMembership({ perspective: 'zones', node: 'n', instance: 'A', mode: 'remove' })

    const yaml = putBody(put)
    expect(yaml).not.toMatch(/members: \[A\]/)
  })
})
