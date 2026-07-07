<script setup lang="ts">
// Inspector › Diagram. Renders the Mermaid source from the store using the
// BUNDLED mermaid package (no CDN — strict CSP). The server exposes the source at
// GET /api/drafts/{id}/preview/diagram; this component fetches it via the store
// and renders it client-side.
import mermaid from 'mermaid'
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'
import { focusDiagram } from '~/lib/focusDiagram'

mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'neutral' })

const store = useDraftStore()
const { selection } = useSelection()
const showTypeHint = computed(
  () => selection.value?.kind === 'enum' || selection.value?.kind === 'instance',
)
const selectedTypeName = computed(() =>
  selection.value?.kind === 'type' ? selection.value.name : null,
)
const rendering = ref(false)

const svgHtml = ref<string | null>(null)
const rawSvg = ref<string | null>(null)
const renderError = ref(false)

let renderSeq = 0

async function renderDiagram() {
  if (!store.diagramSrc) {
    svgHtml.value = null
    rawSvg.value = null
    renderError.value = false
    return
  }
  const id = `uad-${++renderSeq}`
  rendering.value = true
  await nextTick()
  try {
    const { svg } = await mermaid.render(id, store.diagramSrc)
    rawSvg.value = svg
    svgHtml.value = focusDiagram(svg, selectedTypeName.value)
    renderError.value = false
  } catch {
    renderError.value = true
  } finally {
    rendering.value = false
  }
}

onMounted(() => {
  void store.loadDiagram()
})

watch(() => [store.draftId, store.file, store.model], () => {
  void store.loadDiagram()
})

watch(() => store.diagramSrc, () => {
  void renderDiagram()
})

watch(selectedTypeName, () => {
  // Re-apply focus in-memory; the source SVG is unchanged, so don't re-run mermaid.
  if (rawSvg.value !== null) svgHtml.value = focusDiagram(rawSvg.value, selectedTypeName.value)
})
</script>

<template>
  <div>
    <div
      v-if="rendering && !svgHtml"
      data-pane="diagram-loading"
      class="flex items-center justify-center gap-2 rounded-lg border border-default bg-elevated/20 px-4 py-10 text-sm text-muted"
    >
      <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
      Rendering diagram…
    </div>
    <p
      v-if="showTypeHint"
      data-pane="diagram-note"
      class="mb-2 text-xs text-muted"
    >
      This class diagram shows the model's type hierarchy. Select a type to focus it.
    </p>
    <div
      v-if="store.diagramSrc && !renderError"
      data-pane="diagram"
      class="min-h-[240px] overflow-auto rounded-lg border border-default bg-elevated/20 p-3"
      v-html="svgHtml"
    />
    <pre
      v-else-if="store.diagramSrc && renderError"
      data-pane="diagram-raw"
      class="overflow-auto rounded-lg border border-default bg-elevated/20 p-3 font-mono text-xs leading-relaxed text-toned"
    >{{ store.diagramSrc }}</pre>
    <div
      v-else
      data-pane="diagram-empty"
      class="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-default bg-elevated/20 px-4 py-10 text-center"
    >
      <p class="text-sm font-medium text-dimmed">Select a node to see its diagram.</p>
    </div>
  </div>
</template>
