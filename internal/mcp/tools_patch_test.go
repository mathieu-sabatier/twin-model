package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// patchBaseSrc's namespace ("urn:x") deliberately lacks the required trailing
// slash (see CodeNamespaceSlash in internal/dsl/validate.go), so it is already
// error-red on its own. Under strict-by-default writes this means every
// "happy path" test below needs force:true to get past storeMutatedDraft's
// whole-file gate, independent of whatever the add_* call itself splices in —
// see the equivalent note on baseSrc in internal/core/patch_test.go.
const patchBaseSrc = "model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n"

// cleanPatchBaseSrc is patchBaseSrc with a spec-compliant namespace, and
// mixerErrorBody is a type body carrying exactly one error-severity diagnostic
// (CodeUnknownUnit) — both mirror internal/core/patch_test.go's identically
// named constants, used to isolate the strict-refusal test below from
// patchBaseSrc's own pre-existing error.
const cleanPatchBaseSrc = "model:\n  name: Demo\n  namespace: urn:x/\n  version: 1.0.0\n  publication_date: 2026-01-01\n"
const mixerErrorBody = "doc: A mixer\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: Double, unit: notaunit }\n"

func TestAddImportTool_AddsImport(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(patchBaseSrc)})
	c, ctx := newClientFor(t, svc)
	// force:true — patchBaseSrc alone is error-red (missing namespace trailing
	// slash); this test is about import splicing, not strictness.
	res := callTool(t, c, ctx, "add_import", map[string]any{
		"draftId": d.ID, "file": "demo.yaml", "alias": "DI", "namespace": "http://opcfoundation.org/UA/DI/", "force": true})
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
	// force:true — patchBaseSrc alone is error-red (missing namespace trailing
	// slash); this test is about type splicing, not strictness.
	res := callTool(t, c, ctx, "add_type", map[string]any{
		"draftId": d.ID, "name": "WidgetType",
		"body":  "doc: A widget\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: OpcUa:Double }\n",
		"force": true})
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
	// force:true — patchBaseSrc is error-red (missing namespace trailing slash),
	// and "under: Cell1" also references an instance that is never declared
	// (CodeUnknownUnder). This test is about splice mechanics, not
	// strictness/forward-references.
	res := callTool(t, c, ctx, "add_instance", map[string]any{
		"draftId": d.ID, "name": "Widget1", "body": "type: WidgetType\nunder: Cell1\n", "force": true})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "Widget1:") {
		t.Fatalf("instance not stored:\n%s", d.Files["demo.yaml"])
	}
}

// TestAddTypeTool_RefusesErrorSeverity is the MCP-layer counterpart of
// internal/core's TestAddType_ErrorSeverity_Refuses/StoresWithForce: add_type
// must refuse (a tool-error result naming the blocker) when the resulting file
// would carry an error-severity validation diagnostic, unless force:true.
func TestAddTypeTool_RefusesErrorSeverity(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	src := cleanPatchBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n"
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "add_type", map[string]any{
		"draftId": d.ID, "name": "MixerType", "body": mixerErrorBody})
	if !res.IsError {
		t.Fatalf("want refusal for error-severity validation, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "unknown-unit") {
		t.Fatalf("refusal message = %+v, want it to name the unknown-unit blocker", res.Content)
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type was stored despite refusal:\n%s", d.Files["demo.yaml"])
	}

	res = callTool(t, c, ctx, "add_type", map[string]any{
		"draftId": d.ID, "name": "MixerType", "body": mixerErrorBody, "force": true})
	if res.IsError {
		t.Fatalf("force:true should store despite error-severity diagnostics: %s", requireText(t, res))
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type not stored with force:true:\n%s", d.Files["demo.yaml"])
	}
}

// TestAddTypeTool_DryRun_NotStored is the MCP-layer counterpart of
// internal/core's TestAddType_DryRun_ValidatesWithoutStoring: dryRun:true must
// validate in full draft context (the unknown-unit diagnostic shows up) but
// must not be a tool-error result (dry-run never refuses) and must leave the
// draft's stored file byte-unchanged.
func TestAddTypeTool_DryRun_NotStored(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	src := cleanPatchBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n"
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
	before := string(d.Files["demo.yaml"])
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "add_type", map[string]any{
		"draftId": d.ID, "name": "MixerType", "body": mixerErrorBody, "dryRun": true})
	if res.IsError {
		t.Fatalf("dry-run must not be a refusal, got error result: %+v", res.Content)
	}
	text := requireText(t, res)
	if !strings.Contains(text, `"stored": false`) {
		t.Fatalf("result = %s, want it to report stored:false", text)
	}
	if !strings.Contains(text, "unknown-unit") {
		t.Fatalf("result = %s, want the unknown-unit diagnostic previewed", text)
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type was stored despite dryRun:true:\n%s", d.Files["demo.yaml"])
	}
	if string(d.Files["demo.yaml"]) != before {
		t.Fatalf("draft was mutated by a dry-run:\n%s", d.Files["demo.yaml"])
	}
}

// mixerTypeAndInstancePatchSrc mirrors internal/core/patch_test.go's
// mixerTypeAndInstanceSrc: a clean-namespace model with a type (MixerType)
// and an instance (Mixer1) whose type: references it, so removing MixerType
// leaves a dangling reference — CodeUnknownType ("unknown-type"),
// error-severity — with no other diagnostic in play.
const mixerTypeAndInstancePatchSrc = cleanPatchBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
	"object_types:\n  MixerType:\n    base: OpcUa:BaseObjectType\n" +
	"instances:\n  Mixer1:\n    type: MixerType\n"

func TestRemoveTypeTool_Removes(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	src := patchBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
		"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n"
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
	c, ctx := newClientFor(t, svc)
	// force:true — patchBaseSrc alone is error-red (missing namespace trailing
	// slash); this test is about removal mechanics, not strictness.
	res := callTool(t, c, ctx, "remove_type", map[string]any{
		"draftId": d.ID, "name": "WidgetType", "force": true})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "WidgetType") {
		t.Fatalf("type still present after removal:\n%s", d.Files["demo.yaml"])
	}
}

func TestRemoveInstanceTool_Removes(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	src := patchBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
		"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n" +
		"instances:\n  Widget1:\n    type: WidgetType\n"
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
	c, ctx := newClientFor(t, svc)
	// force:true — patchBaseSrc alone is error-red (missing namespace trailing
	// slash); this test is about removal mechanics, not strictness.
	res := callTool(t, c, ctx, "remove_instance", map[string]any{
		"draftId": d.ID, "name": "Widget1", "force": true})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "Widget1") {
		t.Fatalf("instance still present after removal:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveTypeTool_StillReferenced_IsError is the MCP-layer counterpart of
// internal/core's TestRemoveType_StillReferenced_RefusedWithoutForce:
// remove_type must refuse (a tool-error result naming the unknown-type
// blocker) when the removal would leave a dangling instance type reference,
// unless force:true.
func TestRemoveTypeTool_StillReferenced_IsError(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(mixerTypeAndInstancePatchSrc)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "remove_type", map[string]any{"draftId": d.ID, "name": "MixerType"})
	if !res.IsError {
		t.Fatalf("want refusal for a still-referenced type, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "unknown-type") {
		t.Fatalf("refusal message = %+v, want it to name the unknown-type blocker", res.Content)
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type was removed despite refusal:\n%s", d.Files["demo.yaml"])
	}

	res = callTool(t, c, ctx, "remove_type", map[string]any{"draftId": d.ID, "name": "MixerType", "force": true})
	if res.IsError {
		t.Fatalf("force:true should remove despite error-severity diagnostics: %s", requireText(t, res))
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type still present after forced removal:\n%s", d.Files["demo.yaml"])
	}
}
