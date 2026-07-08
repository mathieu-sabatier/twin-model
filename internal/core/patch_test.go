package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func seedDraft(t *testing.T, s *Service, src string) *Draft {
	t.Helper()
	return s.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
}

const baseSrc = "model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n"

func TestAddImport_AppendsAndCanonicalizes(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc)
	resp, err := s.AddImport(d.ID, "demo.yaml", "DI", "http://opcfoundation.org/UA/DI/")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("files = %v", resp.Files)
	}
	// dsl.Format's scalar() quotes any value containing ':' (see needsQuote in
	// internal/dsl/format.go), so a URI import comes back double-quoted.
	if !strings.Contains(string(d.Files["demo.yaml"]), `DI: "http://opcfoundation.org/UA/DI/"`) {
		t.Fatalf("import not stored:\n%s", d.Files["demo.yaml"])
	}
}

func TestAddImport_DuplicateAlias_IsConflict(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"imports:\n  DI: http://opcfoundation.org/UA/DI/\n")
	_, err := s.AddImport(d.ID, "demo.yaml", "DI", "http://x/")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

// TestAddImport_BumpsUpdatedAt guards against storeMutatedDraft writing to the
// draft directly instead of routing through store.Update: a direct write races
// concurrent writers on the shared Files map and never refreshes UpdatedAt,
// letting the TTL sweeper evict a draft mid agent-session.
func TestAddImport_BumpsUpdatedAt(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc)
	before := d.UpdatedAt
	s.Store().now = func() time.Time { return before.Add(time.Minute) }
	if _, err := s.AddImport(d.ID, "demo.yaml", "DI", "http://opcfoundation.org/UA/DI/"); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Store().Get(d.ID)
	if !ok {
		t.Fatal("draft went missing")
	}
	if !got.UpdatedAt.After(before) {
		t.Errorf("UpdatedAt = %v, want strictly after %v (storeMutatedDraft must route through store.Update)", got.UpdatedAt, before)
	}
}

func TestAddImport_UnknownDraft_IsNotFound(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	if _, err := s.AddImport("nope", "demo.yaml", "DI", "http://x/"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAddType_SplicesTypeAndPreservesFile(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")
	body := "doc: A widget\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: OpcUa:Double, unit: kg }\n"
	resp, err := s.AddType(d.ID, "demo.yaml", "WidgetType", body)
	if err != nil {
		t.Fatal(err)
	}
	stored := string(d.Files["demo.yaml"])
	if !strings.Contains(stored, "WidgetType:") || !strings.Contains(stored, "Weight") {
		t.Fatalf("type not spliced:\n%s", stored)
	}
	// dsl.Format's scalar() quotes any value containing ':' (see needsQuote in
	// internal/dsl/format.go and TestAddImport_AppendsAndCanonicalizes above), so
	// the pre-existing URI import comes back double-quoted after the round-trip.
	if !strings.Contains(stored, `OpcUa: "http://opcfoundation.org/UA/"`) {
		t.Fatalf("existing content dropped:\n%s", stored)
	}
	if _, ok := resp.Diagnostics["demo.yaml"]; !ok {
		t.Fatal("expected per-file diagnostics key")
	}
}

func TestAddType_DuplicateName_IsConflict(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n")
	_, err := s.AddType(d.ID, "demo.yaml", "WidgetType", "base: OpcUa:BaseObjectType\n")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestAddType_UnparseableBody_IsParseError(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc)
	_, err := s.AddType(d.ID, "demo.yaml", "WidgetType", "members: [this is not a mapping")
	if !errors.Is(err, ErrParse) {
		t.Fatalf("err = %v, want ErrParse", err)
	}
}

func TestAddInstance_SplicesInstance(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n")
	body := "type: WidgetType\nunder: Cell1\nlevel: Unit\n"
	if _, err := s.AddInstance(d.ID, "demo.yaml", "Widget1", body); err != nil {
		t.Fatal(err)
	}
	stored := string(d.Files["demo.yaml"])
	if !strings.Contains(stored, "Widget1:") || !strings.Contains(stored, "type: WidgetType") {
		t.Fatalf("instance not spliced:\n%s", stored)
	}
}

func TestAddInstance_DuplicateName_IsConflict(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"instances:\n  Widget1:\n    type: WidgetType\n")
	_, err := s.AddInstance(d.ID, "demo.yaml", "Widget1", "type: WidgetType\n")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}
