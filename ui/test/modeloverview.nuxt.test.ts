// @vitest-environment nuxt
//
// ModelOverview integration test. Uses the Harness pattern (store created INSIDE
// setup() so it registers on Nuxt's own Pinia) from diagrampane.nuxt.test.ts.
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { h, defineComponent, nextTick } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile, diagnostics as fixtureDiags } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import ModelOverview from '~/components/ModelOverview.vue'

vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(async () => ({ svg: '<svg data-ok="1"></svg>' })),
  },
}))

const FAKE_DRAFT_ID = 'deadbeef-overview-001'
const DIAGRAM_SRC = 'classDiagram\n  A <|-- B'

function makeFakeClient(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    getSchema: vi.fn().mockResolvedValue({}),
    getModel: vi.fn().mockResolvedValue({ file: seedFile, model: equipmentModel, diagnostics: [] }),
    createDraft: vi.fn().mockResolvedValue({
      id: FAKE_DRAFT_ID, baseRef: 'main', files: [seedFile],
    } satisfies CreateDraftResponse),
    getDraftModel: vi.fn().mockResolvedValue({ file: seedFile, model: equipmentModel, diagnostics: fixtureDiags }),
    getDraftFile: vi.fn().mockResolvedValue('model: Demo\n'),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({ file: seedFile, diagnostics: fixtureDiags } satisfies ValidateResponse),
    previewModelDesign: vi.fn().mockResolvedValue('<ModelDesign/>'),
    previewDiagram: vi.fn().mockResolvedValue(DIAGRAM_SRC),
    diff: vi.fn().mockResolvedValue({ changes: [], text: '' }),
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

let captured: ReturnType<typeof useDraftStore> | null = null
let clientForHarness: ApiClient = makeFakeClient()

const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    captured = store
    await store.createDraft()
    return () => h(UApp, null, { default: () => h(ModelOverview) })
  },
})

async function mountOverview(client: ApiClient = makeFakeClient()) {
  clientForHarness = client
  captured = null
  const wrapper = await mountSuspended(Harness)
  await flushPromises()
  await nextTick()
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

describe('ModelOverview', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('shows the "Model overview" caption and renders the diagram', async () => {
    const { wrapper } = await mountOverview()
    expect(wrapper.text()).toContain('Model overview')
    // Mermaid is mocked to return a real SVG, so the diagram (not the empty
    // fallback) must render — assert the strong signal.
    expect(wrapper.find('[data-pane="diagram"]').exists()).toBe(true)
  })
})
