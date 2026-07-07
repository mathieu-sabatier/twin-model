<script setup lang="ts">
// Editable presentation of a selected object type: identity header (doc, base,
// abstract), then its members via MembersTable in editable mode.
// Every edit builds a new Model clone, mutates it, and persists via store.saveModel
// (determinism contract: the store re-fetches after save and re-renders from server AST).
import { toRaw } from 'vue'
import type { ObjectType } from '~/types'
import type { DiagnosticIndex } from '~/lib/diagnosticPath'
import { useDraftStore } from '~/stores/draft'
import { computeBaseOptions } from '~/lib/baseOptions'

const props = defineProps<{
  type: ObjectType
  diagnosticIndex: DiagnosticIndex
}>()

const store = useDraftStore()
const toast = useToast()

const memberCount = computed(() => props.type.members?.length ?? 0)

// Base type options: every local object type except self and its descendants,
// plus the current base ref when it is a non-local import alias. (M4)
const baseOptions = computed<string[]>(() =>
  computeBaseOptions(store.objectTypes, props.type.name, props.type.base?.raw),
)

// ── Central mutate-and-save helper ──────────────────────────────────────────
async function updateType(mutate: (t: ObjectType) => void): Promise<void> {
  const raw = toRaw(store.model)
  if (!raw) return
  const next = structuredClone(raw)
  const t = next.objectTypes?.find((x) => x.name === props.type.name)
  if (!t) return
  mutate(t)
  await store.saveModel(next)
}

// ── MembersTable event handlers ──────────────────────────────────────────────
function onUpdateMember({ index, patch }: { index: number; member: string; patch: Partial<import('~/types').Member> }): void {
  void updateType((t) => {
    const m = t.members?.[index]
    if (m) Object.assign(m, patch)
  })
}

function onAdd(): void {
  void updateType((t) => {
    ;(t.members ??= []).push({
      name: 'NewMember',
      kind: 'variable',
      rule: 'mandatory',
      pos: { file: '', line: 0, col: 0 },
    })
  })
}

function onRemove({ index }: { index: number; member: string }): void {
  const snapshot = structuredClone(toRaw(store.model))
  const removed = props.type.members?.[index]?.name ?? 'member'
  void updateType((t) => {
    if (t.members) t.members.splice(index, 1)
  }).then(() => {
    toast.add({
      title: `Removed ${removed}`,
      color: 'neutral',
      actions: [{ label: 'Undo', onClick: () => { if (snapshot) void store.saveModel(snapshot) } }],
    })
  })
}

function onReorder({ from, to }: { from: number; to: number }): void {
  void updateType((t) => {
    if (t.members && to >= 0 && to < t.members.length) {
      const [x] = t.members.splice(from, 1)
      t.members.splice(to, 0, x!)
    }
  })
}

// ── Identity header handlers ─────────────────────────────────────────────────
function onDocChange(e: Event): void {
  const val = (e.target as HTMLTextAreaElement).value
  void updateType((t) => { t.doc = val })
}

function onBaseChange(val: string): void {
  void updateType((t) => {
    if (!val) {
      t.base = undefined
    } else {
      t.base = { raw: val, name: val, pos: { file: '', line: 0, col: 0 } }
    }
  })
}

function onAbstractChange(val: boolean): void {
  void updateType((t) => { t.abstract = val })
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- Identity header -->
    <header class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center gap-3">
        <UIcon
          :name="type.abstract ? 'i-lucide-shapes' : 'i-lucide-box'"
          class="size-6 text-primary"
        />
        <h1 class="font-mono text-2xl font-semibold text-highlighted">{{ type.name }}</h1>
        <UBadge
          v-if="type.abstract"
          color="neutral"
          variant="outline"
          size="sm"
          icon="i-lucide-asterisk"
        >
          abstract
        </UBadge>
        <UBadge color="neutral" variant="soft" size="sm">
          {{ memberCount }} {{ memberCount === 1 ? 'member' : 'members' }}
        </UBadge>
      </div>

      <!-- Doc textarea -->
      <UTextarea
        :model-value="type.doc ?? ''"
        placeholder="Add a description…"
        :rows="2"
        class="max-w-2xl text-sm"
        aria-label="Type description"
        :disabled="store.frozen || store.saving"
        @change="onDocChange($event)"
      />

      <div class="rounded-lg border border-default bg-elevated/30 px-4 py-2">
        <DisplayMetaRow label="Base" icon="i-lucide-anchor">
          <USelect
            :model-value="type.base?.raw ?? undefined"
            :items="baseOptions"
            size="sm"
            placeholder="(none)"
            aria-label="Base type"
            :disabled="store.frozen || store.saving"
            @update:model-value="onBaseChange($event as string)"
          />
        </DisplayMetaRow>

        <DisplayMetaRow label="Abstract" icon="i-lucide-asterisk">
          <USwitch
            :model-value="type.abstract ?? false"
            :disabled="store.frozen || store.saving"
            aria-label="Abstract"
            @update:model-value="onAbstractChange($event)"
          />
        </DisplayMetaRow>
      </div>
    </header>

    <USeparator />

    <!-- Members -->
    <section class="flex flex-col gap-3">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-list-tree" class="size-4 text-muted" />
        <h2 class="text-sm font-semibold uppercase tracking-wide text-muted">Members</h2>
      </div>
      <MembersTable
        :type-name="type.name"
        :members="type.members ?? []"
        :diagnostic-index="diagnosticIndex"
        :readonly="store.frozen || store.saving"
        @update:member="onUpdateMember"
        @add="onAdd"
        @remove="onRemove"
        @reorder="onReorder"
      />
    </section>
  </div>
</template>
