package core

import (
	"testing"
	"time"
)

func TestConfigFromEnv_RequiresRepo(t *testing.T) {
	t.Setenv("GIT_REPO", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("want error when GIT_REPO unset")
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("GIT_REPO", "/tmp/repo")
	t.Setenv("ADDR", "")
	t.Setenv("DRAFT_TTL", "")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.TTL != 2*time.Hour {
		t.Fatalf("defaults wrong: %+v", cfg)
	}
}

func TestNewGitHost_RemoteRepo(t *testing.T) {
	cfg := Config{Repo: "https://github.com/o/r.git", Token: "tok"}
	host, err := NewGitHost(cfg)
	if err != nil {
		t.Fatalf("NewGitHost: %v", err)
	}
	gh, ok := host.(*GitHubHost)
	if !ok {
		t.Fatalf("host type = %T, want *GitHubHost", host)
	}
	if gh.Owner != "o" || gh.Repo != "r" {
		t.Errorf("owner/repo = %s/%s, want o/r", gh.Owner, gh.Repo)
	}
}

// A local filesystem path in Repo is accepted as a dev backend (no GitHub URL,
// no owner/repo parse) so the SPA can be developed against a local checkout.
func TestNewGitHost_LocalRepo(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Repo: dir}
	host, err := NewGitHost(cfg)
	if err != nil {
		t.Fatalf("NewGitHost(local path): %v", err)
	}
	gh, ok := host.(*GitHubHost)
	if !ok {
		t.Fatalf("host type = %T, want *GitHubHost", host)
	}
	if gh.RepoURL != dir {
		t.Errorf("RepoURL = %q, want %q", gh.RepoURL, dir)
	}
	if gh.Owner != "" || gh.Repo != "" {
		t.Errorf("owner/repo = %q/%q, want empty for a local repo", gh.Owner, gh.Repo)
	}
}
