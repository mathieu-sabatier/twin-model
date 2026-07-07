import { describe, it, expect, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { defineComponent, h } from 'vue'
import FileSelector from '~/components/FileSelector.vue'
import { useDraftStore } from '~/stores/draft'

function makeClient() {
  return {
    createDraft: vi.fn(),
    getDraftModel: vi.fn().mockResolvedValue({ file: 'b.yaml', model: null, diagnostics: [] }),
    getDraftFile: vi.fn(), putFiles: vi.fn(), validate: vi.fn(),
    previewDiagram: vi.fn(), diff: vi.fn(), propose: vi.fn(),
    getUnits: vi.fn(), resolved: vi.fn(),
  } as any
}

describe('FileSelector', () => {
  it('lists files and switching calls setFile', async () => {
    const client = makeClient()
    let store: ReturnType<typeof useDraftStore>
    const Harness = defineComponent({
      setup() { store = useDraftStore(client); store.files = ['a.yaml', 'b.yaml']; store.file = 'a.yaml'; store.draftId = 'd1'; return () => h(FileSelector) },
    })
    const wrapper = await mountSuspended(Harness)
    expect(wrapper.find('[data-testid="file-selector"]').exists()).toBe(true)
    await store!.setFile('b.yaml')
    expect(client.getDraftModel).toHaveBeenCalledWith('d1', 'b.yaml')
    expect(store!.file).toBe('b.yaml')
  })
})
