// Builds the left-tree item structure from the committed model in the store.
// Two roots:
//   • Types      → object types (abstract flagged) + an Enums sub-group
//   • Instances  → topology nested by `under` (an instance whose `under` names a
//                  declared instance is that instance's child; top-level ones are
//                  those whose `under` is an import target, e.g. OpcUa:ObjectsFolder)
//
// This mirrors the Go validator's `under` reconciliation: an unprefixed name that
// matches a declared instance = nesting; otherwise (prefixed/import target) it is
// a root. See internal/dsl checkInstances.
//
// Each leaf carries a `select` payload (the Selection to set on click) and a
// stable `value` key so the UTree can track selection. Pure derivation from the
// store getters — no side effects, recomputed reactively.
import { computed } from 'vue'
import type { TreeItem } from '@nuxt/ui'
import type { Instance, ObjectType, Enum, Perspective, PerspectiveNode } from '~/types'
import type { Selection } from '~/composables/useSelection'
import type { DiagnosticIndex } from '~/lib/diagnosticPath'
import { isa95Icon as isa95LevelIcon } from '~/lib/isa95'

/** Which nesting the Instances root should render. */
export type View = 'default' | 'isa95' | { perspectiveId: string }

/** Optional view-mode controls, kept decoupled from Pinia for testability. */
export interface UseModelTreeOpts {
  view?: () => View
  perspectives?: () => Perspective[]
  /** Reactive diagnostic index — tree nodes with an error diagnostic get a marker. */
  diagnosticIndex?: () => DiagnosticIndex
}

/** Trailing marker for a node whose any lookup group carries an error diagnostic
 *  (instance field / level / perspective-member). Undefined = no marker. */
function errorBadge(...groups: readonly { severity: string }[][]): string | undefined {
  return groups.some((g) => g.some((d) => d.severity === 'error')) ? 'i-lucide-circle-alert' : undefined
}

/** A tree node augmented with the selection it represents (undefined for group headers). */
export interface ModelTreeItem extends TreeItem {
  /** Stable unique key used for selection tracking + testing hooks. */
  value: string
  /** The selection to apply when this node is activated. Group headers omit it. */
  select?: Selection
  children?: ModelTreeItem[]
}

/** Does this instance's `under` point at another *declared* instance (→ nesting)? */
function parentInstanceName(inst: Instance, declared: Set<string>): string | null {
  const under = inst.under
  // A prefixed ref (alias present) is always an import target, never nesting.
  if (under.alias) return null
  // An unprefixed name that matches a declared instance = nested under it.
  return declared.has(under.name) ? under.name : null
}

function typeNode(t: ObjectType): ModelTreeItem {
  return {
    value: `type:${t.name}`,
    label: t.name,
    icon: t.abstract ? 'i-lucide-shapes' : 'i-lucide-box',
    // Abstract types read as a template, not something you instantiate — surface it.
    trailingIcon: t.abstract ? 'i-lucide-asterisk' : undefined,
    select: { kind: 'type', name: t.name },
  }
}

function enumNode(e: Enum): ModelTreeItem {
  return {
    value: `enum:${e.name}`,
    label: e.name,
    icon: 'i-lucide-list',
    select: { kind: 'enum', name: e.name },
  }
}

/** Default per-instance icon: every instance renders as equipment/boxes. */
function defaultIcon(): string {
  return 'i-lucide-boxes'
}

/** ISA-95 icon: graded per organizational tier (enterprise → site → area →
 *  factory segment → work unit), equipment falls through to boxes. Mapping lives
 *  in the shared isa95 table (single source with the validator/export). */
function isa95Icon(inst: Instance): string {
  return isa95LevelIcon(inst.level)
}

/** Recursively build an instance node and its nested children. */
function instanceNode(
  inst: Instance,
  childrenOf: Map<string, Instance[]>,
  iconFor: (inst: Instance) => string = defaultIcon,
  diag?: DiagnosticIndex,
): ModelTreeItem {
  const kids = childrenOf.get(inst.name) ?? []
  return {
    value: `instance:${inst.name}`,
    label: inst.name,
    icon: iconFor(inst),
    trailingIcon: diag ? errorBadge(diag.forInstance(inst.name), diag.forInstanceLevel(inst.name)) : undefined,
    select: { kind: 'instance', name: inst.name },
    defaultExpanded: kids.length > 0,
    children: kids.length ? kids.map((k) => instanceNode(k, childrenOf, iconFor, diag)) : undefined,
  }
}

// Perspective node id -> ModelTreeItem, recursing through children; leaves list
// member instances. `assigned` collects every instance placed anywhere.
function perspectiveNode(
  p: Perspective, node: PerspectiveNode, byId: Map<string, PerspectiveNode>,
  instByName: Map<string, Instance>, assigned: Set<string>, diag?: DiagnosticIndex,
): ModelTreeItem {
  const childItems = (node.children ?? []).map((cid) => {
    const c = byId.get(cid)
    return c ? perspectiveNode(p, c, byId, instByName, assigned, diag) : undefined
  }).filter(Boolean) as ModelTreeItem[]
  const memberItems = (node.members ?? []).flatMap((mid) => {
    assigned.add(mid)
    const i = instByName.get(mid)
    return i ? [{
      value: `instance:${mid}`, label: mid, icon: 'i-lucide-boxes',
      trailingIcon: diag ? errorBadge(diag.forInstance(mid), diag.forInstanceLevel(mid), diag.forPerspectiveMember(p.id, node.id, mid)) : undefined,
      select: { kind: 'instance', name: mid } as const,
    }] : []
  })
  return {
    value: `pnode:${p.id}:${node.id}`,
    label: node.label || node.id,
    icon: 'i-lucide-folder',
    select: { kind: 'perspectiveNode', perspective: p.id, node: node.id },
    defaultExpanded: true,
    children: [...childItems, ...memberItems],
  }
}

function perspectiveGroup(p: Perspective, insts: Instance[], diag?: DiagnosticIndex): ModelTreeItem {
  const byId = new Map((p.nodes ?? []).map((n) => [n.id, n]))
  const instByName = new Map(insts.map((i) => [i.name, i]))
  const childOf = new Set((p.nodes ?? []).flatMap((n) => n.children ?? []))
  const roots = (p.nodes ?? []).filter((n) => !childOf.has(n.id))
  const assigned = new Set<string>()
  const nodeItems = roots.map((n) => perspectiveNode(p, n, byId, instByName, assigned, diag))
  const unassigned = insts.filter((i) => !assigned.has(i.name))
  const children = [...nodeItems]
  if (unassigned.length) {
    children.push({
      value: 'group:unassigned',
      label: `Unassigned (${unassigned.length})`,
      icon: 'i-lucide-inbox',
      children: unassigned.map((i) => ({
        value: `instance:${i.name}`, label: i.name, icon: 'i-lucide-boxes',
        trailingIcon: diag ? errorBadge(diag.forInstance(i.name), diag.forInstanceLevel(i.name)) : undefined,
        select: { kind: 'instance', name: i.name } as const,
      })),
    })
  }
  return { value: `group:perspective:${p.id}`, label: p.label || p.id, icon: 'i-lucide-layers', defaultExpanded: true, children }
}

/**
 * useModelTree — reactive left-tree items derived from the model getters.
 * Pass the reactive getters from the store (kept decoupled from Pinia for testability).
 */
export function useModelTree(
  objectTypes: () => ObjectType[],
  instances: () => Instance[],
  enums: () => Enum[],
  opts?: UseModelTreeOpts,
) {
  const items = computed<ModelTreeItem[]>(() => {
    const types = objectTypes()
    const insts = instances()
    const enumList = enums()
    const view: View = opts?.view?.() ?? 'default'
    const diag = opts?.diagnosticIndex?.()

    // ── Types root ─────────────────────────────────────────────────────────────
    const typeChildren: ModelTreeItem[] = types.map(typeNode)

    // ── Instance topology by `under` ───────────────────────────────────────────
    // Shared by the `default` and `isa95` views — both nest by the same `under`
    // reconciliation; `isa95` only changes the per-node icon.
    const declared = new Set(insts.map((i) => i.name))
    const childrenOf = new Map<string, Instance[]>()
    const roots: Instance[] = []
    for (const inst of insts) {
      const parent = parentInstanceName(inst, declared)
      if (parent) {
        const list = childrenOf.get(parent) ?? []
        list.push(inst)
        childrenOf.set(parent, list)
      } else {
        roots.push(inst)
      }
    }

    // Three top-level peers: Types, Enums (only when present), Instances.
    const groups: ModelTreeItem[] = [
      {
        value: 'group:types',
        label: 'Types',
        icon: 'i-lucide-shapes',
        defaultExpanded: true,
        children: typeChildren,
      },
    ]
    if (enumList.length) {
      groups.push({
        value: 'group:enums',
        label: 'Enums',
        icon: 'i-lucide-hash',
        defaultExpanded: false,
        children: enumList.map(enumNode),
      })
    }

    if (typeof view === 'object') {
      // Perspective view: the Instances root is replaced by the perspective's
      // node tree + an Unassigned bucket for instances no node claims.
      const perspectives = opts?.perspectives?.() ?? []
      const found = perspectives.find((p) => p.id === view.perspectiveId)
      if (found) {
        groups.push(perspectiveGroup(found, insts, diag))
      }
      return groups
    }

    // `default` and `isa95` share the `group:instances` value (and the same
    // under-driven nesting) so selection keys stay stable across the two —
    // isa95 only swaps the icon for organizational-tier instances.
    const iconFor = view === 'isa95' ? isa95Icon : defaultIcon
    groups.push({
      value: 'group:instances',
      label: 'Instances',
      icon: 'i-lucide-network',
      defaultExpanded: true,
      children: roots.length
        ? roots.map((r) => instanceNode(r, childrenOf, iconFor, diag))
        : undefined,
    })
    return groups
  })

  return { items }
}
