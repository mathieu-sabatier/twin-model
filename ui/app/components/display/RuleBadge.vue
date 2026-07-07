<script setup lang="ts">
// Compact badge for a member's modelling rule. Mandatory reads as the strong
// default (neutral/solid-ish), optional as muted, and the two *_placeholder rules
// get an asterisk affordance since they denote a template slot (e.g. Zone<No>).
// Read-only display primitive.
import type { MemberRule } from '~/types'

const props = defineProps<{ rule: MemberRule }>()

const MAP: Record<MemberRule, { color: 'neutral' | 'error'; variant: 'subtle' | 'soft' | 'outline'; icon?: string; label: string }> = {
  mandatory: { color: 'neutral', variant: 'subtle', label: 'mandatory' },
  optional: { color: 'neutral', variant: 'soft', label: 'optional' },
  optional_placeholder: { color: 'neutral', variant: 'outline', icon: 'i-lucide-asterisk', label: 'opt · placeholder' },
  mandatory_placeholder: { color: 'neutral', variant: 'outline', icon: 'i-lucide-asterisk', label: 'req · placeholder' },
}

const spec = computed(() => MAP[props.rule] ?? MAP.mandatory)
</script>

<template>
  <UBadge
    :color="spec.color"
    :variant="spec.variant"
    size="sm"
    :icon="spec.icon"
    class="font-mono tracking-tight"
  >
    {{ spec.label }}
  </UBadge>
</template>
