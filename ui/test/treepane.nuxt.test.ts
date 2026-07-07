// @vitest-environment nuxt
//
// Tests for TreePane — per-instance kebab (rename/delete) + aria-label.
// Uses the same harness pattern as shell.nuxt.test.ts: create the store with a
// fake client backed by real fixtures INSIDE the mounted component so Pinia's
// cache hands TreePane the same fake-backed instance.
import { describe, expect, it, beforeEach } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import USelect from '@nuxt/ui/components/Select.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { vi } from 'vitest'
import {
  equipmentModel,
  diagnostics as fixtureDiags,
  seedFile,
  resolvedFurnace,
} from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import { useModelTree } from '~/composables/useModelTree'
import { useSelection } from '~/composables/useSelection'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { ValidateResponse } from '~/types'
import type { Enum, Instance, ObjectType, Perspective } from '~/types'
import TreePane from '~/components/TreePane.vue'

const FAKE_DRAFT_ID = 'deadbeefcafe0001'

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
    diff: vi.fn().mockResolvedValue({ changes: [], text: '' }),
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

let captured: ReturnType<typeof useDraftStore> | null = null
let clientForHarness: ApiClient = makeFakeClient()

const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(clientForHarness)
    captured = store
    await store.createDraft()
    return () => h(UApp, null, { default: () => h(TreePane) })
  },
})

async function mountTree(
  client: ApiClient = makeFakeClient(),
  mountOptions: Parameters<typeof mountSuspended>[1] = {},
) {
  clientForHarness = client
  captured = null
  const wrapper = await mountSuspended(Harness, mountOptions)
  await flushPromises()
  return wrapper
}

describe('TreePane — aria-label and instance action menus', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  it('has an accessible tree label', async () => {
    const wrapper = await mountTree()
    expect(wrapper.find('[aria-label="Model tree"]').exists()).toBe(true)
  })

  it('shows a rename/delete menu trigger on instance rows', async () => {
    const wrapper = await mountTree()
    await flushPromises()
    expect(wrapper.find('[data-testid="instance-menu-Furnace01"]').exists()).toBe(true)
  })

  it('marks tree nodes that carry an error diagnostic', async () => {
    const wrapper = await mountTree()
    // Seed a level diagnostic on Furnace01 by mutating the store's reactive
    // diagnostics directly. (The draft store is cached per test-file under
    // mountSuspended, so a per-test client override is NOT re-invoked — mirror
    // the captured.model seed pattern used by the perspective-selector test.)
    captured!.diagnostics = [{
      code: 'unknown-level', severity: 'error', file: seedFile, line: 1, col: 1,
      path: 'instances/Furnace01/level', message: 'unknown level',
    }]
    await flushPromises()
    // useModelTree tags the instance node with a trailing alert icon; TreePane's
    // #item-trailing renders it with a node-badge-<value> testid.
    expect(wrapper.find('[data-testid="node-badge-instance:Furnace01"]').exists()).toBe(true)
  })
})

describe('TreePane — perspective selector (B4)', () => {
  beforeEach(() => {
    captured = null
    setActivePinia(createPinia())
  })

  // MSW fixtures (Task B7) do not include perspectives yet at the time B4 lands,
  // so we seed one ourselves — directly on the mounted store's `model` ref,
  // rather than via a per-test client override. Pinia caches the 'draft' store
  // by id on the NUXT RUNTIME pinia (see the IMPORTANT note in
  // bottombar.nuxt.test.ts / propose.nuxt.test.ts): once the store exists for
  // this test file, a fresh `mountTree(newClient)` call does NOT re-run
  // createDraft/getDraftModel against the new client — the cached store (and
  // its original client) is reused. Mutating `captured.model` directly is the
  // pattern those files use to vary per-test state under that constraint, and
  // is exactly what the store's own state-ref is for (`model` is exposed as a
  // writable ref in the returned object, same as every other test that pokes
  // `store.diagramSrc = ...` etc.).
  const spatialPerspective: Perspective = {
    pos: { file: seedFile, line: 0, col: 0 },
    id: 'spatial_zones',
    label: 'Spatial / fire zones',
    membership: 'exclusive',
    export: false,
    nodes: [
      {
        pos: { file: seedFile, line: 0, col: 0 },
        id: 'hall_b',
        label: 'Hall B',
        members: ['Furnace01'],
      },
    ],
  }

  async function seedPerspective(): Promise<void> {
    captured!.model = { ...captured!.model!, perspectives: [spatialPerspective] }
    await flushPromises()
  }

  it('shows a perspective selector and switches views', async () => {
    const wrapper = await mountTree()
    await seedPerspective()

    const selector = wrapper.find('[data-testid="perspective-selector"]')
    expect(selector.exists()).toBe(true)

    // Default view is 'isa95' (per spec) — the perspective group must not show yet.
    expect(wrapper.html()).not.toContain('Spatial / fire zones')

    // Drive the view switch by emitting the USelect's v-model update directly —
    // clicking the Reka trigger open is unreliable under mountSuspended (see
    // Part 2 C2's UPopover finding), so we bypass the overlay and simulate the
    // component emitting its update:model-value event, exactly as a real
    // selection would. USelect must be located via the imported component
    // (not a data-testid CSS selector) — findComponent('[data-testid=...]')
    // resolves to an inner element/instance whose emit does not reach the
    // v-model binding (confirmed empirically: the view never switched).
    const selectorComp = wrapper.findComponent(USelect)
    await selectorComp.vm.$emit('update:modelValue', 'perspective:spatial_zones')
    await flushPromises()

    expect(wrapper.html()).toContain('Spatial / fire zones')
    expect(wrapper.html()).toContain('Hall B')
  })

  it('reveals the previously-selected instance after switching to a perspective view', async () => {
    const wrapper = await mountTree()
    await seedPerspective()

    // Select Furnace01 as if the user had clicked it in the default view.
    const { select, clear } = useSelection()
    select({ kind: 'instance', name: 'Furnace01' })
    await flushPromises()

    const selectorComp = wrapper.findComponent(USelect)
    await selectorComp.vm.$emit('update:modelValue', 'perspective:spatial_zones')
    await flushPromises()

    // Furnace01 is a member of the Hall B node — it must still resolve/render
    // after the switch (defaultExpanded:true on perspective nodes reveals it,
    // and the `instance:<name>` value key is unchanged across views so
    // selectedItem continues to resolve).
    expect(wrapper.html()).toContain('Furnace01')
    clear()
  })
})

describe('useModelTree — Enums as a top-level group (L1)', () => {
  it('emits Enums as a root group, not a child of Types, when enums exist', () => {
    const { items } = useModelTree(
      () => [{ name: 'T', abstract: false } as unknown as ObjectType],
      () => [] as Instance[],
      () => [{ name: 'State', values: [] } as unknown as Enum],
    )
    const roots = items.value.map((i) => i.value)
    expect(roots).toContain('group:enums')
    // Order: Types, Enums, Instances
    expect(roots).toEqual(['group:types', 'group:enums', 'group:instances'])
    // And Enums must NOT be nested under Types
    const types = items.value.find((i) => i.value === 'group:types')
    expect((types?.children ?? []).some((c) => c.value === 'group:enums')).toBe(false)
  })

  it('omits the Enums group entirely when there are no enums', () => {
    const { items } = useModelTree(
      () => [{ name: 'T', abstract: false } as unknown as ObjectType],
      () => [] as Instance[],
      () => [] as Enum[],
    )
    const roots = items.value.map((i) => i.value)
    expect(roots).toEqual(['group:types', 'group:instances'])
  })
})
