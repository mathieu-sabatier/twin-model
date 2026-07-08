package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/semdiff"
)

// draftBaseYAML/draftUpdatedYAML are a minimal before/after pair that differs by
// exactly one structural edit (a member added to PressType). semdiff.Diff never
// looks at model/version/publication_date (internal/semdiff/semdiff.go), so a
// metadata-only edit would produce zero changes; adding a member is the minimal
// change guaranteed to show up in DraftDiff's result.
const draftBaseYAML = `model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }
imports: { OpcUa: http://opcfoundation.org/UA/ }
object_types:
  PressType:
    base: OpcUa:BaseObjectType
    members:
      Setpoint: { type: Double, access: r }
`

const draftUpdatedYAML = `model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }
imports: { OpcUa: http://opcfoundation.org/UA/ }
object_types:
  PressType:
    base: OpcUa:BaseObjectType
    members:
      Setpoint: { type: Double, access: r }
      Serial: { kind: property, type: String }
`

// TestDraftLoop_CreateUpdateDiff drives the draft loop through core.Service
// directly (no MCP transport), seeding the store the way a real CreateDraft call
// would (Store.Create snapshots BaseFiles/Files identically). This is the
// core-layer half of the coverage: it pins down UpdateDraft's
// canonicalize-and-store behavior and DraftDiff's semantic-diff behavior, so a
// failure here means the bug is in core, not in the MCP tool wiring (see
// TestMCPDraftTools_UpdateAndDiff below for the same fixture pair driven through
// the tool layer).
func TestDraftLoop_CreateUpdateDiff(t *testing.T) {
	c := core.New(nil, core.NewStore(time.Hour))
	d := c.Store().Create("main", map[string][]byte{"Demo.yaml": []byte(draftBaseYAML)})

	filesResp, err := c.UpdateDraft(d.ID, map[string]string{"Demo.yaml": draftUpdatedYAML})
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	if len(filesResp.Files) != 1 || filesResp.Files[0] != "Demo.yaml" {
		t.Fatalf("UpdateDraft files = %v, want [Demo.yaml]", filesResp.Files)
	}

	diff, err := c.DraftDiff(d.ID, "Demo.yaml")
	if err != nil {
		t.Fatalf("DraftDiff: %v", err)
	}
	if len(diff.Changes) != 1 {
		t.Fatalf("DraftDiff changes = %+v, want exactly 1 change", diff.Changes)
	}
	ch := diff.Changes[0]
	if ch.Kind != semdiff.MemberAdded || ch.Type != "PressType" || ch.Member != "Serial" {
		t.Errorf("unexpected change: %+v", ch)
	}
	if !strings.Contains(diff.Text, "PressType: added member Serial") {
		t.Errorf("diff.Text = %q, want it to mention the added member", diff.Text)
	}
}

// TestMCPDraftTools_UpdateAndDiff drives the same before/after pair as
// TestDraftLoop_CreateUpdateDiff, but through the real MCP protocol
// (update_draft then draft_diff via an in-process client) on a draft seeded
// straight into the store — create_draft itself needs a real git host
// (ReadTree), which is Task 9's integration test, but update_draft/draft_diff
// only need an existing draft id, so they're fully exercisable here. draft_diff
// is called with no "file" argument, exercising the documented "defaults to the
// first file" behavior at the same time.
func TestMCPDraftTools_UpdateAndDiff(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"Demo.yaml": []byte(draftBaseYAML)})
	c, ctx := newClientFor(t, svc)

	updRes := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID,
		"files":   map[string]any{"Demo.yaml": draftUpdatedYAML},
	})
	updText := requireText(t, updRes)
	var updOut struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(updText), &updOut); err != nil {
		t.Fatalf("update_draft result is not valid JSON: %v\n%s", err, updText)
	}
	if len(updOut.Files) != 1 || updOut.Files[0] != "Demo.yaml" {
		t.Fatalf("update_draft files = %v, want [Demo.yaml]", updOut.Files)
	}

	diffRes := callTool(t, c, ctx, "draft_diff", map[string]any{"draftId": d.ID})
	diffText := requireText(t, diffRes)
	var diffOut struct {
		Changes []struct {
			Kind   string `json:"kind"`
			Type   string `json:"type"`
			Member string `json:"member"`
		} `json:"changes"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(diffText), &diffOut); err != nil {
		t.Fatalf("draft_diff result is not valid JSON: %v\n%s", err, diffText)
	}
	if len(diffOut.Changes) != 1 || diffOut.Changes[0].Kind != "MemberAdded" ||
		diffOut.Changes[0].Type != "PressType" || diffOut.Changes[0].Member != "Serial" {
		t.Fatalf("unexpected draft_diff changes: %+v", diffOut.Changes)
	}
	if !strings.Contains(diffOut.Text, "PressType: added member Serial") {
		t.Errorf("draft_diff text = %q, want it to mention the added member", diffOut.Text)
	}
}

// TestUpdateDraft_FilesNotObject_IsToolError exercises the stringMap error path
// wired into update_draft's handler: a non-object "files" argument must come
// back as a tool-error result (mcp.NewToolResultError), never a transport error
// or a panic.
func TestUpdateDraft_FilesNotObject_IsToolError(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"Demo.yaml": []byte(draftBaseYAML)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID,
		"files":   "not-an-object",
	})
	if !res.IsError {
		t.Fatalf("want tool error for non-object files, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "files must be an object") {
		t.Errorf("want a stringMap tool error, got: %+v", res.Content)
	}
}

// TestUpdateDraft_UnknownDraft_IsToolError checks core.ErrNotFound is wired
// through toolErr, mirroring the read-tools' not-found tests in
// tools_read_test.go.
func TestUpdateDraft_UnknownDraft_IsToolError(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": "does-not-exist",
		"files":   map[string]any{"Demo.yaml": draftBaseYAML},
	})
	if !res.IsError {
		t.Fatalf("want tool error for unknown draft, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "not found") {
		t.Errorf("want a not-found tool error, got: %+v", res.Content)
	}
}

// TestDraftDiff_UnknownDraft_IsToolError mirrors the above for draft_diff.
func TestDraftDiff_UnknownDraft_IsToolError(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "draft_diff", map[string]any{"draftId": "does-not-exist"})
	if !res.IsError {
		t.Fatalf("want tool error for unknown draft, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "not found") {
		t.Errorf("want a not-found tool error, got: %+v", res.Content)
	}
}

// TestProposePR_ValidationError_ReturnsBlockingResult exercises propose_pr's
// draftId path against a draft with an unparseable file. core.Service.Propose
// runs lintFileset before ever touching the git host, so this is safe to drive
// with a nil host (svc.Host() == nil): if the handler's control flow were wrong
// and it fell through to c.Host().OpenPR, this test would panic on the nil
// interface rather than silently pass. The blocked result must come back as
// normal tool data (jsonResult), not a tool-error result — per the brief, this
// is "a RESULT the model sees, not a Go error".
func TestProposePR_ValidationError_ReturnsBlockingResult(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"Bad.yaml": []byte("not: [valid")})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "propose_pr", map[string]any{
		"draftId": d.ID,
		"branch":  "propose/bad",
		"title":   "Bad model",
	})
	if res.IsError {
		t.Fatalf("want a data result (not a tool error) for a blocked proposal, got: %+v", res.Content)
	}
	text := requireText(t, res)
	var out struct {
		Error    string `json:"error"`
		Blocking []struct {
			Code     string `json:"code"`
			Severity string `json:"severity"`
			File     string `json:"file"`
		} `json:"blocking"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("propose_pr result is not valid JSON: %v\n%s", err, text)
	}
	if out.Error != "validation" {
		t.Fatalf("propose_pr error = %q, want %q", out.Error, "validation")
	}
	if len(out.Blocking) != 1 || out.Blocking[0].Code != "parse-error" || out.Blocking[0].File != "Bad.yaml" {
		t.Fatalf("unexpected blocking diagnostics: %+v", out.Blocking)
	}
}

// TestProposePR_UnknownDraft_IsPlainNotFoundError exercises propose_pr's
// draftId path with an unknown draftId. core.Service.Propose looks up the
// draft and returns ErrNotFound before ever calling branch/title validation
// or touching c.Host() (see core/service.go Propose), so this is safe to
// drive with a nil host: if the handler wrongly classified this as an
// OpenPR/host failure, it would either panic on the nil host interface or —
// as the bug actually did — silently mislabel the error via
// DescribeProposeError's unconditional non-empty return. The regression this
// guards: propose_pr's old `if m, detail := core.DescribeProposeError(err); m
// != ""` check was dead code (DescribeProposeError always returns a
// non-empty msg), so ErrNotFound/ErrInvalid fell through to the PR-failure
// branch and produced a false "branch and commit may have been created
// locally, but pushing to the remote failed" message even though Propose
// never got past the draft lookup. The fix mirrors
// internal/api/handlers.go's handlePropose guard: only render the
// DescribeProposeError message when the error is neither ErrNotFound nor
// ErrInvalid.
func TestProposePR_UnknownDraft_IsPlainNotFoundError(t *testing.T) {
	c, ctx := newTestClient(t) // host-less core.Service; Propose must never reach it here

	res := callTool(t, c, ctx, "propose_pr", map[string]any{
		"draftId": "does-not-exist",
		"branch":  "propose/x",
		"title":   "Some title",
	})
	if !res.IsError {
		t.Fatalf("want a tool-error result for an unknown draftId, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("tool result content is not text: %#v", res.Content[0])
	}
	if !strings.Contains(tc.Text, "not found") || !strings.Contains(tc.Text, "draft not found") {
		t.Errorf("propose_pr error = %q, want the plain not-found message (containing %q and %q)", tc.Text, "not found", "draft not found")
	}
	if strings.Contains(tc.Text, "pushing to the remote failed") || strings.Contains(tc.Text, "may have been created locally") {
		t.Errorf("propose_pr error = %q, must NOT be the PR-failure message — Propose never reached the host for an unknown draftId", tc.Text)
	}
}

// TestProposePR_EmptyBranchOrTitle_IsPlainInvalidError covers the other
// pre-host sentinel: a seeded (existing) draft but an empty branch/title.
// core.Service.Propose checks branch/title right after the draft lookup and
// returns ErrInvalid before lintFileset or c.Host() ever run, so — like the
// unknown-draftId case above — this must come back as the plain sentinel
// message, not the PR-failure message.
func TestProposePR_EmptyBranchOrTitle_IsPlainInvalidError(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"Demo.yaml": []byte(draftBaseYAML)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "propose_pr", map[string]any{
		"draftId": d.ID,
		"branch":  "",
		"title":   "",
	})
	if !res.IsError {
		t.Fatalf("want a tool-error result for empty branch/title, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("tool result content is not text: %#v", res.Content[0])
	}
	if !strings.Contains(tc.Text, "branch and title are required") {
		t.Errorf("propose_pr error = %q, want the plain invalid-argument message", tc.Text)
	}
	if strings.Contains(tc.Text, "pushing to the remote failed") || strings.Contains(tc.Text, "may have been created locally") {
		t.Errorf("propose_pr error = %q, must NOT be the PR-failure message — Propose never reached the host for empty branch/title", tc.Text)
	}
}

// TestStringMap unit-tests the object-arg coercion helper directly (same
// package, no server/client needed): the happy path and both error branches.
func TestStringMap(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"files": map[string]any{"A.yaml": "content-a", "B.yaml": "content-b"},
		}
		got, err := stringMap(req, "files")
		if err != nil {
			t.Fatalf("stringMap: %v", err)
		}
		want := map[string]string{"A.yaml": "content-a", "B.yaml": "content-b"}
		if len(got) != len(want) || got["A.yaml"] != want["A.yaml"] || got["B.yaml"] != want["B.yaml"] {
			t.Errorf("stringMap = %v, want %v", got, want)
		}
	})

	t.Run("non-object value", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"files": "not-an-object"}
		_, err := stringMap(req, "files")
		if err == nil || !strings.Contains(err.Error(), "files must be an object") {
			t.Errorf("stringMap error = %v, want a 'must be an object' error", err)
		}
	})

	t.Run("non-string entry", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"files": map[string]any{"Bad.yaml": 5}}
		_, err := stringMap(req, "files")
		if err == nil || !strings.Contains(err.Error(), "files.Bad.yaml must be a string") {
			t.Errorf("stringMap error = %v, want a 'must be a string' error", err)
		}
	})
}
