// Pure Path→field registry for Diagnostic.path values. No Vue, no Nuxt.
//
// The Diagnostic.path is a slash-separated path that anchors a diagnostic to a
// specific field in the model AST. This module maps those paths to structured
// `FieldRef` descriptors so any form cell can ask "are there diagnostics for me?"
// in O(1) via a DiagnosticIndex built by `indexDiagnostics`.
//
// Grammar (from API-CONTRACT.md):
//   model/<field>
//   enums/<E>[/values/<V>]
//   object_types/<T>[/base]
//   object_types/<T>/members/<M>[/kind|rule|access|type|unit]
//   object_types/<T>/members/<M>/children/<C>
//   object_types/<T>/members/<M>/in/<i>/type
//   object_types/<T>/members/<M>/out/<i>/type
//   instances/<I>[/type|under|level]
//   instances/<I>/values/<M>
//   instances/<I>/children/<C>
//   perspectives/<P>/nodes/<N>/members/<M>
import type { Diagnostic } from '~/types'

// ── FieldRef ─────────────────────────────────────────────────────────────────

/**
 * Discriminated union describing what a Diagnostic.path points at. One variant
 * per structural concept in the grammar. `unknown` is the fallback for paths
 * the parser doesn't recognise (e.g., paths added by a future server version).
 */
export type FieldRef =
  | { scope: 'model'; field: string }
  | { scope: 'enum'; enum: string; value?: string }
  | { scope: 'type'; type: string; field?: 'base' }
  | {
      scope: 'member'
      type: string
      member: string
      field?: 'kind' | 'rule' | 'access' | 'type' | 'unit'
      argDir?: 'in' | 'out'
      argIndex?: number
    }
  | { scope: 'memberChild'; type: string; member: string; child: string }
  | { scope: 'instance'; instance: string; field?: 'type' | 'under' }
  | { scope: 'instanceValue'; instance: string; member: string }
  | { scope: 'instanceChild'; instance: string; child: string }
  | { scope: 'instanceLevel'; instance: string }
  | { scope: 'perspectiveMember'; perspective: string; node: string; member: string }
  | { scope: 'unknown'; raw: string }

// ── parsePath ─────────────────────────────────────────────────────────────────

/**
 * Parse a Diagnostic.path into a structured FieldRef.
 * Robust to any segment combination in the grammar; falls back to
 * `{ scope: 'unknown', raw }` for anything unrecognised.
 */
export function parsePath(path: string): FieldRef {
  if (!path) return { scope: 'unknown', raw: path }

  const parts = path.split('/')

  // model/<field>
  if (parts[0] === 'model' && parts.length === 2) {
    return { scope: 'model', field: parts[1]! }
  }

  // enums/<E>  or  enums/<E>/values/<V>
  if (parts[0] === 'enums' && parts[1]) {
    if (parts.length === 2) {
      return { scope: 'enum', enum: parts[1] }
    }
    if (parts.length === 4 && parts[2] === 'values' && parts[3]) {
      return { scope: 'enum', enum: parts[1], value: parts[3] }
    }
  }

  // object_types/<T>...
  if (parts[0] === 'object_types' && parts[1]) {
    const type = parts[1]

    // object_types/<T>  (type row)
    if (parts.length === 2) {
      return { scope: 'type', type }
    }

    // object_types/<T>/base
    if (parts.length === 3 && parts[2] === 'base') {
      return { scope: 'type', type, field: 'base' }
    }

    // object_types/<T>/members/<M>...
    if (parts[2] === 'members' && parts[3]) {
      const member = parts[3]

      // object_types/<T>/members/<M>  (member row)
      if (parts.length === 4) {
        return { scope: 'member', type, member }
      }

      // object_types/<T>/members/<M>/<field>   (kind|rule|access|type|unit)
      if (parts.length === 5) {
        const seg = parts[4]!
        if (seg === 'kind' || seg === 'rule' || seg === 'access' || seg === 'type' || seg === 'unit') {
          return { scope: 'member', type, member, field: seg }
        }
        // object_types/<T>/members/<M>/children/<C>  — length 6, but check here too
      }

      // object_types/<T>/members/<M>/children/<C>
      if (parts.length === 6 && parts[4] === 'children' && parts[5]) {
        return { scope: 'memberChild', type, member, child: parts[5] }
      }

      // object_types/<T>/members/<M>/in/<i>/type
      // object_types/<T>/members/<M>/out/<i>/type
      if (parts.length === 7 && parts[6] === 'type') {
        const dir = parts[4]
        const idx = parts[5]
        if ((dir === 'in' || dir === 'out') && idx !== undefined) {
          const argIndex = parseInt(idx, 10)
          if (!isNaN(argIndex)) {
            return { scope: 'member', type, member, field: 'type', argDir: dir, argIndex }
          }
        }
      }
    }
  }

  // instances/<I>...
  if (parts[0] === 'instances' && parts[1]) {
    const instance = parts[1]

    // instances/<I>  (instance row)
    if (parts.length === 2) {
      return { scope: 'instance', instance }
    }

    // instances/<I>/type  or  instances/<I>/under
    if (parts.length === 3) {
      const seg = parts[2]!
      if (seg === 'type' || seg === 'under') {
        return { scope: 'instance', instance, field: seg }
      }
      // instances/<I>/level
      if (seg === 'level') {
        return { scope: 'instanceLevel', instance }
      }
    }

    // instances/<I>/values/<M>
    if (parts.length === 4 && parts[2] === 'values' && parts[3]) {
      return { scope: 'instanceValue', instance, member: parts[3] }
    }

    // instances/<I>/children/<C>
    if (parts.length === 4 && parts[2] === 'children' && parts[3]) {
      return { scope: 'instanceChild', instance, child: parts[3] }
    }
  }

  // perspectives/<P>/nodes/<N>/members/<M>
  if (
    parts[0] === 'perspectives' &&
    parts[1] &&
    parts.length === 6 &&
    parts[2] === 'nodes' &&
    parts[3] &&
    parts[4] === 'members' &&
    parts[5]
  ) {
    return { scope: 'perspectiveMember', perspective: parts[1], node: parts[3], member: parts[5] }
  }

  return { scope: 'unknown', raw: path }
}

// ── DiagnosticIndex ───────────────────────────────────────────────────────────

/**
 * O(1)-lookup index built from a Diagnostic[]. Keyed by a canonical string so
 * each form cell can look up its own diagnostics without scanning the array.
 * All lookup methods return a (possibly empty) Diagnostic[].
 */
export interface DiagnosticIndex {
  /** All diagnostics anchored to a specific model-level field (e.g. 'name'). */
  forModelField(field: string): Diagnostic[]
  /** All diagnostics anchored to any field of the given type (incl. its members). */
  forType(type: string): Diagnostic[]
  /** All diagnostics anchored to a member row or any of its fields/children. */
  forMember(type: string, member: string): Diagnostic[]
  /** All diagnostics anchored to a specific member cell (e.g. 'unit'). */
  forMemberField(type: string, member: string, field: string): Diagnostic[]
  /** All diagnostics anchored to any field of the given instance. */
  forInstance(instance: string): Diagnostic[]
  /** All diagnostics anchored to a specific instance-value assignment. */
  forInstanceValue(instance: string, member: string): Diagnostic[]
  /** All diagnostics anchored to an instance's `level` field. */
  forInstanceLevel(instance: string): Diagnostic[]
  /** All diagnostics anchored to a specific perspective-node membership entry. */
  forPerspectiveMember(perspective: string, node: string, member: string): Diagnostic[]
  /**
   * All diagnostics anchored to an enum row or any of its values.
   * If `value` is supplied, returns only diagnostics for that specific enum value;
   * otherwise returns diagnostics for the enum and all its values (broaden).
   */
  forEnum(enumName: string, value?: string): Diagnostic[]
}

/** Canonical key builders — one per lookup dimension. */
function modelKey(field: string): string { return `model:${field}` }
function typeKey(type: string): string { return `type:${type}` }
function memberKey(type: string, member: string): string { return `member:${type}:${member}` }
function memberFieldKey(type: string, member: string, field: string): string {
  return `memberField:${type}:${member}:${field}`
}
function instanceKey(instance: string): string { return `instance:${instance}` }
function instanceValueKey(instance: string, member: string): string {
  return `instanceValue:${instance}:${member}`
}
function enumKey(enumName: string): string { return `enum:${enumName}` }
function enumValueKey(enumName: string, value: string): string { return `enumValue:${enumName}:${value}` }
function instanceLevelKey(instance: string): string { return `instanceLevel:${instance}` }
function perspectiveMemberKey(perspective: string, node: string, member: string): string {
  return `perspectiveMember:${perspective}:${node}:${member}`
}

/**
 * Build a DiagnosticIndex from a diagnostics array. Pure — no side effects.
 * Each diagnostic is inserted under every applicable key so lookups broaden or
 * narrow naturally (e.g. `forMember` also returns diagnostics that are on a
 * specific cell of that member).
 */
export function indexDiagnostics(diags: Diagnostic[]): DiagnosticIndex {
  const map = new Map<string, Diagnostic[]>()

  function add(key: string, d: Diagnostic): void {
    let list = map.get(key)
    if (!list) {
      list = []
      map.set(key, list)
    }
    list.push(d)
  }

  for (const d of diags) {
    const ref = parsePath(d.path)

    switch (ref.scope) {
      case 'model':
        add(modelKey(ref.field), d)
        break

      case 'enum':
        // Index under the enum key; if it's a value diagnostic, also under the value key.
        // Broaden: a value diagnostic is visible at the enum level (same as member→type).
        add(enumKey(ref.enum), d)
        if (ref.value) {
          add(enumValueKey(ref.enum, ref.value), d)
        }
        break

      case 'type':
        add(typeKey(ref.type), d)
        break

      case 'member': {
        // Index under type, member row, and (if present) the specific field.
        add(typeKey(ref.type), d)
        add(memberKey(ref.type, ref.member), d)
        if (ref.field) {
          add(memberFieldKey(ref.type, ref.member, ref.field), d)
        }
        break
      }

      case 'memberChild':
        add(typeKey(ref.type), d)
        add(memberKey(ref.type, ref.member), d)
        break

      case 'instance':
        add(instanceKey(ref.instance), d)
        break

      case 'instanceValue':
        add(instanceKey(ref.instance), d)
        add(instanceValueKey(ref.instance, ref.member), d)
        break

      case 'instanceChild':
        add(instanceKey(ref.instance), d)
        break

      case 'instanceLevel':
        add(instanceLevelKey(ref.instance), d)
        break

      case 'perspectiveMember':
        add(perspectiveMemberKey(ref.perspective, ref.node, ref.member), d)
        break

      case 'unknown':
        // Unrecognised paths are silently ignored (no lookup target).
        break
    }
  }

  const get = (key: string): Diagnostic[] => map.get(key) ?? []

  return {
    forModelField: (field) => get(modelKey(field)),
    forType: (type) => get(typeKey(type)),
    forMember: (type, member) => get(memberKey(type, member)),
    forMemberField: (type, member, field) => get(memberFieldKey(type, member, field)),
    forInstance: (instance) => get(instanceKey(instance)),
    forInstanceValue: (instance, member) => get(instanceValueKey(instance, member)),
    forInstanceLevel: (instance) => get(instanceLevelKey(instance)),
    forPerspectiveMember: (perspective, node, member) =>
      get(perspectiveMemberKey(perspective, node, member)),
    forEnum: (enumName, value) =>
      value ? get(enumValueKey(enumName, value)) : get(enumKey(enumName)),
  }
}
