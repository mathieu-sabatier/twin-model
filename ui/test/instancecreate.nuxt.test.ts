// @vitest-environment nuxt
//
// Component tests for InstanceCreateDialog.vue (RT5a).
// Harness pattern: create the store with a fake client INSIDE the mounted
// component tree (inside setup()) so Pinia's by-id 'draft' cache hands
// InstanceCreateDialog the same fake-backed instance.
//
// UModal teleports its content to document.body (reka-ui portal), so we query
// DOM elements from document.body rather than wrapper. Selects are driven by
// triggering update:modelValue on the USelect component (portal-rendered option
// lists are unreliable in happy-dom, per RT5a brief).
//
// IMPORTANT: Pinia caches store id 'draft' on the NUXT runtime Pinia (not the
// outer test Pinia). A module-level putFilesSpy is used so all tests share the
// same fake client — resetting the spy in beforeEach is sufficient to isolate
// call counts per test. This matches the shell.nuxt.test.ts approach.
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent, ref } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import InstanceCreateDialog from '~/components/InstanceCreateDialog.vue'

const FAKE_DRAFT_ID = 'deadbeefcafe0002'

// Module-level spy so all tests share the same client (Pinia caches by id).
const putFilesSpy = vi.fn().mockResolvedValue(undefined)

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
    putFiles: putFilesSpy,
    validate: vi.fn().mockResolvedValue({ file: seedFile, diagnostics: [] }),
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
  }
}

let captured: ReturnType<typeof useDraftStore> | null = null
const clientForHarness: ApiClient = makeFakeClient()
let openRef = ref(true)

/**
 * Harness wraps InstanceCreateDialog so the store is pre-created with the fake
 * client before the component's own useDraftStore() call (Pinia by-id caching).
 * Uses a reactive ref for open so the test can toggle it.
 */
const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    captured = store
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          h(InstanceCreateDialog, {
            open: openRef.value,
            'onUpdate:open': (v: boolean) => { openRef.value = v },
          }),
      })
  },
})

async function mountDialog() {
  captured = null
  openRef = ref(true)
  const wrapper = await mountSuspended(Harness)
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

describe('InstanceCreateDialog', () => {
  beforeEach(() => {
    captured = null
    putFilesSpy.mockClear()
    // Outer pinia reset keeps test isolation at the unit-test level.
    setActivePinia(createPinia())
  })

  // ── Type picker — abstract filtering ──────────────────────────────────────

  it('golden model has EquipmentType (abstract=true) — excluded from type picker', async () => {
    const { store } = await mountDialog()

    // Verify the golden model's EquipmentType is abstract.
    const equipmentType = store.objectTypes.find((t) => t.name === 'EquipmentType')
    expect(equipmentType, 'EquipmentType should exist in golden model').toBeDefined()
    expect(equipmentType!.abstract).toBe(true)

    // The type picker logic: store.objectTypes.filter(!t.abstract).map(t.name)
    const typePickerItems = store.objectTypes
      .filter((t) => !t.abstract)
      .map((t) => t.name)

    // FurnaceType is concrete — must appear.
    expect(typePickerItems).toContain('FurnaceType')
    // EquipmentType is abstract — must NOT appear.
    expect(typePickerItems).not.toContain('EquipmentType')
    // Must have at least one item.
    expect(typePickerItems.length).toBeGreaterThan(0)
  })

  it('dialog renders — modal content present in document.body (teleported)', async () => {
    await mountDialog()
    // UModal teleports to document.body; the title should be there.
    const bodyText = document.body.textContent ?? ''
    expect(bodyText).toContain('Add instance')
  })

  it('type select exists in document.body (teleported modal content)', async () => {
    await mountDialog()
    const typeSelect = document.body.querySelector('[data-testid="instance-type-select"]')
    expect(typeSelect, 'type select should be in document.body').not.toBeNull()
  })

  // ── Submit / disabled state ───────────────────────────────────────────────

  it('submit button disabled when name is empty (no name, no type)', async () => {
    await mountDialog()
    await flushPromises()

    // Dialog is open (teleported to body). Submit button must be disabled when
    // name is empty — the component's computed submitDisabled gate.
    const submitBtn = document.body.querySelector('[data-testid="instance-submit-button"]')
    expect(submitBtn, 'submit button should be in document.body').not.toBeNull()
    // Default state: empty name + no type → disabled.
    expect((submitBtn as HTMLButtonElement).disabled).toBe(true)
  })

  it('blocks submit and shows a message for a duplicate name', async () => {
    await mountDialog()
    // UModal teleports to document.body; UInput forwards attrs ($attrs) to the
    // native <input>, so data-testid lands directly on the <input> element.
    const nameInput = document.body.querySelector('[data-testid="instance-name-input"]') as HTMLInputElement
    expect(nameInput, 'name input should be in document.body').not.toBeNull()
    nameInput.value = 'Furnace01'
    nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()
    const errorEl = document.body.querySelector('[data-testid="instance-name-error"]')
    expect(errorEl, 'instance-name-error should be visible').not.toBeNull()
    expect(errorEl!.textContent).toContain('already exists')
    const submitBtn = document.body.querySelector('[data-testid="instance-submit-button"]')
    expect((submitBtn as HTMLButtonElement).disabled).toBe(true)
  })

  it('createInstance calls putFiles with YAML containing instance name and type', async () => {
    const { store } = await mountDialog()

    // Reset spy after createDraft (which does not call putFiles).
    putFilesSpy.mockClear()

    // Call the store's createInstance directly — same action the dialog invokes.
    await store.createInstance({
      name: 'Furnace09',
      type: 'FurnaceType',
      under: 'OpcUa:ObjectsFolder',
    })

    expect(putFilesSpy).toHaveBeenCalledTimes(1)
    const [calledDraftId, calledFiles] = putFilesSpy.mock.calls[0] as [string, Record<string, string>]
    expect(calledDraftId).toBe(FAKE_DRAFT_ID)
    const yamlBody = calledFiles[seedFile]!
    expect(yamlBody).toMatch(/Furnace09:/)
    expect(yamlBody).toMatch(/type: FurnaceType/)
  })
})
