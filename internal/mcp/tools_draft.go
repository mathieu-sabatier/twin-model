package mcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
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
		mcp.WithDescription("Write model files into a draft. Strict by default: refuses (stores nothing) if any file fails to parse, or if any file has an error-severity validation diagnostic, returning both; pass force:true to store anyway. On success returns the stored file list plus per-file validation diagnostics. files is a map of filename -> YAML."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithObject("files", mcp.Required(), mcp.Description("map of filename to YAML content")),
		mcp.WithBoolean("force", mcp.Description("store even if a file fails to parse or has error-severity validation diagnostics (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			files, err := stringMap(r, "files")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			force := r.GetBool("force", false)
			diags := make(map[string][]dto.Diagnostic, len(files))
			var parseErrs []string
			var blockers []dto.Diagnostic
			for name, content := range files {
				pr := c.ParseModel(name, []byte(content))
				if pr.ParseError != "" {
					parseErrs = append(parseErrs, name+": "+pr.ParseError)
				}
				diags[name] = pr.Diagnostics
				for _, d := range pr.Diagnostics {
					if d.Severity == "error" {
						blockers = append(blockers, d)
					}
				}
			}
			if (len(parseErrs) > 0 || len(blockers) > 0) && !force {
				var msg strings.Builder
				if len(parseErrs) > 0 {
					sort.Strings(parseErrs)
					msg.WriteString("update_draft refused — unparseable file(s):\n")
					msg.WriteString(strings.Join(parseErrs, "\n"))
				}
				if len(blockers) > 0 {
					if msg.Len() > 0 {
						msg.WriteString("\n")
					}
					msg.WriteString("update_draft refused — error-severity validation diagnostic(s) (pass force:true to store anyway):\n")
					msg.WriteString(formatBlocking(blockers))
				}
				return mcp.NewToolResultError(msg.String()), nil
			}
			resp, uerr := c.UpdateDraft(r.GetString("draftId", ""), files)
			if uerr != nil {
				return toolErr(uerr)
			}
			return jsonResult(map[string]any{"files": resp.Files, "diagnostics": diags})
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

	s.AddTool(mcp.NewTool("add_import",
		mcp.WithDescription("Add a companion-spec import (alias -> namespace URI) to a draft file. Additive: fails if the alias already exists. Strict by default: refuses (stores nothing) if the resulting file has any error-severity validation diagnostic; pass force:true to store anyway. Pass dryRun:true to validate the result in full draft context without storing anything (returns diagnostics only). Returns the file list + validation diagnostics."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file")),
		mcp.WithString("alias", mcp.Required(), mcp.Description("import alias, e.g. DI")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("namespace URI")),
		mcp.WithBoolean("force", mcp.Description("store even if the result has error-severity validation diagnostics (default false)")),
		mcp.WithBoolean("dryRun", mcp.Description("validate the result in full draft context but store nothing; returns diagnostics only (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			force := r.GetBool("force", false)
			dryRun := r.GetBool("dryRun", false)
			resp, err := c.AddImport(r.GetString("draftId", ""), r.GetString("file", ""),
				r.GetString("alias", ""), r.GetString("namespace", ""), core.WriteOpts{Force: force, DryRun: dryRun})
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return mcp.NewToolResultError("add_import refused — error-severity validation diagnostics (pass force:true to store anyway):\n" + formatBlocking(ve.Blocking)), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("add_type",
		mcp.WithDescription("Add an ObjectType to a draft file. name is the type name; body is its DSL body (doc/base/abstract/members, snake_case — exactly what you'd write under the type). Additive: fails if the type name exists. Strict by default: refuses (stores nothing) if the resulting file has any error-severity validation diagnostic; pass force:true to store anyway (e.g. for forward references). Pass dryRun:true to validate the result in full draft context without storing anything (returns diagnostics only). Returns the file list + validation diagnostics."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file")),
		mcp.WithString("name", mcp.Required(), mcp.Description("type name, e.g. WeigherType")),
		mcp.WithString("body", mcp.Required(), mcp.Description("type body YAML: doc/base/abstract/members")),
		mcp.WithBoolean("force", mcp.Description("store even if the result has error-severity validation diagnostics (default false)")),
		mcp.WithBoolean("dryRun", mcp.Description("validate the result in full draft context but store nothing; returns diagnostics only (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			force := r.GetBool("force", false)
			dryRun := r.GetBool("dryRun", false)
			resp, err := c.AddType(r.GetString("draftId", ""), r.GetString("file", ""),
				r.GetString("name", ""), r.GetString("body", ""), core.WriteOpts{Force: force, DryRun: dryRun})
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return mcp.NewToolResultError("add_type refused — error-severity validation diagnostics (pass force:true to store anyway):\n" + formatBlocking(ve.Blocking)), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("add_instance",
		mcp.WithDescription("Add an object instance to a draft file. name is the instance name; body is its DSL body (type, under, level, and any value overrides/children). Additive: fails if the instance name exists. Strict by default: refuses (stores nothing) if the resulting file has any error-severity validation diagnostic; pass force:true to store anyway (e.g. to place an instance before the sibling it nests under exists yet). Pass dryRun:true to validate the result in full draft context without storing anything (returns diagnostics only). Returns the file list + validation diagnostics."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file")),
		mcp.WithString("name", mcp.Required(), mcp.Description("instance name, e.g. Weigher1")),
		mcp.WithString("body", mcp.Required(), mcp.Description("instance body YAML: type, under, level, ...")),
		mcp.WithBoolean("force", mcp.Description("store even if the result has error-severity validation diagnostics (default false)")),
		mcp.WithBoolean("dryRun", mcp.Description("validate the result in full draft context but store nothing; returns diagnostics only (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			force := r.GetBool("force", false)
			dryRun := r.GetBool("dryRun", false)
			resp, err := c.AddInstance(r.GetString("draftId", ""), r.GetString("file", ""),
				r.GetString("name", ""), r.GetString("body", ""), core.WriteOpts{Force: force, DryRun: dryRun})
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return mcp.NewToolResultError("add_instance refused — error-severity validation diagnostics (pass force:true to store anyway):\n" + formatBlocking(ve.Blocking)), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("remove_type",
		mcp.WithDescription("Remove an ObjectType from a draft file by name (reverse of add_type). Refuses if the removal would leave an error-severity validation diagnostic (e.g. an instance still references it) unless force:true. dryRun previews without storing. Fails not-found if the type is absent."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file")),
		mcp.WithString("name", mcp.Required(), mcp.Description("type name to remove")),
		mcp.WithBoolean("force", mcp.Description("remove even if the result has error-severity validation diagnostics (default false)")),
		mcp.WithBoolean("dryRun", mcp.Description("validate the removal in full draft context but store nothing; diagnostics only (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.RemoveType(r.GetString("draftId", ""), r.GetString("file", ""), r.GetString("name", ""),
				core.WriteOpts{Force: r.GetBool("force", false), DryRun: r.GetBool("dryRun", false)})
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return mcp.NewToolResultError("remove_type refused — error-severity validation diagnostics (pass force:true to remove anyway):\n" + formatBlocking(ve.Blocking)), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("remove_instance",
		mcp.WithDescription("Remove an object instance from a draft file by name (reverse of add_instance). Refuses if the removal would leave an error-severity validation diagnostic (e.g. a child still nests under it) unless force:true. dryRun previews without storing. Fails not-found if the instance is absent."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file")),
		mcp.WithString("name", mcp.Required(), mcp.Description("instance name to remove")),
		mcp.WithBoolean("force", mcp.Description("remove even if the result has error-severity validation diagnostics (default false)")),
		mcp.WithBoolean("dryRun", mcp.Description("validate the removal in full draft context but store nothing; diagnostics only (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.RemoveInstance(r.GetString("draftId", ""), r.GetString("file", ""), r.GetString("name", ""),
				core.WriteOpts{Force: r.GetBool("force", false), DryRun: r.GetBool("dryRun", false)})
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return mcp.NewToolResultError("remove_instance refused — error-severity validation diagnostics (pass force:true to remove anyway):\n" + formatBlocking(ve.Blocking)), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("remove_import",
		mcp.WithDescription("Remove a companion-spec import from a draft file by alias (reverse of add_import). Refuses if the removal would leave an error-severity validation diagnostic (e.g. a type's base: or a member's type: still references the alias) unless force:true. dryRun previews without storing. Fails not-found if the alias is absent."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file")),
		mcp.WithString("alias", mcp.Required(), mcp.Description("import alias to remove, e.g. DI")),
		mcp.WithBoolean("force", mcp.Description("remove even if the result has error-severity validation diagnostics (default false)")),
		mcp.WithBoolean("dryRun", mcp.Description("validate the removal in full draft context but store nothing; diagnostics only (default false)"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.RemoveImport(r.GetString("draftId", ""), r.GetString("file", ""), r.GetString("alias", ""),
				core.WriteOpts{Force: r.GetBool("force", false), DryRun: r.GetBool("dryRun", false)})
			if err != nil {
				var ve *core.ValidationError
				if errors.As(err, &ve) {
					return mcp.NewToolResultError("remove_import refused — error-severity validation diagnostics (pass force:true to remove anyway):\n" + formatBlocking(ve.Blocking)), nil
				}
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("list_drafts",
		mcp.WithDescription("List all live server-side drafts (id, baseRef, files, updatedAt). Use it to find and clean up throwaway/orphaned drafts.")),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(c.ListDrafts())
		})

	s.AddTool(mcp.NewTool("discard_draft",
		mcp.WithDescription("Delete a server-side draft by id (throwaway cleanup). Fails with not-found if the id is unknown."),
		mcp.WithString("draftId", mcp.Required())),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := c.DiscardDraft(r.GetString("draftId", "")); err != nil {
				return toolErr(err)
			}
			return jsonResult(map[string]any{"discarded": r.GetString("draftId", "")})
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
				// The composite path created this draft only to carry the inline
				// files into Propose; discard it on the way out so a loop of
				// (failing or succeeding) inline proposes doesn't pile up orphaned
				// drafts in the store until the TTL sweep.
				defer func() { _ = c.DiscardDraft(draftID) }()
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

// formatBlocking renders error-severity validation blockers into a stable,
// human/agent-readable refusal body: one line per diagnostic, sorted for
// determinism (the validator's emission order is not a contract callers should
// rely on for a rendered message).
func formatBlocking(ds []dto.Diagnostic) string {
	lines := make([]string, 0, len(ds))
	for _, d := range ds {
		lines = append(lines, fmt.Sprintf("%s (%s) %s:%d — %s", d.Code, d.Severity, d.File, d.Line, d.Message))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
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
