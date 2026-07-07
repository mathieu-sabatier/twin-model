package dto

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

var update = flag.Bool("update", false, "update golden files")

// TestModelGolden is the drift guard for the SPA's hand-kept types.ts: it marshals
// the real example model through the DTO mappers and pins the JSON shape. Regenerate
// intentionally with `go test ./internal/api/dto -run TestModelGolden -update`.
func TestModelGolden(t *testing.T) {
	data, err := os.ReadFile("../../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := dsl.Parse("equipment.yaml", data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := json.MarshalIndent(FromModel(m), "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')
	golden := filepath.Join("testdata", "model.golden.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("DTO JSON drifted from golden. If intentional, run with -update.\n--- got ---\n%s", got)
	}
}

// TestFromModelMapsPerspectivesAndLevel checks the non-golden mapping of the
// Part-1 `level`/`hierarchy` and Part-3 `perspectives` fields (added for the
// ISA-95 hierarchy feature) into the API DTO.
func TestFromModelMapsPerspectivesAndLevel(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/, ISA95: http://www.OPCFoundation.org/UA/2013/01/ISA95 }\n" +
		"hierarchy: { allowLevelSkip: true }\n" +
		"object_types: { FT: { base: OpcUa:BaseObjectType } }\n" +
		"instances: { Site1: { type: ISA95:EquipmentType, level: Site, under: OpcUa:ObjectsFolder }, A: { type: FT, under: Site1 } }\n" +
		"perspectives: { zones: { nodes: { n: { members: [A] } } } }\n"
	m, err := dsl.Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	d := FromModel(m)
	if d.Hierarchy == nil || !d.Hierarchy.AllowLevelSkip {
		t.Errorf("hierarchy not mapped: %+v", d.Hierarchy)
	}
	if d.Instances[0].Level != "Site" {
		t.Errorf("level not mapped: %q", d.Instances[0].Level)
	}
	if len(d.Perspectives) != 1 || d.Perspectives[0].Nodes[0].Members[0] != "A" {
		t.Errorf("perspectives not mapped: %+v", d.Perspectives)
	}
}

// TestDiagnosticSeverityString maps the dsl Severity enum to a stable JSON string.
func TestDiagnosticSeverityString(t *testing.T) {
	d := FromDiagnostic(dsl.Diagnostic{Severity: dsl.SeverityError, Code: "x", Path: "p"})
	if d.Severity != "error" {
		t.Errorf("severity = %q, want error", d.Severity)
	}
	w := FromDiagnostic(dsl.Diagnostic{Severity: dsl.SeverityWarning})
	if w.Severity != "warning" {
		t.Errorf("severity = %q, want warning", w.Severity)
	}
}

// TestCatalogGolden pins the catalog DTO JSON shape for the SPA's types.ts
// mirror. Regenerate with:
//
//	go test ./internal/api/dto -run TestCatalogGolden -update
func TestCatalogGolden(t *testing.T) {
	// Hand-built representative detail — stable, no dependency on nodeset here.
	detail := CatalogTypeDetail{
		Alias: "DI", URI: "http://opcfoundation.org/UA/DI/",
		Name: "DeviceType", NodeClass: "ObjectType", Abstract: true,
		BaseChain: []CatalogTypeRef{
			{Alias: "", Name: "TopologyElementType", URI: "http://opcfoundation.org/UA/DI/"},
			{Alias: "", Name: "BaseObjectType", URI: "http://opcfoundation.org/UA/"},
		},
		// Manufacturer has a primitive/ns0 type (no linkable Type); ParameterSet's
		// type is a bundled companion type, so it gets a linkable Type ref.
		// Level is an enum-typed variable, so it gets an Enum field.
		Members: FromCatalogMembers([]dsl.CatalogMember{
			{Name: "Manufacturer", Kind: dsl.KindProperty},
			{Name: "ParameterSet", Kind: dsl.KindObject, TypeURI: "http://opcfoundation.org/UA/DI/", TypeName: "FunctionalGroupType"},
			{Name: "Level", Kind: dsl.KindVariable, Enum: []dsl.EnumMember{{Name: "Enterprise", Value: 0}, {Name: "Site", Value: 1}}},
		}, func(uri string) string {
			if uri == "http://opcfoundation.org/UA/DI/" {
				return "DI"
			}
			return ""
		}),
	}
	got, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')
	golden := filepath.Join("testdata", "catalog.golden.json")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("catalog DTO JSON drifted. If intentional, run with -update.\n--- got ---\n%s", got)
	}
}
