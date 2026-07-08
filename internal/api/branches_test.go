package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// TestGitHubHostBranches exercises the real go-git ls-remote path against a
// local repo whose default branch is "trunk" (not "main"), proving the default
// is resolved from HEAD rather than hardcoded.
func TestGitHubHostBranches(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("trunk")},
	})
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if err := os.WriteFile(dir+"/README.md", []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@t"}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	// A second branch pointing at the same commit.
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("feature-x"), head.Hash())); err != nil {
		t.Fatalf("set ref: %v", err)
	}

	host := &GitHubHost{RepoURL: dir}
	got, err := host.Branches(context.Background())
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if got.DefaultBranch != "trunk" {
		t.Errorf("DefaultBranch = %q, want %q", got.DefaultBranch, "trunk")
	}
	// default first, then the rest alphabetically: ["trunk", "feature-x"]
	want := []string{"trunk", "feature-x"}
	if len(got.Branches) != len(want) {
		t.Fatalf("Branches = %v, want %v", got.Branches, want)
	}
	for i := range want {
		if got.Branches[i] != want[i] {
			t.Errorf("Branches[%d] = %q, want %q (full: %v)", i, got.Branches[i], want[i], got.Branches)
		}
	}
}

func TestHandleBranches(t *testing.T) {
	ts, host, _ := newTestServer(t)
	host.branches = dto.BranchList{
		Branches:      []string{"main", "model/press-curve"},
		DefaultBranch: "main",
	}
	resp, err := http.Get(ts.URL + "/api/branches")
	if err != nil {
		t.Fatalf("GET /api/branches: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got dto.BranchList
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.DefaultBranch != "main" || len(got.Branches) != 2 || got.Branches[0] != "main" {
		t.Errorf("unexpected BranchList: %+v", got)
	}
}
