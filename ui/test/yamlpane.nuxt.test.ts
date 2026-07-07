// @vitest-environment nuxt
//
// YamlPane integration test. Uses the Harness pattern from shell.nuxt.test.ts:
// the store is created INSIDE setup() so it registers on Nuxt's own Pinia (not
// an outer createPinia()), and the component-under-test's useDraftStore() then
// returns the same fake-backed instance.
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile, diagnostics as fixtureDiags } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import YamlPane from '~/components/YamlPane.vue'

const FAKE_DRAFT_ID = 'deadbeef-yaml-0001'
const YAML_CONTENT = 'model: Demo\nnamespace: urn:x\n'

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
    getDraftFile: vi.fn().mockResolvedValue(YAML_CONTENT),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({
      file: seedFile,
      diagnostics: fixtureDiags,
    } satisfies ValidateResponse),
    previewModelDesign: vi.fn().mockResolvedValue('<ModelDesign/>'),
    previewDiagram: vi.fn().mockResolvedValue('classDiagram'),
    diff: vi.fn().mockResolvedValue({ changes: [], text: "" }),
    resolved: vi.fn().mockResolvedValue({ type: 'FurnaceType', members: [] }),
    propose: vi.fn().mockResolvedValue({ url: 'https://github.com/org/repo/pull/1' }),
    getUnits: vi.fn().mockResolvedValue([]),
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
    // populate draftId + file so the pane's onMounted(loadYaml) actually runs
    await store.createDraft()
    return () => h(UApp, null, { default: () => h(YamlPane) })
  },
})

async function mountYamlPane(client: ApiClient = makeFakeClient()) {
  clientForHarness = client
  captured = null
  const wrapper = await mountSuspended(Harness)
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

describe('YamlPane', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('renders canonical YAML from the store', async () => {
    const { wrapper } = await mountYamlPane()
    const pane = wrapper.find('[data-pane="yaml"]')
    expect(pane.exists()).toBe(true)
    expect(pane.text()).toContain('model: Demo')
  })

  it('toggles soft-wrap on the YAML block', async () => {
    const { wrapper } = await mountYamlPane()
    const pre = () => wrapper.find('pre')
    // Default is now wrap-on (L2): long lines soft-wrap instead of clipping.
    expect(pre().classes()).toContain('whitespace-pre-wrap')
    await wrapper.find('[data-testid="yaml-wrap-toggle"]').trigger('click')
    expect(pre().classes()).toContain('whitespace-pre')
  })
})
