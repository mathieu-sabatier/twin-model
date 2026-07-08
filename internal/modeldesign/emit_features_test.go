package modeldesign

import (
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

func emit(t *testing.T, src string) string {
	t.Helper()
	m, err := dsl.Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return string(b)
}

const featureHdr = "model: { name: M, namespace: https://x/, prefix: Acme.M, version: 1.0.0, publication_date: 2026-07-02 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/ }\n"

// Explicit / non-contiguous enum ids must switch on ForceEnumValues — a path the
// contiguous example enum does not exercise.
func TestEmitEnumForceValues(t *testing.T) {
	out := emit(t, featureHdr+`enums: { Mode: { values: [ Off, { Manual: 5 }, Auto ] } }`)
	if !strings.Contains(out, `ForceEnumValues="true"`) {
		t.Errorf("expected ForceEnumValues=true for non-contiguous ids:\n%s", out)
	}
	if !strings.Contains(out, `<opc:Field Name="Manual" Identifier="5"/>`) {
		t.Errorf("expected explicit id 5 for Manual:\n%s", out)
	}
	if !strings.Contains(out, `<opc:Field Name="Auto" Identifier="2"/>`) {
		t.Errorf("expected positional id 2 for Auto:\n%s", out)
	}
}

// A contiguous 0..n enum must NOT set ForceEnumValues.
func TestEmitEnumContiguousNoForce(t *testing.T) {
	out := emit(t, featureHdr+`enums: { S: { values: [ A, B, C ] } }`)
	if strings.Contains(out, "ForceEnumValues") {
		t.Errorf("contiguous enum should not force values:\n%s", out)
	}
}

func TestEmitUnitVariable(t *testing.T) {
	out := emit(t, featureHdr+`object_types: { T: { base: OpcUa:BaseObjectType, members: { Force: { type: Double, unit: kN } } } }`)
	for _, want := range []string{
		`TypeDefinition="ua:AnalogUnitType"`,
		`<opc:Property SymbolicName="ua:EngineeringUnits" DataType="ua:EUInformation" ModellingRule="Mandatory">`,
		`<uax:Identifier>i=888</uax:Identifier>`,
		`<uax:UnitId>4338743</uax:UnitId>`,
		`<uax:Text>kN</uax:Text>`,
		`<uax:Text>kilonewton</uax:Text>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("unit output missing %q:\n%s", want, out)
		}
	}
}

func TestEmitPlaceholder(t *testing.T) {
	out := emit(t, featureHdr+`object_types: { T: { base: OpcUa:BaseObjectType, members: { "Zone<No>": { kind: object, type: OpcUa:FolderType, rule: optional_placeholder } } } }`)
	if !strings.Contains(out, `<opc:Object SymbolicName="Zone" TypeDefinition="ua:FolderType" ModellingRule="OptionalPlaceholder">`) {
		t.Errorf("placeholder object wrong:\n%s", out)
	}
	if !strings.Contains(out, `<opc:BrowseName>&lt;ZoneNo&gt;</opc:BrowseName>`) {
		t.Errorf("placeholder browse name wrong:\n%s", out)
	}
}

func TestEmitMethodNoArgsSelfCloses(t *testing.T) {
	out := emit(t, featureHdr+`object_types: { T: { base: OpcUa:BaseObjectType, members: { Stop: { kind: method } } } }`)
	if !strings.Contains(out, `<opc:Method SymbolicName="Stop" ModellingRule="Mandatory"/>`) {
		t.Errorf("expected self-closed method:\n%s", out)
	}
}

func TestEmitInstanceInverseOrganizes(t *testing.T) {
	src := featureHdr +
		`object_types: { FooType: { base: OpcUa:BaseObjectType } }` + "\n" +
		`instances: { Foo1: { type: FooType, under: OpcUa:ObjectsFolder } }`
	out := emit(t, src)
	for _, want := range []string{
		`<opc:Object SymbolicName="Foo1" TypeDefinition="FooType">`,
		`<opc:Reference IsInverse="true">`,
		`<opc:ReferenceType>ua:Organizes</opc:ReferenceType>`,
		`<opc:TargetId>ua:ObjectsFolder</opc:TargetId>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("instance output missing %q:\n%s", want, out)
		}
	}
}

// Extensibility: an import beyond OpcUa gets its own xmlns, namespace entry, and
// QName prefix — proving the emitter isn't hard-wired to a single import.
func TestEmitAdditionalImport(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, prefix: Acme.M, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/, DI: http://opcfoundation.org/UA/DI/ }\n" +
		"object_types: { PumpType: { base: DI:DeviceType } }"
	out := emit(t, src)
	for _, want := range []string{
		`xmlns:DI="http://opcfoundation.org/UA/DI/"`,
		`<opc:Namespace Name="DI" Prefix="DI" XmlNamespace="http://opcfoundation.org/UA/DI/" XmlPrefix="DI">http://opcfoundation.org/UA/DI/</opc:Namespace>`,
		`BaseType="DI:DeviceType"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("multi-import output missing %q:\n%s", want, out)
		}
	}
}

func TestEmitMandatoryPlaceholder(t *testing.T) {
	out := emit(t, featureHdr+`object_types: { T: { base: OpcUa:BaseObjectType, members: { "Item<Id>": { kind: object, type: OpcUa:BaseObjectType, rule: mandatory_placeholder } } } }`)
	if !strings.Contains(out, `ModellingRule="MandatoryPlaceholder"`) {
		t.Errorf("expected MandatoryPlaceholder:\n%s", out)
	}
	if !strings.Contains(out, `<opc:BrowseName>&lt;ItemId&gt;</opc:BrowseName>`) {
		t.Errorf("expected placeholder browse name:\n%s", out)
	}
}

// Determinism: two extra imports keep source order in both the xmlns block and the
// namespace table (own first, imports in order, OpcUa last).
func TestEmitTwoAdditionalImportsOrdered(t *testing.T) {
	out := emit(t, "model: { name: M, namespace: https://x/, prefix: Acme.M, version: 1.0.0, publication_date: 2026-07-02 }\n"+
		"imports: { OpcUa: http://opcfoundation.org/UA/, DI: http://opcfoundation.org/UA/DI/, IA: http://opcfoundation.org/UA/IA/ }\n")
	if di, ia := strings.Index(out, "xmlns:DI="), strings.Index(out, "xmlns:IA="); di < 0 || ia < 0 || di > ia {
		t.Errorf("xmlns order wrong: DI=%d IA=%d", di, ia)
	}
	nDI := strings.Index(out, `<opc:Namespace Name="DI"`)
	nIA := strings.Index(out, `<opc:Namespace Name="IA"`)
	nOpc := strings.Index(out, `<opc:Namespace Name="OpcUa"`)
	if nDI < 0 || nDI >= nIA || nIA >= nOpc {
		t.Errorf("namespace table order wrong: DI=%d IA=%d OpcUa=%d", nDI, nIA, nOpc)
	}
}

// Explicit ids force EnumValues even when they happen to be contiguous.
func TestEmitExplicitEnumForcesValues(t *testing.T) {
	out := emit(t, featureHdr+`enums: { S: { values: [ {A: 0}, {B: 1} ] } }`)
	if !strings.Contains(out, `ForceEnumValues="true"`) {
		t.Errorf("explicit ids should force values:\n%s", out)
	}
}

// A multi-line YAML doc collapses to a single-line Description (no raw newline).
func TestEmitMultilineDocCollapsed(t *testing.T) {
	out := emit(t, featureHdr+"enums:\n  E:\n    doc: |\n      line one\n      line two\n    values: [A]\n")
	if !strings.Contains(out, `<opc:Description>line one line two</opc:Description>`) {
		t.Errorf("multi-line doc should collapse to one line:\n%s", out)
	}
}

func TestSelfCloseEmpties(t *testing.T) {
	in := strings.Join([]string{
		`  <opc:Field Name="Idle" Identifier="0"></opc:Field>`, // empty -> self-close
		`  <opc:Description>hi</opc:Description>`,              // text content -> unchanged
		`      <opc:Children>`,                                 // container open -> unchanged
		`      </opc:Children>`,                                // container close -> unchanged
		`  <uax:Text>°C</uax:Text>`,                            // unicode text -> unchanged
		`  <opc:BrowseName>&lt;ZoneNo&gt;</opc:BrowseName>`,    // escaped text -> unchanged
	}, "\n")
	want := strings.Join([]string{
		`  <opc:Field Name="Idle" Identifier="0"/>`,
		`  <opc:Description>hi</opc:Description>`,
		`      <opc:Children>`,
		`      </opc:Children>`,
		`  <uax:Text>°C</uax:Text>`,
		`  <opc:BrowseName>&lt;ZoneNo&gt;</opc:BrowseName>`,
	}, "\n")
	if got := selfCloseEmpties(in); got != want {
		t.Errorf("selfCloseEmpties mismatch:\n got:  %q\n want: %q", got, want)
	}
}
