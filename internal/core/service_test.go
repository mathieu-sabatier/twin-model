package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

func TestService_ParseModel_ReturnsASTAndDiagnostics(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	src := "model:\n  name: Demo\n  namespace: https://x/\n  version: 1.0.0\n  publication_date: 2026-01-01\n"
	resp := s.ParseModel("Demo.yaml", []byte(src))
	if resp.Model == nil {
		t.Fatal("expected parsed Model, got nil")
	}
}

func TestService_DraftModelDesign_UnknownDraft_IsNotFound(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	_, err := s.DraftModelDesign("nope", "x.yaml")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_Propose_LintRed_IsValidationError(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := s.Store().Create("main", map[string][]byte{"Bad.yaml": []byte("model: Bad\n")}) // missing required fields → lint-red
	_, err := s.Propose(context.Background(), d.ID, "b", "t", "")
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Blocking) == 0 {
		t.Fatalf("err = %v, want *ValidationError with blocking diags", err)
	}
}

// fakeReadHost is a minimal GitHost whose ReadTree serves a fixed tree.
type fakeReadHost struct{ tree map[string][]byte }

func (f fakeReadHost) ReadTree(context.Context, string) (map[string][]byte, error) {
	return f.tree, nil
}
func (f fakeReadHost) OpenPR(context.Context, ProposeParams) (string, error) { return "", nil }
func (f fakeReadHost) ListPRs(context.Context) ([]dto.PullRequest, error)    { return nil, nil }
func (f fakeReadHost) Info() dto.RepoInfo                                    { return dto.RepoInfo{} }
func (f fakeReadHost) Branches(context.Context) (dto.BranchList, error)      { return dto.BranchList{}, nil }

func TestReadModelSource_ReturnsRawBytes(t *testing.T) {
	raw := []byte("model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n")
	s := New(fakeReadHost{tree: map[string][]byte{"demo.yaml": raw}}, NewStore(time.Hour))
	got, err := s.ReadModelSource(context.Background(), "main", "demo.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q, want raw source %q", got, raw)
	}
}

func TestReadModelSource_EmptyRef_IsInvalid(t *testing.T) {
	s := New(fakeReadHost{}, NewStore(time.Hour))
	if _, err := s.ReadModelSource(context.Background(), "", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestReadModelSource_MissingFile_IsNotFound(t *testing.T) {
	s := New(fakeReadHost{tree: map[string][]byte{}}, NewStore(time.Hour))
	if _, err := s.ReadModelSource(context.Background(), "main", "nope.yaml"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
