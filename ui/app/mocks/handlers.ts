// MSW request handlers reproducing the frozen Go API contract (API-CONTRACT.md).
// Shared by the dev browser worker (browser.ts) and the Vitest node server
// (test/setup.ts). Backed by an in-memory draft store so the edit flow is
// realistic: POST /drafts seeds a fileset, PUT stores files, reads reflect state.
//
// MSW is dev/test ONLY; these handlers never run in the SSG production bundle.
import { http, HttpResponse } from 'msw'
import {
  branchListSample,
  diagnostics,
  diffSample,
  diagramMermaid,
  equipmentModel,
  equipmentYaml,
  mockPerspectives,
  modelDesignXml,
  proposeConflict,
  proposeUrl,
  prsSample,
  repoInfoSample,
  resolvedFurnace,
  schemaJson,
  seedFile,
  unitsSample,
  validateWithDiagnostics,
} from './fixtures'
import type { ModelResponse } from '~/types'

/** One stored draft. `lintRed` forces propose to return 409 (for tests). */
interface Draft {
  baseRef: string
  files: Record<string, string>
  lintRed: boolean
}

/** In-memory draft store. Keyed by opaque draft id. */
const drafts = new Map<string, Draft>()

// Deterministic-ish ids so tests can assert on them. Real server uses crypto/rand
// hex; a counter over a fixed prefix is enough for the mock.
let draftSeq = 0
function nextDraftId(): string {
  draftSeq += 1
  return `draft${draftSeq.toString(16).padStart(8, '0')}`
}

/** The YAML seeded into every new draft (placeholder; server owns real content). */
const seedYaml = '# equipment model (seeded by mock)\nmodel: AcmeEquipment\n'

/** Reset all mock state. Call between tests to keep them independent. */
export function resetStore(): void {
  drafts.clear()
  draftSeq = 0
}

/** Force a draft into the lint-red state so propose returns 409. For tests. */
export function flagDraftRed(id: string): void {
  const draft = drafts.get(id)
  if (draft) draft.lintRed = true
}

/** Read-only view of a stored draft, for test assertions. */
export function getDraft(id: string): Draft | undefined {
  return drafts.get(id)
}

/** Build a ModelResponse for a stored draft. Returns the fixture AST + diagnostics. */
function draftModelResponse(file: string): ModelResponse {
  return { file, model: { ...equipmentModel, perspectives: mockPerspectives }, diagnostics: [] }
}

// Wildcard origin so one handler set matches BOTH the dev browser worker (where
// requests are same-origin: `/api/...`) AND the Vitest node server (where the
// client uses an absolute base like `http://localhost/api/...` so MSW's node
// interceptor sees the request). `*/api` matches any origin, path `/api`.
const base = '*/api'

export const handlers = [
  // GET /api/schema -> the real DSL JSON Schema.
  http.get(`${base}/schema`, () => HttpResponse.json(schemaJson)),

  // GET /api/model?ref=&file= -> ModelResponse from the fixture.
  http.get(`${base}/model`, ({ request }) => {
    const url = new URL(request.url)
    const ref = url.searchParams.get('ref')
    if (!ref) {
      return HttpResponse.json({ error: 'missing ref' }, { status: 400 })
    }
    const file = url.searchParams.get('file') ?? seedFile
    return HttpResponse.json(draftModelResponse(file))
  }),

  // POST /api/drafts -> create + seed a draft.
  http.post(`${base}/drafts`, async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as { baseRef?: string }
    const baseRef = body?.baseRef
    if (!baseRef) {
      return HttpResponse.json({ error: 'missing baseRef' }, { status: 400 })
    }
    const id = nextDraftId()
    drafts.set(id, { baseRef, files: { [seedFile]: seedYaml }, lintRed: false })
    return HttpResponse.json({ id, baseRef, files: [seedFile] }, { status: 201 })
  }),

  // GET /api/drafts/:id/model?file= -> ModelResponse reflecting the stored file.
  // Carries the draft's file list so a recovered draft can rebuild the switcher.
  http.get(`${base}/drafts/:id/model`, ({ params, request }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    const url = new URL(request.url)
    const file = url.searchParams.get('file') ?? seedFile
    if (!(file in draft.files)) {
      return HttpResponse.json({ error: 'file not found' }, { status: 404 })
    }
    return HttpResponse.json({ ...draftModelResponse(file), files: Object.keys(draft.files) })
  }),

  // PUT /api/drafts/:id/files -> store files, return 200 {} (mock decision #1).
  http.put(`${base}/drafts/:id/files`, async ({ params, request }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    const body = (await request.json().catch(() => ({}))) as { files?: Record<string, string> }
    if (body?.files) {
      draft.files = { ...draft.files, ...body.files }
    }
    return HttpResponse.json({})
  }),

  // POST /api/drafts/:id/validate?file= -> ValidateResponse (diagnostics fixture).
  http.post(`${base}/drafts/:id/validate`, ({ params, request }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    const url = new URL(request.url)
    const file = url.searchParams.get('file') ?? seedFile
    return HttpResponse.json({ ...validateWithDiagnostics, file })
  }),

  // GET /api/drafts/:id/file?file= -> raw canonical YAML text.
  http.get(`${base}/drafts/:id/file`, ({ params }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    return HttpResponse.text(equipmentYaml)
  }),

  // GET /api/drafts/:id/preview/modeldesign?file= -> ModelDesign XML text.
  http.get(`${base}/drafts/:id/preview/modeldesign`, ({ params }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    return new HttpResponse(modelDesignXml, {
      headers: { 'Content-Type': 'application/xml' },
    })
  }),

  // GET /api/drafts/:id/preview/diagram?file= -> Mermaid source text.
  http.get(`${base}/drafts/:id/preview/diagram`, ({ params }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    return new HttpResponse(diagramMermaid, {
      headers: { 'Content-Type': 'text/plain' },
    })
  }),

  // GET /api/drafts/:id/diff?file= -> bare Change[] (mock decision #2).
  http.get(`${base}/drafts/:id/diff`, ({ params }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    return HttpResponse.json(diffSample)
  }),

  // GET /api/drafts/:id/types/:name/resolved?file= -> ResolvedResponse.
  http.get(`${base}/drafts/:id/types/:name/resolved`, ({ params }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    // The mock only knows FurnaceType's resolved form; echo the requested name.
    return HttpResponse.json({ ...resolvedFurnace, type: String(params.name) })
  }),

  // GET /api/units -> { units: UnitInfo[] }
  http.get(`${base}/units`, () => HttpResponse.json({ units: unitsSample })),

  // GET /api/repo -> repo context, commit identity, propose availability.
  http.get(`${base}/repo`, () => HttpResponse.json(repoInfoSample)),

  // GET /api/prs -> open pull requests (their head branches seed the branch picker).
  http.get(`${base}/prs`, () => HttpResponse.json({ prs: prsSample })),

  // GET /api/branches -> the repo's branches (default first) + resolved default.
  http.get(`${base}/branches`, () => HttpResponse.json(branchListSample)),

  // Catalog discovery (global, draft-independent). `search` is registered before
  // the `:alias/types` param route so the literal wins (MSW matches in order).
  http.get(`${base}/catalog`, () =>
    HttpResponse.json({
      specs: [
        { alias: 'DI', uri: 'http://opcfoundation.org/UA/DI/', version: '1.04', publicationDate: '2022-11-01', dependencies: [] },
        { alias: 'Machinery', uri: 'http://opcfoundation.org/UA/Machinery/', version: '1.03', publicationDate: '2023-05-01', dependencies: ['DI'] },
      ],
    }),
  ),
  http.get(`${base}/catalog/search`, ({ request }) => {
    const q = (new URL(request.url).searchParams.get('q') ?? '').toLowerCase()
    const all = [{ alias: 'DI', name: 'DeviceType', nodeClass: 'ObjectType', abstract: true }]
    return HttpResponse.json({ hits: all.filter((h) => h.name.toLowerCase().includes(q)) })
  }),
  http.get(`${base}/catalog/:alias/types`, () =>
    HttpResponse.json({ types: [{ name: 'DeviceType', nodeClass: 'ObjectType', abstract: true }] }),
  ),
  http.get(`${base}/catalog/:alias/types/:name`, ({ params }) =>
    HttpResponse.json({
      alias: params.alias, uri: 'http://opcfoundation.org/UA/DI/', name: params.name,
      nodeClass: 'ObjectType', abstract: true,
      baseChain: [{ alias: '', name: 'BaseObjectType', uri: 'http://opcfoundation.org/UA/' }],
      members: [{ name: 'Manufacturer', kind: 'property', placeholder: false }],
    }),
  ),

  // POST /api/drafts/:id/propose -> {url}, or 409 {error, diagnostics} if lint-red.
  http.post(`${base}/drafts/:id/propose`, ({ params }) => {
    const draft = drafts.get(String(params.id))
    if (!draft) return HttpResponse.json({ error: 'draft not found' }, { status: 404 })
    if (draft.lintRed) {
      return HttpResponse.json(proposeConflict, { status: 409 })
    }
    return HttpResponse.json({ url: proposeUrl })
  }),
]

// Re-export so a test can build ad-hoc diagnostics-anchored assertions.
export { diagnostics }
