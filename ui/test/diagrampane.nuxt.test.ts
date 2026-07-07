// @vitest-environment nuxt
//
// DiagramPane integration test. Uses the Harness pattern from shell.nuxt.test.ts:
// the store is created INSIDE setup() so it registers on Nuxt's own Pinia (not
// an outer createPinia()), and the component-under-test's useDraftStore() then
// returns the same fake-backed instance.
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { h, defineComponent, nextTick } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile, diagnostics as fixtureDiags } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import DiagramPane from '~/components/DiagramPane.vue'

vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(async () => ({ svg: '<svg data-ok="1"></svg>' })),
  },
}))

const FAKE_DRAFT_ID = 'deadbeef-diagram-0001'
const DIAGRAM_SRC = 'classDiagram\n  A <|-- B'

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
    previewDiagram: vi.fn().mockResolvedValue(DIAGRAM_SRC),
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

let captured: ReturnType<typeof useDraftStore> | null = null
let clientForHarness: ApiClient = makeFakeClient()

const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    captured = store
    // populate draftId + file so the pane's onMounted(loadDiagram) actually runs
    await store.createDraft()
    return () => h(UApp, null, { default: () => h(DiagramPane) })
  },
})

async function mountDiagramPane(client: ApiClient = makeFakeClient()) {
  clientForHarness = client
  captured = null
  const wrapper = await mountSuspended(Harness)
  // First flush: resolves loadDiagram → sets diagramSrc
  await flushPromises()
  // Second flush + tick: watcher fires renderDiagram → sets svgHtml → DOM updates
  await nextTick()
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

describe('DiagramPane', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('renders SVG from Mermaid source in the store', async () => {
    const { wrapper } = await mountDiagramPane()
    const pane = wrapper.find('[data-pane="diagram"]')
    expect(pane.exists()).toBe(true)
    expect(pane.html()).toContain('svg')
  })
})

describe('DiagramPane — selection hint (L3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('shows a type-hierarchy note when an enum or instance is selected', async () => {
    const { wrapper } = await mountDiagramPane()
    const { select, clear } = useSelection()
    select({ kind: 'enum', name: 'EquipmentState' })
    await flushPromises()
    expect(wrapper.find('[data-pane="diagram-note"]').exists()).toBe(true)
    clear()
  })
})

describe('DiagramPane — focus wiring (L1)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('applies focus style to the rendered SVG when a type is selected', async () => {
    const { store, wrapper } = await mountDiagramPane()
    const mermaid = (await import('mermaid')).default
    const { select, clear } = useSelection()

    // Force a fresh render cycle so rawSvg is populated in this component
    // (mountSuspended caches the Harness async setup across tests; nudge the
    //  diagramSrc watcher to fire so renderDiagram runs and sets rawSvg).
    store.diagramSrc = ''
    await flushPromises()
    vi.mocked(mermaid.render).mockResolvedValueOnce({ svg: '<svg id="classId-A-1"></svg>', diagramType: 'classDiagram' })
    store.diagramSrc = DIAGRAM_SRC
    await nextTick()
    await flushPromises()

    // Before selection: no <style> injected by focusDiagram.
    const paneBefore = wrapper.find('[data-pane="diagram"]')
    expect(paneBefore.element.querySelector('style')).toBeNull()

    select({ kind: 'type', name: 'FurnaceType' })
    await nextTick()
    await nextTick()
    await flushPromises()

    // After selection: focusDiagram has injected a <style> into the SVG.
    // (JSDOM strips CSS text from SVG-embedded <style> tags set via innerHTML,
    //  so we assert presence of the <style> element, not its text content.)
    const pane = wrapper.find('[data-pane="diagram"]')
    expect(pane.exists()).toBe(true)
    const styleEl = pane.element.querySelector('style')
    expect(styleEl).not.toBeNull()
    clear()
  })
})

describe('DiagramPane — min-height floor (M1)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('M1: the diagram pane has a min-height floor so cold-load render is measurable', async () => {
    const { wrapper } = await mountDiagramPane()
    const pane = wrapper.find('[data-pane="diagram"]')
    expect(pane.exists()).toBe(true)
    expect(pane.classes()).toContain('min-h-[240px]')
  })
})

describe('DiagramPane — rendering skeleton (L3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('L3: shows a rendering skeleton until the SVG resolves', async () => {
    let resolveRender!: (v: { svg: string }) => void
    const mermaid = (await import('mermaid')).default
    // mount with a client whose previewDiagram won't auto-set diagramSrc, so we
    // can control the render timing manually
    const slowClient = makeFakeClient({
      previewDiagram: vi.fn().mockResolvedValue(DIAGRAM_SRC),
    })
    vi.mocked(mermaid.render).mockImplementationOnce(
      () => new Promise((r) => { resolveRender = r }),
    )
    const { store, wrapper } = await mountDiagramPane(slowClient)
    // force a re-render by mutating diagramSrc (watcher will call renderDiagram)
    store.diagramSrc = ''
    await flushPromises()
    // set deferred render mock for the next render call
    vi.mocked(mermaid.render).mockImplementationOnce(
      () => new Promise((r) => { resolveRender = r }),
    )
    store.diagramSrc = 'classDiagram\n  A <|-- B'
    await nextTick()
    // rendering is in progress — skeleton should be visible
    expect(wrapper.find('[data-pane="diagram-loading"]').exists()).toBe(true)
    // allow renderDiagram's internal nextTick() to complete so mermaid.render is called
    await nextTick()

    resolveRender({ svg: '<svg id="classId-A-1"></svg>' })
    await flushPromises()
    expect(wrapper.find('[data-pane="diagram-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-pane="diagram"]').exists()).toBe(true)
  })
})
