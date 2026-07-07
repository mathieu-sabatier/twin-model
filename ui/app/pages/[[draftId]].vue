<script setup lang="ts">
// The single editor page. Owns draft bootstrap + route recovery (determinism
// contract): the draft id lives in the URL, never localStorage, so a refresh
// re-fetches the server-side draft by id.
//
//   • route has a draftId  → store.loadDraft(id)      (recover after refresh)
//   • route has none       → store.createDraft('main'),
//                            then replace the URL with the returned id
//
// SSG-compatible: ssr:false means this runs client-side on mount. We guard the
// data fetch with import.meta.client so `nuxi generate`'s prerender pass does not
// hit the API (there is no server at build time).
import { useDraftStore } from '~/stores/draft'

const route = useRoute()
const router = useRouter()
const store = useDraftStore()

const routeDraftId = computed(() => {
  const p = route.params.draftId
  return (Array.isArray(p) ? p[0] : p) || null
})

async function bootstrap(): Promise<void> {
  if (store.loading) return
  const id = routeDraftId.value
  if (id) {
    // Recover an existing draft; if the id is stale (server restart) mint a fresh
    // one and repoint the URL so a subsequent refresh recovers the new draft.
    const activeId = await store.recoverOrCreate(id)
    if (activeId !== id) await router.replace({ params: { draftId: activeId } })
  } else {
    // Fresh session: create a draft and pin its id into the URL so a refresh recovers it.
    const newId = await store.createDraft('main')
    await router.replace({ params: { draftId: newId } })
  }
}

onMounted(() => {
  // Client-only: never fetch during the generate prerender.
  if (import.meta.client) {
    bootstrap().catch((err) => {
      // Store-action failures are already captured in store.error and shown by
      // AppShell; log anything else (e.g. a router navigation failure) so it
      // doesn't disappear silently.
      if (!store.error) console.error('[bootstrap] unhandled rejection', err)
    })
  }
})
</script>

<template>
  <AppShell />
</template>
