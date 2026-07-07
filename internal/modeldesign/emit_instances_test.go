package modeldesign

import (
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
)

func emitSrc(t *testing.T, src string) string {
	t.Helper()
	m, err := dsl.Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if diags := dsl.Validate(m); len(diags) != 0 {
		t.Fatalf("validate: %v", diags)
	}
	out, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return string(out)
}

func TestEmitInstanceValueOverride(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, prefix: X, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  FurnaceType:\n    base: OpcUa:BaseObjectType\n    members:\n      SerialNumber: { kind: property, type: String }\n" +
		"instances:\n  Furnace01:\n    type: FurnaceType\n    under: OpcUa:ObjectsFolder\n    values:\n      SerialNumber: \"F-2026-0042\"\n"
	out := emitSrc(t, src)
	want := `<opc:Property SymbolicName="SerialNumber" DataType="ua:String">`
	if !strings.Contains(out, want) {
		t.Errorf("missing override property:\n%s", out)
	}
	if !strings.Contains(out, "<uax:String>F-2026-0042</uax:String>") {
		t.Errorf("missing scalar DefaultValue:\n%s", out)
	}
	// Children block must appear before References inside the instance Object.
	obj := out[strings.Index(out, `SymbolicName="Furnace01"`):]
	if strings.Index(obj, "<opc:Children>") > strings.Index(obj, "<opc:References>") {
		t.Errorf("Children must precede References:\n%s", obj)
	}
	// Override child carries no ModellingRule/ValueRank/AccessLevel (property).
	// Search within the Furnace01 Object (obj) — SerialNumber also appears in the
	// type definition, which legitimately carries ModellingRule.
	propTag := obj[strings.Index(obj, `SymbolicName="SerialNumber"`):]
	propTag = propTag[:strings.Index(propTag, ">")]
	for _, banned := range []string{"ModellingRule", "ValueRank", "AccessLevel"} {
		if strings.Contains(propTag, banned) {
			t.Errorf("override property should omit %s: %q", banned, propTag)
		}
	}
}

// TestEmitInstanceScalarTypes pins the DataType -> uax:* Variant mapping and the
// ValueRank rule (variables carry ValueRank="Scalar"; properties do not).
func TestEmitInstanceScalarTypes(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, prefix: X, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"enums: { Mode: { values: [Off, On, Auto] } }\n" +
		"object_types:\n  T:\n    base: OpcUa:BaseObjectType\n    members:\n" +
		"      Name:  { kind: property, type: String }\n" +
		"      Count: { type: UInt32 }\n" +
		"      Ready: { type: Boolean }\n" +
		"      Mode:  { type: Mode }\n" +
		"instances:\n  I1:\n    type: T\n    under: OpcUa:ObjectsFolder\n    values:\n" +
		"      Name: \"hi\"\n      Count: 7\n      Ready: true\n      Mode: Auto\n"
	out := emitSrc(t, src)
	inst := out[strings.Index(out, `SymbolicName="I1"`):] // scope to the instance Object

	for _, want := range []string{
		`<uax:String>hi</uax:String>`,
		`<uax:UInt32>7</uax:UInt32>`,
		`<uax:Boolean>true</uax:Boolean>`,
		`<uax:Int32>2</uax:Int32>`, // Mode: Auto -> enum id 2
	} {
		if !strings.Contains(inst, want) {
			t.Errorf("missing %s in instance:\n%s", want, inst)
		}
	}
	// The property override (Name) has no ValueRank; each variable override does.
	if tag := childTag(inst, "Name"); strings.Contains(tag, "ValueRank") {
		t.Errorf("property override Name should omit ValueRank: %q", tag)
	}
	for _, v := range []string{"Count", "Ready", "Mode"} {
		if tag := childTag(inst, v); !strings.Contains(tag, `ValueRank="Scalar"`) {
			t.Errorf("variable override %s should carry ValueRank=Scalar: %q", v, tag)
		}
	}
}

// childTag returns the opening tag text of the child with the given SymbolicName.
func childTag(s, name string) string {
	i := strings.Index(s, `SymbolicName="`+name+`"`)
	if i < 0 {
		return ""
	}
	rest := s[i:]
	return rest[:strings.Index(rest, ">")]
}

func emitWithCatalog(t *testing.T, src string) string {
	t.Helper()
	m, err := dsl.Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := nodeset.LoadForImports([]string{dsl.ISA95NamespaceURI})
	if err != nil {
		t.Fatal(err)
	}
	m.Catalog = cat
	b, err := Emit(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEmitEquipmentLevel(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/, ISA95: http://www.OPCFoundation.org/UA/2013/01/ISA95 }\n" +
		"object_types: { FillerType: { base: OpcUa:BaseObjectType } }\n" +
		"instances:\n" +
		"  Site1: { type: ISA95:EquipmentType, level: Site, under: OpcUa:ObjectsFolder }\n" +
		"  Mach1: { type: FillerType, under: Site1 }\n"
	out := emitWithCatalog(t, src)
	for _, want := range []string{
		`SymbolicName="EquipmentLevel"`,
		`<uax:Int32>1</uax:Int32>`, // Site -> 1
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Equipment leaf (Machinery-style local type, no EquipmentLevel member) must not gain one.
	if strings.Count(out, `SymbolicName="EquipmentLevel"`) != 1 {
		t.Errorf("expected exactly one EquipmentLevel node:\n%s", out)
	}
}

func TestEmitInstanceChild(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, prefix: X, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
		"object_types:\n  HeatingZoneType: { base: OpcUa:BaseObjectType }\n" +
		"  FurnaceType:\n    base: OpcUa:BaseObjectType\n    members:\n      Zones:\n        kind: object\n        type: OpcUa:FolderType\n        children:\n          \"Zone<No>\": { kind: object, type: HeatingZoneType, rule: optional_placeholder }\n" +
		"instances:\n  Furnace01:\n    type: FurnaceType\n    under: OpcUa:ObjectsFolder\n    children:\n      Zone1: { of: \"Zone<No>\" }\n"
	out := emitSrc(t, src)
	if !strings.Contains(out, `<opc:Object SymbolicName="Zone1" TypeDefinition="HeatingZoneType">`) &&
		!strings.Contains(out, `<opc:Object SymbolicName="Zone1" TypeDefinition="HeatingZoneType"/>`) {
		t.Errorf("missing instantiated child Zone1:\n%s", out)
	}
	if strings.Contains(out, `SymbolicName="Zone1"`) && strings.Contains(out[strings.Index(out, `"Zone1"`):], "BrowseName") {
		t.Errorf("instantiated child must not carry a placeholder BrowseName:\n%s", out)
	}
}
