// @vitest-environment nuxt
//
// Regression tests for CatalogPicker (use-a-companion-type-as-member flow).
//
// HARNESS NOTE — why module-level vi.mock, not MSW or store injection: the picker
// reaches BOTH useCatalogStore and useDraftStore, and setMemberType internally
// drives saveModel -> putFiles + getDraftModel against a seeded draft model. That
// full path is expensive to stand up here and orthogonal to what these tests lock
// in. vi.mock replaces the store MODULES at import time, so the component's
// `import { useXStore } from '~/stores/…'` resolves to our fakes regardless of the
// Nuxt-runtime Pinia (which is why plain store injection is a no-op under
// mountSuspended — see catalogdetail.nuxt.test.ts).
//
// Locks in two QA findings:
//   1. Import URI must not be empty. loadSpecs is fire-and-forget on open; a quick
//      pick previously read uriFor() before specs loaded and wrote `Alias: ""`.
//      pick() now awaits loadSpecs, so setMemberType gets the real URI.
//   2. The modal must close after a successful pick (emit update:open=false). It
//      previously stayed open because the close ran after the model-refetch
//      re-render; pick() now closes before the mutating save.
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'

const MACHINETOOL_URI = 'http://opcfoundation.org/UA/MachineTool/'

const setMemberType = vi.fn().mockResolvedValue(undefined)
const loadSpecs = vi.fn().mockResolvedValue(undefined)
const runSearch = vi.fn().mockResolvedValue(undefined)

const fakeCatalog = {
  loadSpecs,
  runSearch,
  specs: [{ alias: 'MachineTool', uri: MACHINETOOL_URI, version: '1.02.0', publicationDate: '', dependencies: [] }],
  search: [{ alias: 'MachineTool', name: 'ChannelMonitoringType', nodeClass: 'ObjectType', abstract: false }],
}

vi.mock('~/stores/catalog', () => ({ useCatalogStore: () => fakeCatalog }))
vi.mock('~/stores/draft', () => ({ useDraftStore: () => ({ setMemberType }) }))

// Imported AFTER the mocks so the component picks up the fakes.
import CatalogPicker from '~/components/CatalogPicker.vue'

// UModal teleports its body to document.body, so the hit button lives outside the
// wrapper's subtree. Find and click it on the document.
async function clickHit(): Promise<void> {
  const el = document.querySelector<HTMLButtonElement>(
    '[data-testid="catalog-picker-hit-MachineTool-ChannelMonitoringType"]',
  )
  if (!el) throw new Error('picker hit button not rendered')
  el.click()
  await flushPromises()
}

describe('CatalogPicker', () => {
  beforeEach(() => {
    setMemberType.mockClear()
    loadSpecs.mockClear()
    runSearch.mockClear()
  })

  it('awaits loadSpecs inside pick() so the import URI is never empty', async () => {
    await mountSuspended(CatalogPicker, {
      props: { typeName: 'PumpType', member: 'FlowRate', open: true },
    })
    await flushPromises()

    // Clear the open-watcher's loadSpecs call so the assertion below proves pick()
    // ITSELF awaits loadSpecs. The old pick() never did — it read uriFor() eagerly
    // and could emit `MachineTool: ""`; this would fail on that code.
    loadSpecs.mockClear()
    await clickHit()

    expect(loadSpecs).toHaveBeenCalled()
    expect(setMemberType).toHaveBeenCalledWith({
      typeName: 'PumpType',
      member: 'FlowRate',
      refAlias: 'MachineTool',
      refName: 'ChannelMonitoringType',
      refUri: MACHINETOOL_URI, // NOT '' — the bug this test guards against
    })
  })

  it('closes BEFORE the mutating save resolves (emits update:open=false)', async () => {
    // Deferred save: stays pending so we can assert the close already happened.
    // The old code closed only AFTER awaiting the save, so nothing would be
    // emitted while it is pending — this would fail on that code.
    let resolveSave!: () => void
    setMemberType.mockImplementationOnce(
      () => new Promise<void>((res) => { resolveSave = () => res() }),
    )

    const wrapper = await mountSuspended(CatalogPicker, {
      props: { typeName: 'PumpType', member: 'FlowRate', open: true },
    })
    await flushPromises()

    await clickHit() // pick(): awaits loadSpecs, closes, then awaits the pending save

    expect(setMemberType).toHaveBeenCalled()
    expect(wrapper.emitted('update:open')?.at(-1)).toEqual([false])

    resolveSave()
    await flushPromises()
  })
})
