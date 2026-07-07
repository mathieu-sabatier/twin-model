<script setup lang="ts">
// MembersTable — members grid for a type view, with read-only (default) and
// editable (readonly=false) modes.
//
// Columns: Name · Kind · Type · Rule · Access · Unit. Nested placeholder children
// (e.g. Zones → └ Zone<No>) render as indented sub-rows. Method members show a
// trailing chevron and expand into their in/out arguments via UCollapsible.
//
// ── The readonly→editable seam ─────────────────────────────────────────────
// This component supports two modes via the `readonly` prop (default true):
//   • readonly=true  — static badge/label cells (original read-only path)
//   • readonly=false — editable controls (UInput, USelect) per cell; row affordances
//
// Emits (all consumed by TypeView when readonly=false). Rows are addressed by
// index so duplicate-named members stay distinct; `member` is the display name.
//   update:member  — a cell changed   { index, member, patch }
//   add            — add a member row
//   remove         — remove a member row   { index, member }
//   reorder        — move row up/down       { from, to }
//
// Diagnostics: each row/cell asks the injected DiagnosticIndex whether it carries
// a diagnostic and surfaces a ⚠ anchored to the offending field.
import { onMounted } from 'vue'
import type { Argument, Member, MemberKind } from '~/types'
import type { DiagnosticIndex } from '~/lib/diagnosticPath'
import { useDraftStore } from '~/stores/draft'

const props = withDefaults(
  defineProps<{
    /** The owning type name — used to anchor diagnostics via the index. */
    typeName: string
    /** The type's own declared members (inherited members are not shown here). */
    members: Member[]
    /** Diagnostic index for ⚠ anchoring. */
    diagnosticIndex: DiagnosticIndex
    /** Read-only now (T4). T6 flips this to enable editing. */
    readonly?: boolean
  }>(),
  { readonly: true },
)

const emit = defineEmits<{
  'update:member': [payload: { index: number; member: string; patch: Partial<Member> }]
  add: []
  remove: [payload: { index: number; member: string }]
  reorder: [payload: { from: number; to: number }]
}>()

// ── Store (for units in editable mode) ──────────────────────────────────────
const store = useDraftStore()

onMounted(() => {
  if (!props.readonly) {
    void store.loadUnits()
  }
})

// ── Expansion state for method rows (arg panels) ─────────────────────────────
const expanded = ref<Set<string>>(new Set())
function toggle(name: string): void {
  const next = new Set(expanded.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  expanded.value = next
}

function isMethod(m: Member): boolean {
  return m.kind === 'method'
}
function argCount(m: Member): number {
  return (m.in?.length ?? 0) + (m.out?.length ?? 0)
}

/** Access is only meaningful for property/variable — hide it elsewhere. */
function accessFor(m: Member): Member['access'] | null {
  return m.kind === 'property' || m.kind === 'variable' ? (m.access ?? 'r') : null
}

// Diagnostics helpers scoped to this table's type.
function rowDiags(member: string) {
  return props.diagnosticIndex.forMember(props.typeName, member)
}
function cellDiags(member: string, field: string) {
  return props.diagnosticIndex.forMemberField(props.typeName, member, field)
}
function worst(diags: { severity: string }[]): 'error' | 'warning' | null {
  if (diags.some((d) => d.severity === 'error')) return 'error'
  if (diags.length) return 'warning'
  return null
}

/** Tailwind ring class for a cell's worst diagnostic severity. */
function ringClass(sev: 'error' | 'warning' | null): string {
  if (sev === 'error') return 'ring-2 ring-error'
  if (sev === 'warning') return 'ring-2 ring-warning'
  return ''
}

const hasMembers = computed(() => props.members.length > 0)

const kindLabel: Record<MemberKind, string> = {
  property: 'Property',
  variable: 'Variable',
  object: 'Object',
  method: 'Method',
}

// ── Companion-type picker ─────────────────────────────────────────────────────
// `open` is kept as its own plain ref, decoupled from `pickerMember`. Binding the
// modal's open state to a computed that *also* drives the picker's `v-if` (as a
// prior version did) meant closing the modal unmounted the teleported UModal
// mid-close, so `open = false` never settled and the picker reappeared. Mirror the
// working dialogs (CatalogExtendDialog): plain ref for open, component stays
// mounted while hidden. QA finding: picker did not close after selecting a type.
/** Name of the member whose type is being picked (retained while the modal is hidden). */
const pickerMember = ref<string | null>(null)
/** Whether the companion-type picker modal is visible. */
const pickerOpen = ref(false)

function openPicker(memberName: string): void {
  pickerMember.value = memberName
  pickerOpen.value = true
}

// ── Editable mode helpers ─────────────────────────────────────────────────────
const KIND_OPTIONS = ['property', 'variable', 'object', 'method']
const RULE_OPTIONS = ['mandatory', 'optional', 'optional_placeholder', 'mandatory_placeholder']
const ACCESS_OPTIONS = ['r', 'rw']

/** Build unit picker items: blank/clear first (value=null clears), then all store units. */
const unitItems = computed(() => [
  { label: '—', value: null },
  ...store.units.map((u) => ({ label: u.symbol, value: u.symbol })),
])

/** Emit a patch for the member at `index`. */
function patch(index: number, member: string, p: Partial<Member>): void {
  emit('update:member', { index, member, patch: p })
}

function onNameChange(index: number, m: Member, e: Event): void {
  const val = (e.target as HTMLInputElement).value
  patch(index, m.name, { name: val })
}

function onTypeChange(index: number, m: Member, e: Event): void {
  const val = (e.target as HTMLInputElement).value
  if (!val) {
    patch(index, m.name, { type: undefined })
  } else {
    patch(index, m.name, { type: { raw: val, name: val, pos: { file: '', line: 0, col: 0 } } })
  }
}

// ── Method argument editing (name + type only, v1) ──────────────────────────
// Every mutation below builds a brand-new `in`/`out` array and emits the FULL
// array via the same `patch()` helper the other cells use — no prop mutation.
// New arguments start blank; `pos` is a cosmetic zero-value (only `name` and
// `type.raw` are ever emitted to YAML — see emitYaml.ts emitArgument()).
function zeroPos() {
  return { file: '', line: 0, col: 0 }
}

function newArgument(): Argument {
  return { name: '', type: { raw: '', name: '', pos: zeroPos() }, pos: zeroPos() }
}

function argsPatch(dir: 'in' | 'out', list: Argument[]): Partial<Member> {
  return dir === 'in' ? { in: list } : { out: list }
}

function addArg(dir: 'in' | 'out', index: number, m: Member): void {
  patch(index, m.name, argsPatch(dir, [...(m[dir] ?? []), newArgument()]))
}

function removeArg(dir: 'in' | 'out', index: number, m: Member, argIdx: number): void {
  patch(index, m.name, argsPatch(dir, (m[dir] ?? []).filter((_, i) => i !== argIdx)))
}

function onArgNameChange(dir: 'in' | 'out', index: number, m: Member, argIdx: number, e: Event): void {
  const val = (e.target as HTMLInputElement).value
  const list = (m[dir] ?? []).map((a, i) => (i === argIdx ? { ...a, name: val } : a))
  patch(index, m.name, argsPatch(dir, list))
}

function onArgTypeChange(dir: 'in' | 'out', index: number, m: Member, argIdx: number, e: Event): void {
  const val = (e.target as HTMLInputElement).value
  const list = (m[dir] ?? []).map((a, i) =>
    i === argIdx ? { ...a, type: { ...a.type, raw: val, name: val } } : a,
  )
  patch(index, m.name, argsPatch(dir, list))
}
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full min-w-[640px] border-collapse text-sm">
      <thead>
        <tr class="border-b border-default text-left align-bottom">
          <th class="th">Name</th>
          <th class="th">Kind</th>
          <th class="th">Type</th>
          <th class="th">Rule</th>
          <th class="th">Access</th>
          <th class="th">Unit</th>
          <th v-if="!readonly" class="th" />
        </tr>
      </thead>
      <tbody>
        <template v-for="(m, idx) in members" :key="idx">
          <!-- Member row -->
          <tr
            class="group border-b border-default/60 transition-colors hover:bg-elevated/40"
            :data-member="m.name"
            :class="worst(rowDiags(m.name)) === 'error' ? 'bg-error/5' : worst(rowDiags(m.name)) === 'warning' ? 'bg-warning/5' : ''"
          >
            <!-- Name cell -->
            <td class="td font-medium text-highlighted">
              <span class="inline-flex items-center gap-1.5">
                <button
                  v-if="isMethod(m) && (argCount(m) > 0 || !readonly)"
                  type="button"
                  class="inline-flex size-4 items-center justify-center rounded text-dimmed hover:text-highlighted"
                  :aria-expanded="expanded.has(m.name)"
                  :aria-label="`Toggle arguments for ${m.name}`"
                  @click="toggle(m.name)"
                >
                  <UIcon
                    :name="expanded.has(m.name) ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
                    class="size-4"
                  />
                </button>
                <span v-else class="size-4 shrink-0" />

                <!-- READ-ONLY name -->
                <span v-if="readonly" class="font-mono">{{ m.name }}</span>
                <!-- EDITABLE name -->
                <UInput
                  v-else
                  :model-value="m.name"
                  size="xs"
                  class="font-mono"
                  :data-field="'name'"
                  :aria-label="`Name of ${m.name}`"
                  @change="onNameChange(idx, m, $event)"
                />

                <UIcon
                  v-if="worst(rowDiags(m.name))"
                  :name="worst(rowDiags(m.name)) === 'error' ? 'i-lucide-circle-alert' : 'i-lucide-triangle-alert'"
                  :class="worst(rowDiags(m.name)) === 'error' ? 'text-error' : 'text-warning'"
                  class="size-3.5 shrink-0"
                  :title="rowDiags(m.name)[0]?.message"
                />
              </span>
            </td>

            <!-- Kind cell -->
            <td class="td">
              <DisplayKindBadge v-if="readonly" :kind="m.kind" />
              <USelect
                v-else
                :model-value="m.kind"
                :items="KIND_OPTIONS"
                size="xs"
                :data-field="'kind'"
                :aria-label="`Kind of ${m.name}`"
                @update:model-value="patch(idx, m.name, { kind: $event as Member['kind'] })"
              />
            </td>

            <!-- Type cell (meaningless for methods — the AST has no member.type there) -->
            <td class="td">
              <DisplayTypeRefLabel v-if="readonly" :type-ref="m.type" />
              <span v-else-if="isMethod(m)" class="text-dimmed">—</span>
              <span v-else class="inline-flex items-center gap-1">
                <UInput
                  :model-value="m.type?.raw ?? ''"
                  size="xs"
                  placeholder="TypeRef"
                  :class="ringClass(worst(cellDiags(m.name, 'type')))"
                  :aria-invalid="worst(cellDiags(m.name, 'type')) === 'error' ? true : undefined"
                  :data-field="'type'"
                  :data-testid="`member-type-input-${m.name}`"
                  :aria-label="`Type of ${m.name}`"
                  @change="onTypeChange(idx, m, $event)"
                />
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-library"
                  :aria-label="`Pick companion type for ${m.name}`"
                  title="Use companion type…"
                  @click="openPicker(m.name)"
                />
              </span>
            </td>

            <!-- Rule cell -->
            <td class="td">
              <DisplayRuleBadge v-if="readonly" :rule="m.rule" />
              <USelect
                v-else
                :model-value="m.rule"
                :items="RULE_OPTIONS"
                size="xs"
                :data-field="'rule'"
                :aria-label="`Rule of ${m.name}`"
                @update:model-value="patch(idx, m.name, { rule: $event as Member['rule'] })"
              />
            </td>

            <!-- Access cell (property/variable only) -->
            <td class="td">
              <DisplayAccessBadge v-if="readonly" :access="accessFor(m)" />
              <template v-else>
                <USelect
                  v-if="m.kind === 'property' || m.kind === 'variable'"
                  :model-value="m.access ?? 'r'"
                  :items="ACCESS_OPTIONS"
                  size="xs"
                  :data-field="'access'"
                  :aria-label="`Access of ${m.name}`"
                  @update:model-value="patch(idx, m.name, { access: $event as Member['access'] })"
                />
                <span v-else class="text-dimmed">—</span>
              </template>
            </td>

            <!-- Unit cell -->
            <td class="td">
              <template v-if="readonly">
                <span
                  v-if="m.unit"
                  class="font-mono"
                  :class="worst(cellDiags(m.name, 'unit')) ? 'text-warning' : 'text-toned'"
                >{{ m.unit }}</span>
                <span v-else class="text-dimmed">—</span>
              </template>
              <USelect
                v-else
                :model-value="m.unit ?? undefined"
                :items="unitItems"
                size="xs"
                placeholder="—"
                :class="ringClass(worst(cellDiags(m.name, 'unit')))"
                :aria-invalid="worst(cellDiags(m.name, 'unit')) === 'error' ? true : undefined"
                :data-field="'unit'"
                :aria-label="`Unit of ${m.name}`"
                @update:model-value="patch(idx, m.name, { unit: ($event as string) || undefined })"
              />
            </td>

            <!-- Row affordances (editable only) -->
            <td v-if="!readonly" class="td">
              <span class="inline-flex items-center gap-1">
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-chevron-up"
                  :disabled="idx === 0"
                  :aria-label="`Move ${m.name} up`"
                  @click="emit('reorder', { from: idx, to: idx - 1 })"
                />
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-chevron-down"
                  :disabled="idx === members.length - 1"
                  :aria-label="`Move ${m.name} down`"
                  @click="emit('reorder', { from: idx, to: idx + 1 })"
                />
                <UButton
                  size="xs"
                  color="error"
                  variant="ghost"
                  icon="i-lucide-trash-2"
                  :aria-label="`Remove ${m.name}`"
                  @click="emit('remove', { index: idx, member: m.name })"
                />
              </span>
            </td>
          </tr>

          <!-- Nested placeholder children: └ indented sub-rows -->
          <tr
            v-for="child in m.children ?? []"
            :key="`${m.name}/${child.name}`"
            class="border-b border-default/40 text-toned"
            :data-member-child="`${m.name}/${child.name}`"
          >
            <td class="td">
              <span class="inline-flex items-center gap-1.5 pl-6">
                <UIcon name="i-lucide-corner-down-right" class="size-3.5 text-dimmed" />
                <span class="font-mono">{{ child.browseName || child.name }}</span>
              </span>
            </td>
            <td class="td"><DisplayKindBadge :kind="child.kind" /></td>
            <td class="td"><DisplayTypeRefLabel :type-ref="child.type" /></td>
            <td class="td"><DisplayRuleBadge :rule="child.rule" /></td>
            <td class="td"><DisplayAccessBadge :access="null" /></td>
            <td class="td"><span class="text-dimmed">—</span></td>
            <td v-if="!readonly" class="td" />
          </tr>

          <!-- Method argument panel (in/out) -->
          <tr v-if="isMethod(m) && expanded.has(m.name)" :key="`${m.name}__args`" class="border-b border-default/40">
            <td :colspan="readonly ? 6 : 7" class="bg-elevated/30 px-4 py-3">
              <div class="grid gap-4 sm:grid-cols-2">
                <div>
                  <p class="mb-1.5 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted">
                    <UIcon name="i-lucide-log-in" class="size-3.5" /> Inputs
                  </p>
                  <!-- READ-ONLY: static list -->
                  <template v-if="readonly">
                    <ul v-if="m.in?.length" class="space-y-1">
                      <li v-for="arg in m.in" :key="arg.name" class="flex items-center gap-2 font-mono text-sm">
                        <span class="text-highlighted">{{ arg.name }}</span>
                        <span class="text-dimmed">:</span>
                        <DisplayTypeRefLabel :type-ref="arg.type" />
                      </li>
                    </ul>
                    <p v-else class="text-sm text-dimmed">none</p>
                  </template>
                  <!-- EDITABLE: name + type inputs, add/remove -->
                  <template v-else>
                    <div v-for="(arg, ai) in (m.in ?? [])" :key="ai" class="mb-1.5 flex items-center gap-1.5">
                      <UInput
                        :model-value="arg.name"
                        size="xs"
                        placeholder="name"
                        class="font-mono"
                        :data-testid="`member-arg-name-in-${m.name}-${ai}`"
                        :aria-label="`Input ${ai} name of ${m.name}`"
                        @change="onArgNameChange('in', idx, m, ai, $event)"
                      />
                      <span class="text-dimmed">:</span>
                      <UInput
                        :model-value="arg.type?.raw ?? ''"
                        size="xs"
                        placeholder="TypeRef"
                        class="font-mono"
                        :data-testid="`member-arg-type-in-${m.name}-${ai}`"
                        :aria-label="`Input ${ai} type of ${m.name}`"
                        @change="onArgTypeChange('in', idx, m, ai, $event)"
                      />
                      <UButton
                        size="xs"
                        color="error"
                        variant="ghost"
                        icon="i-lucide-x"
                        :data-testid="`member-arg-remove-in-${m.name}-${ai}`"
                        :aria-label="`Remove input ${ai} of ${m.name}`"
                        @click="removeArg('in', idx, m, ai)"
                      />
                    </div>
                    <UButton
                      size="xs"
                      color="neutral"
                      variant="outline"
                      icon="i-lucide-plus"
                      :data-testid="`member-arg-add-in-${m.name}`"
                      @click="addArg('in', idx, m)"
                    >
                      Add input
                    </UButton>
                  </template>
                </div>
                <div>
                  <p class="mb-1.5 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted">
                    <UIcon name="i-lucide-log-out" class="size-3.5" /> Outputs
                  </p>
                  <!-- READ-ONLY: static list -->
                  <template v-if="readonly">
                    <ul v-if="m.out?.length" class="space-y-1">
                      <li v-for="arg in m.out" :key="arg.name" class="flex items-center gap-2 font-mono text-sm">
                        <span class="text-highlighted">{{ arg.name }}</span>
                        <span class="text-dimmed">:</span>
                        <DisplayTypeRefLabel :type-ref="arg.type" />
                      </li>
                    </ul>
                    <p v-else class="text-sm text-dimmed">none</p>
                  </template>
                  <!-- EDITABLE: name + type inputs, add/remove -->
                  <template v-else>
                    <div v-for="(arg, ao) in (m.out ?? [])" :key="ao" class="mb-1.5 flex items-center gap-1.5">
                      <UInput
                        :model-value="arg.name"
                        size="xs"
                        placeholder="name"
                        class="font-mono"
                        :data-testid="`member-arg-name-out-${m.name}-${ao}`"
                        :aria-label="`Output ${ao} name of ${m.name}`"
                        @change="onArgNameChange('out', idx, m, ao, $event)"
                      />
                      <span class="text-dimmed">:</span>
                      <UInput
                        :model-value="arg.type?.raw ?? ''"
                        size="xs"
                        placeholder="TypeRef"
                        class="font-mono"
                        :data-testid="`member-arg-type-out-${m.name}-${ao}`"
                        :aria-label="`Output ${ao} type of ${m.name}`"
                        @change="onArgTypeChange('out', idx, m, ao, $event)"
                      />
                      <UButton
                        size="xs"
                        color="error"
                        variant="ghost"
                        icon="i-lucide-x"
                        :data-testid="`member-arg-remove-out-${m.name}-${ao}`"
                        :aria-label="`Remove output ${ao} of ${m.name}`"
                        @click="removeArg('out', idx, m, ao)"
                      />
                    </div>
                    <UButton
                      size="xs"
                      color="neutral"
                      variant="outline"
                      icon="i-lucide-plus"
                      :data-testid="`member-arg-add-out-${m.name}`"
                      @click="addArg('out', idx, m)"
                    >
                      Add output
                    </UButton>
                  </template>
                </div>
              </div>
            </td>
          </tr>
        </template>

        <tr v-if="!hasMembers">
          <td :colspan="readonly ? 6 : 7" class="px-4 py-6 text-center text-sm text-muted">
            This type declares no members of its own.
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="!readonly" class="mt-2 px-1">
      <UButton
        size="sm"
        color="neutral"
        variant="outline"
        icon="i-lucide-plus"
        @click="emit('add')"
      >
        Add member
      </UButton>
    </div>
  </div>

  <!-- Companion-type picker modal (editable mode only, teleports to body) -->
  <CatalogPicker
    v-if="!readonly && pickerMember !== null"
    v-model:open="pickerOpen"
    :type-name="typeName"
    :member="pickerMember!"
  />
</template>

<style scoped>
/* Cell rhythm kept in one place so the read-only grid stays dense but legible;
   editable cells inherit the same padding for a seamless swap. */
.th {
  padding: 0.5rem 0.75rem;
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--ui-text-muted);
}
.td {
  padding: 0.5rem 0.75rem;
  vertical-align: middle;
}
</style>
