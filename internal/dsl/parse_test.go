package dsl

import "testing"

func TestParseModelHeader(t *testing.T) {
	src := `
model:
  name: AcmeEquipment
  namespace: https://acme.example/UA/Equipment/
  prefix: Acme.Equipment
  version: 1.0.0
  publication_date: 2026-07-02
imports:
  OpcUa: http://opcfoundation.org/UA/
`
	m, err := Parse("equipment.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if m.Name != "AcmeEquipment" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Namespace != "https://acme.example/UA/Equipment/" {
		t.Errorf("Namespace = %q", m.Namespace)
	}
	if m.Prefix != "Acme.Equipment" {
		t.Errorf("Prefix = %q", m.Prefix)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q", m.Version)
	}
	if m.PublicationDate != "2026-07-02" {
		t.Errorf("PublicationDate = %q", m.PublicationDate)
	}
	if len(m.Imports) != 1 || m.Imports[0].Alias != "OpcUa" ||
		m.Imports[0].URI != "http://opcfoundation.org/UA/" {
		t.Fatalf("Imports = %+v", m.Imports)
	}
}

func TestParseInstanceValuesAndChildren(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  FurnaceType: { base: OpcUa:BaseObjectType }\n" +
		"instances:\n" +
		"  Furnace01:\n" +
		"    type: FurnaceType\n" +
		"    under: OpcUa:ObjectsFolder\n" +
		"    values:\n" +
		"      SerialNumber: \"F-2026-0042\"\n" +
		"    children:\n" +
		"      Zone1: { of: \"Zone<No>\" }\n"
	m, err := Parse("m.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	inst := m.Instances[0]
	if len(inst.Values) != 1 || inst.Values[0].Member != "SerialNumber" || inst.Values[0].Raw != "F-2026-0042" {
		t.Fatalf("values = %+v", inst.Values)
	}
	if inst.Values[0].Line == 0 {
		t.Errorf("value Pos not set: %+v", inst.Values[0].Pos)
	}
	if len(inst.Children) != 1 || inst.Children[0].Name != "Zone1" || inst.Children[0].Of.Raw != "Zone<No>" {
		t.Fatalf("children = %+v", inst.Children)
	}
}

func TestParseReportsUnknownModelKey(t *testing.T) {
	src := `
model:
  name: X
  namespace: https://x/
  version: 1.0.0
  publication_date: 2026-07-02
  bogus: nope
`
	_, err := Parse("f.yaml", []byte(src))
	if err == nil {
		t.Fatal("Parse: want error for unknown model key, got nil")
	}
}

func TestParseImportsExpandedForm(t *testing.T) {
	src := []byte(`
model:
  name: M
  namespace: https://ex/UA/M/
  version: 1.0.0
  publication_date: 2026-07-04
imports:
  DI: http://opcfoundation.org/UA/DI/
  ISA95:
    uri: http://opcfoundation.org/UA/ISA95/
    version: "1.00.5"
`)
	m, err := Parse("m.yaml", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Imports) != 2 {
		t.Fatalf("imports = %d", len(m.Imports))
	}
	if m.Imports[0].Alias != "DI" || m.Imports[0].URI != "http://opcfoundation.org/UA/DI/" || m.Imports[0].Version != "" {
		t.Errorf("scalar import = %+v", m.Imports[0])
	}
	if m.Imports[1].Alias != "ISA95" || m.Imports[1].URI != "http://opcfoundation.org/UA/ISA95/" || m.Imports[1].Version != "1.00.5" {
		t.Errorf("mapping import = %+v", m.Imports[1])
	}
}

func TestParseLevelAndHierarchy(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/, ISA95: http://www.OPCFoundation.org/UA/2013/01/ISA95 }\n" +
		"hierarchy: { allowLevelSkip: true }\n" +
		"instances:\n" +
		"  Plant1: { type: ISA95:EquipmentType, level: Enterprise, under: OpcUa:ObjectsFolder }\n"
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !m.Hierarchy.Set || !m.Hierarchy.AllowLevelSkip {
		t.Errorf("hierarchy = %+v, want Set && AllowLevelSkip", m.Hierarchy)
	}
	if len(m.Instances) != 1 || m.Instances[0].Level != "Enterprise" {
		t.Fatalf("instance level = %q, want Enterprise", m.Instances[0].Level)
	}
}

func TestParseCapturesPositions(t *testing.T) {
	src := "model:\n" +
		"  name: M\n" +
		"  namespace: https://x/\n" +
		"  version: 1.0.0\n" +
		"  publication_date: 2026-07-02\n" +
		"object_types:\n" +
		"  FooType:\n" +
		"    base: OpcUa:BaseObjectType\n" +
		"    members:\n" +
		"      Temp: { type: Double }\n"
	m, err := Parse("f.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ot := m.ObjectTypes[0]
	if ot.File != "f.yaml" {
		t.Errorf("ObjectType.File = %q, want f.yaml", ot.File)
	}
	if ot.Line != 7 { // "  FooType:" is line 7 (1-based)
		t.Errorf("ObjectType.Line = %d, want 7", ot.Line)
	}
	if ot.Col != 3 { // two-space indent -> column 3
		t.Errorf("ObjectType.Col = %d, want 3", ot.Col)
	}
	mem := ot.Members[0]
	if mem.Line != 10 || mem.Col != 7 {
		t.Errorf("Member pos = %d:%d, want 10:7", mem.Line, mem.Col)
	}
}
