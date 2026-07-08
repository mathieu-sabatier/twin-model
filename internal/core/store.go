package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

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
	mu  sync.Mutex
	ttl time.Duration
	now func() time.Time
	m   map[string]*Draft
}

// NewStore returns a store whose drafts expire after ttl of inactivity.
func NewStore(ttl time.Duration) *Store {
	return &Store{ttl: ttl, now: time.Now, m: map[string]*Draft{}}
}

// Create snapshots files into a new draft and returns it.
func (s *Store) Create(baseRef string, files map[string][]byte) *Draft {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// Get returns the draft for id.
func (s *Store) Get(id string) (*Draft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[id]
	return d, ok
}

// Update applies fn to the draft under the lock and bumps UpdatedAt.
func (s *Store) Update(id string, fn func(*Draft)) (*Draft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[id]
	if !ok {
		return nil, false
	}
	fn(d)
	d.UpdatedAt = s.now()
	return d, true
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

func copyFiles(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		b := make([]byte, len(v))
		copy(b, v)
		out[k] = b
	}
	return out
}
