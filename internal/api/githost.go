package api

import (
	"context"

	"github.com/mathieu-sabatier/twin-model/internal/api/dto"
)

// ProposeParams is everything OpenPR needs to branch, commit the fileset, push,
// and open a pull request.
type ProposeParams struct {
	BaseRef string
	Branch  string
	Title   string
	Message string
	Files   map[string][]byte
}

// GitHost is the only persistence boundary. ReadTree returns the repo-relative
// path→bytes of a committed ref; OpenPR branches/commits/pushes the fileset and
// returns the pull request URL. Implementations read their token from env and
// never surface it to the browser. GitHub is the first impl; GitLab can follow.
type GitHost interface {
	ReadTree(ctx context.Context, ref string) (map[string][]byte, error)
	OpenPR(ctx context.Context, p ProposeParams) (url string, err error)
	// ListPRs returns the open pull requests on the model repo.
	ListPRs(ctx context.Context) ([]dto.PullRequest, error)
	// Info reports repo context (owner/repo/url), the commit identity OpenPR uses,
	// and whether proposing is possible — all derivable without a network call.
	Info() dto.RepoInfo
	// Branches lists the repo's head branches and resolves its default branch
	// (from remote HEAD) via an ls-remote — no clone. Works for local paths and
	// remote URLs; needs the token only for a private remote.
	Branches(ctx context.Context) (dto.BranchList, error)
}
