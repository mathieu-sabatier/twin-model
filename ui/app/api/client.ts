// The ONE typed HTTP client for the whole SPA. Nothing else in the app may call
// `fetch`/`$fetch` directly — all backend access funnels through here so that
// "going live" is a single change of NUXT_PUBLIC_API_BASE (the base URL) with no
// other code touched. See API-CONTRACT.md for the frozen endpoint list/shapes.
//
// Design:
//   - `createApiClient(baseURL)` is a plain factory (no Nuxt context) returning
//     one method per endpoint. This is what unit tests drive directly.
//   - `useApi()` (in ./useApi) is the thin Nuxt composable that reads the runtime
//     base and calls the factory. Kept in a separate file so this module stays
//     free of Nuxt auto-imports and is trivially unit-testable.
import { $fetch } from 'ofetch'
import type {
  BranchList,
  CatalogListResponse,
  CatalogSearchHit,
  CatalogSearchResponse,
  CatalogSpec,
  CatalogTypeDetail,
  CatalogTypeSummary,
  CatalogTypesResponse,
  Change,
  Diagnostic,
  DiffResponse,
  ModelResponse,
  PullRequest,
  RepoInfo,
  ResolvedResponse,
  UnitInfo,
  ValidateResponse,
} from '~/types'

// We import `$fetch` from ofetch explicitly rather than relying on Nuxt's global
// auto-import: ofetch is the exact instance Nuxt bundles, so this is identical in
// the app, and it also makes the factory usable in plain node unit tests (where
// there is no Nuxt-provided `$fetch` global) against MSW.

/** A JSON Schema object (GET /api/schema). Loosely typed on purpose. */
export type JsonSchema = Record<string, unknown>

/** Result of creating a draft (POST /api/drafts). */
export interface CreateDraftResponse {
  id: string
  baseRef: string
  files: string[]
}

/** Body for POST /api/drafts/:id/propose. */
export interface ProposeBody {
  branch: string
  title: string
  message: string
}

/** Result of a successful propose (200). */
export interface ProposeResponse {
  url: string
}

/**
 * Thrown when POST .../propose returns 409 (draft is lint-red). Carries the
 * server's diagnostics so a caller can render them at their fields.
 */
export class ProposeConflictError extends Error {
  readonly diagnostics: Diagnostic[]

  constructor(message: string, diagnostics: Diagnostic[]) {
    super(message)
    this.name = 'ProposeConflictError'
    this.diagnostics = diagnostics
    // Restore prototype chain for `instanceof` across the transpilation target.
    Object.setPrototypeOf(this, ProposeConflictError.prototype)
  }
}

/**
 * Thrown when POST .../propose fails to open the PR (non-409). Carries the friendly
 * message plus the raw upstream detail so the UI can disclose it on demand.
 */
export class ProposeError extends Error {
  readonly detail?: string

  constructor(message: string, detail?: string) {
    super(message)
    this.name = 'ProposeError'
    this.detail = detail
    Object.setPrototypeOf(this, ProposeError.prototype)
  }
}

/** Shape of the 409 propose body (see API-CONTRACT.md mock decision #3). */
interface ProposeConflictBody {
  error?: string
  diagnostics?: Diagnostic[]
}

/** Type guard for the fetch error shape ofetch raises on non-2xx. */
function isFetchError(
  err: unknown,
): err is { status?: number; response?: { status?: number }; data?: unknown } {
  return typeof err === 'object' && err !== null && ('status' in err || 'response' in err || 'data' in err)
}

/** The typed API surface returned by createApiClient. */
export interface ApiClient {
  getSchema(): Promise<JsonSchema>
  getModel(ref: string, file?: string): Promise<ModelResponse>
  createDraft(baseRef: string): Promise<CreateDraftResponse>
  getDraftModel(id: string, file?: string): Promise<ModelResponse>
  getDraftFile(id: string, file?: string): Promise<string>
  putFiles(id: string, files: Record<string, string>): Promise<void>
  validate(id: string, file?: string): Promise<ValidateResponse>
  previewModelDesign(id: string, file?: string): Promise<string>
  previewDiagram(id: string, file?: string, view?: 'types' | 'instances'): Promise<string>
  diff(id: string, file?: string): Promise<DiffResponse>
  resolved(id: string, type: string, file?: string): Promise<ResolvedResponse>
  propose(id: string, body: ProposeBody): Promise<ProposeResponse>
  getUnits(): Promise<UnitInfo[]>
  getCatalog(): Promise<CatalogSpec[]>
  getCatalogTypes(alias: string): Promise<CatalogTypeSummary[]>
  getCatalogType(alias: string, name: string): Promise<CatalogTypeDetail>
  searchCatalog(q: string): Promise<CatalogSearchHit[]>
  getRepo(): Promise<RepoInfo>
  listPRs(): Promise<PullRequest[]>
  listBranches(): Promise<BranchList>
}

/**
 * Build a typed API client bound to `baseURL` (e.g. `/api`). Pure — takes no Nuxt
 * context — so tests can point it at MSW without a running Nuxt app.
 */
export function createApiClient(baseURL: string): ApiClient {
  // Normalise so we can safely template `${base}/...` without doubling slashes.
  const base = baseURL.replace(/\/+$/, '')

  // Only-optional `file` becomes a query param when present.
  const fileQuery = (file?: string): Record<string, string> =>
    file === undefined ? {} : { file }

  return {
    getSchema() {
      return $fetch<JsonSchema>(`${base}/schema`)
    },

    getModel(ref, file) {
      return $fetch<ModelResponse>(`${base}/model`, {
        query: { ref, ...fileQuery(file) },
      })
    },

    createDraft(baseRef) {
      return $fetch<CreateDraftResponse>(`${base}/drafts`, {
        method: 'POST',
        body: { baseRef },
      })
    },

    getDraftModel(id, file) {
      return $fetch<ModelResponse>(`${base}/drafts/${encodeURIComponent(id)}/model`, {
        query: fileQuery(file),
      })
    },

    async putFiles(id, files) {
      // Server canonicalizes and returns an (empty) 200 body; we ignore it and
      // the caller re-fetches /model (determinism contract).
      await $fetch(`${base}/drafts/${encodeURIComponent(id)}/files`, {
        method: 'PUT',
        body: { files },
      })
    },

    validate(id, file) {
      return $fetch<ValidateResponse>(
        `${base}/drafts/${encodeURIComponent(id)}/validate`,
        { method: 'POST', query: fileQuery(file) },
      )
    },

    previewModelDesign(id, file) {
      // `responseType: 'text'` makes ofetch resolve to `string` (its
      // MappedResponseType), so we do NOT pass a `<T>` generic here — doing so
      // would pin R back to the default "json" and conflict.
      return $fetch(
        `${base}/drafts/${encodeURIComponent(id)}/preview/modeldesign`,
        { query: fileQuery(file), responseType: 'text' },
      )
    },

    getDraftFile(id, file) {
      return $fetch(
        `${base}/drafts/${encodeURIComponent(id)}/file`,
        { query: fileQuery(file), responseType: 'text' },
      )
    },

    previewDiagram(id, file, view) {
      return $fetch(
        `${base}/drafts/${encodeURIComponent(id)}/preview/diagram`,
        { query: { ...fileQuery(file), ...(view ? { view } : {}) }, responseType: 'text' },
      )
    },

    diff(id, file) {
      return $fetch<DiffResponse>(`${base}/drafts/${encodeURIComponent(id)}/diff`, {
        query: fileQuery(file),
      })
    },

    resolved(id, type, file) {
      return $fetch<ResolvedResponse>(
        `${base}/drafts/${encodeURIComponent(id)}/types/${encodeURIComponent(type)}/resolved`,
        { query: fileQuery(file) },
      )
    },

    async getUnits() {
      const r = await $fetch<{ units: UnitInfo[] }>(`${base}/units`)
      return r.units
    },

    async getCatalog() {
      const r = await $fetch<CatalogListResponse>(`${base}/catalog`)
      return r.specs
    },

    async getCatalogTypes(alias) {
      const r = await $fetch<CatalogTypesResponse>(
        `${base}/catalog/${encodeURIComponent(alias)}/types`,
      )
      return r.types
    },

    getCatalogType(alias, name) {
      return $fetch<CatalogTypeDetail>(
        `${base}/catalog/${encodeURIComponent(alias)}/types/${encodeURIComponent(name)}`,
      )
    },

    async searchCatalog(q) {
      const r = await $fetch<CatalogSearchResponse>(`${base}/catalog/search`, { query: { q } })
      return r.hits
    },

    getRepo() {
      return $fetch<RepoInfo>(`${base}/repo`)
    },

    async listPRs() {
      const r = await $fetch<{ prs: PullRequest[] }>(`${base}/prs`)
      return r.prs
    },

    listBranches() {
      return $fetch<BranchList>(`${base}/branches`)
    },

    async propose(id, body) {
      try {
        return await $fetch<ProposeResponse>(
          `${base}/drafts/${encodeURIComponent(id)}/propose`,
          { method: 'POST', body },
        )
      } catch (err: unknown) {
        // ofetch raises on non-2xx; map a 409 to a typed, diagnostics-carrying error.
        if (isFetchError(err)) {
          const status = err.status ?? err.response?.status
          if (status === 409) {
            const data = err.data as ProposeConflictBody | undefined
            throw new ProposeConflictError(
              data?.error ?? 'draft has lint errors',
              data?.diagnostics ?? [],
            )
          }
          const body = err.data as { error?: string; detail?: string } | undefined
          if (body?.error) throw new ProposeError(body.error, body.detail)
        }
        throw err
      }
    },
  }
}
