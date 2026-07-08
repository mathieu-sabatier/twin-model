package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

const patchBaseSrc = "model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n"

func TestAddImportTool_AddsImport(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(patchBaseSrc)})
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "add_import", map[string]any{
		"draftId": d.ID, "file": "demo.yaml", "alias": "DI", "namespace": "http://opcfoundation.org/UA/DI/"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	// dsl.Format's scalar() quotes any value containing ':' (see needsQuote in
	// internal/dsl/format.go), so a URI import comes back double-quoted.
	if !strings.Contains(string(d.Files["demo.yaml"]), `DI: "http://opcfoundation.org/UA/DI/"`) {
		t.Fatalf("import not stored:\n%s", d.Files["demo.yaml"])
	}
}

func TestAddTypeTool_SplicesType(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	src := patchBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n"
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "add_type", map[string]any{
		"draftId": d.ID, "name": "WidgetType",
		"body": "doc: A widget\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: OpcUa:Double }\n"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "WidgetType:") {
		t.Fatalf("type not stored:\n%s", d.Files["demo.yaml"])
	}
}

func TestAddInstanceTool_SplicesInstance(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	src := patchBaseSrc + "object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n"
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "add_instance", map[string]any{
		"draftId": d.ID, "name": "Widget1", "body": "type: WidgetType\nunder: Cell1\n"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "Widget1:") {
		t.Fatalf("instance not stored:\n%s", d.Files["demo.yaml"])
	}
}
