package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

func TestGetDraftSource_ReturnsRawBytes(t *testing.T) {
	raw := []byte("model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n")
	svc := core.New(nil, core.NewStore(time.Hour))
	d := svc.Store().Create("main", map[string][]byte{"demo.yaml": raw})
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "get_draft_source", map[string]any{"draftId": d.ID, "file": "demo.yaml"})
	if got := requireText(t, res); got != string(raw) {
		t.Fatalf("got %q, want %q", got, raw)
	}
}

func TestGetModelSource_ReturnsRawBytes(t *testing.T) {
	raw := []byte("model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n")
	svc := core.New(fakeMCPHost{tree: map[string][]byte{"demo.yaml": raw}}, core.NewStore(time.Hour))
	c, ctx := newClientFor(t, svc)
	res := callTool(t, c, ctx, "get_model_source", map[string]any{"ref": "main", "file": "demo.yaml"})
	if got := requireText(t, res); got != string(raw) {
		t.Fatalf("got %q, want %q", got, raw)
	}
}

// fakeMCPHost is a minimal core.GitHost for host-backed tool tests.
type fakeMCPHost struct{ tree map[string][]byte }

func (f fakeMCPHost) ReadTree(_ context.Context, _ string) (map[string][]byte, error) {
	return f.tree, nil
}
func (f fakeMCPHost) OpenPR(context.Context, core.ProposeParams) (string, error) {
	return "https://x/pull/1", nil
}
func (f fakeMCPHost) ListPRs(context.Context) ([]dto.PullRequest, error) { return nil, nil }
func (f fakeMCPHost) Info() dto.RepoInfo                                 { return dto.RepoInfo{} }
func (f fakeMCPHost) Branches(context.Context) (dto.BranchList, error)   { return dto.BranchList{}, nil }
