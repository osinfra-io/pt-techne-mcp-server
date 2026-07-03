// MCP tool: render_team_helpers.
//
// Fetches helpers.tofu from both osinfra-io/pt-corpus and
// osinfra-io/pt-pneuma@main, inserts the team's main-production workspace
// into the logos_workspaces list in each, and returns the canonical
// updated bytes for both. Idempotent: returns input bytes unchanged when
// the workspace is already present.
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/helpersrender"
)

// RenderTeamHelpersInput is the input for render_team_helpers.
type RenderTeamHelpersInput struct {
	TeamKey string `json:"team_key" jsonschema:"the team key, e.g. 'pt-arche'. Must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"`
}

// RenderTeamHelpersOutput is the structured result.
type RenderTeamHelpersOutput struct {
	Corpus RenderTeamHelpersResult `json:"corpus"`
	Pneuma RenderTeamHelpersResult `json:"pneuma"`
}

// RenderTeamHelpersResult holds the rendered helpers.tofu for one repo.
type RenderTeamHelpersResult struct {
	HelpersTofu string `json:"helpers_tofu"`
}

// RenderTeamHelpers registers the render_team_helpers tool.
// Requires NOMOS_GITHUB_TOKEN with read access to osinfra-io/pt-corpus and
// osinfra-io/pt-pneuma.
func RenderTeamHelpers(s *mcp.Server, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_team_helpers",
		Description: "Fetch helpers.tofu from both osinfra-io/pt-corpus and osinfra-io/pt-pneuma@main and return canonical bytes with '<team_key>-main-production' inserted into logos_workspaces in each. Idempotent: returns the input bytes unchanged when the workspace is already present. Requires NOMOS_GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Render team helpers",
			ReadOnlyHint: true,
		},
		// Out is intentionally typed as `any` (not *RenderTeamHelpersOutput) so the
		// MCP go-sdk does not substitute a zero-value struct into
		// StructuredContent on error paths. See open_team_docs_pr.go (issue #21).
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderTeamHelpersInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return notConfigured("render_team_helpers"), nil, nil
		}
		if !teamKeyRe.MatchString(in.TeamKey) {
			return errResult(opError{Code: "invalid_input", Message: "team_key must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"}), nil, nil
		}
		workspace := in.TeamKey + "-main-production"

		corpusBytes, _, corpusExists, err := c.GetFileInRepo(ctx, gh.RepoCorpus, "helpers.tofu", gh.Base)
		if err != nil {
			return errResult(*apiError(fmt.Errorf("pt-corpus/helpers.tofu@%s: %w", gh.Base, err))), nil, nil
		}
		if !corpusExists {
			return errResult(opError{Code: "source_parse_error", Message: "helpers.tofu missing at pt-corpus@" + gh.Base}), nil, nil
		}
		corpusRendered, rerr := helpersrender.Render(corpusBytes, workspace)
		if rerr != nil {
			return errResult(opError{Code: "source_parse_error", Message: "pt-corpus/helpers.tofu: " + rerr.Error()}), nil, nil
		}

		pneumaBytes, _, pneumaExists, err := c.GetFileInRepo(ctx, gh.RepoPneuma, "shared/helpers.tofu", gh.Base)
		if err != nil {
			return errResult(*apiError(fmt.Errorf("pt-pneuma/shared/helpers.tofu@%s: %w", gh.Base, err))), nil, nil
		}
		if !pneumaExists {
			return errResult(opError{Code: "source_parse_error", Message: "shared/helpers.tofu missing at pt-pneuma@" + gh.Base}), nil, nil
		}
		pneumaRendered, rerr := helpersrender.Render(pneumaBytes, workspace)
		if rerr != nil {
			return errResult(opError{Code: "source_parse_error", Message: "pt-pneuma/shared/helpers.tofu: " + rerr.Error()}), nil, nil
		}

		return nil, &RenderTeamHelpersOutput{
			Corpus: RenderTeamHelpersResult{HelpersTofu: string(corpusRendered)},
			Pneuma: RenderTeamHelpersResult{HelpersTofu: string(pneumaRendered)},
		}, nil
	})
}
