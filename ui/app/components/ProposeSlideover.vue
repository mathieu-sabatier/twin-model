<script setup lang="ts">
// ProposeSlideover — the real "Propose change…" flow (RT6 Slice 5).
// Opens as a USlideover, loads the semantic diff, prefills branch from title,
// lets the user edit title/branch/message, then calls store.propose().
// Success: shows PR link + draft is frozen (gate flips automatically).
// 409 (lint-red): shows a UAlert; store.diagnostics is populated for fields.
import { useDraftStore } from '~/stores/draft'
import { ProposeConflictError, ProposeError } from '~/api'
import { slugBranch } from '~/lib/slug'

const store = useDraftStore()

// v-model:open — standard @nuxt/ui convention.
const open = defineModel<boolean>('open', { default: false })

// ── Form state ───────────────────────────────────────────────────────────────

const title = ref('Update model')
const message = ref('')
/** True once the user has manually edited the branch field — stops auto-sync. */
const branchDirty = ref(false)
const branch = ref(slugBranch(title.value))

const submitting = ref(false)
const conflictError = ref(false)
const submitError = ref<string | null>(null)
const submitErrorDetail = ref<string | null>(null)
/** The PR URL returned on success. */
const prUrl = ref<string | null>(null)

// ── Computed ─────────────────────────────────────────────────────────────────

const submitDisabled = computed(
  () => submitting.value || title.value.trim() === '' || branch.value.trim() === '',
)

// ── Watchers ─────────────────────────────────────────────────────────────────

/** Keep branch in sync with title unless the user has touched the branch field. */
watch(title, (val) => {
  if (!branchDirty.value) {
    branch.value = slugBranch(val)
  }
})

/** When the slideover opens, load the diff and reset form state. */
watch(open, async (val) => {
  if (val) {
    // Reset
    title.value = 'Update model'
    branch.value = slugBranch(title.value)
    branchDirty.value = false
    message.value = ''
    submitting.value = false
    conflictError.value = false
    submitError.value = null
    submitErrorDetail.value = null
    prUrl.value = null
    await store.loadDiff()
  }
})

// ── Actions ──────────────────────────────────────────────────────────────────

async function onSubmit(): Promise<void> {
  if (submitDisabled.value) return
  submitting.value = true
  conflictError.value = false
  submitError.value = null
  try {
    const url = await store.propose({
      branch: branch.value.trim(),
      title: title.value.trim(),
      message: message.value,
    })
    prUrl.value = url
  } catch (err: unknown) {
    if (err instanceof ProposeConflictError) {
      conflictError.value = true
    } else if (err instanceof ProposeError) {
      submitError.value = err.message
      submitErrorDetail.value = err.detail ?? null
    } else {
      submitError.value = err instanceof Error ? err.message : String(err)
      submitErrorDetail.value = null
    }
  } finally {
    submitting.value = false
  }
}

function onBranchInput(val: string): void {
  branch.value = val
  branchDirty.value = true
}

function onClose(): void {
  open.value = false
}
</script>

<template>
  <USlideover v-model:open="open" title="Propose change">
    <template #body>
      <!-- Success view -->
      <div v-if="prUrl" class="flex flex-col items-center gap-4 py-8 text-center">
        <UIcon name="i-lucide-git-pull-request" class="size-10 text-success" />
        <p class="text-sm font-medium text-highlighted">Pull request opened!</p>
        <UButton
          :to="prUrl"
          target="_blank"
          color="primary"
          variant="outline"
          icon="i-lucide-external-link"
          data-testid="pr-link"
        >
          View pull request
        </UButton>
      </div>

      <!-- Form view -->
      <div v-else class="flex flex-col gap-5 py-2">
        <!-- Conflict alert -->
        <UAlert
          v-if="conflictError"
          color="error"
          variant="subtle"
          icon="i-lucide-circle-x"
          title="Draft has lint errors — resolve them and try again."
          data-testid="propose-conflict-alert"
        />

        <!-- Non-lint error alert -->
        <UAlert
          v-if="submitError"
          role="alert"
          color="error"
          variant="subtle"
          icon="i-lucide-circle-x"
          title="Could not open the pull request"
          :description="submitErrorDetail ? undefined : submitError"
          data-testid="propose-error-alert"
        >
          <template v-if="submitErrorDetail" #description>
            <p>{{ submitError }}</p>
            <details class="mt-2" data-testid="propose-error-details">
              <summary class="cursor-pointer text-xs text-muted">Details</summary>
              <pre class="mt-1 overflow-x-auto whitespace-pre-wrap text-[11px] text-toned">{{ submitErrorDetail }}</pre>
            </details>
          </template>
        </UAlert>

        <!-- Diff preview -->
        <div class="flex flex-col gap-1">
          <p class="text-xs font-medium text-muted uppercase tracking-wide">Changes</p>
          <ul
            v-if="store.changes.length"
            class="flex flex-col gap-1 rounded border border-default bg-muted/30 p-3"
            data-testid="diff-list"
          >
            <li
              v-for="(c, i) in store.changes"
              :key="i"
              class="flex gap-2 text-xs text-toned"
            >
              <span class="select-none text-dimmed">•</span>
              <span class="wrap-anywhere">{{ c.text }}</span>
            </li>
          </ul>
          <div v-else class="text-sm text-muted italic">No changes.</div>
        </div>

        <!-- Title -->
        <UFormField label="Title" required>
          <UInput
            v-model="title"
            class="w-full"
            placeholder="Describe this change…"
            data-testid="propose-title"
          />
        </UFormField>

        <!-- Branch -->
        <UFormField label="Branch" required>
          <UInput
            :model-value="branch"
            class="w-full"
            placeholder="model/change"
            font-mono
            data-testid="propose-branch"
            @update:model-value="onBranchInput"
          />
        </UFormField>

        <!-- Commit message -->
        <UFormField label="Commit message">
          <UTextarea
            v-model="message"
            class="w-full"
            placeholder="Optional extended description…"
            :rows="3"
            data-testid="propose-message"
          />
        </UFormField>
      </div>
    </template>

    <template #footer>
      <div v-if="!prUrl" class="flex justify-end gap-2">
        <UButton
          color="neutral"
          variant="ghost"
          @click="onClose"
        >
          Cancel
        </UButton>
        <UButton
          color="primary"
          :disabled="submitDisabled"
          :loading="submitting"
          icon="i-lucide-git-pull-request-arrow"
          data-testid="propose-submit"
          @click="onSubmit"
        >
          Propose change
        </UButton>
      </div>
      <div v-else class="flex justify-end">
        <UButton color="neutral" variant="ghost" @click="onClose">Close</UButton>
      </div>
    </template>
  </USlideover>
</template>
