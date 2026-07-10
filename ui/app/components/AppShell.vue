<script setup lang="ts">
// The 3-pane application shell + persistent bottom bar.
//
// Layout: left tree · center editor · right inspector. The BottomBar is hosted
// in the center editor UDashboardPanel's #footer slot (framework-pinned by
// @nuxt/ui, rendered outside the scrollable body) so it can never be squeezed
// out of the flex column. The group's default base (`fixed inset-0 … min-h-svh`)
// is overridden so the panes fill the region between the header and that footer.
// Panes collapse/stack on narrow widths (side panels hide below `lg`).
import { onErrorCaptured, onMounted, onUnmounted, ref } from 'vue'
import { useDraftStore } from '~/stores/draft'
import { useSelection } from '~/composables/useSelection'

const store = useDraftStore()
const { clear } = useSelection()

// A child render throw (e.g. a transient bad state) must never tear down the
// shell and take the BottomBar/Propose button with it. Log and contain.
onErrorCaptured((err) => { console.error('shell child error:', err); return false })

const showTreeDrawer = ref(false)
const showInspectorDrawer = ref(false)
const showPalette = ref(false)
const leftTab = ref<'model' | 'catalog'>('model')

function onKey(e: KeyboardEvent): void {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    showPalette.value = true
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="fixed inset-0 flex flex-col overflow-hidden bg-default">
    <!-- App header -->
    <header class="flex h-11 shrink-0 items-center gap-2.5 border-b border-default px-4">
      <button
        type="button"
        class="flex items-center gap-2.5 rounded-md cursor-pointer transition-opacity hover:opacity-80 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        aria-label="Back to model overview"
        data-testid="brand-home"
        @click="clear"
      >
        <UIcon name="i-lucide-boxes" class="size-5 text-primary" />
        <span class="font-semibold tracking-tight text-highlighted">twinmodel</span>
      </button>
      <span class="hidden text-xs text-dimmed sm:inline">OPC UA type &amp; instance editor</span>
      <UButton
        class="lg:hidden"
        size="xs" color="neutral" variant="ghost" icon="i-lucide-menu"
        aria-label="Open model tree"
        data-testid="mobile-tree-toggle"
        @click="() => { showTreeDrawer = true }"
      />
      <div class="flex-1" />
      <UButton
        size="xs" color="neutral" variant="ghost" icon="i-lucide-search"
        label="Find type"
        aria-label="Find companion type (⌘K)"
        data-testid="catalog-palette-trigger"
        @click="() => { showPalette = true }"
      />
      <UButton
        class="xl:hidden"
        size="xs" color="neutral" variant="ghost" icon="i-lucide-panel-right"
        aria-label="Open YAML"
        data-testid="mobile-inspector-toggle"
        @click="() => { showInspectorDrawer = true }"
      />
      <FileSelector />
    </header>

    <!-- Save-error banner: shown when store.error is set, dismissed by user -->
    <UAlert
      v-if="store.error"
      role="alert"
      aria-live="assertive"
      color="error"
      icon="i-lucide-circle-alert"
      title="Something went wrong saving your change"
      :description="store.error"
      close
      class="shrink-0 rounded-none border-b border-default"
      :close-button="{ 'aria-label': 'Dismiss error' }"
      @update:open="(v: boolean) => { if (!v) store.error = null }"
    />

    <!-- 3-pane region -->
    <UDashboardGroup
      :persistent="false"
      class="relative flex min-h-0 flex-1 overflow-hidden"
      :ui="{ base: '' }"
    >
      <!-- Left: tree -->
      <UDashboardPanel
        id="tree"
        :default-size="22"
        :min-size="16"
        :max-size="34"
        resizable
        class="hidden min-h-0 lg:flex"
        :ui="{ root: 'relative flex flex-col min-w-0 h-full min-h-0 lg:not-last:border-e lg:not-last:border-default shrink-0' }"
      >
        <template #body>
          <div class="flex min-h-0 flex-1 flex-col p-0">
            <div class="flex shrink-0 gap-1 border-b border-default px-2 py-1.5">
              <UButton
                size="xs" :color="leftTab === 'model' ? 'primary' : 'neutral'"
                :variant="leftTab === 'model' ? 'soft' : 'ghost'"
                label="Model" icon="i-lucide-list-tree"
                @click="() => { leftTab = 'model' }"
              />
              <UButton
                size="xs" :color="leftTab === 'catalog' ? 'primary' : 'neutral'"
                :variant="leftTab === 'catalog' ? 'soft' : 'ghost'"
                label="Catalog" icon="i-lucide-library"
                @click="() => { leftTab = 'catalog' }"
              />
            </div>
            <div class="min-h-0 flex-1">
              <TreePane v-if="leftTab === 'model'" />
              <CatalogTree v-else />
            </div>
          </div>
        </template>
      </UDashboardPanel>

      <!-- Center: editor -->
      <UDashboardPanel
        id="editor"
        class="min-h-0 flex-1"
        :ui="{
          root: 'relative flex flex-col min-w-0 h-full min-h-0 lg:not-last:border-e lg:not-last:border-default flex-1',
          body: 'flex flex-col flex-1 overflow-y-auto p-0',
        }"
      >
        <template #body>
          <EditorPane />
        </template>
        <template #footer>
          <BottomBar />
        </template>
      </UDashboardPanel>

      <!-- Right: inspector -->
      <UDashboardPanel
        id="inspector"
        :default-size="26"
        :min-size="18"
        :max-size="40"
        resizable
        class="hidden min-h-0 xl:flex"
        :ui="{
          root: 'relative flex flex-col min-w-0 h-full min-h-0 shrink-0',
          body: 'flex flex-col flex-1 overflow-y-auto p-0',
        }"
      >
        <template #body>
          <InspectorPane />
        </template>
      </UDashboardPanel>
    </UDashboardGroup>

    <USlideover v-model:open="showTreeDrawer" side="left" title="Model">
      <template #body>
        <TreePane @navigate="() => { showTreeDrawer = false }" />
      </template>
    </USlideover>
    <USlideover v-model:open="showInspectorDrawer" side="right" title="YAML">
      <template #body><InspectorPane /></template>
    </USlideover>
    <CatalogPalette v-model:open="showPalette" />
  </div>
</template>
