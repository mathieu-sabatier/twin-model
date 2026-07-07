package dsl

import (
	"os"
	"strings"
	"testing"
)

// TestFormatRoundTrip: parse -> Format -> parse yields an equal-enough AST
// (same names, kinds, order). Format is canonical, so re-parsing is stable.
func TestFormatRoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m1, err := Parse("equipment.yaml", data)
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	out, err := Format(m1)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	m2, err := Parse("equipment.yaml", out)
	if err != nil {
		t.Fatalf("re-parse:\n%s\nerror: %v", out, err)
	}
	if len(m2.ObjectTypes) != len(m1.ObjectTypes) {
		t.Fatalf("object type count: got %d, want %d", len(m2.ObjectTypes), len(m1.ObjectTypes))
	}
	for i, ot := range m1.ObjectTypes {
		ot2 := m2.ObjectTypes[i]
		if ot2.Name != ot.Name || ot2.Abstract != ot.Abstract || len(ot2.Members) != len(ot.Members) {
			t.Errorf("object type %d differs: %+v vs %+v", i, ot2, ot)
		}
		for j, mem := range ot.Members {
			mem2 := ot2.Members[j]
			if mem2.Name != mem.Name || mem2.Kind != mem.Kind || mem2.Rule != mem.Rule ||
				mem2.Access != mem.Access || mem2.Type.Raw != mem.Type.Raw || mem2.Unit != mem.Unit {
				t.Errorf("%s.%s differs after round-trip: %+v vs %+v", ot.Name, mem.Name, mem2, mem)
			}
		}
	}
	if len(m2.Instances) != len(m1.Instances) {
		t.Errorf("instance count: got %d, want %d", len(m2.Instances), len(m1.Instances))
	}
}

// TestFormatIdempotent: Format(Format(x)) == Format(x).
func TestFormatIdempotent(t *testing.T) {
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m1, _ := Parse("equipment.yaml", data)
	once, err := Format(m1)
	if err != nil {
		t.Fatalf("format 1: %v", err)
	}
	m2, err := Parse("equipment.yaml", once)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	twice, err := Format(m2)
	if err != nil {
		t.Fatalf("format 2: %v", err)
	}
	if string(once) != string(twice) {
		t.Errorf("Format not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

// TestFormatInstanceValuesPreserveScalarType: numeric/boolean/enum instance values
// are emitted verbatim (unquoted); string values that would parse as non-strings
// stay quoted. Regression cover for QA finding H1.
func TestFormatInstanceValuesPreserveScalarType(t *testing.T) {
	src := `model:
  name: M
  namespace: https://x/
  version: 1.0.0
  publication_date: 2026-07-04

enums:
  EquipmentState:
    values:
      - Idle
      - Running

object_types:
  FurnaceType:
    base: OpcUa:BaseObjectType
    members:
      CycleCount: { type: UInt32 }
      DoorClosed: { type: Boolean }
      State: { type: EquipmentState }
      Serial: { kind: property, type: String }

instances:
  Furnace01:
    type: FurnaceType
    under: OpcUa:ObjectsFolder
    values:
      CycleCount: 42
      DoorClosed: true
      State: Running
      Serial: "42"
`
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := Format(m)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	got := string(out)
	for _, want := range []string{"CycleCount: 42", "DoorClosed: true", "State: Running", `Serial: "42"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, bad := range []string{`CycleCount: "42"`, `DoorClosed: "true"`} {
		if strings.Contains(got, bad) {
			t.Errorf("unexpected %q in:\n%s", bad, got)
		}
	}
}

// TestFormatPreservesHeaderComment: the file's leading comment block survives a
// Parse -> Format round-trip (and stays idempotent). Regression cover for QA H2.
func TestFormatPreservesHeaderComment(t *testing.T) {
	src := `# equipment.yaml — source of truth, PR-reviewed
#
# Conventions the transpiler applies:
#   - kind: property|variable|object|method   (default: variable)

model:
  name: M
  namespace: https://x/
  version: 1.0.0
  publication_date: 2026-07-04
`
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.HeadComment == "" {
		t.Fatal("HeadComment not captured")
	}
	out, err := Format(m)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	got := string(out)
	if !strings.HasPrefix(got, "# equipment.yaml — source of truth, PR-reviewed") {
		t.Errorf("header not emitted at top:\n%s", got)
	}
	if !strings.Contains(got, "#   - kind: property|variable|object|method") {
		t.Errorf("conventions line dropped:\n%s", got)
	}
	// Header must be stable across a re-parse + re-format (idempotent).
	m2, err := Parse("t.yaml", out)
	if err != nil {
		t.Fatalf("re-parse:\n%s\nerror: %v", got, err)
	}
	if m2.HeadComment != m.HeadComment {
		t.Errorf("HeadComment not stable:\n%q\nvs\n%q", m2.HeadComment, m.HeadComment)
	}
	out2, err := Format(m2)
	if err != nil {
		t.Fatalf("format 2: %v", err)
	}
	if string(out2) != got {
		t.Errorf("not idempotent:\n---first---\n%s\n---second---\n%s", got, string(out2))
	}
}

func TestFormatRoundTripsLevelAndHierarchy(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/, ISA95: http://www.OPCFoundation.org/UA/2013/01/ISA95 }\n" +
		"hierarchy: { allowLevelSkip: true }\n" +
		"instances:\n" +
		"  Plant1: { type: ISA95:EquipmentType, level: Enterprise, under: OpcUa:ObjectsFolder }\n"
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Format(m)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := Parse("t.yaml", out)
	if err != nil {
		t.Fatalf("reparse formatted output: %v\n%s", err, out)
	}
	if !m2.Hierarchy.AllowLevelSkip || m2.Instances[0].Level != "Enterprise" {
		t.Errorf("round-trip lost data: hierarchy=%+v level=%q\n%s", m2.Hierarchy, m2.Instances[0].Level, out)
	}
}

// TestFormatOmitsDefaults: default kind/rule/access are not written.
func TestFormatOmitsDefaults(t *testing.T) {
	m := mustParse(t, hdr+`object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { kind: variable, type: Double, rule: mandatory, access: r } } } }`)
	out, err := Format(m)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	s := string(out)
	for _, banned := range []string{"kind: variable", "rule: mandatory", "access: r\n"} {
		if strings.Contains(s, banned) {
			t.Errorf("Format wrote default %q:\n%s", banned, s)
		}
	}
	if !strings.Contains(s, "type: Double") {
		t.Errorf("Format dropped type:\n%s", s)
	}
}
