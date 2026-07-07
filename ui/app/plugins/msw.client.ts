// Nuxt CLIENT plugin that starts the MSW browser worker in DEV ONLY.
//
// Why this is safe for the self-contained SSG production bundle:
//   - `.client` suffix => this plugin runs in the browser only, never in any SSR
//     step (we are ssr:false anyway).
//   - `import.meta.dev` is statically replaced with `false` at `nuxi generate`
//     time. The whole body below then becomes dead code, so Rollup tree-shakes
//     it AND the dynamic `import('~/mocks/browser')` away — msw/browser never
//     enters .output/public/. The real Go API serves /api in production.
//
// To (re)generate the service worker script this needs, run once:
//   npx msw init public/            (writes public/mockServiceWorker.js)
// It is committed so `npm run dev` works without a manual step. It is emitted to
// .output/public/ by `nuxi generate` but is inert unless started (this plugin is
// the only thing that starts it, and only in dev).
export default defineNuxtPlugin(async () => {
  if (!import.meta.dev) return
  if (typeof window === 'undefined') return
  // Opt-in only: the real Go API now serves /api (proxied in dev). Start MSW
  // solely when NUXT_PUBLIC_USE_MOCKS=true, for offline UI work with no backend.
  if (!useRuntimeConfig().public.useMocks) return

  const { worker } = await import('~/mocks/browser')
  await worker.start({
    // Don't spam the console; and let real (unmocked) requests pass through so a
    // live API can be pointed at during dev if desired.
    onUnhandledRequest: 'bypass',
    quiet: false,
    serviceWorker: { url: `${useRuntimeConfig().app.baseURL}mockServiceWorker.js` },
  })
})
