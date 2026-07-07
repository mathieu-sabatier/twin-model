<script setup lang="ts">
// Inspector › YAML of selection. Canonical text comes from the server (raw-file
// endpoint); the SPA never serializes canonical YAML itself (determinism contract).
import { useDraftStore } from '~/stores/draft'
const store = useDraftStore()
// Default wrap ON (L2): long lines (e.g. namespace URLs) soft-wrap rather than
// clipping off the right edge; the toggle switches to alignment-preserving pre.
const wrap = ref(true)
onMounted(() => void store.loadYaml())
watch(() => [store.draftId, store.file, store.model], () => void store.loadYaml())
</script>

<template>
  <div v-if="store.yaml" class="flex flex-col gap-1">
    <div class="flex justify-end">
      <UButton
        size="xs"
        color="neutral"
        variant="ghost"
        :icon="wrap ? 'i-lucide-wrap-text' : 'i-lucide-arrow-right-to-line'"
        :aria-label="wrap ? 'Disable line wrap' : 'Enable line wrap'"
        data-testid="yaml-wrap-toggle"
        @click="() => { wrap = !wrap }"
      />
    </div>
    <pre
      data-pane="yaml"
      :class="['overflow-auto rounded-lg border border-default bg-elevated/20 p-3 font-mono text-xs leading-relaxed text-toned', wrap ? 'whitespace-pre-wrap wrap-break-word' : 'whitespace-pre']"
    >{{ store.yaml }}</pre>
  </div>
  <div v-else class="rounded-lg border border-dashed border-default px-4 py-10 text-center text-xs text-dimmed" data-pane="yaml-empty">
    Select a node to see its canonical YAML.
  </div>
</template>
