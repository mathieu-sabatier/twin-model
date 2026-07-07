// @vitest-environment nuxt
//
// Tests for MembersTable (editable mode) and TypeView (editable wiring).
// Uses the EXACT same Harness pattern as shell.nuxt.test.ts.
//
// KEY INVARIANT: The @nuxt/test-utils Nuxt runtime is shared across ALL tests in
// one file. The Pinia 'draft' store is registered once (by the first mountSuspended
// call) on Nuxt's Pinia and cached for the lifetime of the file. Therefore:
//   1. clientForHarness is module-level and NEVER replaced after the first mount.
//   2. All tests assert against vi.mocked(clientForHarness.putFiles) — the same
//      spy the store holds.
//   3. beforeEach calls mockClear() to isolate per-test counts.
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import USelect from '@nuxt/ui/components/Select.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { useToast } from '#imports'
import {
  equipmentModel,
  diagnostics as fixtureDiags,
  seedFile,
  unitsSample,
} from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import MembersTable from '~/components/MembersTable.vue'
import TypeView from '~/components/TypeView.vue'
import { indexDiagnostics } from '~/lib/diagnosticPath'

const FAKE_DRAFT_ID = 'memberstable-test-001'

// Single shared client — registered with the store on first mountSuspended call
// and reused for the lifetime of this test file (Nuxt Pinia cache behaviour).
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
  resolved: vi.fn().mockResolvedValue({ type: 'FurnaceType', members: [] }),
  propose: vi.fn().mockResolvedValue({ url: 'https://github.com/org/repo/pull/1' }),
  getUnits: vi.fn().mockResolvedValue(unitsSample),
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
const equipType = equipmentModel.objectTypes!.find((t) => t.name === 'EquipmentType')!

// Harness A — MembersTable in editable mode for EquipmentType.
const MembersHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          h(MembersTable, {
            typeName: 'EquipmentType',
            members: equipType.members!,
            diagnosticIndex: diagIndex,
            readonly: false,
          }),
      })
  },
})

// Harness B — TypeView for FurnaceType.
const TypeViewHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    const furnaceType = store.objectTypes.find((t) => t.name === 'FurnaceType')
    return () =>
      h(UApp, null, {
        default: () =>
          furnaceType
            ? h(TypeView, {
                type: furnaceType,
                diagnosticIndex: diagIndex,
              })
            : h('div', 'no FurnaceType'),
      })
  },
})

// ── Test 1: MembersTable emits update:member on Access change ─────────────────
// Pure component-emit test. USelect uses reka-ui portal so DOM-click on dropdown
// items is not reliable in the happy-dom test env; we trigger update:modelValue
// on the USelect component's vm directly (Vue component-level emit).
describe('MembersTable — editable mode emits', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('changing the Access USelect emits update:member with { member, patch: { access: "rw" } }', async () => {
    const wrapper = await mountSuspended(MembersHarness)
    await flushPromises()

    const table = wrapper.findComponent(MembersTable)
    expect(table.exists()).toBe(true)

    // Find all USelect instances inside MembersTable.
    // Access selects have modelValue 'r' or 'rw' (property/variable members only).
    // Manufacturer is the first property in EquipmentType with access='r'.
    const selects = table.findAllComponents(USelect)
    const accessSelects = selects.filter((s) => {
      const mv = s.props('modelValue')
      return mv === 'r' || mv === 'rw'
    })
    expect(accessSelects.length).toBeGreaterThan(0)

    // Simulate selecting 'rw' by triggering the USelect's update:modelValue event.
    await accessSelects[0]!.vm.$emit('update:modelValue', 'rw')
    await flushPromises()

    const emitted = table.emitted('update:member')
    expect(emitted).toBeTruthy()
    expect(emitted!.length).toBeGreaterThan(0)

    // Manufacturer is the first property member
    const payload = emitted![0]![0] as { member: string; patch: { access: string } }
    expect(payload.member).toBe('Manufacturer')
    expect(payload.patch.access).toBe('rw')
  })
})

// ── Test 1b: MembersTable blank unit option emits patch.unit === undefined ─────
describe('MembersTable — blank unit option clears unit', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('selecting the blank unit option emits update:member with patch.unit === undefined', async () => {
    const wrapper = await mountSuspended(MembersHarness)
    await flushPromises()

    const table = wrapper.findComponent(MembersTable)
    expect(table.exists()).toBe(true)

    // Find unit USelect instances: they are the only selects whose items prop
    // includes the blank { label: '—', value: null } entry prepended by unitItems.
    // (Kind selects use string arrays; rule/access selects have no null entry.)
    const selects = table.findAllComponents(USelect)
    const unitSelects = selects.filter((s) => {
      const items = s.props('items') as Array<{ label?: string; value: unknown } | string> | undefined
      return Array.isArray(items) && items.some((it) => typeof it === 'object' && it !== null && it.value === null)
    })
    expect(unitSelects.length).toBeGreaterThan(0)

    // Trigger selecting the blank option (value=null — reka-ui forbids '' as SelectItem value).
    await unitSelects[0]!.vm.$emit('update:modelValue', null)
    await flushPromises()

    const emitted = table.emitted('update:member')
    expect(emitted).toBeTruthy()
    const payload = emitted![emitted!.length - 1]![0] as { member: string; patch: Partial<import('~/types').Member> }
    expect(payload.patch.unit).toBeUndefined()
  })
})

// ── Test 2: TypeView wiring — toggling abstract calls putFiles ────────────────
// Via TypeView (Harness + shared store): toggle abstract → store.saveModel →
// client.putFiles called exactly once with YAML containing 'abstract: true'.
// clientForHarness.putFiles is the spy the store holds (same object reference).
describe('TypeView — editable save wiring', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('toggling the abstract USwitch calls putFiles once and YAML contains abstract: true', async () => {
    const wrapper = await mountSuspended(TypeViewHarness)
    await flushPromises()

    // Clear any calls from createDraft/setup phase
    vi.mocked(clientForHarness.putFiles).mockClear()

    // USwitch renders as <button role="switch">; clicking toggles the value.
    // FurnaceType.abstract is undefined in the fixture (treated as false) → click → true.
    const switchBtn = wrapper.find('button[role="switch"]')
    expect(switchBtn.exists(), 'USwitch renders a button[role=switch]').toBe(true)

    await switchBtn.trigger('click')
    await flushPromises()

    // putFiles called once (saveModel → emit YAML → putFiles → reload)
    expect(clientForHarness.putFiles).toHaveBeenCalledTimes(1)

    const [calledDraftId, filesArg] = vi.mocked(clientForHarness.putFiles).mock.calls[0] as [
      string,
      Record<string, string>,
    ]
    expect(calledDraftId).toBe(FAKE_DRAFT_ID)
    const yamlText = filesArg[seedFile]
    expect(typeof yamlText).toBe('string')
    expect(yamlText).toMatch(/abstract:\s*true/)
  })

  it('the abstract switch has an accessible name (L1)', async () => {
    const wrapper = await mountSuspended(TypeViewHarness)
    await flushPromises()
    const sw = wrapper.find('button[role="switch"]')
    expect(sw.exists()).toBe(true)
    expect(sw.attributes('aria-label')).toBe('Abstract')
  })
})

// ── Test 2b: TypeView — removing a member shows an Undo toast that restores it ─
describe('TypeView — member removal undo toast', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('removing a member shows an Undo toast that restores it', async () => {
    const wrapper = await mountSuspended(TypeViewHarness)
    await flushPromises()
    vi.mocked(clientForHarness.putFiles).mockClear()

    // FurnaceType has DoorClosed, Zones, StartProgram as members.
    // Emit the remove event directly on MembersTable (matching TypeView's @remove handler).
    const typeView = wrapper.findComponent(TypeView)
    expect(typeView.exists()).toBe(true)
    const membersTable = typeView.findComponent(MembersTable)
    expect(membersTable.exists()).toBe(true)

    await membersTable.vm.$emit('remove', { index: 0, member: 'DoorClosed' })
    await flushPromises()

    // A toast should have been added with the right title
    const toast = useToast()
    const last = toast.toasts.value.at(-1)
    expect(last?.title).toMatch(/Removed DoorClosed/)

    // The toast should have an Undo action
    expect(last?.actions).toBeDefined()
    expect(last!.actions!.length).toBeGreaterThan(0)
    expect(last!.actions![0]!.label).toBe('Undo')

    // Clear calls so we can see the undo call
    vi.mocked(clientForHarness.putFiles).mockClear()

    // Invoke the undo action — should re-save the snapshot (which includes DoorClosed)
    await last!.actions![0]!.onClick!(new MouseEvent('click'))
    await flushPromises()

    // putFiles should have been called with a model that still has DoorClosed
    expect(clientForHarness.putFiles).toHaveBeenCalled()
    const filesArg = vi.mocked(clientForHarness.putFiles).mock.calls.at(-1)![1] as Record<string, string>
    const yaml = filesArg[seedFile] ?? Object.values(filesArg)[0]
    expect(yaml).toMatch(/DoorClosed/)
  })
})

// ── Test A11y: member row inputs have accessible labels ──────────────────────────
// Harness C — MembersTable in editable mode for FurnaceType (has DoorClosed member).
const furnaceType = (() => {
  const ft = equipmentModel.objectTypes?.find((t) => t.name === 'FurnaceType')
  return ft
})()

const FurnaceMembersHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          furnaceType
            ? h(MembersTable, {
                typeName: 'FurnaceType',
                members: furnaceType.members ?? [],
                diagnosticIndex: diagIndex,
                readonly: false,
              })
            : h('div', 'no FurnaceType'),
      })
  },
})

describe('MembersTable — a11y input labels (Task 11)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('labels each member row input with column + member name', async () => {
    const wrapper = await mountSuspended(FurnaceMembersHarness)
    await flushPromises()

    // The row element must carry data-member="DoorClosed" (for test targeting)
    const row = wrapper.find('[data-member="DoorClosed"]')
    expect(row.exists()).toBe(true)

    // The type input inside that row must have an aria-label
    const typeInput = row.find('[aria-label="Type of DoorClosed"]')
    expect(typeInput.exists()).toBe(true)
  })
})

// ── Test 3: frozen draft → TypeView passes readonly=true → MembersTable read-only
// When store.frozen is true, TypeView binds :readonly="store.frozen" to MembersTable,
// so the table renders as read-only (no UInput/USelect inside MembersTable).
describe('TypeView — frozen draft renders MembersTable read-only', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('when store.frozen is true, MembersTable has no editable inputs', async () => {
    const wrapper = await mountSuspended(TypeViewHarness)
    await flushPromises()

    const typeView = wrapper.findComponent(TypeView)
    expect(typeView.exists()).toBe(true)

    // Capture the store (it's registered under id 'draft' on Nuxt's Pinia —
    // calling useDraftStore() with no arg inside the mounted tree returns it).
    // We access it via the TypeView's store reference by getting it from Pinia.
    // The simplest approach: find MembersTable and verify it starts in editable mode,
    // then set frozen and verify it switches to read-only.
    const membersTable = typeView.findComponent(MembersTable)
    expect(membersTable.exists()).toBe(true)

    // In editable mode: UInput components present (name, type fields).
    const inputsBefore = membersTable.findAll('input')
    expect(inputsBefore.length).toBeGreaterThan(0)

    // Set store frozen via a fresh useDraftStore() call (same Pinia cache).
    const store = useDraftStore(clientForHarness)
    store.frozen = true
    await flushPromises()

    // In read-only mode: no UInput components inside MembersTable.
    const inputsAfter = membersTable.findAll('input')
    expect(inputsAfter.length).toBe(0)
  })
})

describe('MembersTable — duplicate-name rows are index-addressed (M3)', () => {
  const zero = { file: '', line: 0, col: 0 }
  const dupMembers = [
    { name: 'Dup', kind: 'variable', rule: 'mandatory', type: { raw: 'Double', name: 'Double', pos: zero } },
    { name: 'Dup', kind: 'variable', rule: 'mandatory', type: { raw: 'Int32', name: 'Int32', pos: zero } },
  ]

  it('remove emits the clicked row index, not just the name', async () => {
    const Harness = defineComponent({
      setup: () => () =>
        h(UApp, null, {
          default: () =>
            h(MembersTable, {
              typeName: 'T',
              members: dupMembers as never,
              diagnosticIndex: indexDiagnostics([]),
              readonly: false,
            }),
        }),
    })
    const wrapper = await mountSuspended(Harness)
    await flushPromises()

    const removeBtns = wrapper.findAll('[aria-label="Remove Dup"]')
    expect(removeBtns).toHaveLength(2)
    await removeBtns[1]!.trigger('click')

    const emitted = wrapper.findComponent(MembersTable).emitted('remove')
    expect(emitted?.[0]?.[0]).toMatchObject({ index: 1, member: 'Dup' })
  })
})

// ── Test M2: members table scrolls on mobile (not clips) ─────────────────────────
describe('MembersTable — M2 mobile scrolling', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('M2: members table keeps a min-width so it scrolls (not clips) on mobile', async () => {
    const wrapper = await mountSuspended(FurnaceMembersHarness)
    await flushPromises()
    const table = wrapper.find('table')
    expect(table.classes()).toContain('min-w-[640px]')
  })
})

// ── Test M4: severity-aware ring + aria-invalid on member type cell ───────────
// Harness D — MembersTable for FurnaceType with an error seeded on DoorClosed/type.
const typeErrorDiagIndex = indexDiagnostics([
  {
    code: 'unknown-type',
    severity: 'error',
    file: seedFile,
    line: 10,
    col: 1,
    path: 'object_types/FurnaceType/members/DoorClosed/type',
    message: 'unknown type reference',
  },
])

const MembersWithTypeErrorHarness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () =>
          furnaceType
            ? h(MembersTable, {
                typeName: 'FurnaceType',
                members: furnaceType.members ?? [],
                diagnosticIndex: typeErrorDiagIndex,
                readonly: false,
              })
            : h('div', 'no FurnaceType'),
      })
  },
})

describe('MembersTable — M4 severity-aware ring', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(clientForHarness.putFiles).mockClear()
    vi.mocked(clientForHarness.createDraft).mockClear()
  })

  it('M4: a member type cell with an error gets a red ring and aria-invalid', async () => {
    const wrapper = await mountSuspended(MembersWithTypeErrorHarness)
    await flushPromises()
    const row = wrapper.find('[data-member="DoorClosed"]')
    expect(row.exists()).toBe(true)
    // aria-label="Type of DoorClosed" lands on the inner <input> via $attrs.
    // :class (ring) lands on the root wrapper div.
    const typeInput = row.find('[aria-label="Type of DoorClosed"]')
    expect(typeInput.exists()).toBe(true)
    // ring-error is on the UInput root wrapper (parent of the inner input)
    expect(typeInput.element.parentElement!.classList.contains('ring-error')).toBe(true)
    expect(typeInput.attributes('aria-invalid')).toBe('true')
  })
})

// ── Test: method argument editing (name + type) ───────────────────────────────
// A method member's Type column is meaningless (a method has no Type in the AST),
// and until now the in/out argument panel was read-only. These tests drive the
// component directly with props/emits (no store dependency needed): the harness
// mounts MembersTable with a single method member with empty in/out, and every
// assertion is against emitted `update:member` payloads (mirroring the
// duplicate-name-rows test above).
describe('MembersTable — method argument editing (name+type)', () => {
  const zero = { file: '', line: 0, col: 0 }

  function methodMembers(): Array<Record<string, unknown>> {
    return [
      {
        name: 'Upgrade',
        kind: 'method',
        rule: 'mandatory',
        pos: zero,
        in: [],
        out: [],
      },
    ]
  }

  // A root-level `members` prop (rather than closing over a local variable) so
  // tests can call wrapper.setProps({ members }) to simulate the parent
  // re-rendering after applying an emitted patch (VTU only allows setProps on
  // the mounted root component).
  const MethodHarness = defineComponent({
    props: {
      members: { type: Array, required: true },
    },
    setup(props) {
      return () =>
        h(UApp, null, {
          default: () =>
            h(MembersTable, {
              typeName: 'T',
              members: props.members as never,
              diagnosticIndex: indexDiagnostics([]),
              readonly: false,
            }),
        })
    },
  })

  function mountMethodTable(members: Array<Record<string, unknown>>) {
    return mountSuspended(MethodHarness, { props: { members } })
  }

  it('does not render an editable Type input for a method row', async () => {
    const wrapper = await mountMethodTable(methodMembers())
    await flushPromises()

    const row = wrapper.find('[data-member="Upgrade"]')
    expect(row.exists()).toBe(true)
    expect(row.find('[data-testid="member-type-input-Upgrade"]').exists()).toBe(false)
    expect(row.find('[aria-label="Type of Upgrade"]').exists()).toBe(false)
  })

  it('the expand chevron is present for a method row even with zero args (editable mode)', async () => {
    const wrapper = await mountMethodTable(methodMembers())
    await flushPromises()
    expect(wrapper.find('[aria-label="Toggle arguments for Upgrade"]').exists()).toBe(true)
  })

  it('clicking "+ Add input" emits update:member with in: [{ name: "", type: { raw: "" } }]', async () => {
    const wrapper = await mountMethodTable(methodMembers())
    await flushPromises()

    await wrapper.find('[aria-label="Toggle arguments for Upgrade"]').trigger('click')
    await flushPromises()

    const addBtn = wrapper.find('[data-testid="member-arg-add-in-Upgrade"]')
    expect(addBtn.exists()).toBe(true)
    await addBtn.trigger('click')
    await flushPromises()

    const table = wrapper.findComponent(MembersTable)
    const emitted = table.emitted('update:member')
    expect(emitted).toBeTruthy()
    const payload = emitted![emitted!.length - 1]![0] as {
      member: string
      patch: Partial<import('~/types').Member>
    }
    expect(payload.member).toBe('Upgrade')
    expect(payload.patch.in).toHaveLength(1)
    expect(payload.patch.in![0]!.name).toBe('')
    expect(payload.patch.in![0]!.type.raw).toBe('')
  })

  it('editing a newly-added input arg name+type emits the full updated array', async () => {
    const wrapper = await mountMethodTable(methodMembers())
    await flushPromises()
    const table = wrapper.findComponent(MembersTable)

    await wrapper.find('[aria-label="Toggle arguments for Upgrade"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="member-arg-add-in-Upgrade"]').trigger('click')
    await flushPromises()

    // Simulate the parent (TypeView) re-rendering with the emitted patch applied,
    // which is exactly what happens end-to-end via updateType/Object.assign.
    const addPayload = table.emitted('update:member')!.at(-1)![0] as {
      patch: Partial<import('~/types').Member>
    }
    const updatedMembers = methodMembers()
    updatedMembers[0]!.in = addPayload.patch.in
    await wrapper.setProps({ members: updatedMembers })
    await flushPromises()

    const nameInput = wrapper.find('[data-testid="member-arg-name-in-Upgrade-0"]')
    const typeInput = wrapper.find('[data-testid="member-arg-type-in-Upgrade-0"]')
    expect(nameInput.exists()).toBe(true)
    expect(typeInput.exists()).toBe(true)

    await nameInput.setValue('Level')
    await nameInput.trigger('change')
    await flushPromises()

    let payload = table.emitted('update:member')!.at(-1)![0] as {
      patch: Partial<import('~/types').Member>
    }
    expect(payload.patch.in).toEqual([
      expect.objectContaining({ name: 'Level' }),
    ])

    // Re-render with the name patch applied (as the real parent would after
    // saveModel/reload) so the type edit below composes on top of it.
    const afterName = methodMembers()
    afterName[0]!.in = payload.patch.in
    await wrapper.setProps({ members: afterName })
    await flushPromises()

    const typeInputAfter = wrapper.find('[data-testid="member-arg-type-in-Upgrade-0"]')
    await typeInputAfter.setValue('Double')
    await typeInputAfter.trigger('change')
    await flushPromises()

    payload = table.emitted('update:member')!.at(-1)![0] as {
      patch: Partial<import('~/types').Member>
    }
    expect(payload.patch.in).toEqual([
      expect.objectContaining({ name: 'Level', type: expect.objectContaining({ raw: 'Double' }) }),
    ])
  })

  it('removing an input arg emits the array without it', async () => {
    const withArg = [
      {
        name: 'Upgrade',
        kind: 'method',
        rule: 'mandatory',
        pos: zero,
        in: [{ name: 'ProgramId', type: { raw: 'String', name: 'String', pos: zero }, pos: zero }],
        out: [],
      },
    ]
    const wrapper = await mountMethodTable(withArg)
    await flushPromises()
    const table = wrapper.findComponent(MembersTable)

    await wrapper.find('[aria-label="Toggle arguments for Upgrade"]').trigger('click')
    await flushPromises()

    const removeBtn = wrapper.find('[data-testid="member-arg-remove-in-Upgrade-0"]')
    expect(removeBtn.exists()).toBe(true)
    await removeBtn.trigger('click')
    await flushPromises()

    const payload = table.emitted('update:member')!.at(-1)![0] as {
      patch: Partial<import('~/types').Member>
    }
    expect(payload.patch.in).toEqual([])
  })

  it('clicking "+ Add output" emits update:member with out: [{ name: "", type: { raw: "" } }]', async () => {
    const wrapper = await mountMethodTable(methodMembers())
    await flushPromises()
    const table = wrapper.findComponent(MembersTable)

    await wrapper.find('[aria-label="Toggle arguments for Upgrade"]').trigger('click')
    await flushPromises()

    const addBtn = wrapper.find('[data-testid="member-arg-add-out-Upgrade"]')
    expect(addBtn.exists()).toBe(true)
    await addBtn.trigger('click')
    await flushPromises()

    const payload = table.emitted('update:member')!.at(-1)![0] as {
      patch: Partial<import('~/types').Member>
    }
    expect(payload.patch.out).toHaveLength(1)
    expect(payload.patch.out![0]!.name).toBe('')
  })

  it('read-only mode still renders the static arg list (no add/remove controls)', async () => {
    const withArg = [
      {
        name: 'StartProgram',
        kind: 'method',
        rule: 'mandatory',
        pos: zero,
        in: [{ name: 'ProgramId', type: { raw: 'String', name: 'String', pos: zero }, pos: zero }],
        out: [],
      },
    ]
    const Harness = defineComponent({
      setup: () => () =>
        h(UApp, null, {
          default: () =>
            h(MembersTable, {
              typeName: 'T',
              members: withArg as never,
              diagnosticIndex: indexDiagnostics([]),
              readonly: true,
            }),
        }),
    })
    const wrapper = await mountSuspended(Harness)
    await flushPromises()

    await wrapper.find('[aria-label="Toggle arguments for StartProgram"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="member-arg-add-in-StartProgram"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('ProgramId')
  })
})
