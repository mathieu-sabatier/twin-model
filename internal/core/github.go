package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// PRError is returned when the GitHub API rejects the create-PR request. It keeps
// the raw response body separate from a friendly summary so the API layer can show
// a humane message while still exposing the payload behind a details disclosure.
type PRError struct {
	Status int
	Body   string
}

func (e *PRError) Error() string {
	return fmt.Sprintf("GitHub API %d: %s", e.Status, e.Body)
}

// commitAuthorName/Email is the identity every proposed commit is authored as. It
// is a shared server bot, not a per-user identity; Info surfaces it so the UI can
// tell the user who their changes commit as.
const (
	commitAuthorName  = "twinmodel-bot"
	commitAuthorEmail = "bot@twinmodel"

	// CommitAuthorName/CommitAuthorEmail are exported for the transitional
	// internal/api forwarders; see commitAuthorName/commitAuthorEmail.
	CommitAuthorName  = commitAuthorName
	CommitAuthorEmail = commitAuthorEmail
)

// GitHubHost is the real GitHost implementation: go-git for all git operations
// and a thin stdlib net/http call to the GitHub REST API to open pull requests.
type GitHubHost struct {
	// RepoURL is the git remote URL (https or local file path for tests).
	RepoURL string
	// Token is a GitHub personal access token used for the REST API call and as
	// the HTTP basic-auth password when pushing over HTTPS. May be empty for
	// local-transport tests.
	Token string
	// APIBase is the GitHub REST API root, e.g. "https://api.github.com". Tests
	// inject an httptest.Server URL here to avoid real network calls.
	APIBase string
	// Owner and Repo are the GitHub owner/repo parsed from RepoURL if left
	// empty (and only needed for the REST create-PR call).
	Owner string
	Repo  string
}

// authMethod returns the HTTP basic-auth for private-remote git operations
// (clone, ls-remote, push): the token as the password under the conventional
// "x-access-token" username. It returns a true nil interface when no token is
// set, so public-remote and local file-transport operations run unauthenticated
// (a typed-nil *BasicAuth would make go-git attempt empty auth instead).
func (g *GitHubHost) authMethod() transport.AuthMethod {
	if g.Token == "" {
		return nil
	}
	return &githttp.BasicAuth{Username: "x-access-token", Password: g.Token}
}

// cloneRef clones RepoURL at a branch ref into memory. fs may be nil (ReadTree
// only needs the object store) or a worktree filesystem (OpenPR writes into it).
// Auth is sent only when a token is present (needed for a private remote); local
// file-transport tests clone unauthenticated. Depth is intentionally omitted:
// go-git's local file transport, used by the tests, does not support shallow
// clones ("unsupported capability").
func (g *GitHubHost) cloneRef(ctx context.Context, ref string, fs billy.Filesystem) (*git.Repository, error) {
	return git.CloneContext(ctx, memory.NewStorage(), fs, &git.CloneOptions{
		URL:           g.RepoURL,
		ReferenceName: plumbing.NewBranchReferenceName(ref),
		SingleBranch:  true,
		Auth:          g.authMethod(),
	})
}

// ReadTree clones RepoURL at ref into memory and returns the full file tree as
// path→bytes. It does not retain the clone; each call is fresh.
func (g *GitHubHost) ReadTree(ctx context.Context, ref string) (map[string][]byte, error) {
	repo, err := g.cloneRef(ctx, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("clone %s@%s: %w", g.RepoURL, ref, err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("commit object: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("tree: %w", err)
	}

	files := map[string][]byte{}
	if err := tree.Files().ForEach(func(f *object.File) error {
		content, err := f.Contents()
		if err != nil {
			return err
		}
		files[f.Name] = []byte(content)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk tree: %w", err)
	}
	return files, nil
}

// OpenPR creates a new branch off p.BaseRef, commits the fileset, pushes to
// the remote, then calls the GitHub REST API to open a pull request. It returns
// the PR html_url.
func (g *GitHubHost) OpenPR(ctx context.Context, p ProposeParams) (string, error) {
	fs := memfs.New()
	repo, err := g.cloneRef(ctx, p.BaseRef, fs)
	if err != nil {
		return "", fmt.Errorf("clone for PR: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	// Create and checkout the new branch.
	branchRef := plumbing.NewBranchReferenceName(p.Branch)
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
		Create: true,
	}); err != nil {
		return "", fmt.Errorf("checkout -b %s: %w", p.Branch, err)
	}

	// Write each file from the fileset into the worktree's in-memory filesystem
	// and stage it.
	for path, content := range p.Files {
		// Ensure parent directories exist.
		if err := mkdirAllBilly(fs, dirOf(path)); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", path, err)
		}
		f, err := fs.Create(path)
		if err != nil {
			return "", fmt.Errorf("create %s: %w", path, err)
		}
		_, werr := f.Write(content)
		f.Close()
		if werr != nil {
			return "", fmt.Errorf("write %s: %w", path, werr)
		}
		if _, err := wt.Add(path); err != nil {
			return "", fmt.Errorf("git add %s: %w", path, err)
		}
	}

	// Commit.
	msg := p.Title
	if p.Message != "" {
		msg += "\n\n" + p.Message
	}
	sig := &object.Signature{
		Name:  commitAuthorName,
		Email: commitAuthorEmail,
		When:  time.Now(),
	}
	if _, err := wt.Commit(msg, &git.CommitOptions{
		Author:    sig,
		Committer: sig,
	}); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	// Push the branch to the remote. Auth is only set when a token is present;
	// local file-transport tests run without auth.
	pushOpts := &git.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/heads/" + p.Branch + ":refs/heads/" + p.Branch),
		},
	}
	pushOpts.Auth = g.authMethod()
	if err := repo.PushContext(ctx, pushOpts); err != nil {
		return "", fmt.Errorf("push: %w", err)
	}

	// Open the pull request via the GitHub REST API.
	owner, repoName := g.ownerRepo()
	return g.createPR(ctx, owner, repoName, p)
}

// createPR POSTs to the GitHub REST API to create a pull request and returns
// the html_url field from the response JSON.
func (g *GitHubHost) createPR(ctx context.Context, owner, repoName string, p ProposeParams) (string, error) {
	apiBase := g.APIBase
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	body, _ := json.Marshal(map[string]string{
		"title": p.Title,
		"body":  p.Message,
		"head":  p.Branch,
		"base":  p.BaseRef,
	})
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls", apiBase, owner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST pulls: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", &PRError{Status: resp.StatusCode, Body: string(data)}
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return out.HTMLURL, nil
}

// ownerRepo resolves the GitHub owner/repo, parsing them from RepoURL when the
// struct fields are unset.
func (g *GitHubHost) ownerRepo() (string, string) {
	if g.Owner != "" && g.Repo != "" {
		return g.Owner, g.Repo
	}
	return parseOwnerRepo(g.RepoURL)
}

// Info reports the repo the host edits, the commit identity OpenPR uses, and
// whether proposing is possible. Proposing needs a parseable GitHub owner/repo
// (not a local checkout) AND a push token; ProposeReason explains any gap.
func (g *GitHubHost) Info() dto.RepoInfo {
	owner, repo := g.ownerRepo()
	info := dto.RepoInfo{
		Host:          "github",
		Owner:         owner,
		Repo:          repo,
		DefaultBranch: "main",
		CommitName:    commitAuthorName,
		CommitEmail:   commitAuthorEmail,
	}
	if owner != "" && repo != "" {
		info.URL = "https://github.com/" + owner + "/" + repo
	}
	switch {
	case owner == "" || repo == "":
		info.ProposeReason = "This server is pointed at a local checkout; proposing opens a GitHub pull request and needs a GitHub repository."
	case g.Token == "":
		info.ProposeReason = "No push token is configured on the server (GIT_TOKEN), so changes cannot be pushed."
	default:
		info.ProposeEnabled = true
	}
	return info
}

// Branches lists the repo's head branches (default first, then alphabetical) and
// resolves the default branch from the remote HEAD symref. It runs an ls-remote
// (remote.ListContext) rather than cloning; the file transport (local paths) and
// http transport (remote URLs) both advertise HEAD's symref. Auth mirrors push:
// the token is sent only when present (needed for a private remote).
func (g *GitHubHost) Branches(ctx context.Context) (dto.BranchList, error) {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{g.RepoURL},
	})
	opts := &git.ListOptions{Auth: g.authMethod()}
	refs, err := rem.ListContext(ctx, opts)
	if err != nil {
		return dto.BranchList{}, err
	}
	var heads []string
	defaultBranch := ""
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			heads = append(heads, ref.Name().Short())
		}
		if ref.Type() == plumbing.SymbolicReference && ref.Name() == plumbing.HEAD {
			defaultBranch = ref.Target().Short()
		}
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	sort.Strings(heads)
	// Default branch first, then the remaining heads alphabetically (default
	// de-duplicated). The default is always present even if the repo has no
	// heads yet, so the picker never renders empty.
	ordered := make([]string, 0, len(heads)+1)
	ordered = append(ordered, defaultBranch)
	for _, b := range heads {
		if b != defaultBranch {
			ordered = append(ordered, b)
		}
	}
	return dto.BranchList{Branches: ordered, DefaultBranch: defaultBranch}, nil
}

// ListPRs GETs the open pull requests from the GitHub REST API and maps them to
// the API's PullRequest shape.
func (g *GitHubHost) ListPRs(ctx context.Context) ([]dto.PullRequest, error) {
	apiBase := g.APIBase
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	owner, repoName := g.ownerRepo()
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open", apiBase, owner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET pulls: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, data)
	}
	var raw []struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	prs := make([]dto.PullRequest, len(raw))
	for i, p := range raw {
		prs[i] = dto.PullRequest{
			Number: p.Number, Title: p.Title, URL: p.HTMLURL, Branch: p.Head.Ref, State: p.State,
		}
	}
	return prs, nil
}

// NewGitHubHost builds a GitHubHost from an HTTPS repo URL and token, parsing
// owner/repo from the URL and defaulting the REST API base.
func NewGitHubHost(repoURL, token string) (*GitHubHost, error) {
	owner, repo := parseOwnerRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("cannot parse owner/repo from %q", repoURL)
	}
	return &GitHubHost{
		RepoURL: repoURL,
		Token:   token,
		APIBase: "https://api.github.com",
		Owner:   owner,
		Repo:    repo,
	}, nil
}

// parseOwnerRepo extracts the "owner/repo" from a GitHub HTTPS or SSH URL.
// Returns empty strings for local paths (used in tests, where Owner/Repo are
// set directly).
func parseOwnerRepo(rawURL string) (owner, repo string) {
	// https://github.com/owner/repo[.git][/...] — owner/repo are the two path
	// segments right after the host, not the last two: a browse URL like
	// .../owner/repo/tree/main must not parse as owner=tree, repo=main.
	for _, scheme := range []string{"https://", "http://"} {
		if s, ok := strings.CutPrefix(rawURL, scheme); ok {
			parts := strings.Split(s, "/") // [host, owner, repo, ...]
			if len(parts) >= 3 {
				return parts[1], strings.TrimSuffix(parts[2], ".git")
			}
			return "", ""
		}
	}
	// git@github.com:owner/repo.git — path after the colon is "owner/repo".
	if _, path, ok := strings.Cut(rawURL, ":"); ok {
		parts := strings.SplitN(strings.TrimSuffix(path, ".git"), "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return "", ""
}

// dirOf returns the directory portion of a slash-delimited path.
func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}

// mkdirAllBilly creates all directories in path on the given billy.Filesystem.
func mkdirAllBilly(fs billy.Filesystem, path string) error {
	if path == "" || path == "." {
		return nil
	}
	return fs.MkdirAll(path, 0o755)
}
