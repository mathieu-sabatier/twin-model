// @vitest-environment nuxt
//
// Tests for the editable EnumView (QA finding M5). Harness pattern mirrors
// instancevalues.nuxt.test.ts: the store is created inside setup() with a shared
// fake client so EnumView's useDraftStore() sees the same instance.
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { useToast } from '#imports'
import { equipmentModel, seedFile, diagnostics as fixtureDiags } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import EnumView from '~/components/EnumView.vue'

const FAKE_DRAFT_ID = 'enumview-test-001'

const clientForHarness: ApiClient = {
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
  previewDiagram: vi.fn().mockResolvedValue('classDiagram'),
  diff: vi.fn().mockResolvedValue({ changes: [], text: '' }),
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

const equipmentState = equipmentModel.enums!.find((e) => e.name === 'EquipmentState')!

const EnumHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () => h(UApp, null, { default: () => h(EnumView, { def: equipmentState }) })
  },
})

describe('EnumView — editable doc + add value (M5)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
  })

  it('editing the doc saves via putFiles with the new text', async () => {
    const wrapper = await mountSuspended(EnumHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    const doc = wrapper.find('[data-testid="enum-doc"] textarea')
    expect(doc.exists()).toBe(true)
    await doc.setValue('States of a machine')
    await doc.trigger('change')
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
    const yaml = Object.values(vi.mocked(clientForHarness.putFiles).mock.calls[0]![1])[0] as string
    expect(yaml).toMatch(/States of a machine/)
  })

  it('adding a value appends a new enum value and saves', async () => {
    const wrapper = await mountSuspended(EnumHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    await wrapper.find('[data-testid="enum-add-value"]').trigger('click')
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
    const yaml = Object.values(vi.mocked(clientForHarness.putFiles).mock.calls[0]![1])[0] as string
    // 5 originals + 1 new value name "Value1"
    expect(yaml).toMatch(/Value1/)
  })
})

describe('EnumView — edit/remove value + validation (M5)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
  })

  it('editing an identifier saves it as an explicit value', async () => {
    const wrapper = await mountSuspended(EnumHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // First value row is "Idle" (identifier 0). Change its identifier to 9.
    const idInput = wrapper.find('[data-enum-value="Idle"] input[type="number"]')
    expect(idInput.exists()).toBe(true)
    await idInput.setValue('9')
    await idInput.trigger('change')
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
    const yaml = Object.values(vi.mocked(clientForHarness.putFiles).mock.calls[0]![1])[0] as string
    expect(yaml).toMatch(/\{\s*Idle:\s*9\s*\}/) // explicit form
  })

  it('removing a value shows an Undo toast that restores it', async () => {
    const wrapper = await mountSuspended(EnumHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    await wrapper.find('[aria-label="Remove Fault"]').trigger('click')
    await flushPromises()

    const toast = useToast()
    const last = toast.toasts.value.at(-1)
    expect(last?.title).toMatch(/Removed Fault/)
    expect(last!.actions![0]!.label).toBe('Undo')

    vi.mocked(clientForHarness.putFiles).mockClear()
    await last!.actions![0]!.onClick!(new MouseEvent('click'))
    await flushPromises()
    const yaml = Object.values(vi.mocked(clientForHarness.putFiles).mock.calls.at(-1)![1])[0] as string
    expect(yaml).toMatch(/Fault/)
  })

  it('rejects a duplicate value name without saving', async () => {
    const wrapper = await mountSuspended(EnumHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // Rename "Idle" to "Running" (already exists) → blocked.
    const nameInput = wrapper.find('[data-enum-value="Idle"] input[type="text"]')
    expect(nameInput.exists()).toBe(true)
    await nameInput.setValue('Running')
    await nameInput.trigger('change')
    await flushPromises()

    expect(clientForHarness.putFiles).not.toHaveBeenCalled()
    const toast = useToast()
    expect(toast.toasts.value.at(-1)?.title).toMatch(/[Dd]uplicate/)
  })

  it('rejects clearing an identifier without saving', async () => {
    const wrapper = await mountSuspended(EnumHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // Clear the "Idle" identifier field → blocked (must not save identifier 0).
    const idInput = wrapper.find('[data-enum-value="Idle"] input[type="number"]')
    expect(idInput.exists()).toBe(true)
    await idInput.setValue('')
    await idInput.trigger('change')
    await flushPromises()

    expect(clientForHarness.putFiles).not.toHaveBeenCalled()
    const toast = useToast()
    expect(toast.toasts.value.at(-1)?.title).toMatch(/required|integer/i)
  })
})
