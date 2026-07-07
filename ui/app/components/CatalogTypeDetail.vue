<script setup lang="ts">
// Center-pane detail for a selected companion type: base chain + resolved
// members, plus the authoring action buttons. Read-only view; actions delegate
// to the draft store (which mutates + re-fetches).
import { ref, watch } from 'vue'
import { useCatalogStore } from '~/stores/catalog'
import { useSelection } from '~/composables/useSelection'
import type { CatalogTypeDetail } from '~/types'

const props = defineProps<{ alias: string; name: string }>()
const catalog = useCatalogStore()
const { select } = useSelection()

const detail = ref<CatalogTypeDetail | null>(null)
const showExtend = ref(false)
const showAddInstance = ref(false)

watch(
  () => [props.alias, props.name],
  async () => { detail.value = await catalog.detailFor(props.alias, props.name) },
  { immediate: true },
)
</script>

<template>
  <div v-if="detail" class="flex h-full flex-col overflow-y-auto p-4">
    <div class="mb-2 flex items-center gap-2">
      <UIcon :name="detail.abstract ? 'i-lucide-square-dashed' : 'i-lucide-box'" class="size-5 text-primary" />
      <h2 class="text-lg font-semibold text-highlighted">{{ detail.alias }}:{{ detail.name }}</h2>
      <UBadge v-if="detail.abstract" size="sm" color="neutral" variant="subtle">abstract</UBadge>
      <UBadge size="sm" color="neutral" variant="outline">{{ detail.nodeClass }}</UBadge>
    </div>

    <div class="mb-4 flex flex-wrap gap-2">
      <UButton size="xs" icon="i-lucide-git-branch-plus" label="Extend as new type" @click="() => { showExtend = true }" />
      <UButton
        v-if="detail.nodeClass === 'ObjectType' && !detail.abstract"
        size="xs" color="neutral" variant="soft" icon="i-lucide-plus"
        label="Add instance" @click="() => { showAddInstance = true }"
      />
    </div>

    <section v-if="detail.baseChain.length" class="mb-4">
      <h3 class="mb-1 text-xs font-semibold uppercase tracking-wide text-muted">Base chain</h3>
      <div class="flex flex-wrap items-center gap-1 text-sm text-dimmed">
        <span>{{ detail.name }}</span>
        <template v-for="b in detail.baseChain" :key="b.uri + b.name">
          <UIcon name="i-lucide-chevron-right" class="size-3" />
          <span>{{ b.alias ? b.alias + ':' : '' }}{{ b.name }}</span>
        </template>
      </div>
    </section>

    <section>
      <h3 class="mb-1 text-xs font-semibold uppercase tracking-wide text-muted">Members ({{ detail.members.length }})</h3>
      <ul class="divide-y divide-default rounded border border-default">
        <li v-for="m in detail.members" :key="m.name" class="flex items-center gap-2 px-3 py-1.5 text-sm">
          <span class="font-medium text-highlighted">{{ m.name }}</span>
          <UBadge size="sm" color="neutral" variant="subtle">{{ m.kind }}</UBadge>
          <UBadge v-if="m.placeholder" size="sm" color="warning" variant="subtle">placeholder</UBadge>
          <UPopover v-if="m.enum && m.enum.members.length" mode="click">
            <UBadge
              size="sm"
              color="neutral"
              variant="outline"
              trailing-icon="i-lucide-chevron-down"
              class="cursor-pointer"
              :data-testid="`member-enum-${m.name}`"
            >
              enum
            </UBadge>
            <template #content>
              <div class="max-h-80 w-56 overflow-y-auto p-1" :data-testid="`member-enum-values-${m.name}`">
                <div
                  v-for="ev in m.enum.members"
                  :key="`${ev.value}-${ev.name}`"
                  class="flex items-center gap-2 rounded px-2 py-1 text-xs"
                >
                  <span class="font-mono text-dimmed">{{ ev.value }}</span>
                  <span class="text-dimmed">·</span>
                  <span class="text-highlighted">{{ ev.name }}</span>
                </div>
              </div>
            </template>
          </UPopover>
          <!-- Companion-typed members link to that type's detail. Primitives/ns0
               types carry no `type` and render nothing here. -->
          <button
            v-if="m.type"
            type="button"
            class="ml-auto inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
            :data-testid="`member-type-link-${m.name}`"
            @click="() => select({ kind: 'catalogType', alias: m.type!.alias, name: m.type!.name })"
          >
            <UIcon name="i-lucide-arrow-right" class="size-3" />
            {{ m.type.alias }}:{{ m.type.name }}
          </button>
        </li>
      </ul>
    </section>

    <CatalogExtendDialog v-model:open="showExtend" :alias="detail.alias" :name="detail.name" :uri="detail.uri" />
    <CatalogAddInstanceDialog v-model:open="showAddInstance" :alias="detail.alias" :name="detail.name" :uri="detail.uri" />
  </div>

  <!-- Loading state while detailFor resolves (cached after first fetch, so this
       is a brief flash only on the first open of a given type). -->
  <div v-else class="flex h-full items-center justify-center gap-2 p-4 text-sm text-muted">
    <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
    <span>Loading type…</span>
  </div>
</template>
