<script setup lang="ts">
// Left navigator. Two roots (Types incl. an Enums group · Instances by topology)
// built by useModelTree from the store, rendered with UTree. Selecting a leaf
// updates the shared selection; group headers just expand/collapse.
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'
import { useModelTree, type ModelTreeItem } from '~/composables/useModelTree'

const store = useDraftStore()
const { selection, select, clear } = useSelection()

const emit = defineEmits<{ navigate: [] }>()

/** Controls the InstanceCreateDialog visibility. */
const showInstanceCreate = ref(false)

// Which nesting the Instances root renders. ISA-95 is the default per spec —
// it reads as the canonical hierarchy for equipment/organizational models.
const view = ref<'default' | 'isa95' | { perspectiveId: string }>('isa95')

// ── Re-parent (ParentPicker) + assign membership (AssignMenu) — B6 ──────────
// Both are menu-driven (no drag-and-drop): a row's "Move to…"/"Assign to…"
// entry stashes the target instance name and opens the matching dialog.
const showParentPicker = ref(false)
const moveTarget = ref<string | null>(null)
const showAssignMenu = ref(false)
const assignTarget = ref<string | null>(null)

/** The perspective id the "Assign to…" action targets, or null outside a perspective view. */
const activePerspectiveId = computed<string | null>(() =>
  typeof view.value === 'object' ? view.value.perspectiveId : null,
)

/** Build the dropdown menu items for an instance row. */
function menuFor(name: string) {
  // No "Rename…" here: in the tree it only selected the row (the actual rename
  // input lives in the detail panel's name pencil), so it was dead weight.
  const primary: { label: string; icon: string; color?: 'error'; onSelect: () => void }[] = [
    {
      label: 'Delete',
      icon: 'i-lucide-trash-2',
      color: 'error' as const,
      onSelect: () => void store.deleteInstance(name),
    },
  ]
  if (view.value === 'isa95') {
    primary.push({
      label: 'Move to…',
      icon: 'i-lucide-move',
      onSelect: () => { moveTarget.value = name; showParentPicker.value = true },
    })
  } else if (activePerspectiveId.value) {
    primary.push({
      label: 'Assign to…',
      icon: 'i-lucide-link',
      onSelect: () => { assignTarget.value = name; showAssignMenu.value = true },
    })
  }
  return [primary]
}

// USelect's `items`/model-value must be Nuxt UI's `AcceptableValue` (string |
// number | boolean | ...) — it cannot be the `{ perspectiveId }` object `view`
// uses internally (confirmed by typecheck: object option values fail
// SelectItem's `value` type). So the selector deals in a flat string key
// (`'isa95'` or `perspective:<id>`) and this computed translates it to/from
// the richer `View` shape `useModelTree` expects.
const viewKey = computed<string>({
  get: () => (typeof view.value === 'object' ? `perspective:${view.value.perspectiveId}` : view.value),
  set: (key) => {
    view.value = key.startsWith('perspective:')
      ? { perspectiveId: key.slice('perspective:'.length) }
      : (key as 'default' | 'isa95')
  },
})

/** USelect options: the two built-in views + one entry per named perspective. */
const viewOptions = computed(() => [
  { label: 'ISA-95', value: 'isa95' },
  ...store.perspectives.map((p) => ({ label: p.label || p.id, value: `perspective:${p.id}` })),
])

const { items } = useModelTree(
  () => store.objectTypes,
  () => store.instances,
  () => store.enums,
  { view: () => view.value, perspectives: () => store.perspectives, diagnosticIndex: () => store.diagnosticIndex },
)

/** Flatten the tree once per change for key→item lookups. */
const flat = computed<ModelTreeItem[]>(() => {
  const out: ModelTreeItem[] = []
  const walk = (nodes: ModelTreeItem[]) => {
    for (const n of nodes) {
      out.push(n)
      if (n.children) walk(n.children as ModelTreeItem[])
    }
  }
  walk(items.value)
  return out
})

/** The node key the current selection maps to (undefined = nothing). */
const selectedKey = computed<string | undefined>(() => {
  const s = selection.value
  if (!s) return undefined
  // Perspective-node selections use the composable's `pnode:` value shape
  // (see perspectiveNode in useModelTree); every other kind is `kind:name`.
  return s.kind === 'perspectiveNode' ? `pnode:${s.perspective}:${s.node}` : `${s.kind}:${s.name}`
})

// Controlled selection: expose the selected item OBJECT as UTree's v-model, and
// translate assignments back into the shared selection. Group headers (no `select`
// payload) are ignored so they only ever toggle expansion.
const selectedItem = computed<ModelTreeItem | undefined>({
  get() {
    return flat.value.find((n) => n.value === selectedKey.value)
  },
  set(item) {
    if (item?.select) { select(item.select); emit('navigate') }
    // Re-clicking a selected row deselects it (Reka sets the model to undefined):
    // mirror that into the shared selection so the detail returns to the empty state.
    else if (!item) clear()
    // A defined item with no `select` = a group header; leave the selection as-is.
  },
})

// ── Reveal-on-switch ─────────────────────────────────────────────────────────
// UTree's `defaultExpanded` per-item flag only seeds its *initial* expanded
// state (reka-ui's TreeRoot reads it once at setup to build the starting
// `expanded` ref) — it is NOT re-applied when the `items` prop later changes
// to a brand-new tree shape (e.g. switching to a perspective view). Confirmed
// empirically: a freshly-appearing `defaultExpanded: true` perspective group
// stayed collapsed after a view switch. So we take over `expanded` as a
// controlled v-model and, whenever `items` is rebuilt, expand any node whose
// key did NOT exist in the previous tree and is flagged `defaultExpanded`.
// Scoping to *newly-appeared* keys (rather than re-unioning every default
// every time) avoids forcibly re-expanding a group the user manually
// collapsed on some unrelated reactive update. Since every instance/
// perspective node with children already carries `defaultExpanded: true`,
// this also reveals the path to the current selection with no separate
// "compute the ancestor chain" step.
function collectKeys(nodes: ModelTreeItem[], onlyDefaultExpanded: boolean): string[] {
  const out: string[] = []
  for (const n of nodes) {
    if (!onlyDefaultExpanded || n.defaultExpanded) out.push(n.value)
    if (n.children) out.push(...collectKeys(n.children as ModelTreeItem[], onlyDefaultExpanded))
  }
  return out
}

const expandedKeys = ref<string[]>(collectKeys(items.value, true))

watch(items, (next, prev) => {
  const prevKeys = new Set(prev ? collectKeys(prev, false) : [])
  const newDefaults = collectKeys(next, true).filter((k) => !prevKeys.has(k))
  if (newDefaults.length) {
    expandedKeys.value = [...new Set([...expandedKeys.value, ...newDefaults])]
  }
})

// ── Decouple label-click (open detail) from expand/collapse ──────────────────
// Reka's TreeItem fires BOTH select and toggle on one row click, so opening an
// instance's detail also flipped its expansion. We veto the *click-originated*
// toggle for selectable nodes (those with a `select` payload = a detail view);
// their disclosure moves to the leading chevron, which calls the slot's
// handleToggle directly (bypassing this guard). Structural group headers (no
// `select`) keep toggle-on-click — they have nothing to open. The guard only
// cancels mouse clicks, so keyboard ←/→ expansion still works (a11y intact).
function onToggleGuard(event: Event, item: ModelTreeItem): void {
  const orig = (event as CustomEvent<{ originalEvent?: Event }>).detail?.originalEvent
  if (orig?.type === 'click' && item.select) event.preventDefault()
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="min-h-0 flex-1 overflow-y-auto px-2 py-2">
      <USelect
        v-model="viewKey"
        data-testid="perspective-selector"
        :items="viewOptions"
        size="sm"
        class="mb-2"
      />
      <UTree
        v-model="selectedItem"
        v-model:expanded="expandedKeys"
        :items="items"
        :get-key="(i: ModelTreeItem) => i.value"
        size="sm"
        color="primary"
        aria-label="Model tree"
        :on-toggle="onToggleGuard"
        :ui="{
          link: 'px-1.5 py-1 gap-1',
          listWithChildren: 'ms-2.5',
          itemWithChildren: 'ps-1 -ms-px',
        }"
      >
        <template #item-leading="{ item, expanded, handleToggle }">
          <!-- Disclosure handle: expanding is the chevron's job only, so a label
               click can open the detail without also toggling the tree (onToggleGuard).
               Leaf rows get a spacer so their icon/label stay aligned. -->
          <button
            v-if="item.children?.length"
            type="button"
            class="flex shrink-0 items-center justify-center size-4 rounded text-dimmed hover:text-default hover:bg-elevated"
            :aria-label="expanded ? 'Collapse' : 'Expand'"
            @click.stop="handleToggle()"
          >
            <UIcon
              :name="expanded ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
              class="size-3.5"
            />
          </button>
          <span v-else class="inline-block shrink-0 size-4" aria-hidden="true" />
          <UIcon
            v-if="item.icon"
            :name="item.icon"
            class="shrink-0 size-4 text-muted"
          />
        </template>
        <template #item-trailing="{ item }">
          <span class="flex items-center gap-1">
            <!-- Validation/attribute marker: circle-alert (error) for nodes with a
                 diagnostic, asterisk for abstract types. -->
            <UIcon
              v-if="item.trailingIcon"
              :name="item.trailingIcon"
              :data-testid="`node-badge-${item.value}`"
              :class="item.trailingIcon === 'i-lucide-circle-alert' ? 'size-4 text-error' : 'size-3.5 text-muted'"
            />
            <UDropdownMenu
              v-if="item.select?.kind === 'instance'"
              :items="menuFor(item.select.name)"
            >
              <UButton
                size="xs"
                color="neutral"
                variant="ghost"
                icon="i-lucide-ellipsis"
                :aria-label="`Actions for ${item.select.name}`"
                :data-testid="`instance-menu-${item.select.name}`"
                @click.stop
              />
            </UDropdownMenu>
          </span>
        </template>
      </UTree>
    </div>
    <!-- Instances actions strip -->
    <USeparator />
    <div class="flex items-center gap-1.5 px-3 py-2">
      <UIcon name="i-lucide-network" class="size-3.5 text-muted" />
      <span class="flex-1 text-xs text-muted">Instances</span>
      <UButton
        size="xs"
        color="neutral"
        variant="ghost"
        icon="i-lucide-plus"
        :disabled="store.frozen"
        data-testid="add-instance-button"
        @click="() => { showInstanceCreate = true }"
      >
        Add instance…
      </UButton>
    </div>
    <InstanceCreateDialog v-model:open="showInstanceCreate" />
    <ParentPicker v-model:open="showParentPicker" :target="moveTarget" />
    <AssignMenu v-model:open="showAssignMenu" :perspective="activePerspectiveId" :target="assignTarget" />
  </div>
</template>
