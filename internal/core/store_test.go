package core

import (
	"regexp"
	"sort"
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

func TestStore_List_ReturnsAllSortedByID(t *testing.T) {
	s := NewStore(time.Hour)
	d1 := s.Create("main", map[string][]byte{"b.yaml": []byte("1"), "a.yaml": []byte("2")})
	d2 := s.Create("dev", map[string][]byte{"c.yaml": []byte("3")})

	got := s.List()
	if len(got) != 2 {
		t.Fatalf("List() returned %d drafts, want 2: %+v", len(got), got)
	}
	// sorted by ID for determinism
	ids := []string{d1.ID, d2.ID}
	sort.Strings(ids)
	if got[0].ID != ids[0] || got[1].ID != ids[1] {
		t.Fatalf("List() ids = [%s, %s], want sorted %v", got[0].ID, got[1].ID, ids)
	}
	for _, di := range got {
		switch di.ID {
		case d1.ID:
			if di.BaseRef != "main" {
				t.Errorf("d1 BaseRef = %q, want main", di.BaseRef)
			}
			if len(di.Files) != 2 || di.Files[0] != "a.yaml" || di.Files[1] != "b.yaml" {
				t.Errorf("d1 Files = %v, want sorted [a.yaml b.yaml]", di.Files)
			}
		case d2.ID:
			if di.BaseRef != "dev" {
				t.Errorf("d2 BaseRef = %q, want dev", di.BaseRef)
			}
			if len(di.Files) != 1 || di.Files[0] != "c.yaml" {
				t.Errorf("d2 Files = %v, want [c.yaml]", di.Files)
			}
		default:
			t.Errorf("unexpected draft id %q in List()", di.ID)
		}
	}
}

// TestStore_Create_EvictsOldestOverCap verifies the live-draft cap: once the
// store is full, Create evicts the least-recently-updated draft rather than
// growing without bound (the second backstop, alongside the TTL sweep, against
// an unauthenticated create_draft flood).
func TestStore_Create_EvictsOldestOverCap(t *testing.T) {
	s := NewStore(time.Hour)
	s.maxDrafts = 3
	base := time.Unix(1_000_000, 0)
	var tick int64
	s.now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Second) } // strictly increasing

	d1 := s.Create("main", map[string][]byte{"a.yaml": []byte("1")})
	d2 := s.Create("main", map[string][]byte{"b.yaml": []byte("2")})
	d3 := s.Create("main", map[string][]byte{"c.yaml": []byte("3")})
	d4 := s.Create("main", map[string][]byte{"d.yaml": []byte("4")}) // exceeds cap → evicts d1 (oldest)

	if _, ok := s.Get(d1.ID); ok {
		t.Error("oldest draft d1 should have been evicted at the cap")
	}
	for _, d := range []*Draft{d2, d3, d4} {
		if _, ok := s.Get(d.ID); !ok {
			t.Errorf("draft %s should still be present", d.ID)
		}
	}
	if got := len(s.List()); got != 3 {
		t.Errorf("store holds %d drafts, want the cap of 3", got)
	}
}

func TestStore_Discard_RemovesAndReportsExistence(t *testing.T) {
	s := NewStore(time.Hour)
	d := s.Create("main", map[string][]byte{"a.yaml": []byte("1")})

	if ok := s.Discard(d.ID); !ok {
		t.Fatal("Discard(existing) = false, want true")
	}
	if _, ok := s.Get(d.ID); ok {
		t.Error("draft still present after Discard")
	}
	if ok := s.Discard("does-not-exist"); ok {
		t.Error("Discard(unknown) = true, want false")
	}
}
