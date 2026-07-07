<script setup lang="ts">
// Dialog for extending a companion type as a new local ObjectType.
// Follows the same UModal / defineModel pattern as InstanceCreateDialog.vue.
import { ref } from 'vue'
import { useDraftStore } from '~/stores/draft'

const props = defineProps<{ alias: string; name: string; uri: string }>()

// v-model:open — follows the same @nuxt/ui UModal convention as InstanceCreateDialog.vue
const open = defineModel<boolean>('open', { default: false })

const store = useDraftStore()
const newName = ref('')
const busy = ref(false)
const err = ref<string | null>(null)

async function onSubmit(): Promise<void> {
  const n = newName.value.trim()
  if (!n) { err.value = 'A name is required.'; return }
  busy.value = true
  err.value = null
  try {
    await store.extendType({ name: n, baseAlias: props.alias, baseName: props.name, baseUri: props.uri })
    open.value = false
    newName.value = ''
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

function onCancel(): void {
  open.value = false
  newName.value = ''
  err.value = null
}
</script>

<template>
  <UModal v-model:open="open" title="Extend companion type">
    <template #body>
      <div class="flex flex-col gap-4 py-2">
        <p class="text-sm text-dimmed">
          New local ObjectType based on <b>{{ alias }}:{{ name }}</b>. The <code>{{ alias }}</code> import is added automatically.
        </p>
        <UFormField label="Type name" :error="err ?? undefined">
          <UInput
            v-model="newName"
            placeholder="MyDevice"
            autofocus
            data-testid="extend-name-input"
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
          data-testid="extend-submit-button"
          @click="onSubmit"
        >
          Create
        </UButton>
      </div>
    </template>
  </UModal>
</template>
