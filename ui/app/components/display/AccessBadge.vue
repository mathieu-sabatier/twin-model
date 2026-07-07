<script setup lang="ts">
// Compact read/read-write access badge. `rw` gets a pencil (writable), `r` an eye
// (read-only). Only meaningful for property/variable members; callers pass a
// nullish access for object/method members and we render a dim dash. Read-only
// display primitive.
import type { MemberAccess } from '~/types'

const props = defineProps<{ access?: MemberAccess | null }>()

const spec = computed(() => {
  if (props.access === 'rw') return { icon: 'i-lucide-pencil', label: 'rw' }
  if (props.access === 'r') return { icon: 'i-lucide-eye', label: 'r' }
  return null
})
</script>

<template>
  <span v-if="!spec" class="text-dimmed">—</span>
  <UBadge v-else color="neutral" variant="soft" size="sm" :icon="spec.icon" class="font-mono">
    {{ spec.label }}
  </UBadge>
</template>
