// @vitest-environment nuxt
//
// Component tests for the propose flow (RT6 Slice 5): ProposeSlideover + BottomBar wire.
// Harness pattern: create the store with a fake client INSIDE the mounted
// component tree so Pinia's by-id 'draft' cache hands child components the same
// fake-backed instance.
//
// USlideover teleports its content to document.body (reka-ui portal), so we
// query elements from document.body rather than wrapper.
//
// IMPORTANT: Pinia caches store id 'draft' on the NUXT runtime Pinia — a second
// mountBar() reuses the same store. A module-level proposeSpy is used so tests
// can reconfigure it in beforeEach without needing a new Pinia/client per test.
// store.frozen must be reset to false when a test starts from a state where a
// previous test left it as true.
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { h, defineComponent } from 'vue'
import UApp from '@nuxt/ui/components/App.vue'
import { setActivePinia, createPinia } from 'pinia'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { equipmentModel, seedFile, diffSample } from '~/mocks/fixtures'
import { useDraftStore } from '~/stores/draft'
import { ProposeConflictError, ProposeError } from '~/api'
import type { ApiClient, CreateDraftResponse } from '~/api'
import type { Diagnostic } from '~/types'
import BottomBar from '~/components/BottomBar.vue'

const FAKE_DRAFT_ID = 'deadbeefcafe0010'

const conflictDiag: Diagnostic = {
  code: 'unit-on-non-numeric',
  severity: 'error',
  file: seedFile,
  line: 34,
  col: 46,
  path: 'object_types/FurnaceType/members/Efficiency/unit',
  message: 'unit "%" is only valid on numeric variables',
}

// Module-level spy so all tests share the same client (Pinia caches by id).
const proposeSpy = vi.fn().mockResolvedValue({ url: 'https://github.com/org/repo/pull/7' })

function makeSharedClient(): ApiClient {
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
      diagnostics: [], // clean — no errors so Propose gate opens
    }),
    getDraftFile: vi.fn().mockResolvedValue('model: Demo\n'),
    putFiles: vi.fn().mockResolvedValue(undefined),
    validate: vi.fn().mockResolvedValue({ file: seedFile, diagnostics: [] }),
    previewModelDesign: vi.fn().mockResolvedValue('<ModelDesign/>'),
    previewDiagram: vi.fn().mockResolvedValue('classDiagram'),
    diff: vi.fn().mockResolvedValue(diffSample),
    resolved: vi.fn().mockResolvedValue({ type: 'FurnaceType', members: [] }),
    propose: proposeSpy,
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
const sharedClient: ApiClient = makeSharedClient()

/**
 * Harness: creates the store with the shared fake client inside setup()
 * so Pinia's 'draft' cache gives BottomBar/ProposeSlideover the same instance.
 */
const Harness = defineComponent({
  async setup() {
    const store = useDraftStore(sharedClient)
    captured = store
    await store.createDraft()
    return () =>
      h(UApp, null, {
        default: () => h(BottomBar),
      })
  },
})

let wrapper: Awaited<ReturnType<typeof mountSuspended>> | null = null

/** Mount once at describe scope; tests share the mounted instance. */
async function mountBar() {
  captured = null
  wrapper = await mountSuspended(Harness)
  await flushPromises()
  return { store: captured as ReturnType<typeof useDraftStore>, wrapper }
}

describe('Propose flow — BottomBar + ProposeSlideover', () => {
  beforeEach(() => {
    captured = null
    proposeSpy.mockClear()
    proposeSpy.mockResolvedValue({ url: 'https://github.com/org/repo/pull/7' })
    // Outer pinia reset keeps test isolation at the unit-test level.
    setActivePinia(createPinia())
  })

  it('Propose button is enabled when there are no errors', async () => {
    const { wrapper } = await mountBar()
    const proposeBtn = wrapper.find('[data-testid="propose-button"]')
    expect(proposeBtn.exists()).toBe(true)
    expect(proposeBtn.attributes('disabled')).toBeUndefined()
  })

  it('clicking Propose button opens the slideover and diff list is visible', async () => {
    const { wrapper } = await mountBar()

    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()

    // USlideover teleports to document.body
    const diffListEl = document.body.querySelector('[data-testid="diff-list"]')
    expect(diffListEl).not.toBeNull()
    // The change items' text should be visible
    expect(diffListEl!.textContent).toContain(diffSample.changes[0]!.text)
  })

  it('branch field is prefilled with model/... slug', async () => {
    const { wrapper } = await mountBar()

    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()

    // In Nuxt UI v4, UInput renders the data-testid directly on the <input> element
    const branchInput = document.body.querySelector('[data-testid="propose-branch"]') as HTMLInputElement | null
    expect(branchInput).not.toBeNull()
    expect(branchInput!.value).toMatch(/^model\//)
  })

  it('submitting calls propose with branch/title/message; on success PR link appears and store is frozen', async () => {
    const { store, wrapper } = await mountBar()
    // Ensure a clean state for this test
    store.frozen = false
    store.diagnostics = []

    // Open the slideover
    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()

    // Click submit via native DOM (Vue binds @click to native click)
    const submitBtn = document.body.querySelector('[data-testid="propose-submit"]') as HTMLButtonElement | null
    expect(submitBtn).not.toBeNull()
    expect(submitBtn!.disabled).toBe(false)
    submitBtn!.click()
    await flushPromises()

    // propose was called with correct shape: api.propose(draftId, body)
    expect(proposeSpy).toHaveBeenCalledTimes(1)
    const calledWith = proposeSpy.mock.calls[0]![1] as { branch: string; title: string; message: string }
    expect(calledWith.branch).toMatch(/^model\//)
    expect(typeof calledWith.title).toBe('string')
    expect(typeof calledWith.message).toBe('string')

    // PR link appears in the teleported content
    const prLink = document.body.querySelector('[data-testid="pr-link"]')
    expect(prLink).not.toBeNull()
    expect(prLink!.getAttribute('href')).toBe('https://github.com/org/repo/pull/7')

    // Store is frozen
    expect(store.frozen).toBe(true)
  })

  it('ProposeConflictError leaves frozen=false, populates store.diagnostics, shows error alert', async () => {
    // Reconfigure spy to throw a conflict error
    proposeSpy.mockRejectedValue(new ProposeConflictError('draft has lint errors', [conflictDiag]))

    const { store, wrapper } = await mountBar()
    // Ensure a clean state: frozen was set to true by the previous success test
    store.frozen = false
    store.diagnostics = []

    // Open the slideover
    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()

    // Click submit
    const submitBtn = document.body.querySelector('[data-testid="propose-submit"]') as HTMLButtonElement | null
    expect(submitBtn).not.toBeNull()
    submitBtn!.click()
    await flushPromises()

    // Frozen stays false
    expect(store.frozen).toBe(false)

    // Diagnostics populated from conflict error
    expect(store.diagnostics).toHaveLength(1)
    expect(store.diagnostics[0]!.code).toBe(conflictDiag.code)

    // Error alert visible in the teleported content
    const alert = document.body.querySelector('[data-testid="propose-conflict-alert"]')
    expect(alert).not.toBeNull()
  })

  it('M3: propose error shows a friendly message with raw detail behind a Details disclosure', async () => {
    proposeSpy.mockRejectedValue(
      new ProposeError(
        "Couldn't open the pull request: the repository or branch wasn't found.",
        '{"message":"Not Found"}',
      ),
    )
    const { store, wrapper } = await mountBar()
    store.frozen = false
    store.diagnostics = []

    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()
    const submitBtn = document.body.querySelector('[data-testid="propose-submit"]') as HTMLButtonElement | null
    submitBtn!.click()
    await flushPromises()

    const errorAlert = document.body.querySelector('[data-testid="propose-error-alert"]')
    expect(errorAlert).not.toBeNull()
    expect(errorAlert!.textContent).toContain("Couldn't open the pull request")

    const details = document.body.querySelector('[data-testid="propose-error-details"]')
    expect(details).not.toBeNull()
    expect(details!.textContent).toContain('Not Found')
  })

  it('Title, Branch and Commit fields span full width', async () => {
    const { wrapper } = await mountBar()
    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()
    for (const id of ['propose-title', 'propose-branch', 'propose-message']) {
      const el = document.body.querySelector(`[data-testid="${id}"]`) as HTMLElement | null
      expect(el, id).not.toBeNull()
      // class="w-full" lands on the control's own root wrapper (the field's <input>/
      // <textarea> parent), so the field fills the slideover column.
      expect(el!.parentElement!.classList.contains('w-full'), id).toBe(true)
    }
  })

  it('non-lint propose error shows propose-error-alert and leaves store.frozen false', async () => {
    // Reconfigure spy to throw a generic (non-conflict) error
    proposeSpy.mockRejectedValue(new Error('open pr: GitHub API 404: Not Found'))

    const { store, wrapper } = await mountBar()
    store.frozen = false
    store.diagnostics = []

    // Open the slideover
    await wrapper.find('[data-testid="propose-button"]').trigger('click')
    await flushPromises()

    // Click submit
    const submitBtn = document.body.querySelector('[data-testid="propose-submit"]') as HTMLButtonElement | null
    expect(submitBtn).not.toBeNull()
    submitBtn!.click()
    await flushPromises()

    // Frozen stays false
    expect(store.frozen).toBe(false)

    // Error alert visible with the error message and has ARIA alert role
    const errorAlert = document.body.querySelector('[data-testid="propose-error-alert"]')
    expect(errorAlert).not.toBeNull()
    expect(errorAlert!.getAttribute('role')).toBe('alert')
    expect(errorAlert!.textContent).toContain('open pr: GitHub API 404: Not Found')
  })
})
