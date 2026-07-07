// @vitest-environment nuxt
//
// Tests for the editable InstanceView (RT5b):
//   1. Enum-typed member renders a USelect; plain member renders a UInput.
//   2. Changing the enum select upserts into instance.values and calls putFiles.
//   3. Type-change guard: changing the type opens the confirm modal; confirming
//      calls putFiles.
//
// Harness pattern: same as shell.nuxt.test.ts / memberstable.nuxt.test.ts.
// The @nuxt/test-utils Nuxt runtime shares ONE Pinia per file; the draft store
// is registered once (first mountSuspended) and reused across tests. All tests
// share clientForHarness so they see the same spy object (mock is cleared in
// beforeEach, but never replaced — Pinia's by-id cache would ignore a new client).
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import USelect from '@nuxt/ui/components/Select.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, diagnostics as fixtureDiags, seedFile, resolvedFurnace } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import InstanceView from '~/components/InstanceView.vue'
import { indexDiagnostics } from '~/lib/diagnosticPath'

const FAKE_DRAFT_ID = 'instancevalues-test-001'

// Shared client — registered with the store on first mountSuspended and reused.
const clientForHarness: ApiClient = {
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
  // resolved returns FurnaceType's full inherited-flattened members (includes
  // State:EquipmentState enum-typed variable + SerialNumber:String property).
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
}

const diagIndex = indexDiagnostics(fixtureDiags)

// Furnace02 has values overrides: Manufacturer, SerialNumber, State, CycleCount, DoorClosed
const furnace02 = equipmentModel.instances!.find((i) => i.name === 'Furnace02')!

// Harness — mounts InstanceView for Furnace02 directly inside UApp.
const InstanceHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          h(InstanceView, {
            instance: furnace02,
            diagnosticIndex: diagIndex,
          }),
      })
  },
})

// ── Test 1: enum-typed member → USelect; plain member → UInput ───────────────
describe('InstanceView — value form field types', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('renders a USelect for EquipmentState member and a text input for SerialNumber', async () => {
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()

    // State is of type EquipmentState (an enum) → should render inside [data-value="State"]
    const stateRow = wrapper.find('[data-value="State"]')
    expect(stateRow.exists(), '[data-value="State"] row should exist').toBe(true)

    // USelect is present in the State row (enum member)
    const stateSelect = stateRow.findComponent(USelect)
    expect(stateSelect.exists(), 'State row should have a USelect for enum').toBe(true)

    // SerialNumber is String → plain UInput
    const serialRow = wrapper.find('[data-value="SerialNumber"]')
    expect(serialRow.exists(), '[data-value="SerialNumber"] row should exist').toBe(true)
    const serialInput = serialRow.find('input')
    expect(serialInput.exists(), 'SerialNumber row should have a text input').toBe(true)

    // No USelect in the serial row
    expect(serialRow.findComponent(USelect).exists()).toBe(false)
  })
})

// ── Test 2: changing enum select upserts values and calls putFiles ────────────
describe('InstanceView — enum value upsert', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('selecting a new enum value calls putFiles once with YAML containing the value', async () => {
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // Find the USelect in the State row and trigger update:modelValue.
    const stateRow = wrapper.find('[data-value="State"]')
    expect(stateRow.exists()).toBe(true)
    const stateSelect = stateRow.findComponent(USelect)
    expect(stateSelect.exists()).toBe(true)

    // Choose 'Fault' (not the current value 'Running')
    await stateSelect.vm.$emit('update:modelValue', 'Fault')
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
    const [calledDraftId, filesArg] = vi.mocked(clientForHarness.putFiles).mock.calls[0] as [
      string,
      Record<string, string>,
    ]
    expect(calledDraftId).toBe(FAKE_DRAFT_ID)
    const yaml = filesArg[seedFile]
    expect(typeof yaml).toBe('string')
    // The emitted YAML should contain the member and the new value
    expect(yaml).toMatch(/State/)
    expect(yaml).toMatch(/Fault/)
  })
})

// ── Test 3: nested placeholder — Add button and round-trip upsert ────────────
// Furnace01 has no existing children; FurnaceType has Zones → Zone<ZoneNo>
// (optional_placeholder) nested inside the Zones object member's children[].
// The current placeholderMembers filter is top-level only, so this test fails
// before the recursive walk fix.
const furnace01 = equipmentModel.instances!.find((i) => i.name === 'Furnace01')!

const InstanceHarnessFurnace01 = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          h(InstanceView, {
            instance: furnace01,
            diagnosticIndex: indexDiagnostics([]),
          }),
      })
  },
})

describe('InstanceView — nested placeholder (Zones/Zone<No>)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('renders an Add button for a nested placeholder child (Zones/Zone<No>)', async () => {
    const wrapper = await mountSuspended(InstanceHarnessFurnace01)
    await flushPromises()
    const addBtn = wrapper.find('[data-add-child="Zone"]')
    expect(addBtn.exists()).toBe(true)
  })

  it('adding a Zone child upserts a numbered child and calls putFiles', async () => {
    const wrapper = await mountSuspended(InstanceHarnessFurnace01)
    await flushPromises()
    await wrapper.find('[data-add-child="Zone"]').trigger('click')
    await flushPromises()
    expect(clientForHarness.putFiles).toHaveBeenCalled()
    const yaml = (clientForHarness.putFiles as any).mock.calls.at(-1)[1][seedFile]
      ?? Object.values((clientForHarness.putFiles as any).mock.calls.at(-1)[1])[0]
    expect(yaml).toMatch(/Zone1:\s*\{\s*of:\s*"?Zone<No>"?/)
  })
})

// ── Test 4b: removing a child shows an Undo toast that restores it ────────────
describe('InstanceView — child removal undo toast', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('removing a child shows an Undo toast that restores it', async () => {
    // Furnace02 has Zone1 and Zone2 as children
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // Click the remove button for Zone2
    await wrapper.find('[data-remove-child="Zone2"]').trigger('click')
    await flushPromises()

    // Check that a toast was added with the right title
    const toast = useToast()
    const last = toast.toasts.value.at(-1)
    expect(last?.title).toMatch(/Removed Zone2/)

    // The toast should have an Undo action
    expect(last?.actions).toBeDefined()
    expect(last!.actions!.length).toBeGreaterThan(0)
    expect(last!.actions![0]!.label).toBe('Undo')

    // Clear calls so we can see the undo call
    vi.mocked(clientForHarness.putFiles).mockClear()

    // Invoke the undo action — should re-save the snapshot (which includes Zone2)
    await last!.actions![0]!.onClick!(new MouseEvent('click'))
    await flushPromises()

    // putFiles should have been called with a model that still has Zone2
    expect(clientForHarness.putFiles).toHaveBeenCalled()
    const filesArg = vi.mocked(clientForHarness.putFiles).mock.calls.at(-1)![1] as Record<string, string>
    const yaml = filesArg[seedFile] ?? Object.values(filesArg)[0]
    expect(yaml).toMatch(/Zone2:\s*\{\s*of:/)
  })
})

// ── Test 5: delete instance after confirm ─────────────────────────────────────
describe('InstanceView — delete instance', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('deletes the instance after confirming', async () => {
    const wrapper = await mountSuspended(InstanceHarnessFurnace01)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    await wrapper.find('[data-testid="delete-instance"]').trigger('click')
    await flushPromises()

    // Confirm button is portaled into document.body
    const confirmBtn = document.body.querySelector('[data-testid="confirm-delete-instance"]')
    expect(confirmBtn, 'confirm-delete button should appear in the modal').toBeTruthy()

    // putFiles not called yet
    expect(clientForHarness.putFiles).not.toHaveBeenCalled()

    ;(confirmBtn as HTMLElement).click()
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalled()
  })
})

// ── Test 6: inline rename — blocks duplicate, saves on Enter ─────────────────
describe('InstanceView — inline rename', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('renames the instance and blocks a duplicate target', async () => {
    const wrapper = await mountSuspended(InstanceHarnessFurnace01)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    await wrapper.find('[data-testid="rename-instance"]').trigger('click')
    await flushPromises()

    const input = wrapper.find('[data-testid="rename-input"] input')
    expect(input.exists(), 'rename input should be visible').toBe(true)

    // Set to an existing name → should block
    await input.setValue('Furnace02')
    await flushPromises()
    expect(wrapper.find('[data-testid="rename-error"]').exists(), 'rename-error should show for duplicate').toBe(true)

    // Set to a new unique name and press Enter → should save
    await input.setValue('FurnaceZ')
    await input.trigger('keydown.enter')
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalled()
  })
})

// ── H2: type change clears orphaned incompatible children ─────────────────────
describe('InstanceView — type change clears orphaned children (H2)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('changing Furnace02 to PressType removes children with no matching placeholder', async () => {
    // PressType has NO placeholder → Zone1/Zone2 (of Zone<No>) become orphaned.
    const equipmentType = equipmentModel.objectTypes!.find((t) => t.name === 'EquipmentType')!
    const pressType = equipmentModel.objectTypes!.find((t) => t.name === 'PressType')!
    const resolvedPress = {
      type: 'PressType',
      members: [
        ...equipmentType.members!.map((m) => ({ ...m, declaredIn: 'EquipmentType' })),
        ...pressType.members!.map((m) => ({ ...m, declaredIn: 'PressType' })),
      ],
    }
    vi.mocked(clientForHarness.resolved).mockImplementation(
      async (_id: string, type: string) =>
        (type === 'PressType' ? resolvedPress : resolvedFurnace) as any,
    )

    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    const typeSelect = wrapper
      .findAllComponents(USelect)
      .find((s) => s.props('modelValue') === 'FurnaceType')
    expect(typeSelect).toBeTruthy()
    await typeSelect!.vm.$emit('update:modelValue', 'PressType')
    await flushPromises()

    const confirmBtn = document.body.querySelector('[data-testid="confirm-type-change"]') as HTMLElement | null
    expect(confirmBtn).toBeTruthy()
    confirmBtn!.click()
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
    const filesArg = vi.mocked(clientForHarness.putFiles).mock.calls[0]![1] as Record<string, string>
    const yaml = filesArg[seedFile] ?? Object.values(filesArg)[0]!
    expect(yaml).not.toMatch(/Zone1:/) // orphaned child removed
    expect(yaml).not.toMatch(/Zone2:/)
    expect(yaml).not.toMatch(/DoorClosed: true/) // FurnaceType-only instance value cleared
    expect(yaml).toMatch(/CycleCount/) // inherited value retained
  })
})

// ── M1: typed value editors ───────────────────────────────────────────────────
describe('InstanceView — typed value editors (M1)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('renders a select for Boolean (DoorClosed) and a number input for UInt32 (CycleCount)', async () => {
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()

    const doorRow = wrapper.find('[data-value="DoorClosed"]')
    expect(doorRow.exists()).toBe(true)
    expect(doorRow.findComponent(USelect).exists(), 'Boolean → USelect').toBe(true)

    const cycleRow = wrapper.find('[data-value="CycleCount"]')
    expect(cycleRow.exists()).toBe(true)
    expect(cycleRow.find('input[type="number"]').exists(), 'UInt32 → number input').toBe(true)
  })
})

// ── Test 4: type-change guard opens confirm modal; confirming calls putFiles ──
describe('InstanceView — type-change guard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('changing the type select opens the confirm modal and confirming saves', async () => {
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // The type select is in the identity header.
    // Trigger update:modelValue directly on the USelect (same reka-ui portal avoidance
    // pattern as memberstable.nuxt.test.ts).
    const allSelects = wrapper.findAllComponents(USelect)
    // The type select shows the current instance.type.name ('FurnaceType')
    const typeSelect = allSelects.find((s) => s.props('modelValue') === 'FurnaceType')
    expect(typeSelect, 'type USelect should exist with modelValue FurnaceType').toBeTruthy()

    // Emit a new type selection — this should NOT immediately save, but open the modal.
    await typeSelect!.vm.$emit('update:modelValue', 'PressType')
    await flushPromises()

    // The modal should now be open — look for the confirm button in document.body
    // (portaled overlay content appears outside the wrapper).
    const confirmBtn = document.body.querySelector('[data-testid="confirm-type-change"]')
    expect(confirmBtn, 'confirm button should appear in the modal').toBeTruthy()

    // putFiles not called yet (pending confirmation)
    expect(clientForHarness.putFiles).not.toHaveBeenCalled()

    // Click confirm
    ;(confirmBtn as HTMLElement).click()
    await flushPromises()

    // Now putFiles should have been called once
    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
  })
})

// ── L4: value inputs are labelled ────────────────────────────────────────────
describe('InstanceView — value inputs are labelled (L4)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
  })

  it('gives the numeric value input an accessible name', async () => {
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()
    const input = wrapper.find('[data-value="CycleCount"] input[aria-label="Value of CycleCount"]')
    expect(input.exists()).toBe(true)
  })
})

// ── Test 7: enum inherit option clears the value ──────────────────────────────
describe('InstanceView — enum inherit/clear', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('offers an inherit option that clears an enum value', async () => {
    // Furnace02 has State: Running — mount it and pick "(inherit)" to clear
    const wrapper = await mountSuspended(InstanceHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    const stateRow = wrapper.find('[data-value="State"]')
    expect(stateRow.exists()).toBe(true)
    const stateSelect = stateRow.findComponent(USelect)
    expect(stateSelect.exists()).toBe(true)

    // Emitting the inherit sentinel should call onValueChange with '' (clear)
    await stateSelect.vm.$emit('update:modelValue', '__inherit__')
    await flushPromises()

    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)
    const filesArg = vi.mocked(clientForHarness.putFiles).mock.calls[0]![1] as Record<string, string>
    const yaml = filesArg[seedFile] ?? Object.values(filesArg)[0]
    expect(typeof yaml).toBe('string')
    // The value should have been cleared — State: Running must not appear
    expect(yaml).not.toMatch(/State:\s*Running/)
    // The sentinel must NOT appear in the YAML either (it was mapped to '' = clear)
    expect(yaml).not.toMatch(/__inherit__/)
  })
})

// ── Test M4: severity-aware ring + aria-invalid on instance value input ───────
// Harness with an error diagnostic seeded on Furnace02/values/State.
const valueErrorDiagIndex = indexDiagnostics([
  {
    code: 'invalid-value',
    severity: 'error',
    file: seedFile,
    line: 20,
    col: 1,
    path: 'instances/Furnace02/values/State',
    message: 'invalid enum value',
  },
])

const InstanceWithValueErrorHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          h(InstanceView, {
            instance: furnace02,
            diagnosticIndex: valueErrorDiagIndex,
          }),
      })
  },
})

describe('InstanceView — M4 severity-aware ring on value inputs', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.resolved).mockClear()
  })

  it('M4: an instance value input with an error gets a red ring and aria-invalid', async () => {
    const wrapper = await mountSuspended(InstanceWithValueErrorHarness)
    await flushPromises()
    // State is an enum → USelect; aria-label and :class both land on the SelectTrigger button.
    const stateRow = wrapper.find('[data-value="State"]')
    expect(stateRow.exists()).toBe(true)
    const input = stateRow.find('[aria-label="Value of State"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(input.classes()).toContain('ring-error')
  })
})
