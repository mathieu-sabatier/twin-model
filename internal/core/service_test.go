package core

import (
	"context"
	"errors"
	"testing"
	"time"
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
