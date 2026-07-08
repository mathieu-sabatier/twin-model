package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sort"
	"sync"
	"time"
)

// defaultMaxDrafts caps the number of live drafts a store holds. Drafts are a
// throwaway working cache reclaimed by the TTL sweeper; the cap is a second
// backstop so an unauthenticated burst of create_draft (each retaining two
// in-memory copies of the model tree) cannot exhaust memory before the sweep.
// It is generous enough never to bite normal interactive use.
const defaultMaxDrafts = 256

// Draft is a server-side working copy of a model fileset. BaseFiles is an
// immutable snapshot taken at creation (the baseRef version), used for diffing;
// Files is the mutable working copy the editor writes.
type Draft struct {
	ID        string
	BaseRef   string
	BaseFiles map[string][]byte
	Files     map[string][]byte
	UpdatedAt time.Time
}

// Store is a mutex-guarded, in-memory draft store with a TTL sweeper. It is the
// only server state; the git host is the only persistence.
type Store struct {
	mu        sync.Mutex
	ttl       time.Duration
	now       func() time.Time
	m         map[string]*Draft
	maxDrafts int
}

// NewStore returns a store whose drafts expire after ttl of inactivity.
func NewStore(ttl time.Duration) *Store {
	return &Store{ttl: ttl, now: time.Now, m: map[string]*Draft{}, maxDrafts: defaultMaxDrafts}
}

// Create snapshots files into a new draft and returns it. When the store is at
// capacity, the least-recently-updated draft is evicted first so live-draft
// memory stays bounded.
func (s *Store) Create(baseRef string, files map[string][]byte) *Draft {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxDrafts > 0 && len(s.m) >= s.maxDrafts {
		s.evictOldestLocked()
	}
	d := &Draft{
		ID:        newID(),
		BaseRef:   baseRef,
		BaseFiles: copyFiles(files),
		Files:     copyFiles(files),
		UpdatedAt: s.now(),
	}
	s.m[d.ID] = d
	return d
}

// Get returns a deep copy of the draft for id. It is a copy, not the live
// pointer, so a caller can read (and even mutate) the result off-lock without
// racing a concurrent Update/Mutate on the same draft's maps. Persisting a
// change must go through Update/Mutate.
func (s *Store) Get(id string) (*Draft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[id]
	if !ok {
		return nil, false
	}
	return copyDraft(d), true
}

// Update applies fn to the draft under the lock and bumps UpdatedAt. It returns
// a deep copy of the resulting draft (never the live pointer) for the same
// off-lock-safety reason as Get.
func (s *Store) Update(id string, fn func(*Draft)) (*Draft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[id]
	if !ok {
		return nil, false
	}
	fn(d)
	d.UpdatedAt = s.now()
	return copyDraft(d), true
}

// Mutate runs fn against the live draft under the lock — the atomic
// read-modify-write primitive. fn reports whether it changed the draft (bump ⇒
// UpdatedAt is refreshed) and may return an error, which is propagated. Because
// the whole parse→mutate→validate→store cycle of a patch op runs inside this
// one critical section, two concurrent edits to the same draft cannot lose an
// update (the classic read-snapshot-then-blind-write hazard). found reports
// whether the id existed.
func (s *Store) Mutate(id string, fn func(*Draft) (bump bool, err error)) (found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[id]
	if !ok {
		return false, nil
	}
	bump, err := fn(d)
	if bump {
		d.UpdatedAt = s.now()
	}
	return true, err
}

// evictOldestLocked removes the least-recently-updated draft. Caller holds mu.
func (s *Store) evictOldestLocked() {
	var oldestID string
	var oldest time.Time
	for id, d := range s.m {
		if oldestID == "" || d.UpdatedAt.Before(oldest) {
			oldestID, oldest = id, d.UpdatedAt
		}
	}
	if oldestID != "" {
		delete(s.m, oldestID)
		log.Printf("twinmodel: draft store at capacity (%d), evicted oldest draft %s", s.maxDrafts, oldestID)
	}
}

// DraftInfo is a race-free scalar snapshot of a draft for listing.
type DraftInfo struct {
	ID        string
	BaseRef   string
	Files     []string // sorted file names
	UpdatedAt time.Time
}

// List returns a snapshot of all live drafts, sorted by ID for determinism.
func (s *Store) List() []DraftInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DraftInfo, 0, len(s.m))
	for _, d := range s.m {
		out = append(out, DraftInfo{ID: d.ID, BaseRef: d.BaseRef, Files: sortedKeys(d.Files), UpdatedAt: d.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Discard removes the draft for id, returning whether it existed.
func (s *Store) Discard(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[id]; !ok {
		return false
	}
	delete(s.m, id)
	return true
}

// Sweep evicts drafts idle longer than the TTL.
func (s *Store) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-s.ttl)
	for id, d := range s.m {
		if d.UpdatedAt.Before(cutoff) {
			delete(s.m, id)
		}
	}
}

// StartSweeper runs Sweep on a ticker until ctx is cancelled.
func (s *Store) StartSweeper(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Sweep()
		}
	}
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand failing is unrecoverable
	}
	return hex.EncodeToString(b[:])
}

// copyDraft returns a deep copy of a draft: scalar fields plus independent
// copies of both file maps, so the copy shares no mutable state with the stored
// draft.
func copyDraft(d *Draft) *Draft {
	return &Draft{
		ID:        d.ID,
		BaseRef:   d.BaseRef,
		BaseFiles: copyFiles(d.BaseFiles),
		Files:     copyFiles(d.Files),
		UpdatedAt: d.UpdatedAt,
	}
}

func copyFiles(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		b := make([]byte, len(v))
		copy(b, v)
		out[k] = b
	}
	return out
}
