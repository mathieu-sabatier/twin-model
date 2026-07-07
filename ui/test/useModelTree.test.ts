// Pure unit tests for useModelTree view modes (default/isa95/perspective).
// Direct composable calls with plain data — no store, no mount, no MSW.
import { describe, it, expect } from 'vitest'
import { useModelTree } from '../app/composables/useModelTree'
import type { Instance, Perspective } from '../app/types'

const zero = { file: '', line: 0, col: 0 }
const inst = (name: string, under: string, level?: string): Instance => ({
  pos: zero, name, type: { raw: 'FT', name: 'FT', pos: zero },
  under: under.includes(':') ? { raw: under, alias: under.split(':')[0], name: under.split(':')[1]!, pos: zero }
                             : { raw: under, name: under, pos: zero },
  ...(level ? { level } : {}),
})

it('perspective view groups members under nodes with an Unassigned bucket', () => {
  const instances = [inst('A', 'OpcUa:ObjectsFolder'), inst('B', 'OpcUa:ObjectsFolder'), inst('C', 'OpcUa:ObjectsFolder')]
  const persp: Perspective[] = [{ pos: zero, id: 'zones', nodes: [
    { pos: zero, id: 'n1', label: 'Zone 1', members: ['A'] },
  ] }]
  const { items } = useModelTree(() => [], () => instances, () => [], {
    view: () => ({ perspectiveId: 'zones' }),
    perspectives: () => persp,
  })
  const root = items.value.find((i) => i.value.startsWith('group:perspective'))!
  const labels = (root.children ?? []).map((c) => c.value)
  expect(labels).toContain('pnode:zones:n1')
  expect(labels).toContain('group:unassigned')            // B and C land here
  const unassigned = root.children!.find((c) => c.value === 'group:unassigned')!
  expect(unassigned.children!.map((c) => c.select?.kind === 'instance' && c.select.name)).toEqual(['B', 'C'])
})

it('isa95 view keeps the under-driven nesting', () => {
  const instances = [inst('Site1', 'OpcUa:ObjectsFolder', 'Site'), inst('M', 'Site1')]
  const { items } = useModelTree(() => [], () => instances, () => [], { view: () => 'isa95' })
  const grp = items.value.find((i) => i.value === 'group:instances' || i.value === 'group:isa95')!
  // Site1 is a root; M nests under it.
  const site = grp.children!.find((c) => c.select?.kind === 'instance' && c.select.name === 'Site1')!
  expect(site.children!.some((c) => c.select?.kind === 'instance' && c.select.name === 'M')).toBe(true)
})
