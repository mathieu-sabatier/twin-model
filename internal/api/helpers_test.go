package api

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// fakeHost is an in-memory GitHost: ReadTree serves a fixed tree per ref, and
// OpenPR records the last ProposeParams instead of talking to a git host.
type fakeHost struct {
	trees     map[string]map[string][]byte
	lastPR    ProposeParams
	prURL     string
	prErr     error
	readErr   error
	openedPR  bool
	prs       []dto.PullRequest
	listErr   error
	info      dto.RepoInfo
	branches  dto.BranchList
	branchErr error
}

func newFakeHost() *fakeHost {
	return &fakeHost{trees: map[string]map[string][]byte{}, prURL: "https://git.example/pr/1"}
}

func (f *fakeHost) ReadTree(_ context.Context, ref string) (map[string][]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	t, ok := f.trees[ref]
	if !ok {
		return nil, context.Canceled // any error; handler maps to 502
	}
	return core.CloneFiles(t), nil
}

func (f *fakeHost) OpenPR(_ context.Context, p ProposeParams) (string, error) {
	f.lastPR = p
	f.openedPR = true
	return f.prURL, f.prErr
}

func (f *fakeHost) ListPRs(_ context.Context) ([]dto.PullRequest, error) {
	return f.prs, f.listErr
}

func (f *fakeHost) Info() dto.RepoInfo { return f.info }

func (f *fakeHost) Branches(context.Context) (dto.BranchList, error) {
	return f.branches, f.branchErr
}

func newTestServer(t *testing.T) (*httptest.Server, *fakeHost, *Store) {
	t.Helper()
	host := newFakeHost()
	store := NewStore(time.Hour)
	ts := httptest.NewServer(NewServer(host, store).Routes())
	t.Cleanup(ts.Close)
	return ts, host, store
}
