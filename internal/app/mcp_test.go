package app

import (
	"testing"

	"go.uber.org/fx"
)

func TestMCPStdioApp_GraphIsValid(t *testing.T) {
	t.Setenv("GIT_REPO", t.TempDir())
	app := fx.New(CoreModule(), MCPServerModule(), MCPStdioModule(), fx.NopLogger)
	if err := app.Err(); err != nil {
		t.Fatalf("mcp stdio app graph invalid: %v", err)
	}
}
