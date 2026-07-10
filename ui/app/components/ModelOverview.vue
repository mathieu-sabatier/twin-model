<script setup lang="ts">
// Center-pane overview shown when nothing is selected. Presents the full-model
// UML class diagram (via DiagramPane, which draws the whole model when there is
// no selection) beneath a short instructive caption, so the user can understand
// the current modeling and pick a type or instance from the tree to edit.
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex flex-col items-center gap-1 pt-2 text-center">
      <p class="text-sm font-medium text-toned">Model overview</p>
      <p class="max-w-xs text-xs text-dimmed">
        Pick a type or instance from the tree to view and edit it.
      </p>
    </div>
    <DiagramPane />
  </div>
</template>
