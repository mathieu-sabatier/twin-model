// @vitest-environment nuxt
//
// Mount test for CatalogTypeDetail.
//
// HARNESS NOTE — why MSW, not the injected-fake pattern: under `mountSuspended`
// the component resolves its Pinia stores from the NUXT RUNTIME's Pinia, which is
// NOT the Pinia a test primes via `setActivePinia()` + `useCatalogStore(fake)`.
// So injecting a fake client in the test does NOT reach the component — VERIFIED:
// an injected-fake rewrite returned the MSW default (`abstract:true`) instead of
// the injected `abstract:false`, failing the non-abstract test. The component's
// `useCatalogStore().detailFor -> useApi()` therefore hits the MSW node server
// (test/setup.ts). (catalogtree.nuxt.test.ts appears to use injection, but its
// single assertion — "DI" present — is ALSO satisfied by the MSW handler, so the
// injection is effectively a no-op there too.)
//
// ISOLATION: test/setup.ts's afterEach calls `server.resetHandlers()` (clearing
// each test's `server.use` override); the two tests use DISTINCT alias:name keys
// so the catalog store's detailCache — a Nuxt-runtime singleton we cannot reach to
// reset from here — cannot serve one test's entry to the other; and Vitest
// isolates the Nuxt runtime per test file.
import { describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import { http, HttpResponse } from 'msw'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import CatalogTypeDetail from '~/components/CatalogTypeDetail.vue'
import { useSelection } from '~/composables/useSelection'
import { server } from './setup'

// Explicit per-test detail handler so each test is self-contained (does not rely
// on the shared default handler's exact shape).
function detailHandler(over: { abstract: boolean; members?: unknown[]; baseChain?: unknown[] }) {
  return http.get('/api/catalog/:alias/types/:name', ({ params }) =>
    HttpResponse.json({
      alias: params.alias,
      uri: 'http://opcfoundation.org/UA/DI/',
      name: params.name,
      nodeClass: 'ObjectType',
      abstract: over.abstract,
      baseChain: over.baseChain ?? [{ alias: '', name: 'BaseObjectType', uri: 'http://opcfoundation.org/UA/' }],
      members: over.members ?? [{ name: 'Manufacturer', kind: 'property', placeholder: false }],
    }),
  )
}

describe('CatalogTypeDetail', () => {
  it('renders type name, nodeClass badge, base chain, members, Extend button (abstract type)', async () => {
    server.use(detailHandler({ abstract: true }))
    const wrapper = await mountSuspended(CatalogTypeDetail, {
      props: { alias: 'DI', name: 'DeviceType' },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('DI:DeviceType')
    expect(wrapper.text()).toContain('ObjectType')
    expect(wrapper.text()).toContain('abstract')
    expect(wrapper.text()).toContain('BaseObjectType')
    expect(wrapper.text()).toContain('Manufacturer')
    expect(wrapper.text()).toContain('Extend as new type')
    // Add instance NOT shown for an abstract type.
    expect(wrapper.text()).not.toContain('Add instance')
  })

  it('shows Add instance button for a non-abstract ObjectType', async () => {
    server.use(detailHandler({ abstract: false, members: [{ name: 'SerialNumber', kind: 'property', placeholder: false }] }))
    const wrapper = await mountSuspended(CatalogTypeDetail, {
      props: { alias: 'DI', name: 'ConcreteDevice' },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('DI:ConcreteDevice')
    expect(wrapper.text()).toContain('Extend as new type')
    expect(wrapper.text()).toContain('Add instance')
  })

  it('renders a companion-typed member as a link that selects that type', async () => {
    const ISA95 = 'http://www.OPCFoundation.org/UA/2013/01/ISA95'
    server.use(
      detailHandler({
        abstract: false,
        members: [
          // Recursive composition: <Equipment> is typed as EquipmentType itself.
          { name: '<Equipment>', kind: 'object', placeholder: true, type: { alias: 'ISA95', name: 'EquipmentType', uri: ISA95 } },
          // Primitive/ns0 member: no `type`, so no link.
          { name: 'AssetAssignment', kind: 'variable', placeholder: false },
        ],
      }),
    )
    const wrapper = await mountSuspended(CatalogTypeDetail, {
      props: { alias: 'ISA95', name: 'EquipmentType' },
    })
    await flushPromises()

    const link = wrapper.get('[data-testid="member-type-link-<Equipment>"]')
    expect(link.text()).toContain('ISA95:EquipmentType')
    // The primitive member has no type link.
    expect(wrapper.find('[data-testid="member-type-link-AssetAssignment"]').exists()).toBe(false)

    const { selection, clear } = useSelection()
    clear()
    await link.trigger('click')
    expect(selection.value).toEqual({ kind: 'catalogType', alias: 'ISA95', name: 'EquipmentType' })
    clear()
  })

  it('renders an enum member affordance and opens the value popover', async () => {
    server.use(
      detailHandler({
        abstract: false,
        members: [
          {
            name: 'EquipmentLevel',
            kind: 'variable',
            placeholder: false,
            enum: {
              members: [
                { name: 'Enterprise', value: 0 },
                { name: 'Site', value: 1 },
                { name: 'Area', value: 2 },
              ],
            },
          },
        ],
      }),
    )
    const wrapper = await mountSuspended(CatalogTypeDetail, {
      props: { alias: 'ISA95', name: 'EquipmentClassType' },
    })
    await flushPromises()

    const trigger = wrapper.find('[data-testid="member-enum-EquipmentLevel"]')
    expect(trigger.exists()).toBe(true)

    await trigger.trigger('click')
    // Use nextTick instead of flushPromises here: flushPromises resolves a pending
    // microtask that fires a focusin event on the trigger span, which reka-ui's
    // DismissableLayer treats as a focus-outside (because triggerElement.value is
    // not yet set in the mountSuspended context) and auto-closes the popover.
    // One nextTick is sufficient to flush the Vue scheduler and have the portal
    // content appear in document.body.
    await nextTick()

    // UPopover portals its content to document.body.
    const values = document.body.querySelector('[data-testid="member-enum-values-EquipmentLevel"]')
    expect(values).not.toBeNull()
    expect(values!.textContent).toContain('Enterprise')
    expect(values!.textContent).toContain('Area')
    expect(values!.textContent).toContain('0')
    expect(values!.textContent).toContain('2')
  })
})
