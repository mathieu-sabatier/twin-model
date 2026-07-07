package modeldesign_test

import (
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/modeldesign"
)

func emitPersp(t *testing.T, src string) string {
	t.Helper()
	m, err := dsl.Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	b, err := modeldesign.Emit(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const exportPerspSrc = "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
	"object_types: { FT: { base: OpcUa:BaseObjectType } }\n" +
	"instances: { Casepacker01: { type: FT, under: OpcUa:ObjectsFolder } }\n" +
	"perspectives:\n  zones:\n    export: true\n    nodes:\n" +
	"      hall_b: { children: [zone_b2] }\n      zone_b2: { members: [Casepacker01] }\n"

func TestEmitPerspectiveExport(t *testing.T) {
	out := emitPersp(t, exportPerspSrc)
	for _, want := range []string{
		`SymbolicName="zones_hall_b" TypeDefinition="ua:FolderType"`,
		`SymbolicName="zones_zone_b2" TypeDefinition="ua:FolderType"`,
		`<opc:TargetId>zones_zone_b2</opc:TargetId>`, // hall_b organizes zone_b2
		`<opc:TargetId>Casepacker01</opc:TargetId>`,  // zone_b2 organizes the instance
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestEmitPerspectiveExportOffByDefault(t *testing.T) {
	off := strings.Replace(exportPerspSrc, "export: true", "export: false", 1)
	if out := emitPersp(t, off); strings.Contains(out, "zones_hall_b") {
		t.Errorf("export:false must emit no perspective nodes:\n%s", out)
	}
}
