// Read-only Pinia store for companion-spec discovery. Independent of any draft:
// it browses ALL bundled specs (global catalog endpoints). Mutations live in the
// draft store, never here. Mirrors the draft store's injectable-client pattern
// for testability (pass a fake ApiClient in tests; useApi() in the app).
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi, type ApiClient } from '~/api'
import type { CatalogSpec, CatalogTypeSummary, CatalogTypeDetail, CatalogSearchHit } from '~/types'

export const useCatalogStore = (injectedClient?: ApiClient) => _defineAndUse(injectedClient)

function _defineAndUse(injectedClient?: ApiClient) {
  const store = defineStore('catalog', () => {
    const api: ApiClient = injectedClient ?? useApi()

    const specs = ref<CatalogSpec[]>([])
    const search = ref<CatalogSearchHit[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)

    // Caches keyed by alias / "alias:name" so re-selecting a type is instant.
    const typesCache = ref<Record<string, CatalogTypeSummary[]>>({})
    const detailCache = ref<Record<string, CatalogTypeDetail>>({})

    async function loadSpecs(): Promise<void> {
      if (specs.value.length) return
      loading.value = true
      error.value = null
      try { specs.value = await api.getCatalog() }
      catch (err: unknown) { error.value = err instanceof Error ? err.message : String(err) }
      finally { loading.value = false }
    }

    async function typesFor(alias: string): Promise<CatalogTypeSummary[]> {
      const cached = typesCache.value[alias]
      if (cached) return cached
      const t = await api.getCatalogTypes(alias)
      typesCache.value = { ...typesCache.value, [alias]: t }
      return t
    }

    async function detailFor(alias: string, name: string): Promise<CatalogTypeDetail> {
      const key = `${alias}:${name}`
      const cached = detailCache.value[key]
      if (cached) return cached
      const d = await api.getCatalogType(alias, name)
      detailCache.value = { ...detailCache.value, [key]: d }
      return d
    }

    async function runSearch(q: string): Promise<void> {
      const query = q.trim()
      if (!query) { search.value = []; return }
      error.value = null // clear a prior failure before the new attempt (mirrors loadSpecs)
      try { search.value = await api.searchCatalog(query) }
      catch (err: unknown) { error.value = err instanceof Error ? err.message : String(err) }
    }

    return { specs, search, loading, error, loadSpecs, typesFor, detailFor, runSearch }
  })
  return store()
}
