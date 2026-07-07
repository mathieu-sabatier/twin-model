<script setup lang="ts">
// Left-pane catalog browser: bundled specs → their types. Read-only. Selecting
// a type sets the shared selection to { kind: 'catalogType', alias, name }, which
// CatalogTypeDetail renders in the center pane. Mirrors TreePane's UTree usage:
// controlled v-model with a selectedItem computed (get/set), :get-key, size/color.
import { computed, onMounted, ref } from 'vue'
import { useCatalogStore } from '~/stores/catalog'
import { useSelection } from '~/composables/useSelection'
import type { CatalogTypeSummary } from '~/types'

const catalog = useCatalogStore()
const { selection, select } = useSelection()

// Lazily-loaded per-spec type lists (populated on expand).
const typesByAlias = ref<Record<string, CatalogTypeSummary[]>>({})

onMounted(() => void catalog.loadSpecs())

async function expandSpec(alias: string): Promise<void> {
  if (typesByAlias.value[alias]) return
  typesByAlias.value = { ...typesByAlias.value, [alias]: await catalog.typesFor(alias) }
}

interface CatalogTreeItem {
  value: string
  label: string
  icon?: string
  /** The catalogType selection to set when this leaf is activated. Absent on group nodes. */
  select?: { kind: 'catalogType'; alias: string; name: string }
  children?: CatalogTreeItem[]
}

const items = computed<CatalogTreeItem[]>(() =>
  catalog.specs.map((s) => ({
    value: `spec:${s.alias}`,
    label: `${s.alias}  (${s.version})`,
    icon: 'i-lucide-package',
    children: (typesByAlias.value[s.alias] ?? []).map((t) => ({
      value: `catalogType:${s.alias}:${t.name}`,
      label: t.name,
      icon: t.abstract ? 'i-lucide-square-dashed' : 'i-lucide-box',
      select: { kind: 'catalogType' as const, alias: s.alias, name: t.name },
    })),
  })),
)

/** Flatten tree for key→item lookup (mirrors TreePane). */
const flat = computed<CatalogTreeItem[]>(() => {
  const out: CatalogTreeItem[] = []
  const walk = (nodes: CatalogTreeItem[]) => {
    for (const n of nodes) {
      out.push(n)
      if (n.children) walk(n.children)
    }
  }
  walk(items.value)
  return out
})

/** The key the current catalogType selection maps to (undefined = nothing selected here). */
const selectedKey = computed<string | undefined>(() => {
  const s = selection.value
  return s?.kind === 'catalogType' ? `catalogType:${s.alias}:${s.name}` : undefined
})

// Controlled selection: expose the selected item OBJECT as UTree's v-model,
// translating assignments back into the shared selection. Mirrors TreePane.
const selectedItem = computed<CatalogTreeItem | undefined>({
  get() {
    return flat.value.find((n) => n.value === selectedKey.value)
  },
  set(item) {
    if (item?.select) select(item.select)
  },
})

// UTree emits update:expanded when the user toggles a branch (expand chevron or
// keyboard). The payload is string[] — the full set of currently-expanded node
// keys as returned by reka-ui TreeRoot (TreeRootEmits: 'update:expanded': [val: string[]]).
// We use this to lazily load a spec's types the first time its branch is opened.
// expandSpec is idempotent so re-calling on already-loaded specs is safe.
function onExpandedChange(keys: string[]): void {
  for (const key of keys) {
    if (key.startsWith('spec:')) {
      void expandSpec(key.slice(5))
    }
  }
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="min-h-0 flex-1 overflow-y-auto px-2 py-2">
      <UTree
        v-model="selectedItem"
        :items="items"
        :get-key="(i: CatalogTreeItem) => i.value"
        size="sm"
        color="primary"
        aria-label="Companion-spec catalog"
        @update:expanded="onExpandedChange"
      />
    </div>
  </div>
</template>
