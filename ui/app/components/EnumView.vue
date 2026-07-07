<script setup lang="ts">
// Editable presentation of a selected enum: doc + value/identifier table with
// add/edit/remove. Every edit clones the model, mutates the named enum, and
// persists via store.saveModel (determinism contract). QA finding M5.
import { toRaw } from 'vue'
import type { Enum } from '~/types'
import { useDraftStore } from '~/stores/draft'

// `enum` is a reserved word in template expressions, so the prop is named `def`.
const props = defineProps<{ def: Enum }>()
const def = computed(() => props.def)

const store = useDraftStore()
const toast = useToast()
const zero = { file: '', line: 0, col: 0 }

async function updateEnum(mutate: (e: Enum) => void): Promise<void> {
  const raw = toRaw(store.model)
  if (!raw) return
  const next = structuredClone(raw)
  const e = next.enums?.find((x) => x.name === props.def.name)
  if (!e) return
  mutate(e)
  await store.saveModel(next)
}

function onDocChange(e: Event): void {
  const val = (e.target as HTMLTextAreaElement).value
  void updateEnum((en) => { en.doc = val || undefined })
}

function onAddValue(): void {
  void updateEnum((en) => {
    const nextId = en.values.reduce((max, v) => Math.max(max, v.identifier), -1) + 1
    const names = new Set(en.values.map((v) => v.name))
    let n = 1
    while (names.has(`Value${n}`)) n++
    en.values.push({ pos: zero, name: `Value${n}`, identifier: nextId, explicit: true })
  })
}

function onNameChange(index: number, e: Event): void {
  const val = (e.target as HTMLInputElement).value.trim()
  if (!val) { toast.add({ title: 'Value name is required', color: 'error' }); return }
  if (props.def.values.some((v, i) => i !== index && v.name === val)) {
    toast.add({ title: 'Duplicate value name', color: 'error' })
    return
  }
  void updateEnum((en) => { en.values[index]!.name = val })
}

function onIdentifierChange(index: number, e: Event): void {
  if ((e.target as HTMLInputElement).value.trim() === '') {
    toast.add({ title: 'Identifier is required', color: 'error' })
    return
  }
  const val = Number((e.target as HTMLInputElement).value)
  if (!Number.isInteger(val)) { toast.add({ title: 'Identifier must be an integer', color: 'error' }); return }
  if (props.def.values.some((v, i) => i !== index && v.identifier === val)) {
    toast.add({ title: 'Duplicate identifier', color: 'error' })
    return
  }
  void updateEnum((en) => { en.values[index]!.identifier = val; en.values[index]!.explicit = true })
}

function onRemoveValue(index: number): void {
  const snapshot = structuredClone(toRaw(store.model))
  const removed = props.def.values[index]!.name
  void updateEnum((en) => { en.values.splice(index, 1) }).then(() => {
    toast.add({
      title: `Removed ${removed}`,
      color: 'neutral',
      actions: [{ label: 'Undo', onClick: () => { if (snapshot) void store.saveModel(snapshot) } }],
    })
  })
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center gap-3">
        <UIcon name="i-lucide-hash" class="size-6 text-primary" />
        <h1 class="font-mono text-2xl font-semibold text-highlighted">{{ def.name }}</h1>
        <UBadge color="neutral" variant="soft" size="sm">
          {{ def.values.length }} {{ def.values.length === 1 ? 'value' : 'values' }}
        </UBadge>
      </div>
      <div data-testid="enum-doc">
        <UTextarea
          :model-value="def.doc ?? ''"
          placeholder="Add a description…"
          :rows="2"
          class="max-w-2xl text-sm"
          :disabled="store.frozen || store.saving"
          @change="onDocChange($event)"
        />
      </div>
    </header>

    <USeparator />

    <section class="flex flex-col gap-3">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-list" class="size-4 text-muted" />
        <h2 class="text-sm font-semibold uppercase tracking-wide text-muted">Values</h2>
      </div>
      <div class="overflow-x-auto rounded-lg border border-default">
        <table class="w-full border-collapse text-sm">
          <thead>
            <tr class="border-b border-default bg-elevated/40 text-left">
              <th class="px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted">Name</th>
              <th class="px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted">Identifier</th>
              <th class="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(ev, i) in def.values"
              :key="i"
              class="border-b border-default/60 last:border-0 hover:bg-elevated/30"
              :data-enum-value="ev.name"
            >
              <td class="px-3 py-2">
                <UInput
                  type="text"
                  :model-value="ev.name"
                  size="xs"
                  class="font-mono"
                  :disabled="store.frozen || store.saving"
                  :aria-label="`Name of ${ev.name}`"
                  @change="onNameChange(i, $event)"
                />
              </td>
              <td class="px-3 py-2">
                <UInput
                  type="number"
                  :model-value="ev.identifier"
                  size="xs"
                  class="font-mono tabular-nums w-24"
                  :disabled="store.frozen || store.saving"
                  :aria-label="`Identifier of ${ev.name}`"
                  @change="onIdentifierChange(i, $event)"
                />
              </td>
              <td class="px-3 py-2 text-right">
                <UButton
                  v-if="!store.frozen && !store.saving"
                  size="xs"
                  color="error"
                  variant="ghost"
                  icon="i-lucide-trash-2"
                  :aria-label="`Remove ${ev.name}`"
                  @click="onRemoveValue(i)"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!store.frozen && !store.saving" class="px-1">
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-plus"
          data-testid="enum-add-value"
          @click="onAddValue"
        >
          Add value
        </UButton>
      </div>
    </section>
  </div>
</template>
