package app

import (
	"context"
	"fmt"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/fx"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/mcp"
)

// MCPServerModule provides the *mcpserver.MCPServer built over the shared
// core.Service. Both the stdio app and the /mcp mount consume it.
func MCPServerModule() fx.Option {
	return fx.Module("mcp-server", fx.Provide(func(svc *core.Service) *mcpserver.MCPServer { return mcp.NewServer(svc) }))
}

// MCPStdioModule runs the MCP server over stdio under the fx lifecycle.
func MCPStdioModule() fx.Option {
	return fx.Module("mcp-stdio",
		fx.Invoke(func(lc fx.Lifecycle, s *mcpserver.MCPServer, sd fx.Shutdowner) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						if err := mcpserver.ServeStdio(s); err != nil {
							fmt.Fprintf(os.Stderr, "twinmodel mcp: %v\n", err)
							_ = sd.Shutdown(fx.ExitCode(1))
							return
						}
						_ = sd.Shutdown()
					}()
					return nil
				},
			})
		}),
	)
}

// RunMCPStdio builds and runs the stdio MCP app until stdin closes.
func RunMCPStdio() error {
	fx.New(CoreModule(), MCPServerModule(), MCPStdioModule(), fx.NopLogger).Run()
	return nil
}
