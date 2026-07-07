// Current-selection state for the shell. Small and explicit: the left tree WRITES
// the selection, the center editor and right inspector READ it. Backed by
// Nuxt's useState so it is a single shared singleton across the app (and
// SSG-safe — no localStorage, no module-level mutable ref).
//
// The route owns the draft id (see pages/[[draftId]].vue). Selection is
// deliberately NOT in the route: it is ephemeral view state, cheap to recompute,
// and keeping it out of the URL keeps refresh/deep-link behaviour simple for this
// slice. (If deep-linking to a node is wanted later, this is the one seam to change.)

/** What the user has selected in the tree. `null` = nothing selected (empty state). */
export type Selection =
  | { kind: 'type'; name: string }
  | { kind: 'instance'; name: string }
  | { kind: 'enum'; name: string }
  | { kind: 'catalogType'; alias: string; name: string }
  | { kind: 'perspectiveNode'; perspective: string; node: string }

/**
 * Shared selection singleton. Returns the reactive ref plus a typed setter and a
 * clearer so call sites never poke the ref shape directly.
 */
export function useSelection() {
  const selection = useState<Selection | null>('twinmodel:selection', () => null)

  function select(next: Selection): void {
    selection.value = next
  }

  function clear(): void {
    selection.value = null
  }

  return { selection, select, clear }
}
