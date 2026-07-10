<script setup lang="ts">
// Center editor pane. Resolves the current selection against the store's model and
// renders the matching read-only view (type / instance / enum), or an empty/
// loading/parse-error state. The store is the single source of model truth.
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'

const store = useDraftStore()
const { selection } = useSelection()

const selectedType = computed(() => {
  const sel = selection.value
  return sel?.kind === 'type' ? store.objectTypes.find((t) => t.name === sel.name) ?? null : null
})
const selectedInstance = computed(() => {
  const sel = selection.value
  return sel?.kind === 'instance' ? store.instances.find((i) => i.name === sel.name) ?? null : null
})
const selectedEnum = computed(() => {
  const sel = selection.value
  return sel?.kind === 'enum' ? store.enums.find((e) => e.name === sel.name) ?? null : null
})

const hasSelection = computed(
  () => !!(
    (selection.value?.kind === 'catalogType') ||
    selectedType.value ||
    selectedInstance.value ||
    selectedEnum.value
  ),
)
</script>

<template>
  <div class="mx-auto w-full max-w-4xl p-6">
    <!-- Loading skeleton before the first model resolves -->
    <div v-if="store.loading && !store.model" class="flex flex-col gap-4">
      <USkeleton class="h-8 w-64" />
      <USkeleton class="h-4 w-96" />
      <USkeleton class="h-40 w-full" />
    </div>

    <!-- Parse error surfaced from the server -->
    <UAlert
      v-else-if="store.parseError"
      color="error"
      variant="subtle"
      icon="i-lucide-circle-alert"
      title="Model could not be parsed"
      :description="store.parseError"
    />

    <CatalogTypeDetail
      v-else-if="selection && selection.kind === 'catalogType'"
      :alias="selection.alias"
      :name="selection.name"
    />

    <TypeView
      v-else-if="selectedType"
      :type="selectedType"
      :diagnostic-index="store.diagnosticIndex"
    />
    <InstanceView
      v-else-if="selectedInstance"
      :instance="selectedInstance"
      :diagnostic-index="store.diagnosticIndex"
    />
    <EnumView v-else-if="selectedEnum" :def="selectedEnum" />

    <ModelOverview v-else-if="!hasSelection" />
  </div>
</template>
