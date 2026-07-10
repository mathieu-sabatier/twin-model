// @vitest-environment nuxt
//
// Shell/integration test for the 3-pane app shell (Task 4). Mocks the API at the
// CLIENT boundary via the Task-3 store's DI seam — NOT MSW (its node interceptor
// does not fire under the nuxt test env) and NOT mockNuxtImport (useApi is a manual
// export, not an auto-import). Pattern:
//   1. create the store FIRST with a fake client backed by real fixtures
//   2. await store.createDraft()  → model + diagnostics populated
//   3. THEN mount — Pinia caches the store by id 'draft', so AppShell's own
//      useDraftStore() (no arg) returns this same fake-backed instance.
// Assertions are on real DOM/behaviour, never on the mock.
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
// Import the Nuxt UI provider directly (auto-imported UApp is not resolvable from
// a render function in the test's component context).
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import {
  equipmentModel,
  diagnostics as fixtureDiags,
  seedFile,
  resolvedFurnace,
} from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import AppShell from '~/components/AppShell.vue'

const FAKE_DRAFT_ID = 'deadbeefcafe0001'

/** Fake ApiClient backed by the REAL fixtures (the golden model + sample diags). */
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
    previewDiagram: vi.fn().mockResolvedValue('classDiagram'),
    diff: vi.fn().mockResolvedValue({ changes: [], text: "" }),
    resolved: vi.fn().mockResolvedValue(resolvedFurnace),
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

// Why a harness: @nuxt/test-utils mounts inside a real Nuxt app whose
// @pinia/nuxt plugin creates + activates its OWN Pinia during app init. So the
// fake-backed store must be created on THAT Pinia — i.e. inside the mounted
// component tree — not on an outer createPinia(). The harness therefore:
//   • creates the store with the fake client in setup() (registers id 'draft' on
//     Nuxt's active Pinia BEFORE AppShell's own useDraftStore() runs, so Pinia's
//     by-id cache hands AppShell the SAME fake-backed instance), and
//   • wraps AppShell in <UApp> to supply the Tooltip/overlay providers app.vue
//     gives it in the real app.
// The created store is captured so the test can await createDraft() then assert.
let captured: ReturnType<typeof useDraftStore> | null = null
let clientForHarness: ApiClient = makeFakeClient()

const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    captured = store
    await store.createDraft() // populate model + diagnostics from fixtures
    return () => h(UApp, null, { default: () => h(AppShell) })
  },
})

/** Mount AppShell (in UApp) against a fake-backed store, return store + wrapper. */
async function mountShell(
  client: ApiClient = makeFakeClient(),
  mountOptions: Parameters<typeof mountSuspended>[1] = {},
) {
  clientForHarness = client
  captured = null
  const wrapper = await mountSuspended(Harness, mountOptions)
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

describe('AppShell — read-only render of the committed model', () => {
  beforeEach(() => {
    captured = null
    // A fresh outer Pinia keeps the test isolated; the harness re-creates the
    // store on Nuxt's own Pinia during mount (see harness comment).
    setActivePinia(createPinia())
  })

  it('renders the two tree roots and their contents (FurnaceType + Furnace02)', async () => {
    const { wrapper } = await mountShell()
    const text = wrapper.text()
    // Types root + a declared object type.
    expect(text).toContain('Types')
    expect(text).toContain('FurnaceType')
    // Instances root + a declared instance.
    expect(text).toContain('Instances')
    expect(text).toContain('Furnace02')
  })

  it('starts on the model overview until a node is selected', async () => {
    const { wrapper } = await mountShell()
    await flushPromises()
    // Center now shows the model overview instead of the old placeholder text.
    expect(wrapper.text()).toContain('Model overview')
    expect(wrapper.text()).not.toContain('Select a type or instance')
    // The overview embeds the model diagram.
    const hasDiagram =
      wrapper.find('[data-pane="diagram"]').exists() ||
      wrapper.find('[data-pane="diagram-raw"]').exists() ||
      wrapper.find('[data-pane="diagram-empty"]').exists()
    expect(hasDiagram).toBe(true)
  })

  it('selecting FurnaceType renders its members in the center (DoorClosed, Zones, StartProgram)', async () => {
    const { wrapper } = await mountShell()

    // Click the FurnaceType node in the tree.
    const nodes = wrapper.findAll('button')
    const furnace = nodes.find((b) => b.text().trim().startsWith('FurnaceType'))
    expect(furnace, 'FurnaceType tree node should exist').toBeTruthy()
    await furnace!.trigger('click')
    await flushPromises()

    const text = wrapper.text()
    // The three own members of FurnaceType (from the golden).
    // In editable mode, member names live in <input> value attributes, so we
    // check the data-member attributes instead of text content.
    expect(wrapper.find('[data-member="DoorClosed"]').exists()).toBe(true)
    expect(wrapper.find('[data-member="Zones"]').exists()).toBe(true)
    expect(wrapper.find('[data-member="StartProgram"]').exists()).toBe(true)
    // The nested placeholder child renders as a sub-row (browseName <ZoneNo>).
    expect(wrapper.find('[data-member-child="Zones/Zone"]').exists()).toBe(true)
    // Empty state is gone.
    expect(text).not.toContain('Select a type or instance')
  })

  it('selecting Furnace02 renders its overridden values (SerialNumber → F-2026-0042)', async () => {
    const { wrapper } = await mountShell()

    const nodes = wrapper.findAll('button')
    const inst = nodes.find((b) => b.text().trim().startsWith('Furnace02'))
    expect(inst, 'Furnace02 tree node should exist').toBeTruthy()
    await inst!.trigger('click')
    await flushPromises()

    // Member names appear as text labels in the values form.
    expect(wrapper.text()).toContain('SerialNumber')
    // The override value is set in the UInput (value attribute, not text content).
    const serialInput = wrapper.find('[data-value="SerialNumber"] input')
    expect(serialInput.exists()).toBe(true)
    expect(serialInput.element.getAttribute('value') ?? (serialInput.element as HTMLInputElement).value).toBe('F-2026-0042')
    // Placeholder children rendered.
    expect(wrapper.find('[data-child="Zone1"]').exists()).toBe(true)
  })

  it('bottom bar reflects the fixture error count and DISABLES Propose when lint-red', async () => {
    const { store, wrapper } = await mountShell()

    const errorCount = fixtureDiags.filter((d) => d.severity === 'error').length
    expect(store.errorCount).toBe(errorCount)
    expect(store.hasErrors).toBe(true)

    // Error badge shows the count.
    const errBadge = wrapper.find('[data-testid="error-badge"]')
    expect(errBadge.exists()).toBe(true)
    expect(errBadge.text()).toContain(String(errorCount))

    // Propose button is disabled (real gate: hasErrors → disabled).
    const propose = wrapper.find('[data-testid="propose-button"]')
    expect(propose.exists()).toBe(true)
    expect(propose.attributes('disabled')).toBeDefined()
  })

  it('ENABLES Propose (and shows "valid") when diagnostics clear — reactive gate', async () => {
    // The gate is reactive: clearing the store's error diagnostics must flip the
    // bar from "N errors"/disabled to "valid"/enabled. We drive the SAME store
    // (Pinia caches id 'draft' across mounts in the shared Nuxt runtime, so a
    // second injected client is ignored — mutating the exposed ref is the honest
    // way to exercise the reactive gate here).
    const { store, wrapper } = await mountShell()
    expect(store.hasErrors).toBe(true)
    expect(wrapper.find('[data-testid="propose-button"]').attributes('disabled')).toBeDefined()

    // Clear diagnostics as a successful validate would.
    store.diagnostics = []
    await flushPromises()

    expect(store.hasErrors).toBe(false)
    expect(wrapper.find('[data-testid="valid-badge"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="error-badge"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="propose-button"]').attributes('disabled')).toBeUndefined()
  })

  it('DISABLES Propose when the draft is frozen (proposed), even with no errors', async () => {
    const { store, wrapper } = await mountShell()
    store.diagnostics = []
    store.frozen = true
    await flushPromises()

    expect(store.hasErrors).toBe(false)
    // Frozen chip shows and Propose stays disabled.
    expect(wrapper.find('[data-testid="propose-button"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('frozen')
  })

  it('bottom bar: propose button is present and root element carries shrink-0 (F9)', async () => {
    // F9 fix: BottomBar must be hosted in the editor panel footer (framework-pinned)
    // and its root div must carry shrink-0 so it cannot be squeezed out of the layout.
    const { wrapper } = await mountShell()

    // Propose button must exist in the rendered shell.
    const propose = wrapper.find('[data-testid="propose-button"]')
    expect(propose.exists()).toBe(true)

    // BottomBar root must carry the shrink-0 class.
    const bar = wrapper.find('[data-testid="bottom-bar"]')
    expect(bar.exists()).toBe(true)
    expect(bar.classes()).toContain('shrink-0')
  })

  it('renders a mobile menu trigger for the tree drawer', async () => {
    const { wrapper } = await mountShell()
    expect(wrapper.find('[data-testid="mobile-tree-toggle"]').exists()).toBe(true)
  })

  it('clicking the header brand clears the selection and returns to the overview', async () => {
    const { wrapper } = await mountShell()

    // Select a type: center shows its detail, overview caption is gone.
    const furnace = wrapper.findAll('button').find((b) => b.text().trim().startsWith('FurnaceType'))
    expect(furnace, 'FurnaceType tree node should exist').toBeTruthy()
    await furnace!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-member="DoorClosed"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Model overview')

    // Click the brand: selection clears, center returns to the overview.
    const brand = wrapper.find('[data-testid="brand-home"]')
    expect(brand.exists()).toBe(true)
    await brand.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Model overview')
    expect(wrapper.find('[data-member="DoorClosed"]').exists()).toBe(false)
  })

  it('inspector renders YAML only — the diagram moved to the center overview', async () => {
    const { wrapper } = await mountShell()
    await flushPromises()

    // YAML section is present (empty-state variant is fine — nothing selected yet).
    const hasYaml =
      wrapper.find('[data-pane="yaml"]').exists() ||
      wrapper.find('[data-pane="yaml-empty"]').exists()
    expect(hasYaml).toBe(true)

    // Select a type: the center switches to its detail, so the overview diagram
    // unmounts and — since the inspector no longer hosts one — NO diagram pane
    // exists anywhere. This proves the diagram was removed from the inspector.
    const furnace = wrapper.findAll('button').find((b) => b.text().trim().startsWith('FurnaceType'))
    expect(furnace, 'FurnaceType tree node should exist').toBeTruthy()
    await furnace!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-pane="diagram"]').exists()).toBe(false)
    expect(wrapper.find('[data-pane="diagram-raw"]').exists()).toBe(false)
    expect(wrapper.find('[data-pane="diagram-empty"]').exists()).toBe(false)

    // Old static stubs must remain absent.
    expect(wrapper.find('[data-pane="diagram-stub"]').exists()).toBe(false)
    expect(wrapper.find('[data-pane="yaml-stub"]').exists()).toBe(false)
  })

  it('error banner has role=alert and dismiss clears store.error', async () => {
    // Approach 2: stub UAlert with a minimal component that emits the Vue event
    // on click. VTU trigger() fires DOM events, not Vue component events, so the
    // real UAlert's internal close button never propagates @update:open to the
    // parent under happy-dom. The stub drives the ACTUAL AppShell wiring:
    //   @update:open="(v) => { if (!v) store.error = null }"
    // without depending on UAlert's internal DOM structure.
    const UAlertStub = defineComponent({
      name: 'UAlert',
      emits: ['update:open'],
      template: `<div role="alert"><button aria-label="Dismiss error" @click="$emit('update:open', false)"></button></div>`,
    })
    const { store, wrapper } = await mountShell(makeFakeClient(), {
      global: { stubs: { UAlert: UAlertStub } },
    })

    // Set an error — banner should appear via the stub.
    store.error = 'boom'
    await flushPromises()

    const banner = wrapper.find('[role="alert"]')
    expect(banner.exists()).toBe(true)

    // Click the stub's dismiss button — this fires the Vue update:open event
    // which AppShell's @update:open handler receives and sets store.error = null.
    const closeBtn = wrapper.find('[aria-label="Dismiss error"]')
    expect(closeBtn.exists()).toBe(true)
    await closeBtn.trigger('click')
    await flushPromises()

    // The dismiss wiring must have cleared the error — NOT a direct assignment.
    expect(store.error).toBeNull()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
})
