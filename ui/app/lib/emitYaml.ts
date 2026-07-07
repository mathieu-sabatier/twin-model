// Client-side YAML emitter — mirrors internal/dsl/format.go in TypeScript.
//
// Produces parseable block-style YAML that the Go server can ingest and
// re-canonicalize. The server owns canonical form: after every PUT the UI
// re-fetches the server's AST (determinism contract). So this emitter only
// needs structural fidelity, not byte-exact output.
//
// Key mapping: camelCase TS → snake_case DSL YAML
//   model.publicationDate → publication_date
//   model.objectTypes     → object_types
//   All other keys are identical.
//
// Pure module: NO Nuxt/Vue imports.
import type {
  Model,
  Member,
  Instance,
  InstanceChild,
  Argument,
  TypeRef,
} from '~/types'

// ── Scalar quoting (mirrors format.go scalar/needsQuote) ────────────────────

function needsQuote(v: string): boolean {
  if (v !== v.trim()) return true
  switch (v) {
    case 'true':
    case 'false':
    case 'null':
    case 'yes':
    case 'no':
    case '~':
      return true
  }
  if (!isNaN(Number(v)) && v !== '') return true
  if (/[:#{}[\]&*!|>'"% @`,]/.test(v)) return true
  return false
}

function scalar(v: string): string {
  if (v === '') return '""'
  if (needsQuote(v)) return JSON.stringify(v)
  return v
}

function quoteKey(k: string): string {
  if (k.includes('<') || k.includes('>')) return JSON.stringify(k)
  return k
}

// ── Builder ──────────────────────────────────────────────────────────────────

function indent(n: number): string {
  return '  '.repeat(n)
}

function kv(n: number, key: string, val: string): string {
  return `${indent(n)}${key}: ${scalar(val)}\n`
}

function line(n: number, s: string): string {
  return `${indent(n)}${s}\n`
}

// ── Member inline fields (mirrors Member.inlineFields in format.go) ──────────

function memberInlineFields(mem: Member): string[] {
  const f: string[] = []
  if (mem.kind !== 'variable') f.push(`kind: ${mem.kind}`)
  if (mem.type) f.push(`type: ${mem.type.raw}`)
  if (mem.rule !== 'mandatory') f.push(`rule: ${mem.rule}`)
  if ((mem.kind === 'variable' || mem.kind === 'property') && mem.access === 'rw') {
    f.push(`access: ${mem.access}`)
  }
  if (mem.unit !== undefined && mem.unit !== '') f.push(`unit: ${scalar(mem.unit)}`)
  if (mem.doc) f.push(`doc: ${scalar(mem.doc)}`)
  return f
}

// Reconstruct the member key: for placeholders, build Name<Suffix> from
// member.name + member.browseName (strips <> from browseName, drops leading
// member.name to get suffix).
function memberKey(mem: Member): string {
  if (mem.rule === 'optional_placeholder' || mem.rule === 'mandatory_placeholder') {
    if (mem.browseName) {
      const inner = mem.browseName.replace(/^</, '').replace(/>$/, '')
      const suffix = inner.startsWith(mem.name) ? inner.slice(mem.name.length) : inner
      return `${mem.name}<${suffix}>`
    }
  }
  return mem.name
}

function emitMembers(n: number, members: Member[]): string {
  let out = ''
  for (const mem of members) {
    const key = memberKey(mem)
    const fields = memberInlineFields(mem)
    const hasExpandedBlock = (mem.children && mem.children.length > 0) ||
      (mem.in && mem.in.length > 0) ||
      (mem.out && mem.out.length > 0)

    if (!hasExpandedBlock) {
      out += line(n, `${quoteKey(key)}: { ${fields.join(', ')} }`)
    } else {
      out += line(n, `${quoteKey(key)}:`)
      for (const f of fields) {
        out += line(n + 1, f)
      }
      if (mem.in && mem.in.length > 0) {
        out += line(n + 1, 'in:')
        for (const a of mem.in) {
          out += emitArgument(n + 2, a)
        }
      }
      if (mem.out && mem.out.length > 0) {
        out += line(n + 1, 'out:')
        for (const a of mem.out) {
          out += emitArgument(n + 2, a)
        }
      }
      if (mem.children && mem.children.length > 0) {
        out += line(n + 1, 'children:')
        out += emitMembers(n + 2, mem.children)
      }
    }
  }
  return out
}

function emitArgument(n: number, a: Argument): string {
  return line(n, `- { name: ${a.name}, type: ${a.type.raw} }`)
}

// ── Instance value scalar typing (mirrors dsl format.go valueLiteral) ─────────

const NUMERIC_BUILTINS = new Set([
  'SByte', 'Byte', 'Int16', 'UInt16', 'Int32', 'UInt32',
  'Int64', 'UInt64', 'Float', 'Double', 'Number', 'Integer',
  'UInteger', 'Duration',
])

/** Resolve a value member's declared type by walking the local base chain
 *  (mirror of dsl.ResolveMembers). Returns undefined if not found. */
function valueMemberType(model: Model, typeName: string, member: string): TypeRef | undefined {
  const byName = new Map((model.objectTypes ?? []).map((t) => [t.name, t]))
  const seen = new Set<string>()
  let cur = byName.get(typeName)
  while (cur && !seen.has(cur.name)) {
    seen.add(cur.name)
    const m = cur.members?.find((x) => x.name === member)
    if (m?.type) return m.type
    cur = cur.base && !cur.base.alias ? byName.get(cur.base.name) : undefined
  }
  return undefined
}

/** True when the value should be emitted verbatim (numeric/boolean/local enum). */
function verbatimValue(model: Model, t: TypeRef | undefined, raw: string): boolean {
  if (!t || raw === '' || t.alias) return false
  if (t.name === 'Boolean' || NUMERIC_BUILTINS.has(t.name)) return true
  return (model.enums ?? []).some((e) => e.name === t.name)
}

function emitInstance(n: number, inst: Instance, model: Model): string {
  let out = line(n, `${inst.name}:`)
  out += line(n + 1, `type: ${inst.type.raw}`)
  out += line(n + 1, `under: ${inst.under.raw}`)
  if (inst.level) {
    out += line(n + 1, `level: ${inst.level}`)
  }
  if (inst.values && inst.values.length > 0) {
    out += line(n + 1, 'values:')
    for (const v of inst.values) {
      const t = valueMemberType(model, inst.type.name, v.member)
      const lit = verbatimValue(model, t, v.raw) ? v.raw : scalar(v.raw)
      out += line(n + 2, `${v.member}: ${lit}`)
    }
  }
  if (inst.children && inst.children.length > 0) {
    out += line(n + 1, 'children:')
    for (const ch of inst.children) {
      out += emitInstanceChild(n + 2, ch)
    }
  }
  return out
}

function emitInstanceChild(n: number, ch: InstanceChild): string {
  return line(n, `${ch.name}: { of: ${scalar(ch.of.raw)} }`)
}

// ── Public API ───────────────────────────────────────────────────────────────

/**
 * Emit a Model as parseable canonical-ish YAML.
 *
 * Maps camelCase TS properties to snake_case DSL keys:
 *   publicationDate → publication_date
 *   objectTypes     → object_types
 *
 * The server re-canonicalizes on every PUT (determinism contract); this emitter
 * only needs to produce YAML the Go parser can ingest correctly.
 */
export function emitModel(model: Model): string {
  let out = ''

  // ── model header ──────────────────────────────────────────────────────────
  out += line(0, 'model:')
  out += kv(1, 'name', model.name)
  out += kv(1, 'namespace', model.namespace)
  if (model.prefix) {
    out += kv(1, 'prefix', model.prefix)
  }
  out += kv(1, 'version', model.version)
  out += kv(1, 'publication_date', model.publicationDate)

  // ── hierarchy ─────────────────────────────────────────────────────────────
  if (model.hierarchy) {
    out += '\n'
    out += line(0, `hierarchy: { allowLevelSkip: ${model.hierarchy.allowLevelSkip} }`)
  }

  // ── imports ───────────────────────────────────────────────────────────────
  if (model.imports && model.imports.length > 0) {
    out += '\n'
    out += line(0, 'imports:')
    for (const im of model.imports) {
      out += kv(1, im.alias, im.uri)
    }
  }

  // ── enums ─────────────────────────────────────────────────────────────────
  if (model.enums && model.enums.length > 0) {
    out += '\n'
    out += line(0, 'enums:')
    for (const e of model.enums) {
      out += line(1, `${e.name}:`)
      if (e.doc) {
        out += kv(2, 'doc', e.doc)
      }
      out += line(2, 'values:')
      for (const val of e.values) {
        if (val.explicit) {
          out += line(3, `- { ${val.name}: ${val.identifier} }`)
        } else {
          out += line(3, `- ${val.name}`)
        }
      }
    }
  }

  // ── object_types ──────────────────────────────────────────────────────────
  if (model.objectTypes && model.objectTypes.length > 0) {
    out += '\n'
    out += line(0, 'object_types:')
    for (const ot of model.objectTypes) {
      out += line(1, `${ot.name}:`)
      if (ot.doc) {
        out += kv(2, 'doc', ot.doc)
      }
      if (ot.base) {
        out += line(2, `base: ${ot.base.raw}`)
      }
      if (ot.abstract) {
        out += line(2, 'abstract: true')
      }
      if (ot.members && ot.members.length > 0) {
        out += line(2, 'members:')
        out += emitMembers(3, ot.members)
      }
    }
  }

  // ── instances ─────────────────────────────────────────────────────────────
  if (model.instances && model.instances.length > 0) {
    out += '\n'
    out += line(0, 'instances:')
    for (const inst of model.instances) {
      out += emitInstance(1, inst, model)
    }
  }

  // ── perspectives ──────────────────────────────────────────────────────────
  if (model.perspectives && model.perspectives.length > 0) {
    out += '\n'
    out += line(0, 'perspectives:')
    for (const p of model.perspectives) {
      out += line(1, `${p.id}:`)
      if (p.label) out += kv(2, 'label', p.label)
      if (p.membership) out += line(2, `membership: ${p.membership}`)
      out += line(2, `export: ${p.export ?? false}`)
      if (p.nodes && p.nodes.length > 0) {
        out += line(2, 'nodes:')
        for (const nd of p.nodes) {
          out += line(3, `${nd.id}:`)
          if (nd.label) out += kv(4, 'label', nd.label)
          if (nd.children && nd.children.length > 0) {
            out += line(4, `children: [${nd.children.join(', ')}]`)
          }
          if (nd.members && nd.members.length > 0) {
            out += line(4, `members: [${nd.members.join(', ')}]`)
          }
        }
      }
    }
  }

  return out
}
