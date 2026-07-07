<script setup lang="ts">
// CatalogPicker — modal for choosing a companion type to use as a member's type.
// Searches the global catalog; on pick, delegates to draftStore.setMemberType
// (import added automatically by the store).
//
// Usage:
//   <CatalogPicker v-model:open="showPicker" :type-name="typeName" :member="memberName" />
import { ref, watch } from 'vue'
import { useCatalogStore } from '~/stores/catalog'
import { useDraftStore } from '~/stores/draft'

const props = defineProps<{ typeName: string; member: string }>()

// v-model:open — follows the same @nuxt/ui UModal convention as CatalogExtendDialog.vue
const open = defineModel<boolean>('open', { default: false })

const catalog = useCatalogStore()
const draft = useDraftStore()
const q = ref('')
const busy = ref(false)
const err = ref<string | null>(null)

// On open: load specs (idempotent) for the alias→uri mapping AND run the current
// query so the list isn't stale from a prior session (q is '' on open → empties
// it). On close (X / escape / overlay / Cancel / successful pick): reset state so
// the next open starts clean.
watch(open, (isOpen) => {
  if (isOpen) {
    void catalog.loadSpecs()
    void catalog.runSearch(q.value)
  } else {
    q.value = ''
    err.value = null
  }
})

// Trigger a search whenever the query changes.
watch(q, (val) => void catalog.runSearch(val))

/** Map an alias back to its namespace URI via the loaded spec list. */
function uriFor(alias: string): string {
  return catalog.specs.find((s) => s.alias === alias)?.uri ?? ''
}

async function pick(alias: string, name: string): Promise<void> {
  busy.value = true
  err.value = null
  try {
    // Specs carry the alias→uri map. loadSpecs is kicked off on open but is
    // fire-and-forget there; await it here (idempotent) so a quick pick can't add
    // an import with an empty URI (`MachineTool: ""`). QA finding.
    await catalog.loadSpecs()
    const refUri = uriFor(alias)
    // Close BEFORE the mutating save. setMemberType refetches the model, which
    // re-renders this picker's host (MembersTable) mid-close and leaves the modal
    // stuck open if we close afterwards. The pick still validates server-side on
    // reload, matching the other authoring flows. QA finding.
    open.value = false // the watch(open) resets q + err on close
    await draft.setMemberType({
      typeName: props.typeName,
      member: props.member,
      refAlias: alias,
      refName: name,
      refUri,
    })
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : String(e)
    open.value = true // reopen so the error is visible if the save failed
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UModal v-model:open="open" title="Use companion type as member type">
    <template #body>
      <div class="flex flex-col gap-3 py-1">
        <UInput
          v-model="q"
          placeholder="Search companion types…"
          icon="i-lucide-search"
          autofocus
          data-testid="catalog-picker-search"
        />
        <p v-if="err" class="text-sm text-error" data-testid="catalog-picker-error">{{ err }}</p>
        <ul
          class="max-h-72 divide-y divide-default overflow-y-auto rounded border border-default"
          data-testid="catalog-picker-list"
        >
          <li v-if="catalog.search.length === 0 && q.length > 0" class="px-3 py-2 text-sm text-dimmed">
            No results for "{{ q }}"
          </li>
          <li v-for="h in catalog.search" :key="h.alias + ':' + h.name">
            <button
              type="button"
              :disabled="busy"
              class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-elevated disabled:opacity-50"
              :data-testid="`catalog-picker-hit-${h.alias}-${h.name}`"
              @click="() => pick(h.alias, h.name)"
            >
              <span class="font-mono font-medium text-highlighted">{{ h.alias }}:{{ h.name }}</span>
              <UBadge size="sm" color="neutral" variant="subtle">{{ h.nodeClass }}</UBadge>
              <UBadge v-if="h.abstract" size="sm" color="neutral" variant="outline">abstract</UBadge>
            </button>
          </li>
        </ul>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end">
        <UButton color="neutral" variant="ghost" @click="() => { open = false }">Cancel</UButton>
      </div>
    </template>
  </UModal>
</template>
