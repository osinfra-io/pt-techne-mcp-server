// pt-techne-mcp-server is the platform's MCP server. It exposes deterministic,
// typed tools that platform agents call instead of writing HCL by hand.
//
// Tools:
//   - validate_team_spec  — validate a team spec against schema/team.schema.json
//   - render_team_tfvars  — render a validated spec to canonical pt-logos tfvars
//   - open_team_pr        — open or update a PR on osinfra-io/pt-logos with the
//     rendered tfvars (requires GITHUB_TOKEN)
//   - list_teams          — summary index of every team in pt-logos@main (requires GITHUB_TOKEN)
//   - get_team            — parsed spec for one team (requires GITHUB_TOKEN)
//   - lookup_user         — every team and role a user appears in (requires GITHUB_TOKEN)
//   - find_repo           — which team owns a github repository (requires GITHUB_TOKEN)
//
// Transport is stdio only.
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// version is overwritten at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx := context.Background()

	v, err := spec.NewValidator()
	if err != nil {
		log.Fatalf("init validator: %v", err)
	}

	var ghClient gh.Client
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		ghClient = gh.New(ctx, token)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "pt-techne-mcp-server",
		Title:   "osinfra-io platform MCP server",
		Version: version,
	}, nil)

	tools.Validate(server, v)
	tools.Render(server, v)
	tools.OpenTeamPR(server, v, ghClient)
	tools.ListTeams(server, v, ghClient)
	tools.GetTeam(server, v, ghClient)
	tools.LookupUser(server, v, ghClient)
	tools.FindRepo(server, v, ghClient)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}
