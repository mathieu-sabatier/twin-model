package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// modelYAML is a minimal, self-contained model with one base type and one
// derived type, used to exercise the inline-YAML tools (parse_model,
// resolve_type, preview_modeldesign, preview_diagram).
const modelYAML = `model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }
imports: { OpcUa: http://opcfoundation.org/UA/ }
object_types:
  BaseT:
    base: OpcUa:BaseObjectType
    members:
      Manufacturer: { kind: property, type: String }
  DerivedT:
    base: BaseT
    members:
      Extra: { type: Boolean }
`

// newTestClient builds an MCP server over a host-less core.Service — fine for the
// stateless/catalog tools exercised here; get_model/list_model_files need a real
// git host and are covered by mounted-/mcp integration test — and
// connects an in-process client to it. This drives the real MCP protocol
// (initialize, tools/list, tools/call) rather than calling core directly, so the
// tool registrations (names, argument wiring, result encoding) are what's under
// test, not just the core.Service methods behind them.
func newTestClient(t *testing.T) (*client.Client, context.Context) {
	t.Helper()
	return newClientFor(t, core.New(nil, core.NewStore(time.Hour)))
}

// newClientFor wires an in-process MCP client to a server built over the given
// core.Service. Split out from newTestClient so a test can seed the service's
// Store (e.g. via svc.Store().Create) before any tool call reaches it — used by
// the draft-tool tests in tools_draft_test.go, which need a pre-existing draft
// but still want to exercise the real MCP protocol, not call core directly.
func newClientFor(t *testing.T, svc *core.Service) (*client.Client, context.Context) {
	t.Helper()
	srv := NewServer(svc)
	c, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "twinmodel-mcp-test", Version: "0.0.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}
	return c, ctx
}

// callTool invokes a tool by name through the real MCP request path. The
// returned result may have IsError set — callers expecting success should use
// requireText, callers expecting failure should assert res.IsError themselves.
func callTool(t *testing.T, c *client.Client, ctx context.Context, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := c.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("CallTool(%s): transport error: %v", name, err)
	}
	return res
}

// requireText asserts the tool call succeeded (IsError unset) and returns its
// text content.
func requireText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool call returned an error result: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("tool result content is not text: %#v", res.Content[0])
	}
	return tc.Text
}

func TestNewServer_RegistersExpectedReadTools(t *testing.T) {
	c, ctx := newTestClient(t)
	res, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	want := []string{
		"get_schema", "parse_model", "get_model", "get_model_source", "list_model_files",
		"preview_modeldesign", "preview_diagram", "resolve_type", "find_unit",
		"list_namespaces", "list_types", "get_type_details", "search_types",
		"get_draft_source",
		"repo_info", "list_prs", "list_branches", "create_draft", "update_draft",
		"draft_diff", "propose_pr",
		"add_import",
		"add_type",
		"add_instance",
		"remove_type",
		"remove_instance",
		"remove_import",
		"list_drafts",
		"discard_draft",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q not registered; got %v", name, res.Tools)
		}
	}
	// registerReadTools + registerDraftTools together should be the full set —
	// catches an accidental duplicate/missing registration.
	if len(res.Tools) != len(want) {
		t.Errorf("got %d tools, want %d: %v", len(res.Tools), len(want), got)
	}
}

func TestGetSchema_ReturnsJSONSchemaText(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "get_schema", nil)
	text := requireText(t, res)
	if !strings.Contains(text, `"$schema"`) {
		t.Errorf("get_schema result does not look like a JSON Schema: %.200s", text)
	}
}

func TestParseModel_ReturnsASTAndDiagnostics(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "parse_model", map[string]any{"file": "Demo.yaml", "yaml": modelYAML})
	text := requireText(t, res)

	var out struct {
		File       string         `json:"file"`
		Model      map[string]any `json:"model"`
		ParseError string         `json:"parseError"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse_model result is not valid JSON: %v\n%s", err, text)
	}
	if out.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", out.ParseError)
	}
	if out.File != "Demo.yaml" || out.Model == nil {
		t.Fatalf("parse_model result missing model: %+v", out)
	}
}

func TestParseModel_BadYAML_ReturnsParseErrorAsData(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "parse_model", map[string]any{"file": "Bad.yaml", "yaml": "not: [valid"})
	// A structural parse error is data (ParseError set on the response), never a
	// tool error — mirrors core.Service.parseModelResponse's contract.
	text := requireText(t, res)
	var out struct {
		ParseError string `json:"parseError"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse_model result is not valid JSON: %v\n%s", err, text)
	}
	if out.ParseError == "" {
		t.Errorf("want a non-empty parseError for malformed YAML, got: %s", text)
	}
}

func TestResolveType_ReturnsFlattenedMembers(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "resolve_type", map[string]any{"file": "Demo.yaml", "yaml": modelYAML, "name": "DerivedT"})
	text := requireText(t, res)
	if !strings.Contains(text, "Manufacturer") || !strings.Contains(text, "Extra") {
		t.Errorf("resolve_type result missing inherited/own members: %s", text)
	}
}

func TestResolveType_UnknownType_IsToolError(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "resolve_type", map[string]any{"file": "Demo.yaml", "yaml": modelYAML, "name": "NopeType"})
	if !res.IsError {
		t.Fatalf("want tool error for unknown type, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "not found") {
		t.Errorf("want a not-found tool error, got: %+v", res.Content)
	}
}

func TestPreviewModelDesign_ReturnsXML(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "preview_modeldesign", map[string]any{"file": "Demo.yaml", "yaml": modelYAML})
	text := requireText(t, res)
	if !strings.Contains(text, "ModelDesign") {
		t.Errorf("preview_modeldesign result is not ModelDesign XML: %.120s", text)
	}
}

func TestPreviewDiagram_DefaultViewIsClassDiagram(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "preview_diagram", map[string]any{"file": "Demo.yaml", "yaml": modelYAML})
	text := requireText(t, res)
	if !strings.HasPrefix(text, "classDiagram") {
		t.Errorf("default preview_diagram view should be classDiagram, got: %.60s", text)
	}
}

func TestFindUnit_ReturnsUnits(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "find_unit", nil)
	text := requireText(t, res)
	var out struct {
		Units []map[string]any `json:"units"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("find_unit result is not valid JSON: %v", err)
	}
	if len(out.Units) == 0 {
		t.Fatal("want a non-empty unit set")
	}
}

func TestListNamespaces_HasBundledSpecs(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "list_namespaces", nil)
	text := requireText(t, res)
	if !strings.Contains(text, `"DI"`) || !strings.Contains(text, `"Machinery"`) {
		t.Errorf("list_namespaces missing bundled specs: %s", text)
	}
}

func TestListTypes_DI_HasDeviceType(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "list_types", map[string]any{"alias": "DI"})
	text := requireText(t, res)
	if !strings.Contains(text, "DeviceType") {
		t.Errorf("list_types(DI) missing DeviceType: %.200s", text)
	}
}

func TestListTypes_UnknownAlias_IsToolError(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "list_types", map[string]any{"alias": "nope"})
	if !res.IsError {
		t.Fatalf("want tool error for unknown alias, got: %+v", res.Content)
	}
}

func TestGetTypeDetails_DIDeviceType(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "get_type_details", map[string]any{"alias": "DI", "name": "DeviceType"})
	text := requireText(t, res)
	var out struct {
		Name      string `json:"name"`
		NodeClass string `json:"nodeClass"`
		Abstract  bool   `json:"abstract"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("get_type_details result is not valid JSON: %v", err)
	}
	if out.Name != "DeviceType" || out.NodeClass != "ObjectType" || !out.Abstract {
		t.Errorf("unexpected type detail: %+v", out)
	}
}

func TestSearchTypes_MatchesAcrossSpecs(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "search_types", map[string]any{"q": "Device"})
	text := requireText(t, res)
	if !strings.Contains(text, "DeviceType") {
		t.Errorf("search_types(Device) missing DeviceType: %.200s", text)
	}
}
