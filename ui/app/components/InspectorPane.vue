<script setup lang="ts">
// Right inspector: two stacked sections with clear headers — Diagram and YAML.
// Both are stubs in T4 (see DiagramPane / YamlPane); this container owns the
// section chrome and headers so the swap-in components stay focused.
</script>

<template>
  <div class="flex flex-col gap-6 p-4">
    <section class="flex flex-col gap-2.5">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-waypoints" class="size-4 text-muted" />
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted">Diagram</h3>
      </div>
      <DiagramPane />
    </section>

    <section class="flex flex-col gap-2.5">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-file-code-2" class="size-4 text-muted" />
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted">YAML of selection</h3>
      </div>
      <YamlPane />
    </section>
  </div>
</template>
