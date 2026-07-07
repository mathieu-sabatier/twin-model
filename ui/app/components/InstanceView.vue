<script setup lang="ts">
// Editable presentation of a selected instance: identity header (type editable with
// guard, under), a values form generated from the RESOLVED type (typed inputs + enum
// dropdowns), a placeholder "Add …" section, and per-child remove buttons.
// Every edit builds a new Model clone, mutates it, and persists via store.saveModel
// (determinism contract: the store re-fetches after save and re-renders from server AST).
import { toRaw, nextTick } from 'vue'
import type { Instance, ResolvedMember } from '~/types'
import type { DiagnosticIndex } from '~/lib/diagnosticPath'
import { isa95Rank, ISA95_ORG_RUNGS } from '~/lib/isa95'
import { useDraftStore } from '~/stores/draft'

const props = defineProps<{
  instance: Instance
  diagnosticIndex: DiagnosticIndex
}>()

const store = useDraftStore()
const toast = useToast()

const zero = { file: '', line: 0, col: 0 }

// ── Resolved members (fetched from server, cached in store) ──────────────────

const resolvedMembers = ref<ResolvedMember[]>([])

async function fetchResolved(typeName: string): Promise<void> {
  resolvedMembers.value = await store.resolvedFor(typeName)
}

onMounted(() => void fetchResolved(props.instance.type.name))
watch(() => props.instance.type.name, (name) => void fetchResolved(name))

/** Value-bearing: property or variable, rule NOT ending _placeholder */
const valueMembers = computed(() =>
  resolvedMembers.value.filter(
    (m) =>
      (m.kind === 'property' || m.kind === 'variable') &&
      !m.rule.endsWith('_placeholder'),
  ),
)

/** Every placeholder member (rule ends with _placeholder), including those
 *  nested inside object members' children (e.g. Zones → Zone<No>). */
const placeholderMembers = computed<ResolvedMember[]>(() => {
  const out: ResolvedMember[] = []
  const walk = (members: readonly ResolvedMember[] | readonly import('~/types').Member[] | undefined) => {
    for (const m of members ?? []) {
      if (m.rule.endsWith('_placeholder')) out.push(m as ResolvedMember)
      if (m.children?.length) walk(m.children)
    }
  }
  walk(resolvedMembers.value)
  return out
})

const children = computed(() => props.instance.children ?? [])

// ── ISA-95 breadcrumb + level (B5) ───────────────────────────────────────────

/** Canonical ISA-95 path: walks the `under` chain via `store.instances`, stopping
 *  at the first import-target `under` (alias-qualified TypeRef = a root). */
const breadcrumb = computed<string[]>(() => {
  const byName = new Map(store.instances.map((i) => [i.name, i]))
  const path: string[] = []
  let cur: Instance | undefined = props.instance
  const seen = new Set<string>()
  while (cur && !seen.has(cur.name)) {
    seen.add(cur.name)
    path.unshift(cur.name)
    const parentName: string | null = cur.under.alias ? null : cur.under.name
    cur = parentName ? byName.get(parentName) : undefined
  }
  return path
})

const levelDiag = computed(() => props.diagnosticIndex.forInstanceLevel(props.instance.name))

/** 1-indexed ISA-95 org rank (Enterprise=1 .. WorkUnit-tier=5) for the rank chip,
 *  or null for equipment-only / unknown levels (no org rung → no chip). */
const levelRank = computed(() => isa95Rank(props.instance.level))

function currentValue(memberName: string): string {
  return props.instance.values?.find((v) => v.member === memberName)?.raw ?? ''
}

function valueDiags(member: string) {
  return props.diagnosticIndex.forInstanceValue(props.instance.name, member)
}

/** Tailwind ring class for a value member's worst diagnostic severity. */
function valueRing(name: string): string {
  const ds = valueDiags(name)
  if (ds.some((d) => d.severity === 'error')) return 'ring-2 ring-error'
  if (ds.length) return 'ring-2 ring-warning'
  return ''
}

function valueInvalid(name: string): true | undefined {
  return valueDiags(name).some((d) => d.severity === 'error') ? true : undefined
}

// ── Enum lookup ──────────────────────────────────────────────────────────────

function enumOptions(typeName: string | undefined): string[] | null {
  if (!typeName) return null
  const e = store.enums.find((en) => en.name === typeName)
  return e ? e.values.map((v) => v.name) : null
}

const NUMERIC_TYPES = new Set([
  'SByte', 'Byte', 'Int16', 'UInt16', 'Int32', 'UInt32',
  'Int64', 'UInt64', 'Float', 'Double', 'Number', 'Integer',
  'UInteger', 'Duration',
])

function isBooleanMember(m: ResolvedMember): boolean {
  return m.type?.name === 'Boolean' && !m.type?.alias
}
function isNumericMember(m: ResolvedMember): boolean {
  return !!m.type && !m.type.alias && NUMERIC_TYPES.has(m.type.name)
}

// ── Mutate-and-save helper ───────────────────────────────────────────────────

async function updateInstance(mutate: (inst: import('~/types').Instance) => void): Promise<void> {
  const raw = toRaw(store.model)
  if (!raw) return
  const next = structuredClone(raw)
  const inst = next.instances?.find((x) => x.name === props.instance.name)
  if (!inst) return
  mutate(inst)
  await store.saveModel(next)
}

// ── Deliverable 1 — value upsert/clear ───────────────────────────────────────

function onValueChange(memberName: string, newRaw: string): void {
  void updateInstance((inst) => {
    if (!inst.values) inst.values = []
    const idx = inst.values.findIndex((v) => v.member === memberName)
    if (newRaw === '') {
      // Clear: remove entry
      if (idx !== -1) inst.values.splice(idx, 1)
    } else {
      const entry = { pos: zero, member: memberName, raw: newRaw }
      if (idx !== -1) {
        inst.values[idx] = entry
      } else {
        inst.values.push(entry)
      }
    }
  })
}

// ── Deliverable 2 — placeholder "Add …" ─────────────────────────────────────

/**
 * Reconstruct the placeholder key (Name<Suffix>) from member name + browseName.
 * Mirrors emitYaml.ts memberKey logic exactly.
 */
function placeholderOf(member: ResolvedMember): string {
  if (member.browseName) {
    const inner = member.browseName.replace(/^</, '').replace(/>$/, '')
    const suffix = inner.startsWith(member.name) ? inner.slice(member.name.length) : inner
    return `${member.name}<${suffix}>`
  }
  return member.name
}

function onAddChild(member: ResolvedMember): void {
  void updateInstance((inst) => {
    const baseName = member.name
    const existing = inst.children?.map((c) => c.name) ?? []
    let n = 1
    while (existing.includes(`${baseName}${n}`)) n++
    ;(inst.children ??= []).push({
      pos: zero,
      name: `${baseName}${n}`,
      of: { pos: zero, name: placeholderOf(member), raw: placeholderOf(member) },
    })
  })
}

function onRemoveChild(childName: string): void {
  const snapshot = structuredClone(toRaw(store.model))
  void updateInstance((inst) => {
    if (inst.children) inst.children = inst.children.filter((c) => c.name !== childName)
  }).then(() => {
    toast.add({
      title: `Removed ${childName}`,
      color: 'neutral',
      actions: [{ label: 'Undo', onClick: () => { if (snapshot) void store.saveModel(snapshot) } }],
    })
  })
}

// ── Deliverable 3 — type-change guard ────────────────────────────────────────

const pendingType = ref<string | null>(null)
const showTypeModal = ref(false)

// ── Deliverable 4 — inline rename ────────────────────────────────────────────

const editingName = ref(false)
const nameDraft = ref('')

const renameConflict = computed(() => {
  const n = nameDraft.value.trim()
  return n !== '' && n !== props.instance.name && store.nameTaken(n)
})
const renameInvalid = computed(() => nameDraft.value.trim() === '' || renameConflict.value)

// Guard: skip the blur that fires immediately after programmatic autofocus on mount.
// UInput fires focus() via setTimeout(0); we clear this flag after the same tick.
let justFocused = false

function startRename(): void {
  nameDraft.value = props.instance.name
  justFocused = true
  editingName.value = true
  void nextTick(() => { justFocused = false })
}
async function commitRename(): Promise<void> {
  if (renameInvalid.value) return
  const target = nameDraft.value.trim()
  editingName.value = false
  if (target !== props.instance.name) await store.renameInstance(props.instance.name, target)
}
function cancelRename(): void { editingName.value = false }
function onBlur(): void {
  if (justFocused || !editingName.value) return
  void commitRename()
}

// ── Deliverable 4 — delete instance ──────────────────────────────────────────

const showDeleteModal = ref(false)

async function confirmDelete(): Promise<void> {
  showDeleteModal.value = false
  await store.deleteInstance(props.instance.name)
}

const typeOptions = computed<string[]>(() =>
  store.objectTypes.filter((t) => !t.abstract).map((t) => t.name),
)

function onTypeSelect(val: string): void {
  if (val === props.instance.type.name) return
  pendingType.value = val
  showTypeModal.value = true
}

async function confirmTypeChange(): Promise<void> {
  const newType = pendingType.value
  if (!newType) return
  showTypeModal.value = false
  const members = await store.resolvedFor(newType)
  const validMemberNames = new Set(members.map((m) => m.name))
  // Placeholder keys (Name<Suffix>) valid on the new type, including nested ones,
  // so we keep only children whose `of` still resolves to a placeholder there.
  const validPlaceholders = new Set<string>()
  const collect = (
    ms: readonly (ResolvedMember | import('~/types').Member)[] | undefined,
  ): void => {
    for (const m of ms ?? []) {
      if (m.rule.endsWith('_placeholder')) validPlaceholders.add(placeholderOf(m as ResolvedMember))
      if (m.children?.length) collect(m.children)
    }
  }
  collect(members)
  await updateInstance((inst) => {
    inst.type = { pos: zero, name: newType, raw: newType }
    if (inst.values) {
      inst.values = inst.values.filter((v) => validMemberNames.has(v.member))
    }
    if (inst.children) {
      inst.children = inst.children.filter((c) => validPlaceholders.has(c.of.raw))
    }
  })
  pendingType.value = null
}

function cancelTypeChange(): void {
  showTypeModal.value = false
  pendingType.value = null
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- Identity header -->
    <header class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center gap-3">
        <UIcon name="i-lucide-boxes" class="size-6 text-primary" />
        <template v-if="editingName">
          <div data-testid="rename-input">
            <UInput
              v-model="nameDraft"
              size="lg"
              autofocus
              class="font-mono"
              @keydown.enter="commitRename"
              @keydown.escape="cancelRename"
              @blur="onBlur"
            />
          </div>
          <p v-if="editingName && renameConflict" class="text-xs text-error" data-testid="rename-error">Name already exists.</p>
          <p v-else-if="editingName && nameDraft.trim() === ''" class="text-xs text-error" data-testid="rename-error">Name cannot be empty.</p>
        </template>
        <template v-else>
          <h1 class="font-mono text-2xl font-semibold text-highlighted">{{ instance.name }}</h1>
          <UButton
            size="xs"
            color="neutral"
            variant="ghost"
            icon="i-lucide-pencil"
            :disabled="store.frozen || store.saving"
            aria-label="Rename instance"
            data-testid="rename-instance"
            @click="startRename"
          />
        </template>
        <div class="flex-1" />
        <UButton
          size="xs"
          color="error"
          variant="ghost"
          icon="i-lucide-trash-2"
          :disabled="store.frozen || store.saving"
          aria-label="Delete instance"
          data-testid="delete-instance"
          @click="() => { showDeleteModal = true }"
        >
          Delete
        </UButton>
      </div>

      <div class="rounded-lg border border-default bg-elevated/30 px-4 py-2">
        <DisplayMetaRow label="Type" icon="i-lucide-box">
          <USelect
            :model-value="instance.type.name"
            :items="typeOptions"
            size="sm"
            placeholder="(select type)"
            :disabled="store.frozen || store.saving"
            @update:model-value="onTypeSelect($event as string)"
          />
        </DisplayMetaRow>
        <DisplayMetaRow label="Under" icon="i-lucide-folder-input">
          <DisplayTypeRefLabel :type-ref="instance.under" />
        </DisplayMetaRow>
        <DisplayMetaRow label="ISA-95 path" icon="i-lucide-list-tree">
          <span data-testid="isa95-breadcrumb" class="font-mono text-xs text-muted">
            {{ breadcrumb.join(' / ') }}
          </span>
        </DisplayMetaRow>
        <DisplayMetaRow v-if="instance.level" label="Level" icon="i-lucide-layers">
          <span
            class="inline-flex items-center gap-1.5"
            :class="levelDiag.length ? 'ring-2 ring-error rounded px-1' : ''"
          >
            <span
              v-if="levelRank"
              class="inline-flex items-center justify-center rounded bg-elevated text-muted text-[0.7rem] font-mono font-medium size-4 leading-none tabular-nums"
              :title="`ISA-95 tier ${levelRank} of ${ISA95_ORG_RUNGS}`"
            >{{ levelRank }}</span>
            {{ instance.level }}
          </span>
        </DisplayMetaRow>
      </div>
    </header>

    <!-- Type-change confirm modal -->
    <UModal v-model:open="showTypeModal">
      <template #content>
        <div class="p-6 flex flex-col gap-4">
          <p class="text-sm text-default">
            Changing the type may clear values and children that don't exist on
            <strong class="font-mono">{{ pendingType }}</strong>. Continue?
          </p>
          <div class="flex justify-end gap-2">
            <UButton
              color="neutral"
              variant="outline"
              size="sm"
              @click="cancelTypeChange"
            >
              Cancel
            </UButton>
            <UButton
              color="primary"
              size="sm"
              data-testid="confirm-type-change"
              @click="confirmTypeChange"
            >
              Change type
            </UButton>
          </div>
        </div>
      </template>
    </UModal>

    <!-- Delete-instance confirm modal -->
    <UModal v-model:open="showDeleteModal">
      <template #content>
        <div class="p-6 flex flex-col gap-4">
          <p class="text-sm text-default">
            Delete instance <strong class="font-mono">{{ instance.name }}</strong>? This cannot be undone.
          </p>
          <div class="flex justify-end gap-2">
            <UButton color="neutral" variant="outline" size="sm" @click="() => { showDeleteModal = false }">
              Cancel
            </UButton>
            <UButton
              color="error"
              size="sm"
              data-testid="confirm-delete-instance"
              @click="confirmDelete"
            >
              Delete
            </UButton>
          </div>
        </div>
      </template>
    </UModal>

    <USeparator />

    <!-- Values form (from resolved type) -->
    <section class="flex flex-col gap-3">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-equal" class="size-4 text-muted" />
        <h2 class="text-sm font-semibold uppercase tracking-wide text-muted">Values</h2>
      </div>

      <div v-if="valueMembers.length" class="overflow-x-auto rounded-lg border border-default">
        <table class="w-full border-collapse text-sm">
          <thead>
            <tr class="border-b border-default bg-elevated/40 text-left">
              <th class="px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted">Member</th>
              <th class="px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted">Value</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="m in valueMembers"
              :key="m.name"
              class="border-b border-default/60 last:border-0 hover:bg-elevated/30"
              :data-value="m.name"
            >
              <td class="px-3 py-2 font-mono font-medium text-highlighted">
                <span class="inline-flex items-center gap-1.5">
                  {{ m.name }}
                  <UIcon
                    v-if="valueDiags(m.name).length"
                    name="i-lucide-circle-alert"
                    class="size-3.5 text-error"
                    :title="valueDiags(m.name)[0]?.message"
                  />
                </span>
              </td>
              <td class="px-3 py-2">
                <div class="flex items-center gap-2">
                  <!-- Enum member → USelect of value names -->
                  <USelect
                    v-if="enumOptions(m.type?.name)"
                    :model-value="currentValue(m.name) || '__inherit__'"
                    :items="[{ label: '(inherit)', value: '__inherit__' }, ...enumOptions(m.type!.name)!.map((v) => ({ label: v, value: v }))]"
                    size="sm"
                    :class="valueRing(m.name)"
                    :aria-invalid="valueInvalid(m.name)"
                    :aria-label="`Value of ${m.name}`"
                    :disabled="store.frozen || store.saving"
                    @update:model-value="onValueChange(m.name, ($event === '__inherit__' ? '' : ($event as string)) ?? '')"
                  />
                  <!-- Boolean member → true/false/(inherit) select -->
                  <USelect
                    v-else-if="isBooleanMember(m)"
                    :model-value="currentValue(m.name) || '__inherit__'"
                    :items="[{ label: '(inherit)', value: '__inherit__' }, { label: 'true', value: 'true' }, { label: 'false', value: 'false' }]"
                    size="sm"
                    :class="valueRing(m.name)"
                    :aria-invalid="valueInvalid(m.name)"
                    :aria-label="`Value of ${m.name}`"
                    :disabled="store.frozen || store.saving"
                    @update:model-value="onValueChange(m.name, ($event === '__inherit__' ? '' : ($event as string)) ?? '')"
                  />
                  <!-- Numeric member → number input -->
                  <UInput
                    v-else-if="isNumericMember(m)"
                    type="number"
                    :model-value="currentValue(m.name)"
                    size="sm"
                    placeholder="(inherit)"
                    :class="valueRing(m.name)"
                    :aria-invalid="valueInvalid(m.name)"
                    :aria-label="`Value of ${m.name}`"
                    :readonly="store.frozen || store.saving"
                    @change="onValueChange(m.name, ($event.target as HTMLInputElement).value)"
                  >
                    <template v-if="m.unit" #trailing>
                      <span class="text-xs text-muted select-none">{{ m.unit }}</span>
                    </template>
                  </UInput>
                  <!-- Plain (textual) member → UInput -->
                  <UInput
                    v-else
                    :model-value="currentValue(m.name)"
                    size="sm"
                    :placeholder="`(inherit)`"
                    :class="valueRing(m.name)"
                    :aria-invalid="valueInvalid(m.name)"
                    :aria-label="`Value of ${m.name}`"
                    :readonly="store.frozen || store.saving"
                    @change="onValueChange(m.name, ($event.target as HTMLInputElement).value)"
                  >
                    <template v-if="m.unit" #trailing>
                      <span class="text-xs text-muted select-none">{{ m.unit }}</span>
                    </template>
                  </UInput>
                  <span v-if="m.unit && enumOptions(m.type?.name)" class="text-xs text-muted">{{ m.unit }}</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else-if="!resolvedMembers.length" class="text-sm text-muted">Loading members…</p>
      <p v-else class="text-sm text-muted">No value-bearing members.</p>
    </section>

    <!-- Placeholder children: Add buttons + list with remove -->
    <section class="flex flex-col gap-3">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-list-tree" class="size-4 text-muted" />
        <h2 class="text-sm font-semibold uppercase tracking-wide text-muted">Children</h2>
      </div>

      <!-- Add buttons for each placeholder member -->
      <div v-if="placeholderMembers.length && !store.frozen && !store.saving" class="flex flex-wrap gap-2">
        <UButton
          v-for="pm in placeholderMembers"
          :key="pm.name"
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-plus"
          :data-add-child="pm.name"
          @click="onAddChild(pm)"
        >
          Add {{ pm.name }}…
        </UButton>
      </div>

      <!-- Existing children list -->
      <ul v-if="children.length" class="flex flex-col gap-1.5">
        <li
          v-for="c in children"
          :key="c.name"
          class="flex items-center gap-2 rounded-md border border-default bg-elevated/20 px-3 py-2 text-sm"
          :data-child="c.name"
        >
          <UIcon name="i-lucide-boxes" class="size-4 text-muted" />
          <span class="font-mono font-medium text-highlighted">{{ c.name }}</span>
          <span class="text-dimmed">of</span>
          <DisplayTypeRefLabel :type-ref="c.of" />
          <UButton
            v-if="!store.frozen && !store.saving"
            class="ml-auto"
            size="xs"
            color="error"
            variant="ghost"
            icon="i-lucide-trash-2"
            :aria-label="`Remove ${c.name}`"
            :data-remove-child="c.name"
            @click="onRemoveChild(c.name)"
          />
        </li>
      </ul>

      <p v-if="!children.length && !placeholderMembers.length" class="text-sm text-muted">
        No children.
      </p>
    </section>
  </div>
</template>
