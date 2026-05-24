// MCP tool: render_pneuma_helpers.
//
// Fetches helpers.tofu from osinfra-io/pt-pneuma@main, inserts the
// team's main-production workspace into the logos_workspaces list, and
// returns the canonical updated bytes. Idempotent: returns the input
// bytes unchanged when the workspace is already present.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
)

// RenderPneumaHelpersInput is the input for render_pneuma_helpers.
type RenderPneumaHelpersInput struct {
	TeamKey string `json:"team_key" jsonschema:"the team key, e.g. 'pt-arche'. Must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"`
}

// RenderPneumaHelpersOutput is the structured result.
type RenderPneumaHelpersOutput struct {
	HelpersTofu string `json:"helpers_tofu"`
}

// RenderPneumaHelpers registers the render_pneuma_helpers tool.
// Requires GITHUB_TOKEN with read access to osinfra-io/pt-pneuma.
func RenderPneumaHelpers(s *mcp.Server, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_pneuma_helpers",
		Description: "Fetch helpers.tofu from osinfra-io/pt-pneuma@main and return canonical bytes with '<team_key>-main-production' inserted into logos_workspaces. Idempotent: returns the input bytes unchanged when the workspace is already present. Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Render pneuma helpers",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderPneumaHelpersInput) (*mcp.CallToolResult, any, error) {
		return renderHelpersTool(ctx, c, "render_pneuma_helpers", "pt-pneuma", in.TeamKey, func(b []byte) any {
			return &RenderPneumaHelpersOutput{HelpersTofu: string(b)}
		})
	})
}
