package mcp

import (
	"context"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// registerDraftTools registers the tier-2 tools: the git-backed draft lifecycle
// (create/update/diff) and the propose_pr composite that opens a pull request.
// Unlike registerReadTools, these mutate server state (the draft Store) and, for
// propose_pr, the remote repo via c.Host().
func registerDraftTools(s *server.MCPServer, c *core.Service) {
	s.AddTool(mcp.NewTool("repo_info",
		mcp.WithDescription("Report repo context (owner/repo/url), the commit identity, and whether proposing PRs is enabled (proposeEnabled + reason).")),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(c.RepoInfo())
		})

	s.AddTool(mcp.NewTool("list_prs",
		mcp.WithDescription("List the open pull requests on the model repo.")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.ListPRs(ctx)
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("list_branches",
		mcp.WithDescription("List the repo's branches (default branch first).")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.Branches(ctx)
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("create_draft",
		mcp.WithDescription("Create a server-side draft from a branch's model files; returns a draftId for iterative edits."),
		mcp.WithString("baseRef", mcp.Required(), mcp.Description("base branch name"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.CreateDraft(ctx, r.GetString("baseRef", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("update_draft",
		mcp.WithDescription("Write model files into a draft (server canonicalizes each parseable file). files is a map of filename->YAML."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithObject("files", mcp.Required(), mcp.Description("map of filename to YAML content"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			files, err := stringMap(r, "files")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			resp, uerr := c.UpdateDraft(r.GetString("draftId", ""), files)
			if uerr != nil {
				return toolErr(uerr)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("draft_diff",
		mcp.WithDescription("Return the semantic changelist of a draft file vs its base branch."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.DraftDiff(r.GetString("draftId", ""), r.GetString("file", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("propose_pr",
		mcp.WithDescription("Open a GitHub pull request. Either pass draftId (propose an existing draft) OR pass baseRef+files (draft, validate, and propose in one call). Refuses if the model has error-severity diagnostics."),
		mcp.WithString("draftId", mcp.Description("existing draft to propose")),
		mcp.WithString("baseRef", mcp.Description("base branch (when not using draftId)")),
		mcp.WithObject("files", mcp.Description("filename->YAML (when not using draftId)")),
		mcp.WithString("branch", mcp.Required(), mcp.Description("new branch name for the PR")),
		mcp.WithString("title", mcp.Required()),
		mcp.WithString("message", mcp.Description("PR body"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			draftID := r.GetString("draftId", "")
			if draftID == "" { // composite path: create a draft from inline files first
				files, err := stringMap(r, "files")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				cd, err := c.CreateDraft(ctx, r.GetString("baseRef", ""))
				if err != nil {
					return toolErr(err)
				}
				if _, err := c.UpdateDraft(cd.ID, files); err != nil {
					return toolErr(err)
				}
				draftID = cd.ID
			}
			resp, err := c.Propose(ctx, draftID, r.GetString("branch", ""), r.GetString("title", ""), r.GetString("message", ""))
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return jsonResult(map[string]any{"error": "validation", "blocking": ve.Blocking})
				}
				// Only an OpenPR (host) failure gets the friendly PR-failure message;
				// ErrNotFound (unknown draftId) and ErrInvalid (empty branch/title) are
				// returned before the host is ever touched, so they must fall through
				// to the plain sentinel mapping below. Mirrors internal/api/handlers.go
				// handlePropose. QA finding: the old `m != "" ` guard was dead code —
				// DescribeProposeError always returns a non-empty msg — so every
				// Propose failure, including these two, was mislabeled as a PR/push
				// failure.
				if !errors.Is(err, core.ErrNotFound) && !errors.Is(err, core.ErrInvalid) {
					m, detail := core.DescribeProposeError(err)
					return mcp.NewToolResultError(m + " — " + detail), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})
}

// stringMap reads an object tool-argument as map[string]string.
func stringMap(r mcp.CallToolRequest, key string) (map[string]string, error) {
	raw := r.GetArguments()[key]
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New(key + " must be an object of filename->YAML")
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New(key + "." + k + " must be a string")
		}
		out[k] = s
	}
	return out, nil
}
