package core

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the runtime configuration for the long-running apps (serve, mcp),
// read from the environment. It is shared so both transports resolve the repo,
// token, and git host identically.
type Config struct {
	Addr      string
	TTL       time.Duration
	Repo      string
	Token     string
	GitHubAPI string
}

// ConfigFromEnv reads GIT_REPO (required), GIT_TOKEN, DRAFT_TTL (default 2h),
// ADDR (default :8080), and GITHUB_API (REST base override).
func ConfigFromEnv() (Config, error) {
	repo := os.Getenv("GIT_REPO")
	if repo == "" {
		return Config{}, fmt.Errorf("GIT_REPO is required")
	}
	ttl := 2 * time.Hour
	if v := os.Getenv("DRAFT_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("DRAFT_TTL %q: %w", v, err)
		}
		ttl = d
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{Addr: addr, TTL: ttl, Repo: repo, Token: os.Getenv("GIT_TOKEN"), GitHubAPI: os.Getenv("GITHUB_API")}, nil
}

// NewGitHost builds the GitHost for cfg: a GitHubHost for a remote URL, or a
// local-path dev backend (reads/drafts/previews work; propose is unavailable).
func NewGitHost(cfg Config) (GitHost, error) {
	var host *GitHubHost
	if isRemoteRepo(cfg.Repo) {
		h, err := NewGitHubHost(cfg.Repo, cfg.Token)
		if err != nil {
			return nil, err
		}
		host = h
	} else {
		// A local repo path (dev backend): reads, drafts, validation, and previews
		// work against the local checkout via go-git. propose needs the GitHub REST
		// API, so it is unavailable in this mode (owner/repo are unset).
		host = &GitHubHost{RepoURL: cfg.Repo, Token: cfg.Token, APIBase: "https://api.github.com"}
	}
	if cfg.GitHubAPI != "" {
		host.APIBase = cfg.GitHubAPI
	}
	return host, nil
}

// isRemoteRepo reports whether repo is a remote URL (https/http/ssh) rather
// than a local filesystem path used as a dev backend.
func isRemoteRepo(s string) bool { return strings.Contains(s, "://") || strings.HasPrefix(s, "git@") }
