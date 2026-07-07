package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/api/dto"
)

func TestListPRs(t *testing.T) {
	ts, host, _ := newTestServer(t)
	host.prs = []dto.PullRequest{
		{Number: 7, Title: "Update model", URL: "https://git.example/pr/7", Branch: "model/x", State: "open"},
	}
	resp, err := http.Get(ts.URL + "/api/prs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		PRs []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"url"`
			Branch string `json:"branch"`
			State  string `json:"state"`
		} `json:"prs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.PRs) != 1 {
		t.Fatalf("got %d PRs, want 1: %+v", len(out.PRs), out.PRs)
	}
	if pr := out.PRs[0]; pr.Number != 7 || pr.Title != "Update model" || pr.URL != "https://git.example/pr/7" || pr.Branch != "model/x" || pr.State != "open" {
		t.Fatalf("pr = %+v", pr)
	}
}

func TestListPRsHostError(t *testing.T) {
	ts, host, _ := newTestServer(t)
	host.listErr = errors.New("boom")
	resp, err := http.Get(ts.URL + "/api/prs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

// TestGitHubHostListPRs exercises the real REST path against a fake GitHub API.
func TestGitHubHostListPRs(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/o/r/pulls" {
			t.Errorf("got %s %s, want GET /repos/o/r/pulls", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("state") != "open" {
			t.Errorf("state = %q, want open", r.URL.Query().Get("state"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":   7,
				"title":    "Update",
				"html_url": "https://github.com/o/r/pull/7",
				"state":    "open",
				"head":     map[string]string{"ref": "model/x"},
			},
		})
	}))
	defer api.Close()

	h := &GitHubHost{APIBase: api.URL, Owner: "o", Repo: "r"}
	prs, err := h.ListPRs(context.Background())
	if err != nil {
		t.Fatalf("ListPRs: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 7 || prs[0].Branch != "model/x" || prs[0].URL != "https://github.com/o/r/pull/7" || prs[0].State != "open" {
		t.Fatalf("prs = %+v", prs)
	}
}
