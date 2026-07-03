// pt-techne-mcp-server is the platform's MCP server. It exposes deterministic,
// typed tools that platform agents call instead of writing HCL by hand.
//
// Tools:
//   - validate_team_spec      — validate a team spec against schema/team.schema.json
//   - render_team_tfvars      — render a validated spec to canonical pt-logos tfvars
//   - open_team_pr            — open or update a PR on osinfra-io/pt-logos with the
//     rendered tfvars (requires NOMOS_GITHUB_TOKEN)
//   - list_teams              — summary index of every team in pt-logos@main (requires NOMOS_GITHUB_TOKEN)
//   - get_team                — parsed spec + docs_pages for one team (requires NOMOS_GITHUB_TOKEN)
//   - lookup_user             — every team and role a user appears in (requires NOMOS_GITHUB_TOKEN)
//   - find_repo               — which team owns a github repository (requires NOMOS_GITHUB_TOKEN)
//   - next_available_cidrs    — compute next N unallocated GKE subnet CIDR slots (requires NOMOS_GITHUB_TOKEN)
//   - render_corpus_helpers   — insert a team's main-production workspace into
//     osinfra-io/pt-corpus/helpers.tofu (requires NOMOS_GITHUB_TOKEN)
//   - render_pneuma_helpers   — same for osinfra-io/pt-pneuma/helpers.tofu (requires NOMOS_GITHUB_TOKEN)
//   - render_team_docs_index  — render the team's docs/<section>/<team>/index.md page
//   - render_sidebar_patch    — patch a supplied pt-ekklesia-docs sidebars.js
//   - open_team_docs_pr       — open or update a PR on osinfra-io/pt-ekklesia-docs
//     with the rendered docs index + sidebars patch (requires NOMOS_GITHUB_TOKEN)
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
	if token := os.Getenv("NOMOS_GITHUB_TOKEN"); token != "" {
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
	tools.NextAvailableCidrs(server, v, ghClient)
	tools.RenderTeamHelpers(server, ghClient)
	tools.RenderTeamDocsIndex(server, v)
	tools.RenderTeamSidebarPatch(server, v)
	tools.OpenTeamDocsPR(server, v, ghClient)
	tools.OpenTeamHelpersPR(server, ghClient)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}
