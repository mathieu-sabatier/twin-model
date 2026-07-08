package semdiff

import (
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

func parse(t *testing.T, src string) *dsl.Model {
	t.Helper()
	m, err := dsl.Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

const base = "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
	"object_types:\n  PressType:\n    base: OpcUa:BaseObjectType\n    members:\n      Setpoint: { type: Double, access: r }\n" +
	"instances:\n  Press01: { type: PressType, under: OpcUa:ObjectsFolder }\n"

func TestDiffMemberFieldChange(t *testing.T) {
	draft := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  PressType:\n    base: OpcUa:BaseObjectType\n    members:\n      Setpoint: { type: Double, access: rw }\n" +
		"instances:\n  Press01: { type: PressType, under: OpcUa:ObjectsFolder }\n"
	changes := Diff(parse(t, base), parse(t, draft))
	var found *Change
	for i := range changes {
		if changes[i].Kind == MemberChanged && changes[i].Field == "access" {
			found = &changes[i]
		}
	}
	if found == nil {
		t.Fatalf("no access MemberChanged in %+v", changes)
	}
	if found.Old != "r" || found.New != "rw" || found.Type != "PressType" || found.Member != "Setpoint" {
		t.Errorf("bad change: %+v", *found)
	}
	if found.Text != "PressType.Setpoint: access r → rw" {
		t.Errorf("Text = %q", found.Text)
	}
}

func TestDiffInstanceAdded(t *testing.T) {
	draft := base + "  Press02: { type: PressType, under: OpcUa:ObjectsFolder }\n"
	changes := Diff(parse(t, base), parse(t, draft))
	var found *Change
	for i := range changes {
		if changes[i].Kind == InstanceAdded {
			found = &changes[i]
		}
	}
	if found == nil {
		t.Fatalf("no InstanceAdded in %+v", changes)
	}
	if found.Instance != "Press02" || found.Type != "PressType" || found.Under != "OpcUa:ObjectsFolder" {
		t.Errorf("bad change: %+v", *found)
	}
	if found.Text != "Added instance Press02 (PressType) under OpcUa:ObjectsFolder" {
		t.Errorf("Text = %q", found.Text)
	}
}

func TestDiffAddedInstanceItemizesValuesAndChildren(t *testing.T) {
	draft := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  PressType:\n    base: OpcUa:BaseObjectType\n    members:\n      Setpoint: { type: Double, access: r }\n      Serial: { kind: property, type: String }\n      Slot: { kind: object, type: OpcUa:FolderType, rule: optional_placeholder }\n" +
		"instances:\n  Press01: { type: PressType, under: OpcUa:ObjectsFolder }\n" +
		"  Press02:\n    type: PressType\n    under: OpcUa:ObjectsFolder\n    values:\n      Serial: \"S-9\"\n    children:\n      Slot1: { of: \"Slot<Nr>\" }\n"
	changes := Diff(parse(t, base), parse(t, draft))
	var added, valued, childed bool
	for _, c := range changes {
		switch {
		case c.Kind == InstanceAdded && c.Instance == "Press02":
			added = true
		case c.Kind == ValueChanged && c.Instance == "Press02" && c.Member == "Serial" && c.New == "S-9":
			valued = true
		case c.Kind == ChildAdded && c.Instance == "Press02" && c.Child == "Slot1":
			childed = true
		}
	}
	if !added || !valued || !childed {
		t.Errorf("added=%v valued=%v childed=%v in %+v", added, valued, childed, changes)
	}
}

func TestDiffInstanceTypeChange(t *testing.T) {
	draft := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  PressType:\n    base: OpcUa:BaseObjectType\n    members:\n      Setpoint: { type: Double, access: r }\n" +
		"  OtherType: { base: OpcUa:BaseObjectType }\n" +
		"instances:\n  Press01: { type: OtherType, under: OpcUa:ObjectsFolder }\n"
	changes := Diff(parse(t, base), parse(t, draft))
	var found *Change
	for i := range changes {
		if changes[i].Kind == InstanceChanged && changes[i].Field == "type" {
			found = &changes[i]
		}
	}
	if found == nil {
		t.Fatalf("no InstanceChanged(type) in %+v", changes)
	}
	if found.Instance != "Press01" || found.Old != "PressType" || found.New != "OtherType" {
		t.Errorf("bad change: %+v", *found)
	}
	if found.Text != "Press01: type PressType → OtherType" {
		t.Errorf("Text = %q", found.Text)
	}
}

func TestDiffType_AbstractAndBaseAndDocChanges(t *testing.T) {
	base := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  PressType:\n    doc: old doc\n    base: OpcUa:BaseObjectType\n    abstract: true\n    members:\n      Setpoint: { type: Double }\n" +
		"instances: {}\n"
	draft := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  PressType:\n    doc: new doc\n    base: OpcUa:BaseDataVariableType\n    abstract: false\n    members:\n      Setpoint: { type: Double }\n" +
		"instances: {}\n"
	changes := Diff(parse(t, base), parse(t, draft))

	got := map[string]string{} // field -> Text
	for _, c := range changes {
		if c.Kind == TypeChanged && c.Type == "PressType" {
			got[c.Field] = c.Text
		}
	}
	if got["doc"] != "PressType: doc old doc → new doc" {
		t.Errorf("doc change Text = %q", got["doc"])
	}
	if got["base"] != "PressType: base OpcUa:BaseObjectType → OpcUa:BaseDataVariableType" {
		t.Errorf("base change Text = %q", got["base"])
	}
	if got["abstract"] != "PressType: abstract true → false" {
		t.Errorf("abstract change Text = %q", got["abstract"])
	}
}

func TestDiffEnum_DocChange(t *testing.T) {
	// Note: enum values must be a YAML list, not a mapping — parser requires SequenceNode.
	base := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"enums:\n  State:\n    doc: old\n    values: [Idle, Running]\n"
	draft := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"enums:\n  State:\n    doc: new\n    values: [Idle, Running]\n"
	changes := Diff(parse(t, base), parse(t, draft))

	var found *Change
	for i := range changes {
		if changes[i].Kind == EnumChanged && changes[i].Enum == "State" {
			found = &changes[i]
		}
	}
	if found == nil {
		t.Fatalf("no EnumChanged for State in %+v", changes)
	}
	if found.Text != "Enum State: doc old → new" {
		t.Errorf("Text = %q", found.Text)
	}
}

func TestDiff_ReportsImportAdded(t *testing.T) {
	base := &dsl.Model{Namespace: "urn:x", Version: "1.0.0"}
	draft := &dsl.Model{Namespace: "urn:x", Version: "1.0.0",
		Imports: []dsl.Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}}}
	changes := Diff(base, draft)
	var found bool
	for _, c := range changes {
		if c.Kind == ImportAdded && c.Field == "DI" {
			found = true
			if c.Text == "" {
				t.Error("ImportAdded change has empty Text")
			}
		}
	}
	if !found {
		t.Fatalf("no ImportAdded change in %+v", changes)
	}
}

func TestDiff_ReportsVersionChanged(t *testing.T) {
	base := &dsl.Model{Version: "1.0.0"}
	draft := &dsl.Model{Version: "1.0.1"}
	changes := Diff(base, draft)
	for _, c := range changes {
		if c.Kind == VersionChanged {
			return
		}
	}
	t.Fatalf("no VersionChanged change in %+v", changes)
}

func TestDiffTypeAndValueChanges(t *testing.T) {
	draft := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  PressType:\n    base: OpcUa:BaseObjectType\n    members:\n      Setpoint: { type: Double, access: r }\n      Serial: { kind: property, type: String }\n" +
		"  NewType: { base: OpcUa:BaseObjectType }\n" +
		"instances:\n  Press01:\n    type: PressType\n    under: OpcUa:ObjectsFolder\n    values:\n      Serial: \"S-1\"\n"
	changes := Diff(parse(t, base), parse(t, draft))
	kinds := map[ChangeKind]int{}
	for _, c := range changes {
		kinds[c.Kind]++
	}
	if kinds[TypeAdded] != 1 {
		t.Errorf("TypeAdded = %d, want 1 (%+v)", kinds[TypeAdded], changes)
	}
	if kinds[MemberAdded] != 1 {
		t.Errorf("MemberAdded = %d, want 1", kinds[MemberAdded])
	}
	if kinds[ValueChanged] != 1 {
		t.Errorf("ValueChanged = %d, want 1", kinds[ValueChanged])
	}
}
