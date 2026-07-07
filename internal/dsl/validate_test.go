package dsl

import (
	"os"
	"strings"
	"testing"
)

const hdr = "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/ }\n"

func codesFor(t *testing.T, src string) []string {
	t.Helper()
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse (should succeed, error is semantic): %v", err)
	}
	var codes []string
	for _, d := range Validate(m) {
		codes = append(codes, d.Code)
	}
	return codes
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{"unknown-type", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: Nope } } } }`, "unknown-type"},
		{"unknown-import-alias", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: Foo:Bar } } } }`, "unknown-import-alias"},
		{"unknown-base", hdr + `object_types: { T: { base: Missing } }`, "unknown-base"},
		{"duplicate-type", hdr + `enums: { Foo: { values: [A] } }` + "\n" + `object_types: { Foo: { base: OpcUa:BaseObjectType } }`, "duplicate-type"},
		{"duplicate-member", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { State: { type: Double }, "State<Id>": { kind: object, type: OpcUa:FolderType, rule: optional_placeholder } } } }`, "duplicate-member"},
		{"inheritance-cycle", hdr + `object_types: { A: { base: B }, B: { base: A } }`, "inheritance-cycle"},
		{"placeholder-without-rule", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { "Zone<No>": { kind: object, type: OpcUa:FolderType } } } }`, "placeholder-without-rule"},
		{"rule-without-placeholder", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { Zone: { kind: object, type: OpcUa:FolderType, rule: optional_placeholder } } } }`, "rule-without-placeholder"},
		{"unknown-unit", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: Double, unit: furlong } } } }`, "unknown-unit"},
		{"unit-on-non-numeric", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: String, unit: "°C" } } } }`, "unit-on-non-numeric"},
		{"unit-on-property", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { kind: property, type: Double, unit: kN } } } }`, "unit-on-property"},
		{"abstract-instance", hdr + `object_types: { AbsT: { base: OpcUa:BaseObjectType, abstract: true } }` + "\n" + `instances: { i1: { type: AbsT, under: OpcUa:ObjectsFolder } }`, "abstract-instance"},
		{"namespace-trailing-slash", "model: { name: M, namespace: https://x, version: 1.0.0, publication_date: 2026-07-02 }", "namespace-trailing-slash"},
		{"version-semver", `model: { name: M, namespace: https://x/, version: "1.0", publication_date: 2026-07-02 }`, "version-semver"},
		{"invalid-kind", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { kind: widget, type: Double } } } }`, "invalid-kind"},
		{"empty-enum", hdr + `enums: { E: { doc: x } }`, "empty-enum"},
		{"duplicate-enum-value", hdr + `enums: { E: { values: [A, A] } }`, "duplicate-enum-value"},
		{"duplicate-enum-id", hdr + `enums: { E: { values: [ {A: 1}, {B: 1} ] } }`, "duplicate-enum-id"},
		{"negative-enum-id", hdr + `enums: { E: { values: [ {A: -1} ] } }`, "negative-enum-id"},
		{"unit-requires-variable", hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { M: { kind: method, unit: kN } } } }`, "unit-requires-variable"},
		{"unknown-value-member", hdr + `object_types: { FT: { base: OpcUa:BaseObjectType, members: { Serial: { kind: property, type: String } } } }` + "\n" +
			`instances: { F1: { type: FT, under: OpcUa:ObjectsFolder, values: { Nope: "x" } } }`, "unknown-value-member"},
		{"value-not-value-bearing", hdr + `object_types: { FT: { base: OpcUa:BaseObjectType, members: { Zones: { kind: object, type: OpcUa:FolderType } } } }` + "\n" +
			`instances: { F1: { type: FT, under: OpcUa:ObjectsFolder, values: { Zones: "x" } } }`, "value-not-value-bearing"},
		{"unknown-placeholder", hdr + `object_types: { FT: { base: OpcUa:BaseObjectType, members: { Real: { kind: property, type: String } } } }` + "\n" +
			`instances: { F1: { type: FT, under: OpcUa:ObjectsFolder, children: { Z1: { of: "Zone<No>" } } } }`, "unknown-placeholder"},
		{"unknown-under", hdr + `object_types: { FT: { base: OpcUa:BaseObjectType } }` + "\n" +
			`instances: { F1: { type: FT, under: Ghost } }`, "unknown-under"},
		{"unknown-under-builtin", hdr + `object_types: { FT: { base: OpcUa:BaseObjectType } }` + "\n" +
			`instances: { F1: { type: FT, under: Double } }`, "unknown-under"},
		{"instance-cycle", hdr + `object_types: { FT: { base: OpcUa:BaseObjectType } }` + "\n" +
			`instances: { A: { type: FT, under: B }, B: { type: FT, under: A } }`, "instance-cycle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			codes := codesFor(t, c.src)
			if !hasCode(codes, c.code) {
				t.Errorf("want diagnostic %q; got %v", c.code, codes)
			}
		})
	}
}

func TestValidateNestingUnderInstance(t *testing.T) {
	src := hdr + `object_types: { FT: { base: OpcUa:BaseObjectType } }` + "\n" +
		`instances: { Parent: { type: FT, under: OpcUa:ObjectsFolder }, Child: { type: FT, under: Parent } }`
	codes := codesFor(t, src)
	if len(codes) != 0 {
		t.Errorf("valid nesting should be clean, got %v", codes)
	}
}

func TestValidateMissingType(t *testing.T) {
	// A non-method member with no type must be flagged (QA finding M1).
	src := hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { NewMember: {} } } }`
	if !hasCode(codesFor(t, src), CodeMissingType) {
		t.Errorf("expected %q for a typeless member", CodeMissingType)
	}
	// A method member legitimately has no type — must NOT be flagged.
	okSrc := hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { Go: { kind: method } } } }`
	if hasCode(codesFor(t, okSrc), CodeMissingType) {
		t.Errorf("method members should not require a type")
	}
}

func diagCodes(diags []Diagnostic) map[string]bool {
	out := map[string]bool{}
	for _, d := range diags {
		out[d.Code] = true
	}
	return out
}

func TestValidateUnknownImportType(t *testing.T) {
	m := &Model{
		Pos:  Pos{File: "m.yaml"},
		Name: "M", Namespace: "https://ex/UA/M/", Version: "1.0.0", PublicationDate: "2026-07-04",
		Imports: []Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}},
		ObjectTypes: []*ObjectType{{
			Name: "PumpType", Pos: Pos{File: "m.yaml"},
			Base: TypeRef{Alias: "DI", Name: "Nope", Raw: "DI:Nope"},
		}},
		Catalog: fakeCatalog{
			ns:    map[string]CatalogNamespace{"http://opcfoundation.org/UA/DI/": {URI: "http://opcfoundation.org/UA/DI/"}},
			types: map[string]CatalogType{},
		},
	}
	if !diagCodes(Validate(m))[CodeUnknownImportType] {
		t.Error("expected unknown-import-type for DI:Nope")
	}
}

func TestValidateImportNotBundled(t *testing.T) {
	m := &Model{
		Pos:  Pos{File: "m.yaml"},
		Name: "M", Namespace: "https://ex/UA/M/", Version: "1.0.0", PublicationDate: "2026-07-04",
		Imports: []Import{{Alias: "X", URI: "http://example.com/UA/X/", Pos: Pos{File: "m.yaml"}}},
		Catalog: fakeCatalog{ns: map[string]CatalogNamespace{}, types: map[string]CatalogType{}},
	}
	if !diagCodes(Validate(m))[CodeImportNotBundled] {
		t.Error("expected import-not-bundled for an unbundled URI")
	}
}

func TestValidateAbstractImportInstance(t *testing.T) {
	m := &Model{
		Pos:  Pos{File: "m.yaml"},
		Name: "M", Namespace: "https://ex/UA/M/", Version: "1.0.0", PublicationDate: "2026-07-04",
		Imports: []Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}},
		Instances: []*Instance{{
			Name: "D1", Pos: Pos{File: "m.yaml"},
			Type:  TypeRef{Alias: "DI", Name: "DeviceType", Raw: "DI:DeviceType"},
			Under: TypeRef{Alias: "OpcUa", Name: "ObjectsFolder", Raw: "OpcUa:ObjectsFolder"},
		}},
		Catalog: fakeCatalog{
			ns: map[string]CatalogNamespace{"http://opcfoundation.org/UA/DI/": {URI: "http://opcfoundation.org/UA/DI/"}},
			types: map[string]CatalogType{
				"http://opcfoundation.org/UA/DI/|DeviceType": {NamespaceURI: "http://opcfoundation.org/UA/DI/", Name: "DeviceType", NodeClass: "ObjectType", Abstract: true},
			},
		},
	}
	if !diagCodes(Validate(m))[CodeAbstractInstance] {
		t.Error("expected abstract-instance for an abstract companion type")
	}
}

// isaHdr is the shared header for hierarchy tests: OpcUa + ISA95 imports.
const isaHdr = "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/, ISA95: http://www.OPCFoundation.org/UA/2013/01/ISA95 }\n"

// eqTypes declares a stand-in equipment type with no EquipmentLevel member.
const eqTypes = "object_types: { FillerType: { base: OpcUa:BaseObjectType } }\n"

// isa95Catalog is a minimal dsl.Catalog knowing only ISA95:EquipmentType and its
// EquipmentLevel member, so validator tests need neither NodeSet2 XML nor an
// import of internal/nodeset.
type isa95Catalog struct{}

func (isa95Catalog) Namespace(uri string) (CatalogNamespace, bool) {
	return CatalogNamespace{URI: uri}, true
}
func (isa95Catalog) LookupType(uri, name string) (CatalogType, bool) {
	if uri == ISA95NamespaceURI && name == "EquipmentType" {
		return CatalogType{
			NamespaceURI: uri, Name: "EquipmentType", NodeClass: "ObjectType",
			Members: []CatalogMember{{
				Name: ISA95EquipmentLevelMember, Kind: KindVariable, TypeURI: uri, TypeName: ISA95EquipmentLevelEnum,
			}},
		}, true
	}
	if uri == OpcUaNamespaceURI {
		// Production loads ns0 in full, so valid OpcUa refs (e.g. ObjectsFolder,
		// used as under-targets in these fixtures) resolve. Mirror that here so the
		// A5 OpcUa narrowing does not emit a spurious unknown-import-type.
		return CatalogType{NamespaceURI: uri, Name: name, NodeClass: "ObjectType"}, true
	}
	return CatalogType{}, false
}

func codesForCatalog(t *testing.T, src string) []string {
	t.Helper()
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse (semantic errors expected, not parse errors): %v", err)
	}
	m.Catalog = isa95Catalog{}
	var codes []string
	for _, d := range Validate(m) {
		codes = append(codes, d.Code)
	}
	return codes
}

func TestValidateHierarchy(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"unknown-level",
			isaHdr + `instances: { P: { type: ISA95:EquipmentType, level: Nope, under: OpcUa:ObjectsFolder } }`,
			"unknown-level"},
		{"level-on-unsupported-type",
			isaHdr + eqTypes + `instances: { P: { type: FillerType, level: Site, under: OpcUa:ObjectsFolder } }`,
			"level-on-unsupported-type"},
		{"level-order-inverted",
			isaHdr + "instances:\n" +
				"  A: { type: ISA95:EquipmentType, level: Area, under: OpcUa:ObjectsFolder }\n" +
				"  S: { type: ISA95:EquipmentType, level: Site, under: A }\n",
			"hierarchy-level-order"},
		{"level-skip-strict",
			isaHdr + "instances:\n" +
				"  S: { type: ISA95:EquipmentType, level: Site, under: OpcUa:ObjectsFolder }\n" +
				"  W: { type: ISA95:EquipmentType, level: WorkCenter, under: S }\n",
			"hierarchy-level-skip"},
		{"equipment-parent",
			isaHdr + eqTypes + "instances:\n" +
				"  A: { type: ISA95:EquipmentType, level: Area, under: OpcUa:ObjectsFolder }\n" +
				"  M: { type: FillerType, under: A }\n",
			"equipment-parent"},
		{"machine-under-machine",
			isaHdr + eqTypes + "instances:\n" +
				"  M1: { type: FillerType, under: OpcUa:ObjectsFolder }\n" +
				"  M2: { type: FillerType, under: M1 }\n",
			"machine-under-machine"},
		{"equipment-type-no-level-nested",
			isaHdr + "instances:\n" +
				"  M1: { type: ISA95:EquipmentType, under: OpcUa:ObjectsFolder }\n" +
				"  M2: { type: ISA95:EquipmentType, under: M1 }\n",
			"machine-under-machine"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if codes := codesForCatalog(t, c.src); !hasCode(codes, c.want) {
				t.Errorf("want %q; got %v", c.want, codes)
			}
		})
	}
}

func TestValidateLevelSkipFlagRelaxes(t *testing.T) {
	body := "instances:\n" +
		"  S: { type: ISA95:EquipmentType, level: Site, under: OpcUa:ObjectsFolder }\n" +
		"  W: { type: ISA95:EquipmentType, level: WorkCenter, under: S }\n"
	if codes := codesForCatalog(t, isaHdr+"hierarchy: { allowLevelSkip: true }\n"+body); hasCode(codes, "hierarchy-level-skip") {
		t.Errorf("allowLevelSkip:true should suppress hierarchy-level-skip; got %v", codes)
	}
}

// TestValidateUnderNS0ObjectsFolderNotFlagged pins the accepted behavior: when
// OpcUa (ns0) is present in the catalog but "ObjectsFolder" is not resolvable
// as a type (it's a UAObject instance, not a UAObjectType), using it as an
// under: target must NOT produce a CodeUnknownImportType diagnostic.
func TestValidateUnderNS0ObjectsFolderNotFlagged(t *testing.T) {
	m := &Model{
		Pos:  Pos{File: "m.yaml"},
		Name: "M", Namespace: "https://ex/UA/M/", Version: "1.0.0", PublicationDate: "2026-07-06",
		Imports: []Import{{Alias: "OpcUa", URI: OpcUaNamespaceURI}},
		ObjectTypes: []*ObjectType{{
			Name: "FurnaceType", Pos: Pos{File: "m.yaml"},
			Base: TypeRef{Alias: "OpcUa", Name: "BaseObjectType", Raw: "OpcUa:BaseObjectType"},
		}},
		Instances: []*Instance{{
			Name:  "Furnace1",
			Pos:   Pos{File: "m.yaml"},
			Type:  TypeRef{Name: "FurnaceType", Raw: "FurnaceType"},
			Under: TypeRef{Alias: "OpcUa", Name: "ObjectsFolder", Raw: "OpcUa:ObjectsFolder"},
		}},
		// ns0 namespace IS present, but ObjectsFolder is NOT in the type catalog
		// (it is a UAObject instance node i=85, not a UAObjectType).
		// BaseObjectType is included so checkInheritance does not emit a separate
		// unknown-import-type diagnostic that would obscure the assertion.
		Catalog: fakeCatalog{
			ns: map[string]CatalogNamespace{OpcUaNamespaceURI: {URI: OpcUaNamespaceURI}},
			types: map[string]CatalogType{
				OpcUaNamespaceURI + "|BaseObjectType": {NamespaceURI: OpcUaNamespaceURI, Name: "BaseObjectType", NodeClass: "ObjectType"},
				// ObjectsFolder intentionally absent — it is a UAObject instance, not a type
			},
		},
	}
	codes := diagCodes(Validate(m))
	if codes[CodeUnknownImportType] {
		t.Error("OpcUa:ObjectsFolder as under: target must not produce unknown-import-type; ns0 instance nodes are not in the type catalog")
	}
}

// TestValidateExampleClean: the canonical example must have zero diagnostics.
func TestValidateExampleClean(t *testing.T) {
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse("examples/equipment.yaml", data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(m)
	if len(diags) != 0 {
		var b strings.Builder
		for _, d := range diags {
			b.WriteString("\n  " + d.String())
		}
		t.Errorf("example should be clean, got %d diagnostic(s):%s", len(diags), b.String())
	}
}
