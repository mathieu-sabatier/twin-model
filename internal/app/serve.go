package app

import (
	"context"
	"fmt"
	"net/http"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/fx"

	"github.com/mathieu-sabatier/twin-model/internal/api"
	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/web"
)

// ServeModule builds the single-origin HTTP handler (/api JSON API + /mcp MCP
// transport + / embedded SPA) and runs it as an *http.Server bound to the fx
// lifecycle.
func ServeModule() fx.Option {
	return fx.Module("serve",
		fx.Provide(newHTTPHandler, newHTTPServer),
		fx.Invoke(func(*http.Server) {}), // force construction
	)
}

// newHTTPHandler wires the root mux. /api and /mcp both operate on the same
// svc, so the web editor's drafts and the MCP tool calls share the one
// in-process Store — there is no separate state to sync.
func newHTTPHandler(svc *core.Service, m *mcpserver.MCPServer) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api/", api.NewServerFromService(svc).Routes())
	root.Handle("/mcp", mcpserver.NewStreamableHTTPServer(m)) // shares svc's in-process store
	root.Handle("/", web.Handler())
	return root
}

func newHTTPServer(lc fx.Lifecycle, cfg core.Config, h http.Handler, sd fx.Shutdowner) *http.Server {
	srv := &http.Server{Addr: cfg.Addr, Handler: h}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			fmt.Printf("twinmodel serve: listening on %s (repo %s; SPA at /, API at /api)\n", cfg.Addr, cfg.Repo)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					fmt.Fprintf(os.Stderr, "twinmodel serve: %v\n", err)
					_ = sd.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error { return srv.Shutdown(ctx) },
	})
	return srv
}

// RunServe builds and runs the serve app until interrupted.
func RunServe() error {
	app := fx.New(CoreModule(), MCPServerModule(), ServeModule(), fx.NopLogger)
	app.Run()
	return nil
}
