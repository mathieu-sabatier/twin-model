// https://nuxt.com/docs/api/configuration/nuxt-config
//
// Self-contained SSG skeleton for the twinmodel web SPA.
//
// This bundle is generated with `nuxi generate`, embedded in a distroless Go
// binary, and served behind a strict CSP that blocks EVERY third-party host.
// The load-bearing rule for this file: the runtime must make ZERO third-party
// network requests. Every choice below serves that goal.
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-01',

  // Static single-page app: no SSR, no Nitro server routes, no runtime server.
  // `nuxi generate` emits a fully static `.output/public/` served as flat files.
  ssr: false,

  modules: [
    '@nuxt/ui',
    '@pinia/nuxt',
    // Enables the Vitest/Nuxt test environment integration (see vitest.config.ts).
    '@nuxt/test-utils/module',
  ],

  // Nuxt UI (Tailwind v4) global stylesheet. The system-font stack lives here,
  // so no @nuxt/fonts and no Google Fonts CDN request at runtime.
  css: ['~/assets/css/main.css'],

  // Icons: bundled at build time, never fetched from the Iconify runtime CDN.
  //   - provider: 'none'   -> no server endpoint (we are SSG, there is no server)
  //     and no 'iconify' CDN provider.
  //   - fallbackToApi: false -> the HARD guarantee. @iconify/vue ships
  //     "https://api.iconify.design" as an inert default constant in its runtime;
  //     this flag guarantees no code path ever fetches it. A missing icon renders
  //     blank instead of leaking a third-party request (the built bundle is served
  //     behind a strict CSP that would block it anyway).
  //   - clientBundle.scan -> statically scans templates and embeds the icons
  //     actually used (`i-lucide-*`) into the client bundle. @iconify-json/lucide
  //     must be installed. (Completeness of Nuxt UI's dynamic icons is verified by
  //     a headless offline load in the final self-contained check.)
  icon: {
    provider: 'none',
    fallbackToApi: false,
    clientBundle: {
      scan: true,
      includeCustomCollections: true,
      icons: [
        'lucide:x',
        'lucide:check',
        'lucide:minus',
        'lucide:chevron-down',
        'lucide:chevron-up',
        'lucide:chevron-right',
        'lucide:chevron-left',
        'lucide:loader-circle',
        'lucide:circle-check',
        'lucide:circle-alert',
        'lucide:circle-x',
        'lucide:search',
        'lucide:network',
        'lucide:arrow-up',
        'lucide:arrow-down',
        // Found by Step 1 grep (dynamic :name bindings in app/components):
        'lucide:shapes',
        'lucide:box',
        'lucide:triangle-alert',
        // Script-defined icons in display/ components: the template scanner only
        // catches `:icon=` bindings in templates, NOT string values assigned in
        // <script> MAP objects. Any icon string set in JS/TS and bound via
        // `:icon="spec.icon"` must be listed here manually.
        // KindBadge.vue:
        'lucide:tag',
        'lucide:variable',
        'lucide:square-function',
        // RuleBadge.vue:
        'lucide:asterisk',
        // AccessBadge.vue:
        'lucide:pencil',
        'lucide:eye',
        // TreePane.vue instance kebab menu:
        'lucide:ellipsis',
        'lucide:trash-2',
        // YamlPane.vue soft-wrap toggle:
        'lucide:wrap-text',
        'lucide:arrow-right-to-line',
        // useModelTree.ts tree-node icons (script-defined, bound via :icon so the
        // template scanner never sees them). ISA-95 org tier, perspective folders,
        // Unassigned bucket, group/level/enum markers. Missing here = blank icon.
        'lucide:building',
        'lucide:layers',
        'lucide:folder',
        'lucide:inbox',
        'lucide:hash',
        'lucide:list',
        'lucide:boxes',
        // ISA-95 tier-graded instance icons (isa95Icon in lib/isa95.ts):
        // enterprise / site / area / factory segment / work unit.
        'lucide:building-2',
        'lucide:land-plot',
        'lucide:factory',
        'lucide:hammer',
        'lucide:cog',
      ],
    },
  },

  runtimeConfig: {
    public: {
      // Same-origin base for the Go API. In dev it is proxied to the backend
      // (see nitro.devProxy below); in prod the Go binary serves /api itself.
      // Override at runtime via NUXT_PUBLIC_API_BASE.
      apiBase: '/api',
      // MSW mocks are opt-in now that the real Go API exists. Default false, so
      // `npm run dev` talks to the backend through the /api dev proxy. Enable
      // offline UI work (no backend) with NUXT_PUBLIC_USE_MOCKS=true.
      useMocks: false,
    },
  },

  // Dev-only: forward the SPA's same-origin /api calls to the backend process
  // (`twinmodel serve`, or the local-fixture demoserve). This is what lets the UI
  // and API run as two processes in dev without CORS. It is inert in
  // `nuxi generate` (an SSG bundle has no runtime server), and prod needs no
  // proxy because one Go binary serves both / and /api.
  //
  // Nitro's devProxy STRIPS the matched `/api` prefix, so the target must include
  // `/api`: a request to /api/schema forwards to <target>/schema. The target is
  // therefore the backend's API base. Override with NUXT_DEV_API_TARGET.
  nitro: {
    devProxy: {
      '/api': {
        target: process.env.NUXT_DEV_API_TARGET || 'http://localhost:8080/api',
        changeOrigin: true,
      },
    },
  },

  // Allow Vite (dev server + Vitest transform) to read files ABOVE ui/ so the
  // types drift guard can import the REAL Go golden at
  // ../internal/dto/testdata/model.golden.json (single source of truth). This
  // affects the dev/test filesystem allowlist only — it does not put those files
  // in the SSG production bundle.
  vite: {
    server: {
      fs: {
        allow: ['..'],
      },
    },
  },

  // Fully static, relative asset URLs so the bundle works when served from any
  // path by the embedding Go binary.
  app: {
    baseURL: '/',
    cdnURL: '',
  },

  typescript: {
    strict: true,
    // Keep type-check out of dev/build for speed; run explicitly via `npm run typecheck`.
    typeCheck: false,
  },

  hooks: {
    // MSW is dev/test ONLY. Its service worker (public/mockServiceWorker.js) must
    // exist in dev so `npm run dev` can register it, but it must NOT ship in the
    // self-contained SSG bundle (the real Go API serves /api in prod, and the file
    // carries a third-party host reference in its banner comment). Nitro copies
    // everything under public/ into .output/public/; after it does, delete the
    // worker so the generated bundle stays MSW-free and third-party-clean.
    'nitro:build:public-assets': (nitro) => {
      const swPath = join(nitro.options.output.publicDir, 'mockServiceWorker.js')
      if (existsSync(swPath)) {
        rmSync(swPath)
      }
    },
  },

  devtools: { enabled: true },
})
