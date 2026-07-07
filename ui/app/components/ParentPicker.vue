<script setup lang="ts">
// Re-parent dialog opened from TreePane's "Move to…" row action (ISA-95 view).
// Lists candidate parents (the well-known root + all other instances) and
// calls store.reparentInstance(target, chosen) — a one-line `under` diff.
// Follows the same v-model:open / UModal shape as InstanceCreateDialog.vue.
import { useDraftStore } from '~/stores/draft'

const store = useDraftStore()

const open = defineModel<boolean>('open', { default: false })

/** The instance being re-parented (set by TreePane when the menu item fires). */
const props = defineProps<{ target: string | null }>()

const selectedUnder = ref<string>('OpcUa:ObjectsFolder')
const submitError = ref<string | null>(null)
const submitting = ref(false)

/** Candidate parents: the canonical root + every instance other than the target itself. */
const underOptions = computed<string[]>(() => [
  'OpcUa:ObjectsFolder',
  ...store.instances.filter((i) => i.name !== props.target).map((i) => i.name),
])

const submitDisabled = computed(() => submitting.value || !props.target || selectedUnder.value === '')

async function onSubmit(): Promise<void> {
  if (submitDisabled.value || !props.target) return
  submitError.value = null
  submitting.value = true
  try {
    await store.reparentInstance(props.target, selectedUnder.value)
    open.value = false
    reset()
  } catch (err: unknown) {
    submitError.value = err instanceof Error ? err.message : String(err)
  } finally {
    submitting.value = false
  }
}

function onCancel(): void {
  open.value = false
  reset()
}

function reset(): void {
  selectedUnder.value = 'OpcUa:ObjectsFolder'
  submitError.value = null
}
</script>

<template>
  <UModal v-model:open="open" :title="target ? `Move ${target}` : 'Move instance'">
    <template #body>
      <div class="flex flex-col gap-4 py-2">
        <UFormField label="New parent (under)">
          <USelect
            v-model="selectedUnder"
            :items="underOptions"
            data-testid="parent-picker-under-select"
          />
        </UFormField>
        <p v-if="submitError" class="text-sm text-error">{{ submitError }}</p>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="onCancel">Cancel</UButton>
        <UButton
          color="primary"
          :disabled="submitDisabled"
          :loading="submitting"
          data-testid="parent-picker-submit-button"
          @click="onSubmit"
        >
          Move
        </UButton>
      </div>
    </template>
  </UModal>
</template>
