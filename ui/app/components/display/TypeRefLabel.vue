<script setup lang="ts">
// Monospace rendering of a TypeRef. The import alias (e.g. `OpcUa:`) is dimmed so
// the eye lands on the type name; imported refs get a small link glyph to signal
// "this lives in another namespace". A bare "—" is shown when there is no ref
// (e.g. a method member has no type). Read-only display primitive.
import type { TypeRef } from '~/types'

const props = defineProps<{ typeRef?: TypeRef | null }>()
</script>

<template>
  <span v-if="!props.typeRef" class="text-dimmed">—</span>
  <span v-else class="font-mono text-sm inline-flex items-center gap-1 whitespace-nowrap">
    <UIcon
      v-if="props.typeRef.alias"
      name="i-lucide-link"
      class="size-3 text-dimmed shrink-0"
    />
    <span v-if="props.typeRef.alias" class="text-dimmed">{{ props.typeRef.alias }}:</span>
    <span class="text-highlighted">{{ props.typeRef.name }}</span>
  </span>
</template>
