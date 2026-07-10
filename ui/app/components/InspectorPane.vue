<script setup lang="ts">
// Right inspector: the YAML of the current selection. The model diagram now
// lives in the center pane's overview (see ModelOverview / EditorPane), so this
// pane holds a single YAML section.
</script>

<template>
  <div class="flex flex-col p-4">
    <section class="flex flex-col gap-2.5">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-file-code-2" class="size-4 text-muted" />
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted">YAML of selection</h3>
      </div>
      <YamlPane />
    </section>
  </div>
</template>
