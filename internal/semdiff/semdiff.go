// Package semdiff computes a typed, human-renderable changelist between two
// parsed models (base ref vs draft). It is a pure function of the ASTs — used by
// the UI review screen and as a PR comment. Output order is deterministic:
// types (draft source order), enums, then instances (draft source order);
// removals are anchored after additions/changes.
package semdiff

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

type ChangeKind string

const (
	TypeAdded        ChangeKind = "TypeAdded"
	TypeRemoved      ChangeKind = "TypeRemoved"
	TypeChanged      ChangeKind = "TypeChanged"
	MemberAdded      ChangeKind = "MemberAdded"
	MemberRemoved    ChangeKind = "MemberRemoved"
	MemberChanged    ChangeKind = "MemberChanged"
	EnumAdded        ChangeKind = "EnumAdded"
	EnumRemoved      ChangeKind = "EnumRemoved"
	EnumChanged      ChangeKind = "EnumChanged"
	EnumValueChanged ChangeKind = "EnumValueChanged"
	InstanceAdded    ChangeKind = "InstanceAdded"
	InstanceRemoved  ChangeKind = "InstanceRemoved"
	InstanceChanged  ChangeKind = "InstanceChanged"
	ValueChanged     ChangeKind = "ValueChanged"
	ChildAdded       ChangeKind = "ChildAdded"
	ChildRemoved     ChangeKind = "ChildRemoved"
)

// Change is one semantic difference. Only the fields relevant to Kind are set.
type Change struct {
	Kind     ChangeKind `json:"kind"`
	Type     string     `json:"type,omitempty"`
	Member   string     `json:"member,omitempty"`
	Instance string     `json:"instance,omitempty"`
	Under    string     `json:"under,omitempty"`
	Child    string     `json:"child,omitempty"`
	Enum     string     `json:"enum,omitempty"`
	Field    string     `json:"field,omitempty"`
	Old      string     `json:"old,omitempty"`
	New      string     `json:"new,omitempty"`
	Text     string     `json:"text"`
}

// Diff returns the changelist transforming base into draft.
func Diff(base, draft *dsl.Model) []Change {
	var out []Change
	out = append(out, diffTypes(base, draft)...)
	out = append(out, diffEnums(base, draft)...)
	out = append(out, diffInstances(base, draft)...)
	for i := range out {
		out[i].Text = render(out[i])
	}
	return out
}

func diffTypes(base, draft *dsl.Model) []Change {
	var out []Change
	baseTypes := indexTypes(base)
	draftTypes := indexTypes(draft)
	for _, ot := range draft.ObjectTypes {
		b, ok := baseTypes[ot.Name]
		if !ok {
			out = append(out, Change{Kind: TypeAdded, Type: ot.Name})
			continue
		}
		out = append(out, typeFieldChanges(ot.Name, b, ot)...)
		out = append(out, diffMembers(ot.Name, b.Members, ot.Members)...)
	}
	for _, ot := range base.ObjectTypes {
		if _, ok := draftTypes[ot.Name]; !ok {
			out = append(out, Change{Kind: TypeRemoved, Type: ot.Name})
		}
	}
	return out
}

func typeFieldChanges(typeName string, b, d *dsl.ObjectType) []Change {
	var out []Change
	add := func(field, old, nw string) {
		if old != nw {
			out = append(out, Change{Kind: TypeChanged, Type: typeName, Field: field, Old: old, New: nw})
		}
	}
	add("doc", b.Doc, d.Doc)
	add("base", b.Base.Raw, d.Base.Raw)
	add("abstract", strconv.FormatBool(b.Abstract), strconv.FormatBool(d.Abstract))
	return out
}

func diffMembers(typeName string, baseM, draftM []*dsl.Member) []Change {
	var out []Change
	baseByName := indexMembers(baseM)
	draftByName := indexMembers(draftM)
	for _, mem := range draftM {
		b, ok := baseByName[mem.Name]
		if !ok {
			out = append(out, Change{Kind: MemberAdded, Type: typeName, Member: mem.Name})
			continue
		}
		for _, fc := range memberFieldChanges(b, mem) {
			out = append(out, Change{Kind: MemberChanged, Type: typeName, Member: mem.Name, Field: fc.field, Old: fc.old, New: fc.new})
		}
	}
	for _, mem := range baseM {
		if _, ok := draftByName[mem.Name]; !ok {
			out = append(out, Change{Kind: MemberRemoved, Type: typeName, Member: mem.Name})
		}
	}
	return out
}

type fieldChange struct{ field, old, new string }

func memberFieldChanges(b, d *dsl.Member) []fieldChange {
	var out []fieldChange
	add := func(field, old, nw string) {
		if old != nw {
			out = append(out, fieldChange{field, old, nw})
		}
	}
	add("kind", string(b.Kind), string(d.Kind))
	add("type", b.Type.Raw, d.Type.Raw)
	add("rule", string(b.Rule), string(d.Rule))
	add("access", string(b.Access), string(d.Access))
	add("unit", b.Unit, d.Unit)
	add("doc", b.Doc, d.Doc)
	return out
}

func diffEnums(base, draft *dsl.Model) []Change {
	var out []Change
	baseE := map[string]*dsl.Enum{}
	for _, e := range base.Enums {
		baseE[e.Name] = e
	}
	draftE := map[string]*dsl.Enum{}
	for _, e := range draft.Enums {
		draftE[e.Name] = e
	}
	for _, e := range draft.Enums {
		b, ok := baseE[e.Name]
		if !ok {
			out = append(out, Change{Kind: EnumAdded, Enum: e.Name})
			continue
		}
		if b.Doc != e.Doc {
			out = append(out, Change{Kind: EnumChanged, Enum: e.Name, Field: "doc", Old: b.Doc, New: e.Doc})
		}
		if enumSig(b) != enumSig(e) {
			out = append(out, Change{Kind: EnumValueChanged, Enum: e.Name, Old: enumSig(b), New: enumSig(e)})
		}
	}
	for _, e := range base.Enums {
		if _, ok := draftE[e.Name]; !ok {
			out = append(out, Change{Kind: EnumRemoved, Enum: e.Name})
		}
	}
	return out
}

// enumSig is a stable signature of an enum's values for equality.
func enumSig(e *dsl.Enum) string {
	var b strings.Builder
	for _, v := range e.Values {
		fmt.Fprintf(&b, "%s=%d;", v.Name, v.Identifier)
	}
	return b.String()
}

func diffInstances(base, draft *dsl.Model) []Change {
	var out []Change
	baseInst := indexInstances(base)
	draftInst := indexInstances(draft)
	for _, inst := range draft.Instances {
		b, ok := baseInst[inst.Name]
		if !ok {
			out = append(out, Change{Kind: InstanceAdded, Instance: inst.Name, Type: inst.Type.Raw, Under: inst.Under.Raw})
			// Itemize the new instance's initial values and instantiated children,
			// so the review screen shows what was set, not just that it was added.
			for _, v := range inst.Values {
				out = append(out, Change{Kind: ValueChanged, Instance: inst.Name, Member: v.Member, Old: "", New: v.Raw})
			}
			for _, c := range inst.Children {
				out = append(out, Change{Kind: ChildAdded, Instance: inst.Name, Child: c.Name})
			}
			continue
		}
		if b.Type.Raw != inst.Type.Raw {
			out = append(out, Change{Kind: InstanceChanged, Instance: inst.Name, Field: "type", Old: b.Type.Raw, New: inst.Type.Raw})
		}
		if b.Under.Raw != inst.Under.Raw {
			out = append(out, Change{Kind: InstanceChanged, Instance: inst.Name, Field: "under", Old: b.Under.Raw, New: inst.Under.Raw})
		}
		out = append(out, diffValues(inst.Name, b, inst)...)
		out = append(out, diffChildren(inst.Name, b, inst)...)
	}
	for _, inst := range base.Instances {
		if _, ok := draftInst[inst.Name]; !ok {
			out = append(out, Change{Kind: InstanceRemoved, Instance: inst.Name, Type: inst.Type.Raw, Under: inst.Under.Raw})
		}
	}
	return out
}

func diffValues(name string, b, d *dsl.Instance) []Change {
	var out []Change
	baseVals := map[string]string{}
	for _, v := range b.Values {
		baseVals[v.Member] = v.Raw
	}
	draftVals := map[string]string{}
	for _, v := range d.Values {
		draftVals[v.Member] = v.Raw
	}
	for _, v := range d.Values {
		if old := baseVals[v.Member]; old != v.Raw {
			out = append(out, Change{Kind: ValueChanged, Instance: name, Member: v.Member, Old: old, New: v.Raw})
		}
	}
	for _, v := range b.Values {
		if _, ok := draftVals[v.Member]; !ok {
			out = append(out, Change{Kind: ValueChanged, Instance: name, Member: v.Member, Old: v.Raw, New: ""})
		}
	}
	return out
}

func diffChildren(name string, b, d *dsl.Instance) []Change {
	var out []Change
	baseCh := map[string]bool{}
	for _, c := range b.Children {
		baseCh[c.Name] = true
	}
	draftCh := map[string]bool{}
	for _, c := range d.Children {
		draftCh[c.Name] = true
	}
	for _, c := range d.Children {
		if !baseCh[c.Name] {
			out = append(out, Change{Kind: ChildAdded, Instance: name, Child: c.Name})
		}
	}
	for _, c := range b.Children {
		if !draftCh[c.Name] {
			out = append(out, Change{Kind: ChildRemoved, Instance: name, Child: c.Name})
		}
	}
	return out
}

func indexTypes(m *dsl.Model) map[string]*dsl.ObjectType {
	out := map[string]*dsl.ObjectType{}
	for _, ot := range m.ObjectTypes {
		out[ot.Name] = ot
	}
	return out
}

func indexMembers(members []*dsl.Member) map[string]*dsl.Member {
	out := map[string]*dsl.Member{}
	for _, mem := range members {
		out[mem.Name] = mem
	}
	return out
}

func indexInstances(m *dsl.Model) map[string]*dsl.Instance {
	out := map[string]*dsl.Instance{}
	for _, inst := range m.Instances {
		out[inst.Name] = inst
	}
	return out
}
