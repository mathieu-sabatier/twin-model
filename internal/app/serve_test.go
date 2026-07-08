package app

import (
	"testing"

	"go.uber.org/fx"
)

func TestServeApp_GraphIsValid(t *testing.T) {
	t.Setenv("GIT_REPO", t.TempDir()) // local-path backend; no network
	app := fx.New(CoreModule(), MCPServerModule(), ServeModule(), fx.NopLogger)
	if err := app.Err(); err != nil {
		t.Fatalf("serve app graph invalid: %v", err)
	}
}
