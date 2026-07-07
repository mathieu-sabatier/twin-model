// @vitest-environment nuxt
//
// Mount tests for CatalogTree. Data is driven via MSW (the shared handlers in
// app/mocks/handlers.ts already serve /api/catalog and /api/catalog/:alias/types).
// NOTE: mountSuspended resolves stores from the Nuxt-runtime Pinia, not the
// test's setActivePinia() — so injected-fake DI is a no-op here. MSW is the
// correct way to prime data.
import { describe, expect, it, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import CatalogTree from '~/components/CatalogTree.vue'

describe('CatalogTree', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders bundled specs after mount', async () => {
    // Data comes from MSW (GET /api/catalog → DI + Machinery), not an injected
    // fake — see the file header on why injection is a no-op under mountSuspended.
    const wrapper = await mountSuspended(CatalogTree)
    await flushPromises()
    expect(wrapper.text()).toContain('DI')
  })

  // Tripwire for the expand→load path (Finding #1).
  //
  // Verified genuine tripwire: using ArrowRight keyboard on a collapsed spec node
  // fires ONLY 'update:expanded' (confirmed via uTree.emitted() — 'update:modelValue'
  // is undefined). The OLD code only wired @update:model-value and ignored
  // @update:expanded, so ArrowRight would expand the node visually (reka-ui manages
  // that internally) but NEVER call expandSpec — types stayed unloaded.
  // The NEW code wires @update:expanded → onExpandedChange → expandSpec → typesFor.
  //
  // Evidence: ran this same test against the old CatalogTreeOLD.vue (the pre-fix
  // code): 'update:expanded' fired with ['spec:DI'] but DeviceType did NOT appear,
  // confirming the test would have failed before the fix.
  //
  // MSW handler for /api/catalog/:alias/types returns
  // { types: [{ name: 'DeviceType', nodeClass: 'ObjectType', abstract: true }] }.
  it('loads types when a spec is expanded via keyboard (expand-only path)', async () => {
    const wrapper = await mountSuspended(CatalogTree)
    // Wait for the initial loadSpecs() to settle (MSW /api/catalog returns DI + Machinery)
    await flushPromises()

    // Verify DI spec is rendered before expand
    expect(wrapper.text()).toContain('DI')

    // DeviceType should NOT be visible yet (types are lazily loaded on expand)
    expect(wrapper.text()).not.toContain('DeviceType')

    // Press ArrowRight on the collapsed DI treeitem.
    // reka-ui fires ONLY 'update:expanded' for keyboard expand (NOT 'update:modelValue').
    // This isolates the @update:expanded wiring path — the exact path the old code missed.
    const button = wrapper.find('button[role="treeitem"]')
    expect(button.text()).toContain('DI')
    await button.trigger('keydown', { key: 'ArrowRight', code: 'ArrowRight' })

    // Wait for expandSpec → catalog.typesFor('DI') → MSW /api/catalog/DI/types to resolve
    await flushPromises()

    // DeviceType should now appear in the rendered tree as a child of DI
    expect(wrapper.text()).toContain('DeviceType')

    // Confirm only update:expanded fired (not update:modelValue) — validates this
    // test is a genuine tripwire for the expand-only path
    const uTree = wrapper.findComponent({ name: 'UTree' })
    expect(uTree.emitted('update:expanded')).toBeDefined()
    expect(uTree.emitted('update:modelValue')).toBeUndefined()
  })
})
