package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
	twinmcp "github.com/mathieu-sabatier/twin-model/internal/mcp"
)

// fakeGitHost is a self-contained, in-memory core.GitHost double for this
// integration test. It is defined locally (rather than reusing internal/api's
// fakeHost in helpers_test.go) because that type is package-private to
// internal/api. ReadTree serves one fixed tree keyed by baseRef; OpenPR records
// the call and returns a fixed PR URL instead of talking to a real git host.
type fakeGitHost struct {
	baseRef string
	tree    map[string][]byte
	prURL   string

	openedPR bool
	lastPR   core.ProposeParams
}

func (f *fakeGitHost) ReadTree(_ context.Context, ref string) (map[string][]byte, error) {
	if ref != f.baseRef {
		return nil, fmt.Errorf("fakeGitHost: no such ref %q", ref)
	}
	return core.CloneFiles(f.tree), nil
}

func (f *fakeGitHost) OpenPR(_ context.Context, p core.ProposeParams) (string, error) {
	f.openedPR = true
	f.lastPR = p
	return f.prURL, nil
}

func (f *fakeGitHost) ListPRs(context.Context) ([]dto.PullRequest, error) { return nil, nil }

func (f *fakeGitHost) Info() dto.RepoInfo {
	return dto.RepoInfo{Host: "github", Owner: "demo", Repo: "model", ProposeEnabled: true}
}

func (f *fakeGitHost) Branches(context.Context) (dto.BranchList, error) {
	return dto.BranchList{Branches: []string{f.baseRef}, DefaultBranch: f.baseRef}, nil
}

// callMCPTool invokes a tool over a real MCP client connection (in this test,
// the streamable-HTTP client mounted at /mcp) and returns the raw result. A
// transport-level failure (err != nil) fails the test immediately; a tool-level
// error (res.IsError) is left for the caller to assert on.
func callMCPTool(t *testing.T, c *mcpclient.Client, ctx context.Context, name string, args map[string]any) *mcp.CallToolResult {
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

// mcpText asserts a tool call succeeded (IsError unset) and returns its text
// content.
func mcpText(t *testing.T, res *mcp.CallToolResult) string {
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

// TestServeMCP_EndToEnd mounts the real serve handler (newHTTPHandler, the same
// function RunServe's fx graph wires up) on httptest, then drives the mounted
// /mcp streamable-HTTP transport with a real mcp-go client through the full
// create_draft -> update_draft -> propose_pr loop, proving:
//  1. a tool call succeeds over the mounted /mcp endpoint (the non-negotiable
//     assertion — list_namespaces, checked first);
//  2. /mcp shares svc's one in-process draft Store with the rest of serve (the
//     draft created over MCP is read back both via svc.Store() directly and via
//     later MCP calls against the same draftId);
//  3. /api and / still work with /mcp mounted alongside them.
func TestServeMCP_EndToEnd(t *testing.T) {
	equipment, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatalf("read equipment.yaml fixture: %v", err)
	}

	host := &fakeGitHost{
		baseRef: "main",
		tree:    map[string][]byte{"equipment.yaml": equipment},
		prURL:   "https://github.com/x/y/pull/1",
	}
	svc := core.New(host, core.NewStore(time.Hour))

	ts := httptest.NewServer(newHTTPHandler(svc, twinmcp.NewServer(svc)))
	t.Cleanup(ts.Close)

	// --- /api and / still serve correctly with /mcp mounted alongside them ---
	apiResp, err := http.Get(ts.URL + "/api/schema")
	if err != nil {
		t.Fatalf("GET /api/schema: %v", err)
	}
	apiResp.Body.Close()
	if apiResp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/schema status = %d, want 200", apiResp.StatusCode)
	}

	spaResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	spaResp.Body.Close()
	if spaResp.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want 200 (embedded SPA shell)", spaResp.StatusCode)
	}

	// --- drive /mcp with the real mcp-go streamable-HTTP client ---
	ctx := context.Background()
	cl, err := mcpclient.NewStreamableHttpClient(ts.URL + "/mcp")
	if err != nil {
		t.Fatalf("NewStreamableHttpClient: %v", err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	if err := cl.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "twinmodel-mcp-integration-test", Version: "0.0.0"}
	if _, err := cl.Initialize(ctx, initReq); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}

	// Non-negotiable assertion: a tool call succeeds over the mounted /mcp
	// endpoint. list_namespaces returns the bundled companion specs.
	nsText := mcpText(t, callMCPTool(t, cl, ctx, "list_namespaces", nil))
	if !strings.Contains(nsText, `"DI"`) || !strings.Contains(nsText, `"ISA95"`) {
		t.Errorf("list_namespaces over /mcp missing bundled aliases (DI, ISA95): %s", nsText)
	}

	// --- full create_draft -> update_draft -> propose_pr loop over /mcp ---
	createText := mcpText(t, callMCPTool(t, cl, ctx, "create_draft", map[string]any{"baseRef": "main"}))
	var created struct {
		ID    string   `json:"id"`
		Files []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(createText), &created); err != nil {
		t.Fatalf("create_draft result is not valid JSON: %v\n%s", err, createText)
	}
	if created.ID == "" {
		t.Fatal("create_draft over /mcp returned an empty draft id")
	}
	if len(created.Files) != 1 || created.Files[0] != "equipment.yaml" {
		t.Fatalf("create_draft files = %v, want [equipment.yaml]", created.Files)
	}

	// Prove the shared in-process store directly: the draft created through the
	// /mcp transport must already be visible via svc.Store() in this same
	// process — /mcp and /api both operate on the one svc, so there is nothing
	// to synchronize.
	if _, ok := svc.Store().Get(created.ID); !ok {
		t.Fatalf("draft %s created over /mcp is not visible via svc.Store() — /mcp is not sharing the in-process store", created.ID)
	}

	// A minimal, proven-clean edit (mirrors internal/api/integration_test.go's
	// TestDraftLifecycleAgainstLocalGitFixture): add a Double member with a unit
	// right after FurnaceType's DoorClosed member.
	original := string(equipment)
	inject := "\n      Pressure: { type: Double, unit: bar }"
	edited := strings.Replace(original, "      DoorClosed: { type: Boolean }", "      DoorClosed: { type: Boolean }"+inject, 1)
	if edited == original {
		t.Fatal("Pressure injection did not change equipment.yaml — the DoorClosed anchor line has drifted")
	}

	updText := mcpText(t, callMCPTool(t, cl, ctx, "update_draft", map[string]any{
		"draftId": created.ID,
		"files":   map[string]any{"equipment.yaml": edited},
	}))
	var updated struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(updText), &updated); err != nil {
		t.Fatalf("update_draft result is not valid JSON: %v\n%s", err, updText)
	}
	if len(updated.Files) != 1 || updated.Files[0] != "equipment.yaml" {
		t.Fatalf("update_draft files = %v, want [equipment.yaml]", updated.Files)
	}

	proposeText := mcpText(t, callMCPTool(t, cl, ctx, "propose_pr", map[string]any{
		"draftId": created.ID,
		"branch":  "model/add-pressure",
		"title":   "Add Pressure variable to FurnaceType",
	}))
	var proposed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(proposeText), &proposed); err != nil {
		t.Fatalf("propose_pr result is not valid JSON: %v\n%s", err, proposeText)
	}
	if proposed.URL != host.prURL {
		t.Errorf("propose_pr url = %q, want %q (fakeGitHost.prURL)", proposed.URL, host.prURL)
	}
	if !host.openedPR {
		t.Error("propose_pr did not reach fakeGitHost.OpenPR — the propose loop did not complete over /mcp")
	}
	if host.lastPR.Branch != "model/add-pressure" {
		t.Errorf("OpenPR branch = %q, want %q", host.lastPR.Branch, "model/add-pressure")
	}
}
