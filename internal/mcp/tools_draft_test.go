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
// (ReadTree) but update_draft/draft_diff
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

// TestListDraftsTool_ListsDrafts seeds a draft straight into the store (the
// same pattern used throughout this file) and checks the list_drafts tool
// surfaces it — the MCP-layer half of the ListDrafts coverage.
func TestListDraftsTool_ListsDrafts(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"Demo.yaml": []byte(draftBaseYAML)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "list_drafts", nil)
	text := requireText(t, res)
	if !strings.Contains(text, d.ID) {
		t.Errorf("list_drafts result missing draft id %q: %s", d.ID, text)
	}
}

// TestDiscardDraftTool_Discards drives discard_draft through the real MCP
// protocol and confirms the draft is actually gone from the store afterward.
func TestDiscardDraftTool_Discards(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"Demo.yaml": []byte(draftBaseYAML)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "discard_draft", map[string]any{"draftId": d.ID})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if _, ok := svc.Store().Get(d.ID); ok {
		t.Error("draft still present after discard_draft")
	}
}

// TestDiscardDraftTool_Unknown_IsError mirrors the not-found tests elsewhere
// in this package for discard_draft's unknown-id path.
func TestDiscardDraftTool_Unknown_IsError(t *testing.T) {
	c, ctx := newTestClient(t)
	res := callTool(t, c, ctx, "discard_draft", map[string]any{"draftId": "does-not-exist"})
	if !res.IsError {
		t.Fatalf("want tool error for unknown draft, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "not found") {
		t.Errorf("want a not-found tool error, got: %+v", res.Content)
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

// TestProposePR_CompositePath_ProposesAndDiscards drives propose_pr's inline
// (draftId=="") path — CreateDraft + UpdateDraft from baseRef+files, then
// Propose — which the existing tests never exercised (they all pass an existing
// draftId). It also pins the orphan-discard fix: the throwaway draft the
// composite path creates must be gone from the store afterward, not leaked
// until the TTL sweep.
func TestProposePR_CompositePath_ProposesAndDiscards(t *testing.T) {
	clean := "model:\n  name: Demo\n  namespace: urn:x/\n  version: 1.0.0\n  publication_date: 2026-01-01\n"
	svc := core.New(fakeMCPHost{tree: map[string][]byte{"demo.yaml": []byte(clean)}}, core.NewStore(time.Hour))
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "propose_pr", map[string]any{
		"baseRef": "main",
		"files":   map[string]any{"demo.yaml": clean},
		"branch":  "propose/demo",
		"title":   "Add demo",
	})
	if res.IsError {
		t.Fatalf("composite propose_pr errored: %s", requireText(t, res))
	}
	if !strings.Contains(requireText(t, res), "https://x/pull/1") {
		t.Fatalf("want the PR url in the composite result, got: %s", requireText(t, res))
	}
	if n := len(svc.ListDrafts().Drafts); n != 0 {
		t.Errorf("composite propose_pr leaked %d draft(s), want 0 (inline draft must be discarded)", n)
	}
}

func TestUpdateDraft_RefusesUnparseable(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(patchBaseSrc)})
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID, "files": map[string]any{"demo.yaml": "model: [unterminated"}})
	if !res.IsError {
		t.Fatal("expected a tool error refusing the unparseable write")
	}
	// The original content must be untouched (nothing stored).
	if string(d.Files["demo.yaml"]) != patchBaseSrc {
		t.Fatalf("draft was mutated despite refusal:\n%s", d.Files["demo.yaml"])
	}
}

func TestUpdateDraft_EchoesDiagnosticsOnSuccess(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(patchBaseSrc)})
	c, ctx := newClientFor(t, svc)
	// force:true — patchBaseSrc's namespace ("urn:x") lacks the required
	// trailing slash, an error-severity diagnostic on its own; this test is
	// about the "diagnostics" field being echoed on success, not strictness.
	res := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID, "force": true, "files": map[string]any{"demo.yaml": patchBaseSrc}})
	if res.IsError {
		t.Fatalf("unexpected error: %s", requireText(t, res))
	}
	if !strings.Contains(requireText(t, res), "diagnostics") {
		t.Fatalf("response missing diagnostics field:\n%s", requireText(t, res))
	}
}

// mixerErrorYAML is a full model file that parses cleanly but carries exactly
// one error-severity validation diagnostic (CodeUnknownUnit on
// MixerType.Weight's "unit: notaunit") — distinct from
// TestUpdateDraft_RefusesUnparseable, which covers a structural parse failure.
const mixerErrorYAML = "model:\n  name: Demo\n  namespace: urn:x/\n  version: 1.0.0\n  publication_date: 2026-01-01\n" +
	"imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
	"object_types:\n  MixerType:\n    base: OpcUa:BaseObjectType\n    members:\n      Weight: { type: Double, unit: notaunit }\n"

// TestUpdateDraft_RefusesErrorSeverityValidation covers update_draft's new
// strict gate: a file that parses fine but carries an error-severity
// validation diagnostic must be refused without force, and stored with
// force:true.
func TestUpdateDraft_RefusesErrorSeverityValidation(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(patchBaseSrc)})
	c, ctx := newClientFor(t, svc)

	res := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID, "files": map[string]any{"demo.yaml": mixerErrorYAML}})
	if !res.IsError {
		t.Fatalf("want refusal for error-severity validation, got: %+v", res.Content)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.Contains(tc.Text, "unknown-unit") {
		t.Fatalf("refusal message = %+v, want it to name the unknown-unit blocker", res.Content)
	}
	if string(d.Files["demo.yaml"]) != patchBaseSrc {
		t.Fatalf("draft was mutated despite refusal:\n%s", d.Files["demo.yaml"])
	}

	res = callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID, "force": true, "files": map[string]any{"demo.yaml": mixerErrorYAML}})
	if res.IsError {
		t.Fatalf("force:true should store despite error-severity diagnostics: %s", requireText(t, res))
	}
	// UpdateDraft canonicalizes on write (see core.canonicalize), so compare on
	// the substring rather than byte-for-byte equality.
	if !strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("draft not stored with force:true:\n%s", d.Files["demo.yaml"])
	}
}

func TestUpdateDraft_ForceStoresUnparseable(t *testing.T) {
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": []byte(patchBaseSrc)})
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "update_draft", map[string]any{
		"draftId": d.ID, "force": true, "files": map[string]any{"demo.yaml": "model: [unterminated"}})
	if res.IsError {
		t.Fatalf("force write should not error: %s", requireText(t, res))
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
