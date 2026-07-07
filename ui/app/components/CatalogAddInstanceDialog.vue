<script setup lang="ts">
// Dialog for adding a companion instance under a parent node.
// Follows the same UModal / defineModel pattern as InstanceCreateDialog.vue.
import { ref } from 'vue'
import { useDraftStore } from '~/stores/draft'

const props = defineProps<{ alias: string; name: string; uri: string }>()

// v-model:open — follows the same @nuxt/ui UModal convention as InstanceCreateDialog.vue
const open = defineModel<boolean>('open', { default: false })

const store = useDraftStore()
const instName = ref('')
const under = ref('')
const busy = ref(false)
const err = ref<string | null>(null)

async function onSubmit(): Promise<void> {
  const n = instName.value.trim()
  const u = under.value.trim()
  if (!n || !u) { err.value = 'Name and parent are required.'; return }
  busy.value = true
  err.value = null
  try {
    await store.addCompanionInstance({ name: n, under: u, typeAlias: props.alias, typeName: props.name, typeUri: props.uri })
    open.value = false
    instName.value = ''
    under.value = ''
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

function onCancel(): void {
  open.value = false
  instName.value = ''
  under.value = ''
  err.value = null
}
</script>

<template>
  <UModal v-model:open="open" title="Add companion instance">
    <template #body>
      <div class="flex flex-col gap-4 py-2">
        <p class="text-sm text-dimmed">Instance of <b>{{ alias }}:{{ name }}</b>. Adds the import if needed.</p>
        <UFormField label="Instance name">
          <UInput
            v-model="instName"
            placeholder="Pump1"
            autofocus
            data-testid="add-instance-name-input"
          />
        </UFormField>
        <UFormField label="Parent (under)" :error="err ?? undefined">
          <UInput
            v-model="under"
            placeholder="OpcUa:ObjectsFolder"
            data-testid="add-instance-under-input"
            @keyup.enter="onSubmit"
          />
        </UFormField>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="onCancel">Cancel</UButton>
        <UButton
          color="primary"
          :loading="busy"
          data-testid="add-instance-submit-button"
          @click="onSubmit"
        >
          Add
        </UButton>
      </div>
    </template>
  </UModal>
</template>
