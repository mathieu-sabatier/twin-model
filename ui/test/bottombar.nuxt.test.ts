// @vitest-environment nuxt
//
// Component tests for the BottomBar diagnostics popover (Task 4 / F3).
// Harness pattern: create the store with a fake client inside setup() so Pinia's
// by-id 'draft' cache gives BottomBar the same fake-backed instance.
//
// UPopover teleports its #content slot to document.body (reka-ui portal), so
// diagnostic rows are queried from document.body after the badge is clicked.
//
// IMPORTANT: Pinia caches store id 'draft' on the NUXT runtime Pinia. Since the
// propose.nuxt.test.ts already used FAKE_DRAFT_ID 'deadbeefcafe0010', this test
// uses a distinct id to avoid cross-file pollution if tests run in the same worker.
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import UTooltip from '@nuxt/ui/components/Tooltip.vue'
import USelect from '@nuxt/ui/components/Select.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { Diagnostic } from '~/types'
import BottomBar from '~/components/BottomBar.vue'

const FAKE_DRAFT_ID = 'deadbeefcafe0020'

const errA: Diagnostic = {
  code: 'type-mismatch',
  severity: 'error',
  file: seedFile,
  line: 10,
  col: 1,
  path: 'instances/Furnace01/type',
  message: 'Unknown type reference',
}

const errB: Diagnostic = {
  code: 'unit-on-non-numeric',
  severity: 'error',
  file: seedFile,
  line: 34,
  col: 46,
  path: 'object_types/FurnaceType/members/X/unit',
  message: 'unit "%" is only valid on numeric variables',
}

// M3 regression-lock diagnostic: path maps to { kind: 'type', name: 'FurnaceType' }
const errFurnace: Diagnostic = {
  code: 'unknown-member',
  severity: 'error',
  file: seedFile,
  line: 5,
  col: 3,
  path: 'object_types/FurnaceType/members/DoorClosed',
  message: 'unknown member "DoorClosed"',
}

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
      diagnostics: [errA, errB],
    }),
    getDraftFile: vi.fn().mockResolvedValue('model: Demo\n'),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({ file: seedFile, diagnostics: [errA, errB] }),
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
    ...overrides,
  }
}

let captured: ReturnType<typeof useDraftStore> | null = null
const sharedClient: ApiClient = makeFakeClient()

const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(sharedClient)
    captured = store
    await store.createDraft()
    return () => h(UApp, null, { default: () => h(BottomBar) })
  },
})

async function mountBar() {
  captured = null
  const wrapper = await mountSuspended(Harness)
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

function harnessFor(client: ApiClient) {
  return defineComponent({
    async setup() {
      const store = useDraftStore(client)
      captured = store
      await store.createDraft()
      return () => h(UApp, null, { default: () => h(BottomBar) })
    },
  })
}

/**
 * Fully evict the previous test's 'draft' store from the Nuxt runtime's Pinia
 * before mounting a fresh one bound to a different fake client.
 *
 * `$dispose()` alone only removes the store from `pinia._s` (the by-id store
 * cache) — it does NOT clear `pinia.state.value['draft']` (the raw state
 * tree). Pinia setup-stores hydrate newly-created refs from any leftover
 * `pinia.state.value[id]` entry (see createSetupStore's `initialState`
 * handling), so without also deleting that entry, the next store's `repo` ref
 * would be silently re-hydrated with the PREVIOUS test's repo value before
 * `loadRepo()` ever runs — and `loadRepo()`'s `if (repo.value) return` guard
 * then skips calling the new mock entirely. `store._p`/`store.$id` are
 * Pinia-internal (untyped) but stable across the versions this repo pins.
 */
function evictDraftStore(store: ReturnType<typeof useDraftStore> | null): void {
  if (!store) return
  const pinia = (store as unknown as { _p?: { state: { value: Record<string, unknown> } } })._p
  const id = (store as unknown as { $id: string }).$id
  store.$dispose()
  if (pinia?.state?.value) delete pinia.state.value[id]
}

async function mountBarWith(client: ApiClient) {
  evictDraftStore(captured)
  captured = null
  const wrapper = await mountSuspended(harnessFor(client))
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}


describe('BottomBar — a11y live region (Task 11)', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('announces validity via a live region', async () => {
    const { wrapper } = await mountBar()
    const region = wrapper.find('[data-testid="validate-state"]')
    expect(region.attributes('role')).toBe('status')
    expect(region.attributes('aria-live')).toBe('polite')
  })
})

describe('BottomBar — diagnostics popover (F3)', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('shows the error-badge with the correct count', async () => {
    const { wrapper } = await mountBar()
    const badge = wrapper.find('[data-testid="error-badge"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toContain('2')
    expect(badge.text()).toContain('error')
  })

  it('clicking the error-badge opens the popover and renders two diagnostic rows', async () => {
    const { wrapper } = await mountBar()

    // Click the badge (the UPopover trigger)
    const badge = wrapper.find('[data-testid="error-badge"]')
    expect(badge.exists()).toBe(true)
    await badge.trigger('click')
    await flushPromises()

    // UPopover portals content to document.body
    const rows = document.body.querySelectorAll('[data-testid="diagnostic-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0]!.textContent).toContain(errA.message)
    expect(rows[1]!.textContent).toContain(errB.message)
  })

  it('clicking the first diagnostic row navigates to the instance', async () => {
    const { wrapper } = await mountBar()

    // Clear any prior selection
    const { clear, selection } = useSelection()
    clear()
    await flushPromises()
    expect(selection.value).toBeNull()

    // Open popover
    const badge = wrapper.find('[data-testid="error-badge"]')
    await badge.trigger('click')
    await flushPromises()

    // Click the first row (instances/Furnace01/type → { kind: 'instance', name: 'Furnace01' })
    const firstRow = document.body.querySelector('[data-testid="diagnostic-row"]') as HTMLElement | null
    expect(firstRow).not.toBeNull()
    firstRow!.click()
    await flushPromises()

    expect(selection.value).toEqual({ kind: 'instance', name: 'Furnace01' })
  })

  // ── Regression-lock M2: propose button disabled with reason while red ────────
  it('M2: propose button is disabled and shows resolve-errors reason when store has errors', async () => {
    const { wrapper } = await mountBar()

    // The button element must carry the disabled attribute
    const btn = wrapper.find('[data-testid="propose-button"]')
    expect(btn.exists()).toBe(true)
    expect((btn.element as HTMLButtonElement).disabled).toBe(true)

    // proposeReason is bound to the wrapping UTooltip :text prop.
    // The tooltip portals its text to document.body only on hover, so we assert the
    // component prop directly instead of the rendered HTML.
    const tooltip = wrapper.findComponent(UTooltip)
    expect(tooltip.exists()).toBe(true)
    const tooltipText = tooltip.props('text') as string
    expect(tooltipText).toContain('Resolve 2 error')
    expect(tooltipText).toContain('before proposing')
  })

  // ── Regression-lock M3: diagnostic click on type-scoped path navigates to type ─
  it('M3: clicking a type-scoped diagnostic row selects the FurnaceType node', async () => {
    // Use the standard harness (same Nuxt-runtime Pinia store) and inject the
    // target diagnostic directly so we don't depend on a second store instance.
    const { store, wrapper } = await mountBar()

    // Replace diagnostics with the single type-scoped one under test.
    store.diagnostics = [errFurnace]
    await flushPromises()

    // Clear any prior selection
    const { clear, selection } = useSelection()
    clear()
    await flushPromises()
    expect(selection.value).toBeNull()

    // Open popover (store now has 1 error so badge should exist)
    const badge = wrapper.find('[data-testid="error-badge"]')
    expect(badge.exists()).toBe(true)
    await badge.trigger('click')
    await flushPromises()

    // UPopover portals its content to document.body.
    // Select the row whose text matches our injected diagnostic's message.
    const allRows = Array.from(
      document.body.querySelectorAll('[data-testid="diagnostic-row"]'),
    ) as HTMLElement[]
    const row = allRows.find((el) => el.textContent?.includes(errFurnace.message)) ?? null
    expect(row).not.toBeNull()
    row!.click()
    await flushPromises()

    // object_types/FurnaceType/members/DoorClosed → member scope → { kind: 'type', name: 'FurnaceType' }
    expect(selection.value).toEqual({ kind: 'type', name: 'FurnaceType' })
  })

  it('diagnostic rows use a pointer cursor (L4)', async () => {
    const { wrapper } = await mountBar()
    await wrapper.find('[data-testid="error-badge"]').trigger('click')
    await flushPromises()
    const row = document.body.querySelector('[data-testid="diagnostic-row"]') as HTMLElement | null
    expect(row).not.toBeNull()
    expect(row!.className).toContain('cursor-pointer')
  })
})

describe('BottomBar — repo context & branch picker', () => {
  beforeEach(() => {
    // Each test here needs a store bound to a DIFFERENT fake client (see
    // evictDraftStore's doc comment for why `mountBarWith` alone handles the
    // eviction). Also evict here so the FIRST test in this describe doesn't
    // inherit whatever the last old-style `mountBar()` test left behind.
    evictDraftStore(captured)
    captured = null
    setActivePinia(createPinia())
  })

  it('renders the repo chip with owner/repo', async () => {
    const { wrapper } = await mountBarWith(makeFakeClient({
      getRepo: vi.fn().mockResolvedValue({
        host: 'github', owner: 'mathieu-sabatier', repo: 'twin-model',
        url: 'https://github.com/mathieu-sabatier/twin-model', defaultBranch: 'main',
        commitName: 'twinmodel-bot', commitEmail: 'bot@twinmodel',
        proposeEnabled: true, proposeReason: '',
      }),
    }))
    const chip = wrapper.find('[data-testid="repo-chip"]')
    expect(chip.exists()).toBe(true)
    expect(chip.text()).toContain('mathieu-sabatier/twin-model')
    // Propose is not blocked by the host (only the empty-diff/enabled state).
    expect(wrapper.find('[data-testid="pr-off-chip"]').exists()).toBe(false)
  })

  it('disables Propose with the server reason when proposing is unavailable', async () => {
    const { wrapper } = await mountBarWith(makeFakeClient({
      getRepo: vi.fn().mockResolvedValue({
        host: 'github', owner: '', repo: '', url: '', defaultBranch: 'main',
        commitName: 'twinmodel-bot', commitEmail: 'bot@twinmodel',
        proposeEnabled: false,
        proposeReason: 'This server is pointed at a local checkout; proposing opens a GitHub pull request and needs a GitHub repository.',
      }),
      // no lint errors, so only the host gate is in play
      getDraftModel: vi.fn().mockResolvedValue({ file: seedFile, model: equipmentModel, diagnostics: [] }),
      validate: vi.fn().mockResolvedValue({ file: seedFile, diagnostics: [] }),
    }))

    const btn = wrapper.find('[data-testid="propose-button"]')
    expect((btn.element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.find('[data-testid="pr-off-chip"]').exists()).toBe(true)

    const tooltip = wrapper.findComponent(UTooltip)
    expect(tooltip.props('text') as string).toContain('local checkout')
  })

  it('seeds the branch select from listBranches', async () => {
    const { wrapper } = await mountBarWith(makeFakeClient({
      listBranches: vi.fn().mockResolvedValue({
        branches: ['main', 'model/furnace-zones'],
        defaultBranch: 'main',
      }),
    }))
    const select = wrapper.findComponent(USelect)
    expect(select.exists()).toBe(true)
    expect(select.props('items') as string[]).toContain('model/furnace-zones')
  })

  it('choosing a branch switches the draft base', async () => {
    const createDraft = vi.fn()
      .mockResolvedValueOnce({ id: 'draftinit01', baseRef: 'main', files: [seedFile] })   // initial mount
      .mockResolvedValueOnce({ id: 'draftnew02', baseRef: 'model/press-curve', files: [seedFile] }) // the switch
    const { store, wrapper } = await mountBarWith(makeFakeClient({
      createDraft,
      listBranches: vi.fn().mockResolvedValue({
        branches: ['main', 'model/press-curve'],
        defaultBranch: 'main',
      }),
    }))
    wrapper.findComponent(USelect).vm.$emit('update:modelValue', 'model/press-curve')
    await flushPromises()
    expect(createDraft).toHaveBeenLastCalledWith('model/press-curve')
    expect(store.baseRef).toBe('model/press-curve')
  })
})
