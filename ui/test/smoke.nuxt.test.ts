// @vitest-environment nuxt
import { describe, expect, it } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import App from '~/app.vue'

// Smoke test: proves Vitest + @nuxt/test-utils + Nuxt UI + the pages router mount
// together and the app shell actually renders. Mounting App routes to the editor
// page ([[draftId]].vue), which bootstraps a draft through the real useApi() ->
// MSW handlers (seeded, honest). Not a placeholder — it asserts real DOM output.
// Deep behaviour (tree, selection, bottom-bar gate) is covered by shell.nuxt.test.ts.
describe('app shell', () => {
  it('mounts the routed shell and renders the twinmodel brand', async () => {
    const wrapper = await mountSuspended(App)
    await flushPromises()

    // The shell header brand renders (proves the page + AppShell mounted).
    expect(wrapper.text()).toContain('twinmodel')
    // The persistent bottom bar is present.
    expect(wrapper.find('[data-testid="bottom-bar"]').exists()).toBe(true)
  })
})
