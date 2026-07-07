<script setup lang="ts">
// Assign-membership dialog opened from TreePane's "Assign to…" row action
// (perspective view). Lists the active perspective's nodes and calls
// store.assignMembership({perspective, node, instance, mode}) — edits only
// that node's `members`. Follows the same v-model:open / UModal shape as
// InstanceCreateDialog.vue / ParentPicker.vue.
import { useDraftStore } from '~/stores/draft'

const store = useDraftStore()

const open = defineModel<boolean>('open', { default: false })

/** The active perspective id and the instance being assigned (set by TreePane). */
const props = defineProps<{ perspective: string | null; target: string | null }>()

const selectedNode = ref<string>('')
const submitError = ref<string | null>(null)
const submitting = ref(false)

const activePerspective = computed(() =>
  store.perspectives.find((p) => p.id === props.perspective),
)

/** USelect options: every node in the active perspective. */
const nodeOptions = computed<{ label: string; value: string }[]>(() =>
  (activePerspective.value?.nodes ?? []).map((n) => ({ label: n.label || n.id, value: n.id })),
)

/** Whether the target instance is already a member of the selected node. */
const isMember = computed<boolean>(() => {
  const nd = activePerspective.value?.nodes?.find((n) => n.id === selectedNode.value)
  return !!props.target && !!nd?.members?.includes(props.target)
})

const actionDisabled = computed(() => submitting.value || !props.target || !props.perspective || selectedNode.value === '')

async function onAssign(mode: 'add' | 'remove'): Promise<void> {
  if (actionDisabled.value || !props.target || !props.perspective) return
  submitError.value = null
  submitting.value = true
  try {
    await store.assignMembership({
      perspective: props.perspective,
      node: selectedNode.value,
      instance: props.target,
      mode,
    })
  } catch (err: unknown) {
    submitError.value = err instanceof Error ? err.message : String(err)
  } finally {
    submitting.value = false
  }
}

function onClose(): void {
  open.value = false
  selectedNode.value = ''
  submitError.value = null
}

watch(open, (v) => {
  if (v) selectedNode.value = ''
})
</script>

<template>
  <UModal v-model:open="open" :title="target ? `Assign ${target}` : 'Assign membership'">
    <template #body>
      <div class="flex flex-col gap-4 py-2">
        <UFormField label="Perspective node">
          <USelect
            v-model="selectedNode"
            :items="nodeOptions"
            placeholder="Select a node…"
            data-testid="assign-menu-node-select"
          />
        </UFormField>
        <p v-if="submitError" class="text-sm text-error">{{ submitError }}</p>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="onClose">Close</UButton>
        <UButton
          color="error"
          variant="soft"
          :disabled="actionDisabled || !isMember"
          :loading="submitting"
          data-testid="assign-menu-remove-button"
          @click="onAssign('remove')"
        >
          Remove
        </UButton>
        <UButton
          color="primary"
          :disabled="actionDisabled || isMember"
          :loading="submitting"
          data-testid="assign-menu-add-button"
          @click="onAssign('add')"
        >
          Add
        </UButton>
      </div>
    </template>
  </UModal>
</template>
