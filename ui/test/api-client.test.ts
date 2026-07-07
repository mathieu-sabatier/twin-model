// @vitest-environment node
//
// The typed client is pure ($fetch/ofetch only, no Nuxt runtime), so this runs in
// the plain `node` env where MSW's node server reliably intercepts ofetch. It
// still resolves `~` aliases via the Nuxt Vitest project config.
import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { createApiClient, ProposeConflictError } from '~/api'
import { flagDraftRed, getDraft } from '~/mocks/handlers'
import { server } from './setup'

// The typed client is exercised against the SHARED MSW handlers (the same mock
// dev uses). We drive createApiClient directly with a fixed base so no Nuxt
// runtime config is needed. This is the contract test for the data boundary:
// endpoints hit the right URLs/methods, text endpoints return text, and — the
// load-bearing case — propose maps a 409 to a typed, diagnostics-carrying error.
// Use an ABSOLUTE base so requests hit the network layer MSW's node server
// intercepts. A relative `/api` would be resolved by Nuxt's own test $fetch
// handler (which 404s) instead of reaching MSW. The handlers are registered on
// the `/api` path; MSW matches on the path regardless of origin.
// The shared test/setup.ts resets the in-memory draft store after each test.
const api = createApiClient('http://localhost/api')

describe('api client happy paths (vs MSW)', () => {
  it('getSchema returns the JSON Schema object', async () => {
    const schema = await api.getSchema()
    expect(schema).toMatchObject({ title: 'twinmodel DSL' })
  })

  it('getModel requires a ref and returns a ModelResponse', async () => {
    const res = await api.getModel('main')
    expect(res.file).toBe('equipment.yaml')
    expect(res.model?.name).toBe('AcmeEquipment')
  })

  it('createDraft seeds a draft and returns its id + files', async () => {
    const draft = await api.createDraft('main')
    expect(draft.id).toMatch(/^draft[0-9a-f]{8}$/)
    expect(draft.baseRef).toBe('main')
    expect(draft.files).toEqual(['equipment.yaml'])
    // The store actually holds it.
    expect(getDraft(draft.id)).toBeDefined()
  })

  it('putFiles stores the file and resolves void', async () => {
    const { id } = await api.createDraft('main')
    const res = await api.putFiles(id, { 'equipment.yaml': 'model: X\n' })
    expect(res).toBeUndefined()
    expect(getDraft(id)!.files['equipment.yaml']).toBe('model: X\n')
  })

  it('validate returns diagnostics with a Path-anchored error', async () => {
    const { id } = await api.createDraft('main')
    const res = await api.validate(id)
    expect(res.diagnostics.some((d) => d.path.includes('members'))).toBe(true)
    expect(res.diagnostics[0]!.severity).toMatch(/error|warning/)
  })

  it('previewModelDesign returns XML TEXT (not JSON)', async () => {
    const { id } = await api.createDraft('main')
    const xml = await api.previewModelDesign(id)
    expect(typeof xml).toBe('string')
    expect(xml).toContain('<ModelDesign')
  })

  it('previewDiagram returns Mermaid TEXT (not JSON)', async () => {
    const { id } = await api.createDraft('main')
    const mmd = await api.previewDiagram(id)
    expect(typeof mmd).toBe('string')
    expect(mmd.startsWith('classDiagram')).toBe(true)
  })

  it('getDraftFile returns the raw canonical YAML text', async () => {
    const { id } = await api.createDraft('main')
    const yaml = await api.getDraftFile(id, 'Equipment.yaml')
    expect(typeof yaml).toBe('string')
    expect(yaml).toContain('model')
  })

  it('diff returns a DiffResponse envelope {changes, text}', async () => {
    const { id } = await api.createDraft('main')
    const res = await api.diff(id)
    expect(res).toHaveProperty('changes')
    expect(res).toHaveProperty('text')
    expect(Array.isArray(res.changes)).toBe(true)
    expect(typeof res.text).toBe('string')
    expect(res.changes[0]!.text).toBeTypeOf('string')
    expect(res.changes[0]!.kind).toBeTypeOf('string')
  })

  it('resolved flattens members with declaredIn and URL-encodes the type name', async () => {
    const { id } = await api.createDraft('main')
    const res = await api.resolved(id, 'FurnaceType')
    expect(res.type).toBe('FurnaceType')
    // Inherited members are tagged with their declaring type.
    expect(res.members.some((m) => m.declaredIn === 'EquipmentType')).toBe(true)
    expect(res.members.some((m) => m.declaredIn === 'FurnaceType')).toBe(true)
  })

  it('propose returns the PR url on success', async () => {
    const { id } = await api.createDraft('main')
    const res = await api.propose(id, { branch: 'b', title: 't', message: 'm' })
    expect(res.url).toMatch(/^https:\/\/github\.com\//)
  })

  it('getUnits returns the unit list', async () => {
    const units = await api.getUnits()
    expect(units.length).toBeGreaterThan(0)
    expect(units[0]).toHaveProperty('symbol')
  })

  it('getRepo returns repo context + commit identity', async () => {
    const repo = await api.getRepo()
    expect(repo.owner).toBe('mathieu-sabatier')
    expect(repo.repo).toBe('twin-model')
    expect(repo.commitName).toBe('twinmodel-bot')
    expect(repo.proposeEnabled).toBe(true)
  })

  it('listPRs returns open pull requests with head branches', async () => {
    const prs = await api.listPRs()
    expect(prs).toHaveLength(2)
    expect(prs[0]!.branch).toBe('model/furnace-zones')
  })

  it('listBranches returns branches + resolved default', async () => {
    const list = await api.listBranches()
    expect(list.defaultBranch).toBe('main')
    expect(list.branches).toContain('main')
    expect(list.branches).toContain('model/press-curve')
  })
})

describe('propose non-2xx -> surfaces backend error body', () => {
  it('propose surfaces the backend error body, not the HTTP status', async () => {
    const client = createApiClient('http://localhost/api')
    // Override the propose endpoint for this test to return a 502 with an error body.
    server.use(
      http.post('*/api/drafts/*/propose', () =>
        HttpResponse.json({ error: 'open pr: GitHub API 404: Not Found' }, { status: 502 }),
      ),
    )
    await expect(client.propose('d1', { title: 't', branch: 'b', message: '' }))
      .rejects.toThrow(/GitHub API 404/)
  })
})

describe('propose 409 -> ProposeConflictError', () => {
  it('throws ProposeConflictError carrying diagnostics when the draft is lint-red', async () => {
    const { id } = await api.createDraft('main')
    flagDraftRed(id)

    await expect(
      api.propose(id, { branch: 'b', title: 't', message: 'm' }),
    ).rejects.toBeInstanceOf(ProposeConflictError)

    // And the thrown error carries the server diagnostics for field rendering.
    let caught: unknown
    const { id: id2 } = await api.createDraft('main')
    flagDraftRed(id2)
    try {
      await api.propose(id2, { branch: 'b', title: 't', message: 'm' })
    } catch (err) {
      caught = err
    }
    expect(caught).toBeInstanceOf(ProposeConflictError)
    const conflict = caught as ProposeConflictError
    expect(conflict.diagnostics.length).toBeGreaterThan(0)
    expect(conflict.diagnostics[0]!.code).toBeTypeOf('string')
    expect(conflict.message).toBe('draft has lint errors')
  })
})
