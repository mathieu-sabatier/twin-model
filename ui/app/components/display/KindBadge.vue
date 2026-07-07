<script setup lang="ts">
// Semantic badge for a member kind. Each of the four OPC UA member kinds gets a
// distinct colour + icon so the members table reads at a glance: what is a
// property vs a variable vs an object vs a method. Read-only display primitive.
import type { MemberKind } from '~/types'

const props = defineProps<{ kind: MemberKind }>()

// One row per kind: colour maps to a Nuxt UI semantic colour, icon to a lucide glyph.
const MAP: Record<MemberKind, { color: 'info' | 'primary' | 'warning' | 'success'; icon: string }> = {
  property: { color: 'info', icon: 'i-lucide-tag' },
  variable: { color: 'primary', icon: 'i-lucide-variable' },
  object: { color: 'warning', icon: 'i-lucide-box' },
  method: { color: 'success', icon: 'i-lucide-square-function' },
}

const spec = computed(() => MAP[props.kind] ?? MAP.variable)
</script>

<template>
  <UBadge
    :color="spec.color"
    variant="subtle"
    size="sm"
    :icon="spec.icon"
    class="font-mono tracking-tight"
  >
    {{ kind }}
  </UBadge>
</template>
