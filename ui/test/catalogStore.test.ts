import { describe, expect, it, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useCatalogStore } from '~/stores/catalog'
import type { ApiClient } from '~/api'

function fakeApi(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    getCatalog: async () => [{ alias: 'DI', uri: 'u', version: '1', publicationDate: '', dependencies: [] }],
    getCatalogTypes: async () => [{ name: 'DeviceType', nodeClass: 'ObjectType', abstract: true }],
    getCatalogType: async () => ({ alias: 'DI', uri: 'u', name: 'DeviceType', nodeClass: 'ObjectType', abstract: true, baseChain: [], members: [] }),
    searchCatalog: async () => [{ alias: 'DI', name: 'DeviceType', nodeClass: 'ObjectType', abstract: true }],
    ...overrides,
  } as unknown as ApiClient
}

describe('catalog store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loadSpecs populates specs', async () => {
    const s = useCatalogStore(fakeApi())
    await s.loadSpecs()
    expect(s.specs.map((x) => x.alias)).toContain('DI')
  })

  it('detailFor caches by alias:name (one network call)', async () => {
    let calls = 0
    const s = useCatalogStore(fakeApi({
      getCatalogType: async (a, n) => { calls++; return { alias: a, uri: 'u', name: n, nodeClass: 'ObjectType', abstract: true, baseChain: [], members: [] } },
    }))
    await s.detailFor('DI', 'DeviceType')
    await s.detailFor('DI', 'DeviceType')
    expect(calls).toBe(1)
  })

  it('typesFor caches by alias (one network call)', async () => {
    let calls = 0
    const s = useCatalogStore(fakeApi({
      getCatalogTypes: async () => { calls++; return [{ name: 'DeviceType', nodeClass: 'ObjectType', abstract: true }] },
    }))
    await s.typesFor('DI')
    await s.typesFor('DI')
    expect(calls).toBe(1)
  })

  it('runSearch stores hits', async () => {
    const s = useCatalogStore(fakeApi())
    await s.runSearch('device')
    expect(s.search.length).toBe(1)
  })
})
