// Package mcp exposes twinmodel's modeling operations as an MCP server, calling
// core.Service directly (never internal/api) so the tool layer stays
// transport-agnostic alongside the HTTP API.
package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// NewServer builds an MCP server exposing twinmodel's modeling operations over
// the given core.Service. Used by both the stdio subcommand and the /mcp mount.
func NewServer(c *core.Service) *server.MCPServer {
	s := server.NewMCPServer("twinmodel", "0.1.0", server.WithToolCapabilities(true))
	registerReadTools(s, c)
	registerDraftTools(s, c)
	return s
}
