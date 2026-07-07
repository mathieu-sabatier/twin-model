# twinmodel web

Self-contained Nuxt SPA for browsing and editing OPC UA **object types** and **instances**.
The terminal action is "Propose change" — which opens a Pull Request. This is a form-based
editor, **not** a text editor.

## Commands

```sh
npm install         # install deps
npm run dev         # dev server (talks to NUXT_PUBLIC_API_BASE, default /api)
npm run generate    # static SSG build -> .output/public/
npm run test        # Vitest component tests (MSW-mocked API, no live server)
npm run typecheck   # vue-tsc / nuxi typecheck
```

## Architecture

- **Nuxt 4 + Nuxt UI 4** (Tailwind v4), TypeScript **strict**, **Pinia**.
- Rendering mode is **static SSG** (`ssr: false`). The generated `.output/public/` is copied to
  the repo's `internal/web/dist/` (a later Taskfile step) and embedded in a distroless Go binary.
- **All HTTP** goes through one typed `api/` module keyed off `NUXT_PUBLIC_API_BASE` (default
  `/api`). The Go backend is developed separately; the SPA runs against **MSW** mocks in dev and
  test. Going live = change the base URL only.

## Self-contained / no-CDN (HARD requirement)

The built bundle is served behind a **strict CSP** that blocks every third-party host, so it must
make **zero** third-party network requests at runtime:

- **Icons** are bundled at build time from `@iconify-json/lucide` (Nuxt Icon `provider: 'none'` +
  `clientBundle.scan`). The Iconify runtime CDN is never called.
- **Fonts** are a pure **system-font stack** (see `app/assets/css/main.css`). No Google Fonts.
- **Mermaid** (diagram rendering) is bundled from npm, not a CDN.
- No Nuxt server routes, no NuxtHub, no external services.

If you add an asset, keep it local. `npm run generate` output must have no references to
`api.iconify.design`, `fonts.googleapis.com`, `cdn.jsdelivr.net`, etc.

## Determinism contract

The **server owns the canonical form**. After every edit the client `PUT`s the full YAML, then
**re-fetches** the model AST and re-renders from it — never trusting the locally-edited AST (the
server reorders members and drops defaults). Validation (`POST …/validate`) is debounced. Draft id
lives in the route; there is no localStorage draft.
