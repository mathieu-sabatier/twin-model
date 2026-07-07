import type { ObjectType } from '~/types'

/**
 * Valid base-type options for `selfName`: every other local object type, minus
 * any that (transitively) derive from `selfName` — offering those as a base would
 * create an inheritance cycle. A non-local current base ref (e.g. an import alias
 * like `OpcUa:BaseObjectType`) is prepended so the select keeps its current value.
 * QA finding M4.
 */
export function computeBaseOptions(
  types: readonly Pick<ObjectType, 'name' | 'base'>[],
  selfName: string,
  currentRaw?: string,
): string[] {
  const byName = new Map(types.map((t) => [t.name, t]))

  function derivesFrom(name: string): boolean {
    const seen = new Set<string>()
    let cur = byName.get(name)
    while (cur?.base && !cur.base.alias && !seen.has(cur.name)) {
      seen.add(cur.name)
      if (cur.base.name === selfName) return true
      cur = byName.get(cur.base.name)
    }
    return false
  }

  const ownTypes = types
    .map((t) => t.name)
    .filter((n) => n !== selfName && !derivesFrom(n))
  const extras = currentRaw && !ownTypes.includes(currentRaw) ? [currentRaw] : []
  return [...extras, ...ownTypes]
}
