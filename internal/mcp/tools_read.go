package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// jsonResult renders v as a pretty-printed JSON tool result.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// toolErr converts a core sentinel/error into an MCP tool error result (never a
// transport error — the AI sees the message and can recover).
func toolErr(err error) (*mcp.CallToolResult, error) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		return mcp.NewToolResultError("not found: " + err.Error()), nil
	default:
		return mcp.NewToolResultError(err.Error()), nil
	}
}

// registerReadTools registers the tier-1 tools: parsed data, companion-spec
// catalog lookups, and previews. None of these mutate server state
func registerReadTools(s *server.MCPServer, c *core.Service) {
	s.AddTool(mcp.NewTool("get_schema",
		mcp.WithDescription("Return the twinmodel DSL JSON Schema.")),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText(c.Schema()), nil
		})

	s.AddTool(mcp.NewTool("parse_model",
		mcp.WithDescription("Parse inline twinmodel YAML and return its AST-as-JSON plus diagnostics (file:line, severity, code). Validation is a byproduct — the point is the parsed structure."),
		mcp.WithString("file", mcp.Required(), mcp.Description("logical filename, e.g. Furnace.yaml")),
		mcp.WithString("yaml", mcp.Required(), mcp.Description("the model YAML"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(c.ParseModel(r.GetString("file", ""), []byte(r.GetString("yaml", ""))))
		})

	s.AddTool(mcp.NewTool("get_model",
		mcp.WithDescription("Read a committed model file from the repo at a branch ref and return its AST-as-JSON plus diagnostics. The first tool to reach for to understand an existing model."),
		mcp.WithString("ref", mcp.Required(), mcp.Description("branch name")),
		mcp.WithString("file", mcp.Description("model file path or basename; defaults to the first model file"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.ReadModel(ctx, r.GetString("ref", ""), r.GetString("file", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("get_model_source",
		mcp.WithDescription("Return the raw YAML source text of a model file at a committed branch ref — the exact bytes to edit and write back (unlike get_model, which returns the AST). Reach for this before editing an existing model."),
		mcp.WithString("ref", mcp.Required(), mcp.Description("branch name")),
		mcp.WithString("file", mcp.Description("model file path or basename; defaults to the first model file"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := c.ReadModelSource(ctx, r.GetString("ref", ""), r.GetString("file", ""))
			if err != nil {
				return toolErr(err)
			}
			return mcp.NewToolResultText(string(data)), nil
		})

	s.AddTool(mcp.NewTool("get_draft_source",
		mcp.WithDescription("Return the raw canonical YAML source of a file in a draft (server-side read-back). Use it to fetch the current draft text before a whole-file update_draft, so you never reconstruct and drop fields."),
		mcp.WithString("draftId", mcp.Required()),
		mcp.WithString("file", mcp.Description("file path or basename; defaults to the first file"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := c.DraftFileRaw(r.GetString("draftId", ""), r.GetString("file", ""))
			if err != nil {
				return toolErr(err)
			}
			return mcp.NewToolResultText(string(data)), nil
		})

	s.AddTool(mcp.NewTool("list_model_files",
		mcp.WithDescription("List the model files present at a branch ref."),
		mcp.WithString("ref", mcp.Required(), mcp.Description("branch name"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			files, err := c.ListModelFiles(ctx, r.GetString("ref", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(files)
		})

	s.AddTool(mcp.NewTool("preview_modeldesign",
		mcp.WithDescription("Transpile inline YAML to UA-ModelCompiler ModelDesign XML."),
		mcp.WithString("file", mcp.Required()), mcp.WithString("yaml", mcp.Required())),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			xml, err := c.PreviewModelDesign(r.GetString("file", ""), []byte(r.GetString("yaml", "")))
			if err != nil {
				return toolErr(err)
			}
			return mcp.NewToolResultText(string(xml)), nil
		})

	s.AddTool(mcp.NewTool("preview_diagram",
		mcp.WithDescription("Render a Mermaid class (view=types) or instance (view=instances) diagram from inline YAML."),
		mcp.WithString("file", mcp.Required()), mcp.WithString("yaml", mcp.Required()),
		mcp.WithString("view", mcp.Description("types (default) or instances"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, err := c.PreviewDiagram(r.GetString("file", ""), []byte(r.GetString("yaml", "")), r.GetString("view", "types"))
			if err != nil {
				return toolErr(err)
			}
			return mcp.NewToolResultText(src), nil
		})

	s.AddTool(mcp.NewTool("resolve_type",
		mcp.WithDescription("Return the flattened inherited members of a type defined in inline YAML."),
		mcp.WithString("file", mcp.Required()), mcp.WithString("yaml", mcp.Required()),
		mcp.WithString("name", mcp.Required(), mcp.Description("type name"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.ResolveType(r.GetString("file", ""), []byte(r.GetString("yaml", "")), r.GetString("name", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("find_unit",
		mcp.WithDescription("List engineering units (UNECE Rec. 20) known to twinmodel; filter by q substring on the symbol/name."),
		mcp.WithString("q", mcp.Description("optional case-insensitive filter"))),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(c.Units()) // client filters; Units() is the full sorted set
		})

	// --- catalog / companion types ---
	s.AddTool(mcp.NewTool("list_namespaces",
		mcp.WithDescription("List the bundled OPC UA companion specs (DI, Machinery, ISA-95) with version and dependency aliases.")),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.CatalogList()
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("list_types",
		mcp.WithDescription("List the ObjectTypes/VariableTypes in one companion spec."),
		mcp.WithString("alias", mcp.Required(), mcp.Description("spec alias, e.g. di, machinery, isa95"))),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.CatalogTypes(r.GetString("alias", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("get_type_details",
		mcp.WithDescription("Return a companion type's base chain and resolved members."),
		mcp.WithString("alias", mcp.Required()), mcp.WithString("name", mcp.Required())),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.CatalogType(r.GetString("alias", ""), r.GetString("name", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})

	s.AddTool(mcp.NewTool("search_types",
		mcp.WithDescription("Search companion types by case-insensitive substring across all specs."),
		mcp.WithString("q", mcp.Required())),
		func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.CatalogSearch(r.GetString("q", ""))
			if err != nil {
				return toolErr(err)
			}
			return jsonResult(resp)
		})
}
