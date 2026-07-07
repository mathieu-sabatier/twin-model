<script setup lang="ts">
// Persistent bottom status bar:
//   • draft status  — short draft id + base ref (+ a frozen chip once proposed)
//   • validate state — green "valid" vs "N errors" from the store diagnostics
//   • Propose change… — DISABLED when hasErrors OR frozen (real gate)
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'
import { diagnosticToSelection } from '~/lib/diagnosticToSelection'

const store = useDraftStore()
const router = useRouter()

onMounted(() => {
  void store.loadRepo()
  void store.loadBranchOptions()
})

const { select } = useSelection()
const errorDiags = computed(() => store.diagnostics.filter((d) => d.severity === 'error'))

function goToDiagnostic(d: (typeof errorDiags.value)[number]): void {
  const sel = diagnosticToSelection(d)
  if (sel) select(sel)
}

/** Short, human-friendly draft id (opaque hex from the server). */
const shortId = computed(() => {
  const id = store.draftId
  return id ? id.slice(0, 8) : '—'
})

const repoLabel = computed(() => {
  const r = store.repo
  if (!r) return '—'
  return r.owner && r.repo ? `${r.owner}/${r.repo}` : 'local checkout'
})

const proposeBlockedByHost = computed(() => store.repo !== null && !store.repo.proposeEnabled)

async function onBranchChange(value: string): Promise<void> {
  const id = await store.switchBranch(value)
  if (id) await router.replace({ params: { draftId: id } }).catch(() => {})
}

// Propose gate — the whole point of the bar. Disabled while the draft is lint-red,
// already frozen (proposed), or the host doesn't support opening a PR.
const proposeDisabled = computed(() => store.hasErrors || store.frozen || proposeBlockedByHost.value)
const proposeReason = computed(() => {
  if (store.frozen) return 'This draft has been proposed and is frozen.'
  if (store.hasErrors) return `Resolve ${store.errorCount} error${store.errorCount === 1 ? '' : 's'} before proposing.`
  if (proposeBlockedByHost.value) return store.repo!.proposeReason
  return 'Open a pull request with these changes.'
})

const showPropose = ref(false)
function onPropose(): void {
  if (proposeDisabled.value) return
  showPropose.value = true
}
</script>

<template>
  <div
    class="flex min-h-10 shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-t border-default bg-elevated/50 px-4 py-1 text-xs backdrop-blur"
    data-testid="bottom-bar"
  >
    <!-- Repo context (owner/repo + commit identity + propose availability) -->
    <UPopover v-if="store.repo" mode="click">
      <button
        type="button"
        class="flex cursor-pointer items-center gap-1.5 text-muted hover:text-toned"
        data-testid="repo-chip"
      >
        <UIcon :name="store.repo.url ? 'i-lucide-github' : 'i-lucide-folder-git-2'" class="size-3.5" />
        <span class="font-mono text-toned">{{ repoLabel }}</span>
      </button>
      <template #content>
        <div class="flex w-72 flex-col gap-2 p-3 text-xs" data-testid="repo-popover">
          <a
            v-if="store.repo.url"
            :href="store.repo.url"
            target="_blank"
            class="inline-flex items-center gap-1 font-mono text-primary hover:underline"
          >
            {{ store.repo.owner }}/{{ store.repo.repo }}
            <UIcon name="i-lucide-external-link" class="size-3" />
          </a>
          <p v-else class="font-mono text-toned">local checkout</p>
          <p class="text-muted">
            Commits authored as
            <span class="font-mono text-toned">{{ store.repo.commitName }} &lt;{{ store.repo.commitEmail }}&gt;</span>
          </p>
          <p v-if="!store.repo.proposeEnabled" class="text-warning" data-testid="repo-propose-reason">
            {{ store.repo.proposeReason }}
          </p>
          <p v-else class="text-success">Proposing enabled.</p>
        </div>
      </template>
    </UPopover>

    <!-- Draft status: editable branch picker + short draft id -->
    <div class="flex items-center gap-1.5 text-muted">
      <USelect
        :model-value="store.baseRef"
        :items="store.branchOptions"
        size="xs"
        icon="i-lucide-git-branch"
        class="min-w-44 font-mono"
        data-testid="branch-select"
        @update:model-value="onBranchChange"
      />
      <span class="text-dimmed">·</span>
      <span class="text-dimmed">draft</span>
      <span class="font-mono text-toned">{{ shortId }}</span>
    </div>

    <!-- Propose unavailable cue -->
    <UBadge
      v-if="proposeBlockedByHost"
      color="warning"
      variant="subtle"
      size="sm"
      icon="i-lucide-git-pull-request-closed"
      data-testid="pr-off-chip"
    >
      PR off
    </UBadge>

    <UBadge
      v-if="store.frozen"
      color="info"
      variant="subtle"
      size="sm"
      icon="i-lucide-snowflake"
    >
      frozen
    </UBadge>

    <div class="flex-1" />

    <!-- Validate state -->
    <div class="flex items-center gap-2" data-testid="validate-state" role="status" aria-live="polite">
      <UBadge
        v-if="store.loading && !store.model"
        color="neutral"
        variant="soft"
        size="sm"
        icon="i-lucide-loader-circle"
      >
        loading
      </UBadge>
      <UPopover v-else-if="store.hasErrors" mode="click">
        <UBadge
          color="error"
          variant="subtle"
          size="sm"
          icon="i-lucide-circle-alert"
          class="cursor-pointer"
          data-testid="error-badge"
        >
          {{ store.errorCount }} {{ store.errorCount === 1 ? 'error' : 'errors' }}
        </UBadge>
        <template #content>
          <div class="max-h-80 w-96 overflow-y-auto p-1" data-testid="diagnostics-list">
            <button
              v-for="(d, i) in errorDiags"
              :key="i"
              type="button"
              class="flex w-full cursor-pointer flex-col gap-0.5 rounded px-2 py-1.5 text-left hover:bg-elevated"
              data-testid="diagnostic-row"
              @click="goToDiagnostic(d)"
            >
              <span class="text-xs font-medium text-highlighted">{{ d.message }}</span>
              <span class="font-mono text-[10px] text-dimmed">{{ d.path || d.file }}</span>
            </button>
          </div>
        </template>
      </UPopover>
      <UBadge
        v-else
        color="success"
        variant="subtle"
        size="sm"
        icon="i-lucide-circle-check"
        data-testid="valid-badge"
      >
        valid
      </UBadge>
    </div>

    <!-- Propose -->
    <UTooltip :text="proposeReason">
      <UButton
        color="primary"
        size="sm"
        icon="i-lucide-git-pull-request-arrow"
        :disabled="proposeDisabled"
        data-testid="propose-button"
        @click="onPropose"
      >
        Propose change…
      </UButton>
    </UTooltip>

    <!-- Real propose slideover (RT6 Slice 5). -->
    <ProposeSlideover v-model:open="showPropose" />
  </div>
</template>
