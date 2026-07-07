// Map a Diagnostic to the tree Selection that reveals it, reusing the shared
// path parser. Returns null for model-level / unrecognised paths (nothing to
// navigate to in the tree).
import type { Diagnostic } from '~/types'
import type { Selection } from '~/composables/useSelection'
import { parsePath } from '~/lib/diagnosticPath'

export function diagnosticToSelection(d: Diagnostic): Selection | null {
  const ref = parsePath(d.path)
  switch (ref.scope) {
    case 'type':
    case 'member':
    case 'memberChild':
      return { kind: 'type', name: ref.type }
    case 'instance':
    case 'instanceValue':
    case 'instanceChild':
      return { kind: 'instance', name: ref.instance }
    case 'enum':
      return { kind: 'enum', name: ref.enum }
    default:
      return null
  }
}
