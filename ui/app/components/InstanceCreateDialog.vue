<script setup lang="ts">
// Dialog for creating a new instance: pick a non-abstract type, enter a name,
// choose a parent (under). Values/placeholders are a separate later task (RT5b).
import { useDraftStore } from '~/stores/draft'

const store = useDraftStore()

// v-model:open — follows the same @nuxt/ui UModal convention as BottomBar.vue
const open = defineModel<boolean>('open', { default: false })

// ── Form state ───────────────────────────────────────────────────────────────

const selectedType = ref<string>('')
const instanceName = ref<string>('')
const selectedUnder = ref<string>('OpcUa:ObjectsFolder')
const submitError = ref<string | null>(null)
const submitting = ref(false)

// ── Derived options ──────────────────────────────────────────────────────────

/** Only non-abstract object types may be instantiated. */
const typeOptions = computed<string[]>(() =>
  store.objectTypes.filter((t) => !t.abstract).map((t) => t.name),
)

/**
 * Parent options: the well-known root + all existing instance names.
 * OpcUa:ObjectsFolder is the canonical top-level container in every OPC UA server.
 */
const underOptions = computed<string[]>(() => [
  'OpcUa:ObjectsFolder',
  ...store.instances.map((i) => i.name),
])

const trimmedName = computed(() => instanceName.value.trim())
const nameConflict = computed(() => trimmedName.value !== '' && store.nameTaken(trimmedName.value))

/** Disable submit when name is empty, no type selected, or name conflicts. */
const submitDisabled = computed(() =>
  submitting.value ||
  trimmedName.value === '' ||
  selectedType.value === '' ||
  nameConflict.value,
)

// ── Actions ──────────────────────────────────────────────────────────────────

async function onSubmit(): Promise<void> {
  if (submitDisabled.value) return
  submitError.value = null
  submitting.value = true
  try {
    await store.createInstance({
      name: instanceName.value.trim(),
      type: selectedType.value,
      under: selectedUnder.value,
    })
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
  selectedType.value = ''
  instanceName.value = ''
  selectedUnder.value = 'OpcUa:ObjectsFolder'
  submitError.value = null
}
</script>

<template>
  <UModal v-model:open="open" title="Add instance">
    <template #body>
      <div class="flex flex-col gap-4 py-2">
        <!-- Type picker: non-abstract types only -->
        <UFormField label="Type" required>
          <USelect
            v-model="selectedType"
            :items="typeOptions"
            placeholder="Select a type…"
            data-testid="instance-type-select"
          />
        </UFormField>

        <!-- Instance name -->
        <UFormField label="Name" required>
          <UInput
            v-model="instanceName"
            placeholder="e.g. Furnace03"
            data-testid="instance-name-input"
          />
          <p
            v-if="nameConflict"
            class="mt-1 text-xs text-error"
            data-testid="instance-name-error"
          >
            An instance named "{{ trimmedName }}" already exists.
          </p>
        </UFormField>

        <!-- Parent (under) -->
        <UFormField label="Parent (under)">
          <USelect
            v-model="selectedUnder"
            :items="underOptions"
            data-testid="instance-under-select"
          />
        </UFormField>

        <!-- Error from saveModel -->
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
          data-testid="instance-submit-button"
          @click="onSubmit"
        >
          Add instance
        </UButton>
      </div>
    </template>
  </UModal>
</template>
