// pt-techne-mcp-server is the platform's MCP server. It exposes deterministic,
// typed tools that platform agents call instead of writing HCL by hand.
//
// v0 tools:
//   - validate_team_spec  — validate a team spec against schema/team.schema.json
//   - render_team_tfvars  — render a validated spec to canonical pt-logos tfvars
//
// Transport is stdio only.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// version is overwritten at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	v, err := spec.NewValidator()
	if err != nil {
		log.Fatalf("init validator: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "pt-techne-mcp-server",
		Title:   "osinfra-io platform MCP server",
		Version: version,
	}, nil)

	tools.Validate(server, v)
	tools.Render(server, v)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}
