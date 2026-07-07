<script setup lang="ts">
// A labelled metadata row for the type/instance detail headers: a fixed-width
// muted label and a value slot. Keeps the header grids aligned and consistent.
defineProps<{ label: string; icon?: string }>()
</script>

<template>
  <div class="flex items-baseline gap-3 py-1">
    <span class="flex w-28 shrink-0 items-center gap-1.5 text-xs font-medium uppercase tracking-wide text-muted">
      <UIcon v-if="icon" :name="icon" class="size-3.5" />
      {{ label }}
    </span>
    <div class="min-w-0 text-sm text-toned">
      <slot />
    </div>
  </div>
</template>
