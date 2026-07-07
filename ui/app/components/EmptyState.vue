<script setup lang="ts">
// Center-pane empty state shown when nothing is selected. Quietly instructive.
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center gap-3 py-24 text-center">
    <div class="flex size-14 items-center justify-center rounded-2xl border border-dashed border-default bg-elevated/30">
      <UIcon name="i-lucide-mouse-pointer-click" class="size-7 text-dimmed" />
    </div>
    <p class="text-sm font-medium text-toned">Select a type or instance</p>
    <p class="max-w-xs text-xs text-dimmed">
      Choose a type or instance to view and edit its definition.
    </p>
  </div>
</template>
