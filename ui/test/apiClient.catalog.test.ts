// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { createApiClient } from '~/api'
import './setup' // starts the shared MSW node server (module-scope beforeAll/afterEach/afterAll)

const api = createApiClient('http://localhost/api')

describe('api client — catalog (vs MSW)', () => {
  it('getCatalog unwraps {specs}', async () => {
    const specs = await api.getCatalog()
    expect(specs.map((s) => s.alias)).toContain('DI')
  })

  it('getCatalogTypes unwraps {types}', async () => {
    const types = await api.getCatalogTypes('DI')
    expect(types[0]!.name).toBe('DeviceType')
  })

  it('getCatalogType returns the detail with base chain + members', async () => {
    const d = await api.getCatalogType('DI', 'DeviceType')
    expect(d.alias).toBe('DI') // handler echoes params.alias — triangulates the full URL
    expect(d.name).toBe('DeviceType')
    expect(d.baseChain.length).toBeGreaterThan(0)
    expect(d.members[0]!.name).toBe('Manufacturer')
  })

  it('searchCatalog unwraps {hits} and filters by q', async () => {
    const hits = await api.searchCatalog('device')
    expect(hits[0]!.name).toBe('DeviceType')
    expect(await api.searchCatalog('zzz')).toEqual([])
  })
})
