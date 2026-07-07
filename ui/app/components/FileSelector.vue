<script setup lang="ts">
// Active model-file selector. The draft may hold several model files; this picks
// which one the tree/editor/diagram/YAML/propose all operate on. Hidden when the
// draft has one file (nothing to choose).
import { useDraftStore } from '~/stores/draft'

const store = useDraftStore()

function onSelect(f: string): void {
  void store.setFile(f)
}
</script>

<template>
  <USelect
    v-if="store.files.length > 1"
    :model-value="store.file ?? undefined"
    :items="store.files"
    size="xs"
    icon="i-lucide-file-code"
    class="min-w-40 font-mono"
    data-testid="file-selector"
    @update:model-value="onSelect"
  />
</template>
