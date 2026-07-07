package api

import (
	"regexp"
	"testing"
	"time"
)

func TestStoreCreateGet(t *testing.T) {
	s := NewStore(time.Hour)
	d := s.Create("main", map[string][]byte{"equipment.yaml": []byte("x")})
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(d.ID) {
		t.Errorf("id = %q, want 32 hex chars", d.ID)
	}
	got, ok := s.Get(d.ID)
	if !ok || got.BaseRef != "main" {
		t.Fatalf("Get(%q) = %+v, %v", d.ID, got, ok)
	}
	// Create snapshots the input: mutating the caller's map must not affect the draft.
	got.Files["equipment.yaml"] = []byte("y")
	if string(got.BaseFiles["equipment.yaml"]) != "x" {
		t.Error("BaseFiles must be an immutable snapshot, independent of Files")
	}
}

func TestStoreUpdate(t *testing.T) {
	s := NewStore(time.Hour)
	d := s.Create("main", map[string][]byte{"a.yaml": []byte("1")})
	before := d.UpdatedAt
	s.now = func() time.Time { return before.Add(time.Minute) }
	got, ok := s.Update(d.ID, func(dr *Draft) { dr.Files["a.yaml"] = []byte("2") })
	if !ok {
		t.Fatal("Update returned not-ok")
	}
	if string(got.Files["a.yaml"]) != "2" {
		t.Errorf("Files not updated: %q", got.Files["a.yaml"])
	}
	if !got.UpdatedAt.After(before) {
		t.Error("Update did not bump UpdatedAt")
	}
}

func TestStoreSweepEvictsExpired(t *testing.T) {
	s := NewStore(time.Hour)
	base := time.Unix(1_000_000, 0)
	s.now = func() time.Time { return base }
	d := s.Create("main", map[string][]byte{"a.yaml": []byte("1")})
	// Advance past the TTL and sweep.
	s.now = func() time.Time { return base.Add(2 * time.Hour) }
	s.Sweep()
	if _, ok := s.Get(d.ID); ok {
		t.Error("expired draft should have been evicted")
	}
}

func TestStoreIDsUnique(t *testing.T) {
	s := NewStore(time.Hour)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := s.Create("main", nil).ID
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}
