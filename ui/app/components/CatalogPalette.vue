<script setup lang="ts">
// CatalogPalette — global ⌘K command palette for searching companion types.
// Opens from any context; on pick, sets the shared selection to
// { kind: 'catalogType', alias, name } which drives the center pane to show
// CatalogTypeDetail (Task 9). Read-only: no draft mutation, no catalog mutation
// beyond runSearch.
import { ref, watch } from 'vue'
import { useCatalogStore } from '~/stores/catalog'
import { useSelection } from '~/composables/useSelection'

// v-model:open — follows the same @nuxt/ui UModal convention as CatalogPicker.vue
const open = defineModel<boolean>('open', { default: false })

const catalog = useCatalogStore()
const { select } = useSelection()
const q = ref('')

// On open: run the current query so the list isn't stale from a prior session.
// On close: reset q so the next open starts clean.
watch(open, (isOpen) => {
  if (isOpen) {
    void catalog.runSearch(q.value)
  } else {
    q.value = ''
  }
})

// Trigger a search whenever the query changes.
watch(q, (val) => void catalog.runSearch(val))

function choose(alias: string, name: string): void {
  select({ kind: 'catalogType', alias, name })
  open.value = false
}
</script>

<template>
  <UModal v-model:open="open" title="Find companion type">
    <template #body>
      <div class="flex flex-col gap-3 py-1">
        <UInput
          v-model="q"
          placeholder="Search companion types (⌘K)…"
          icon="i-lucide-search"
          autofocus
          data-testid="catalog-palette-search"
        />
        <ul
          class="max-h-80 divide-y divide-default overflow-y-auto rounded border border-default"
          data-testid="catalog-palette-list"
        >
          <li v-if="catalog.search.length === 0 && q.length > 0" class="px-3 py-2 text-sm text-dimmed">
            No results for "{{ q }}"
          </li>
          <li v-for="h in catalog.search" :key="h.alias + ':' + h.name">
            <button
              type="button"
              class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-elevated"
              :data-testid="`catalog-palette-hit-${h.alias}-${h.name}`"
              @click="() => choose(h.alias, h.name)"
            >
              <UIcon
                :name="h.abstract ? 'i-lucide-square-dashed' : 'i-lucide-box'"
                class="size-4 text-primary"
              />
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
