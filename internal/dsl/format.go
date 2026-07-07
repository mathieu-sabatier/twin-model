package dsl

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Format renders a Model back to canonical YAML: source order preserved, fixed
// two-space indentation, block style, defaults (kind: variable, rule: mandatory,
// access: r) omitted. It is deterministic and idempotent — the UI writes this
// form, and re-formatting formatted output is a no-op.
//
// Format emits from the AST, which retains only the leading file header comment
// (Model.HeadComment) — inline comments and original scalar styling are not kept,
// so canonicalizing a hand-edited file normalizes defaults and drops inline notes
// while preserving the top-of-file conventions header (see README).
func Format(m *Model) ([]byte, error) {
	var b bytes.Buffer
	w := &yamlWriter{b: &b}

	if m.HeadComment != "" {
		w.b.WriteString(m.HeadComment)
		w.b.WriteString("\n\n")
	}

	w.line(0, "model:")
	w.kv(1, "name", m.Name)
	w.kv(1, "namespace", m.Namespace)
	if m.Prefix != "" {
		w.kv(1, "prefix", m.Prefix)
	}
	w.kv(1, "version", m.Version)
	w.kv(1, "publication_date", m.PublicationDate)

	if m.Hierarchy.Set {
		w.blank()
		w.line(0, "hierarchy: { allowLevelSkip: "+strconv.FormatBool(m.Hierarchy.AllowLevelSkip)+" }")
	}

	if len(m.Imports) > 0 {
		w.blank()
		w.line(0, "imports:")
		for _, im := range m.Imports {
			if im.Version != "" {
				w.line(1, im.Alias+":")
				w.kv(2, "uri", im.URI)
				w.kv(2, "version", im.Version)
			} else {
				w.kv(1, im.Alias, im.URI)
			}
		}
	}

	if len(m.Enums) > 0 {
		w.blank()
		w.line(0, "enums:")
		for _, e := range m.Enums {
			w.line(1, e.Name+":")
			if e.Doc != "" {
				w.kv(2, "doc", e.Doc)
			}
			w.line(2, "values:")
			for _, val := range e.Values {
				if val.Explicit {
					w.line(3, fmt.Sprintf("- { %s: %d }", val.Name, val.Identifier))
				} else {
					w.line(3, "- "+val.Name)
				}
			}
		}
	}

	if len(m.ObjectTypes) > 0 {
		w.blank()
		w.line(0, "object_types:")
		for _, ot := range m.ObjectTypes {
			w.line(1, ot.Name+":")
			if ot.Doc != "" {
				w.kv(2, "doc", ot.Doc)
			}
			if !ot.Base.IsZero() {
				w.line(2, "base: "+ot.Base.Raw) // type refs are safe plain scalars; keep them unquoted
			}
			if ot.Abstract {
				w.line(2, "abstract: true")
			}
			if len(ot.Members) > 0 {
				w.line(2, "members:")
				w.members(3, ot.Members)
			}
		}
	}

	if len(m.Instances) > 0 {
		w.blank()
		w.line(0, "instances:")
		for _, inst := range m.Instances {
			w.instance(1, inst, m)
		}
	}

	if len(m.Perspectives) > 0 {
		w.blank()
		w.line(0, "perspectives:")
		for _, p := range m.Perspectives {
			w.line(1, p.ID+":")
			if p.Label != "" {
				w.kv(2, "label", p.Label)
			}
			if p.Membership != "" {
				w.line(2, "membership: "+p.Membership)
			}
			w.line(2, "export: "+strconv.FormatBool(p.Export))
			if len(p.Nodes) > 0 {
				w.line(2, "nodes:")
				for _, nd := range p.Nodes {
					w.line(3, nd.ID+":")
					if nd.Label != "" {
						w.kv(4, "label", nd.Label)
					}
					if len(nd.Children) > 0 {
						w.line(4, "children: ["+strings.Join(nd.Children, ", ")+"]")
					}
					if len(nd.Members) > 0 {
						w.line(4, "members: ["+strings.Join(nd.Members, ", ")+"]")
					}
				}
			}
		}
	}

	return b.Bytes(), w.err
}

type yamlWriter struct {
	b   *bytes.Buffer
	err error
}

func (w *yamlWriter) indent(n int) { w.b.WriteString(strings.Repeat("  ", n)) }
func (w *yamlWriter) blank()       { w.b.WriteByte('\n') }

func (w *yamlWriter) line(n int, s string) {
	w.indent(n)
	w.b.WriteString(s)
	w.b.WriteByte('\n')
}

func (w *yamlWriter) kv(n int, k, v string) {
	w.indent(n)
	w.b.WriteString(k)
	w.b.WriteString(": ")
	w.b.WriteString(scalar(v))
	w.b.WriteByte('\n')
}

func (w *yamlWriter) members(n int, members []*Member) {
	for _, mem := range members {
		key := mem.Name
		if mem.IsPlaceholder() {
			// Reconstruct the Name<Suffix> key from the placeholder BrowseName
			// (<NameSuffix>): strip the base name to recover the suffix.
			inner := strings.TrimSuffix(strings.TrimPrefix(mem.BrowseName, "<"), ">")
			suffix := strings.TrimPrefix(inner, mem.Name)
			key = mem.Name + "<" + suffix + ">"
		}
		fields := mem.inlineFields()
		if len(mem.Children) == 0 && len(mem.In) == 0 && len(mem.Out) == 0 {
			w.line(n, quoteKey(key)+": { "+strings.Join(fields, ", ")+" }")
			continue
		}
		w.line(n, quoteKey(key)+":")
		for _, f := range fields {
			w.line(n+1, f)
		}
		if len(mem.In) > 0 {
			w.line(n+1, "in:")
			for _, a := range mem.In {
				w.line(n+2, fmt.Sprintf("- { name: %s, type: %s }", a.Name, a.Type.Raw))
			}
		}
		if len(mem.Out) > 0 {
			w.line(n+1, "out:")
			for _, a := range mem.Out {
				w.line(n+2, fmt.Sprintf("- { name: %s, type: %s }", a.Name, a.Type.Raw))
			}
		}
		if len(mem.Children) > 0 {
			w.line(n+1, "children:")
			w.members(n+2, mem.Children)
		}
	}
}

// inlineFields returns the non-default member fields in canonical order.
func (mem *Member) inlineFields() []string {
	var f []string
	if mem.Kind != KindVariable {
		f = append(f, "kind: "+string(mem.Kind))
	}
	if !mem.Type.IsZero() {
		f = append(f, "type: "+mem.Type.Raw)
	}
	if mem.Rule != RuleMandatory {
		f = append(f, "rule: "+string(mem.Rule))
	}
	if (mem.Kind == KindVariable || mem.Kind == KindProperty) && mem.Access != AccessRead {
		f = append(f, "access: "+string(mem.Access))
	}
	if mem.Unit != "" {
		f = append(f, "unit: "+scalar(mem.Unit))
	}
	if mem.Doc != "" {
		f = append(f, "doc: "+scalar(mem.Doc))
	}
	return f
}

func (w *yamlWriter) instance(n int, inst *Instance, m *Model) {
	w.line(n, inst.Name+":")
	w.line(n+1, "type: "+inst.Type.Raw)
	w.line(n+1, "under: "+inst.Under.Raw)
	if inst.Level != "" {
		w.line(n+1, "level: "+inst.Level)
	}
	if len(inst.Values) > 0 {
		memberTypes := m.valueMemberTypes(inst.Type.Name)
		w.line(n+1, "values:")
		for _, val := range inst.Values {
			w.line(n+2, val.Member+": "+m.valueLiteral(memberTypes[val.Member], val.Raw))
		}
	}
	if len(inst.Children) > 0 {
		w.line(n+1, "children:")
		for _, ch := range inst.Children {
			w.line(n+2, ch.Name+": { of: "+scalar(ch.Of.Raw)+" }")
		}
	}
}

// scalar quotes a value only when YAML requires it (leading/trailing space,
// special chars, or a form that would otherwise parse as a non-string).
func scalar(v string) string {
	if v == "" {
		return `""`
	}
	if needsQuote(v) {
		return strconv.Quote(v)
	}
	return v
}

func needsQuote(v string) bool {
	if v != strings.TrimSpace(v) {
		return true
	}
	switch v {
	case "true", "false", "null", "yes", "no", "~":
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	return strings.ContainsAny(v, ":#{}[]&*!|>'\"%@`,")
}

// quoteKey quotes a member key when it contains placeholder angle brackets.
func quoteKey(k string) string {
	if strings.ContainsAny(k, "<>") {
		return strconv.Quote(k)
	}
	return k
}

// numericBuiltins are the ns0 scalar DataTypes whose instance values are written
// as bare numbers (never quoted). Boolean and local enums are handled separately.
var numericBuiltins = map[string]bool{
	"SByte": true, "Byte": true, "Int16": true, "UInt16": true,
	"Int32": true, "UInt32": true, "Int64": true, "UInt64": true,
	"Float": true, "Double": true, "Number": true, "Integer": true,
	"UInteger": true, "Duration": true,
}

// valueLiteral renders an instance value using the target member's declared type:
// numeric, Boolean, and local-enum values are emitted verbatim (so UInt32 42 stays
// 42 and Boolean true stays true); everything else (String/textual/unresolved) falls
// back to scalar() quoting. An empty raw always uses scalar(). Fixes QA finding H1.
func (m *Model) valueLiteral(memberType TypeRef, raw string) string {
	if raw == "" || memberType.IsZero() {
		return scalar(raw)
	}
	r := m.ResolveType(memberType)
	switch r.Kind {
	case RefBuiltin:
		if r.Name == "Boolean" || numericBuiltins[r.Name] {
			return raw
		}
	case RefLocal:
		for _, e := range m.Enums {
			if e.Name == r.Name {
				return raw // enum member identifier — bare
			}
		}
	}
	return scalar(raw)
}

// valueMemberTypes maps each value-bearing member of typeName to its declared type,
// using the inheritance-flattened member set. Returns an empty map when the type
// can't be resolved (unknown/imported base), so the caller falls back to scalar().
func (m *Model) valueMemberTypes(typeName string) map[string]TypeRef {
	out := map[string]TypeRef{}
	members, err := m.ResolveMembers(typeName)
	if err != nil {
		return out
	}
	for _, rm := range members {
		if !rm.Type.IsZero() {
			out[rm.Name] = rm.Type
		}
	}
	return out
}
