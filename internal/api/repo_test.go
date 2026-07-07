package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/api/dto"
)

func TestGitHubHostInfo(t *testing.T) {
	cases := []struct {
		name           string
		host           *GitHubHost
		wantOwner      string
		wantURL        string
		wantEnabled    bool
		reasonContains string
	}{
		{
			name:        "remote with token is enabled",
			host:        &GitHubHost{RepoURL: "https://github.com/acme/model.git", Token: "t", Owner: "acme", Repo: "model"},
			wantOwner:   "acme",
			wantURL:     "https://github.com/acme/model",
			wantEnabled: true,
		},
		{
			name:           "remote without token is disabled",
			host:           &GitHubHost{RepoURL: "https://github.com/acme/model.git", Owner: "acme", Repo: "model"},
			wantOwner:      "acme",
			wantURL:        "https://github.com/acme/model",
			wantEnabled:    false,
			reasonContains: "GIT_TOKEN",
		},
		{
			name:           "local checkout is disabled",
			host:           &GitHubHost{RepoURL: "/tmp/local/model", Token: "t"},
			wantOwner:      "",
			wantURL:        "",
			wantEnabled:    false,
			reasonContains: "local checkout",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.host.Info()
			if got.Owner != c.wantOwner {
				t.Errorf("Owner = %q, want %q", got.Owner, c.wantOwner)
			}
			if got.URL != c.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, c.wantURL)
			}
			if got.ProposeEnabled != c.wantEnabled {
				t.Errorf("ProposeEnabled = %v, want %v", got.ProposeEnabled, c.wantEnabled)
			}
			if c.reasonContains != "" && !strings.Contains(got.ProposeReason, c.reasonContains) {
				t.Errorf("ProposeReason = %q, want contains %q", got.ProposeReason, c.reasonContains)
			}
			if c.wantEnabled && got.ProposeReason != "" {
				t.Errorf("enabled host should have empty reason, got %q", got.ProposeReason)
			}
			if got.CommitName != commitAuthorName || got.CommitEmail != commitAuthorEmail {
				t.Errorf("identity = %q/%q, want %q/%q", got.CommitName, got.CommitEmail, commitAuthorName, commitAuthorEmail)
			}
		})
	}
}

func TestHandleRepo(t *testing.T) {
	ts, host, _ := newTestServer(t)
	host.info = dto.RepoInfo{
		Host: "github", Owner: "acme", Repo: "model",
		URL: "https://github.com/acme/model", DefaultBranch: "main",
		CommitName: "twinmodel-bot", CommitEmail: "bot@twinmodel",
		ProposeEnabled: true,
	}
	resp, err := http.Get(ts.URL + "/api/repo")
	if err != nil {
		t.Fatalf("GET /api/repo: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got dto.RepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Owner != "acme" || got.Repo != "model" || !got.ProposeEnabled || got.CommitName != "twinmodel-bot" {
		t.Errorf("unexpected RepoInfo: %+v", got)
	}
}

func TestCreateDraftBranchNotFound(t *testing.T) {
	ts, host, _ := newTestServer(t)
	// Simulate go-git's missing-branch error (verified string).
	host.readErr = errors.New(`clone /x@nope: couldn't find remote ref "refs/heads/nope"`)

	resp, err := http.Post(ts.URL+"/api/drafts", "application/json", strings.NewReader(`{"baseRef":"nope"}`))
	if err != nil {
		t.Fatalf("POST /api/drafts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(body.Error, `branch "nope" not found`) {
		t.Errorf("error = %q, want it to contain 'branch \"nope\" not found'", body.Error)
	}
}
