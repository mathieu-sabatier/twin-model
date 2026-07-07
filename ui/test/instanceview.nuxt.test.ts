// @vitest-environment nuxt
//
// Tests for InstanceView — ISA-95 breadcrumb + level row (Task B5).
// Harness pattern: create the draft store with a fake client inside setup() so
// Pinia's by-id 'draft' cache hands InstanceView the same fake-backed instance
// (see treepane.nuxt.test.ts / bottombar.nuxt.test.ts). The breadcrumb walks
// `store.instances`, which comes from `store.model.instances`, so we seed it by
// mutating `captured.model` directly after mount — a per-test client override
// does NOT work under mountSuspended (the store is cached per test file; see
// B4's confirmed finding referenced in treepane.nuxt.test.ts).
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import { indexDiagnostics } from '~/lib/diagnosticPath'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { Instance, ValidateResponse } from '~/types'
import InstanceView from '~/components/InstanceView.vue'

const FAKE_DRAFT_ID = 'deadbeefcafe0030'

function makeFakeClient(): ApiClient {
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
      diagnostics: [],
    }),
    getDraftFile: vi.fn().mockResolvedValue('model: Demo\n'),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({ file: seedFile, diagnostics: [] } satisfies ValidateResponse),
    previewModelDesign: vi.fn().mockResolvedValue('<ModelDesign/>'),
    previewDiagram: vi.fn().mockResolvedValue('classDiagram'),
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
  }
}

const zero = { file: seedFile, line: 0, col: 0 }

/** Site1 (level Site) <- Line1 (level WorkCenter) <- M (equipment), an under-chain
 *  rooted at an import-target (alias-qualified `under`), per the B2/B5 design. */
const site1: Instance = {
  pos: zero,
  name: 'Site1',
  type: { pos: zero, name: 'FurnaceType', raw: 'FurnaceType' },
  under: { pos: zero, alias: 'OpcUa', name: 'ObjectsFolder', raw: 'OpcUa:ObjectsFolder' },
  level: 'Site',
}
const line1: Instance = {
  pos: zero,
  name: 'Line1',
  type: { pos: zero, name: 'FurnaceType', raw: 'FurnaceType' },
  under: { pos: zero, name: 'Site1', raw: 'Site1' },
  level: 'WorkCenter',
}
const equipmentM: Instance = {
  pos: zero,
  name: 'M',
  type: { pos: zero, name: 'FurnaceType', raw: 'FurnaceType' },
  under: { pos: zero, name: 'Line1', raw: 'Line1' },
}

let captured: ReturnType<typeof useDraftStore> | null = null
const sharedClient: ApiClient = makeFakeClient()

const Harness = defineComponent({
  props: { instance: { type: Object as () => Instance, required: true } },
  async setup(props) {
    const store = useDraftStore(sharedClient)
    captured = store
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          h(InstanceView, { instance: props.instance, diagnosticIndex: indexDiagnostics([]) }),
      })
  },
})

async function mountInstanceView(instance: Instance) {
  captured = null
  const wrapper = await mountSuspended(Harness, { props: { instance } })
  await flushPromises()
  // Seed the under-chain onto the store's model so the breadcrumb computed
  // (which walks `store.instances`) can resolve Site1 <- Line1 <- M.
  captured!.model = { ...captured!.model!, instances: [site1, line1, equipmentM] }
  await flushPromises()
  return wrapper
}

describe('InstanceView — ISA-95 breadcrumb + level (B5)', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('shows the ISA-95 breadcrumb for a nested instance', async () => {
    const wrapper = await mountInstanceView(equipmentM)
    const crumb = wrapper.find('[data-testid="isa95-breadcrumb"]')
    expect(crumb.exists()).toBe(true)
    expect(crumb.text()).toContain('Site1')
    expect(crumb.text()).toContain('Line1')
    expect(crumb.text()).toContain('M')
  })

  it('shows the Level row when the instance has a level', async () => {
    const wrapper = await mountInstanceView(site1)
    expect(wrapper.text()).toContain('Level')
    expect(wrapper.text()).toContain('Site')
  })
})
