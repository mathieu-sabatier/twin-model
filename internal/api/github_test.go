package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestParseOwnerRepo(t *testing.T) {
	cases := []struct{ url, owner, repo string }{
		{"https://github.com/acme/model.git", "acme", "model"},
		{"https://github.com/acme/model", "acme", "model"},
		{"https://github.com/acme/model/tree/main", "acme", "model"}, // browse URL must not parse as tree/main
		{"git@github.com:acme/model.git", "acme", "model"},
		{"/tmp/local/fixture", "", ""}, // local path: no owner/repo
	}
	for _, c := range cases {
		owner, repo := parseOwnerRepo(c.url)
		if owner != c.owner || repo != c.repo {
			t.Errorf("parseOwnerRepo(%q) = %q/%q, want %q/%q", c.url, owner, repo, c.owner, c.repo)
		}
	}
}

// makeFixtureRepo creates a temporary bare-ish local git repo with one commit
// on the default branch containing "equipment.yaml". Returns the repo path and
// the name of the default branch.
func makeFixtureRepo(t *testing.T) (repoPath string, defaultBranch string) {
	t.Helper()
	dir := t.TempDir()

	// PlainInit creates a non-bare repo. We use it as the "remote" by pushing
	// to it; go-git local file transport supports pushing to non-bare repos in
	// memory mode too.
	repo, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			// Keep the default (master) so the test is independent of any
			// global git config on the host.
			DefaultBranch: plumbing.Master,
		},
	})
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	// Write the seed file.
	content := []byte("model:\n  name: Equipment\n")
	if err := os.WriteFile(filepath.Join(dir, "equipment.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("equipment.yaml"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	sig := &object.Signature{Name: "fixture", Email: "fixture@test", When: time.Now()}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author:    sig,
		Committer: sig,
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Discover the actual default branch name from HEAD.
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	branch := head.Name().Short() // "master"
	return dir, branch
}

// TestGitHubHostReadTree clones the local fixture repo and checks that
// equipment.yaml is present in the returned tree.
func TestGitHubHostReadTree(t *testing.T) {
	repoPath, branch := makeFixtureRepo(t)

	host := &GitHubHost{RepoURL: repoPath}
	tree, err := host.ReadTree(t.Context(), branch)
	if err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if _, ok := tree["equipment.yaml"]; !ok {
		t.Errorf("ReadTree missing equipment.yaml; got keys: %v", sortedKeys(tree))
	}
}

// TestGitHubHostOpenPR branches, commits, pushes to the local fixture and
// verifies:
//   - the pushed branch exists in the fixture repo
//   - the GitHub REST endpoint was called correctly
//   - OpenPR returns the URL from the fake endpoint
func TestGitHubHostOpenPR(t *testing.T) {
	repoPath, branch := makeFixtureRepo(t)

	// Fake GitHub REST API — responds to POST /repos/.../pulls with a created PR.
	const wantPRURL = "https://github.com/test-owner/test-repo/pull/42"
	fakeGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		// Verify the request targets the right repo and carries the right PR body,
		// so a regression in URL construction or field naming is caught here.
		if r.URL.Path != "/repos/test-owner/test-repo/pulls" {
			t.Errorf("PR request path = %q, want /repos/test-owner/test-repo/pulls", r.URL.Path)
		}
		var got map[string]string
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode PR body: %v", err)
		}
		if got["head"] != "model/change" {
			t.Errorf("PR head = %q, want model/change", got["head"])
		}
		if got["base"] != branch {
			t.Errorf("PR base = %q, want %q", got["base"], branch)
		}
		if got["title"] != "Update equipment model" {
			t.Errorf("PR title = %q, want Update equipment model", got["title"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"html_url": wantPRURL})
	}))
	t.Cleanup(fakeGitHub.Close)

	host := &GitHubHost{
		RepoURL: repoPath,
		APIBase: fakeGitHub.URL,
		Owner:   "test-owner",
		Repo:    "test-repo",
	}

	params := ProposeParams{
		BaseRef: branch,
		Branch:  "model/change",
		Title:   "Update equipment model",
		Message: "adds new fields",
		Files: map[string][]byte{
			"equipment.yaml": []byte("model:\n  name: EquipmentV2\n"),
		},
	}

	gotURL, err := host.OpenPR(t.Context(), params)
	if err != nil {
		t.Fatalf("OpenPR: %v", err)
	}
	if gotURL != wantPRURL {
		t.Errorf("url = %q, want %q", gotURL, wantPRURL)
	}

	// Confirm the branch was actually pushed into the fixture repo.
	fixtureRepo, err := git.PlainOpen(repoPath)
	if err != nil {
		t.Fatalf("PlainOpen fixture: %v", err)
	}
	branchRef := plumbing.NewBranchReferenceName("model/change")
	ref, err := fixtureRepo.Reference(branchRef, true)
	if err != nil {
		t.Fatalf("branch model/change not found in fixture: %v", err)
	}
	if ref.Hash().IsZero() {
		t.Error("pushed branch has zero hash")
	}
}
