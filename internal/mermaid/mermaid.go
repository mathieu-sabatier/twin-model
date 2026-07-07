// Package mermaid renders a parsed model to Mermaid diagram source: a classDiagram
// of the object types (with local inheritance edges) and a flowchart of the
// instance topology. It is a pure, deterministic function of the AST — no XML, no
// I/O — so the diagram reviews cleanly and is golden-testable, like the emitter.
package mermaid

import (
	"fmt"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// TypesDiagram renders the object types as a Mermaid classDiagram. Each type is a
// class listing its direct members ("<type> <name>"); a local base yields an
// inheritance edge (Base <|-- Derived). Imported bases are omitted (opaque).
func TypesDiagram(m *dsl.Model) string {
	var b strings.Builder
	b.WriteString("classDiagram\n")
	for _, ot := range m.ObjectTypes {
		fmt.Fprintf(&b, "  class %s {\n", safeID(ot.Name))
		for _, mem := range ot.Members {
			typ := mem.Type.Raw
			if typ == "" {
				typ = string(mem.Kind) // methods have no type
			}
			fmt.Fprintf(&b, "    +%s %s\n", sanitizeLabel(typ), sanitizeLabel(mem.Name))
		}
		b.WriteString("  }\n")
	}
	for _, ot := range m.ObjectTypes {
		if ot.Base.IsZero() {
			continue
		}
		if m.ResolveType(ot.Base).Kind == dsl.RefLocal {
			fmt.Fprintf(&b, "  %s <|-- %s\n", safeID(ot.Base.Name), safeID(ot.Name))
		}
	}
	return b.String()
}

// InstancesDiagram renders instance topology as a Mermaid flowchart: an edge from
// each instance's `under` parent to the instance, and from each instance to its
// instantiated children. Parents that are import targets (e.g. OpcUa:ObjectsFolder)
// appear as nodes labelled by their local name.
func InstancesDiagram(m *dsl.Model) string {
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	// declared tracks node ids already given a label line, so an `under` parent
	// (e.g. OpcUa:ObjectsFolder) is declared once instead of re-labelled on every
	// edge. It is a membership set only — never iterated for output — so diagram
	// order stays source-ordered and deterministic.
	declared := map[string]bool{}
	for _, inst := range m.Instances {
		id := safeID(inst.Name)
		declared[id] = true
		fmt.Fprintf(&b, "  %s[\"%s: %s\"]\n", id, sanitizeLabel(inst.Name), sanitizeLabel(inst.Type.Raw))
	}
	for _, inst := range m.Instances {
		if !inst.Under.IsZero() {
			parent := inst.Under.Name
			if parent == "" {
				parent = inst.Under.Raw
			}
			pid := safeID(parent)
			if !declared[pid] {
				declared[pid] = true
				fmt.Fprintf(&b, "  %s[\"%s\"]\n", pid, sanitizeLabel(parent))
			}
			fmt.Fprintf(&b, "  %s --> %s\n", pid, safeID(inst.Name))
		}
		for _, ch := range inst.Children {
			childID := safeID(inst.Name + "_" + ch.Name)
			declared[childID] = true
			fmt.Fprintf(&b, "  %s --> %s[\"%s: %s\"]\n", safeID(inst.Name), childID, sanitizeLabel(ch.Name), sanitizeLabel(ch.Of.Raw))
		}
	}
	return b.String()
}

// safeID makes a Mermaid-safe node identifier (alphanumerics and underscores).
func safeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// sanitizeLabel escapes characters that would break a quoted Mermaid label.
func sanitizeLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
