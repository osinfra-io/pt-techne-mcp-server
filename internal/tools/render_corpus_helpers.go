// MCP tool: render_corpus_helpers.
//
// Fetches helpers.tofu from osinfra-io/pt-corpus@main, inserts the
// team's main-production workspace into the logos_workspaces list, and
// returns the canonical updated bytes. Idempotent: returns the input
// bytes unchanged when the workspace is already present.
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/helpersrender"
)

// RenderCorpusHelpersInput is the input for render_corpus_helpers.
type RenderCorpusHelpersInput struct {
	TeamKey string `json:"team_key" jsonschema:"the team key, e.g. 'pt-arche'. Must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"`
}

// RenderCorpusHelpersOutput is the structured result.
type RenderCorpusHelpersOutput struct {
	HelpersTofu string `json:"helpers_tofu"`
}

// RenderCorpusHelpers registers the render_corpus_helpers tool.
// Requires GITHUB_TOKEN with read access to osinfra-io/pt-corpus.
func RenderCorpusHelpers(s *mcp.Server, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_corpus_helpers",
		Description: "Fetch helpers.tofu from osinfra-io/pt-corpus@main and return canonical bytes with '<team_key>-main-production' inserted into logos_workspaces. Idempotent: returns the input bytes unchanged when the workspace is already present. Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Render corpus helpers",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderCorpusHelpersInput) (*mcp.CallToolResult, any, error) {
		return renderHelpersTool(ctx, c, "render_corpus_helpers", "pt-corpus", in.TeamKey, func(b []byte) any {
			return &RenderCorpusHelpersOutput{HelpersTofu: string(b)}
		})
	})
}

// renderHelpersTool is the shared body for render_corpus_helpers and
// render_pneuma_helpers. The wrap callback keeps each tool's concrete
// output struct intact for clients while the handler's Out type stays
// `any` — see open_team_docs_pr.go (issue #21) for why.
func renderHelpersTool(ctx context.Context, c gh.Client, toolName, repo, teamKey string, wrap func([]byte) any) (*mcp.CallToolResult, any, error) {
	if c == nil {
		return notConfigured(toolName), nil, nil
	}
	if !teamKeyRe.MatchString(teamKey) {
		return errResult(opError{Code: "invalid_input", Message: "team_key must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"}), nil, nil
	}
	body, _, exists, err := c.GetFileInRepo(ctx, repo, "helpers.tofu", gh.Base)
	if err != nil {
		return errResult(*apiError(fmt.Errorf("%s/helpers.tofu@%s: %w", repo, gh.Base, err))), nil, nil
	}
	if !exists {
		return errResult(opError{Code: "source_parse_error", Message: "helpers.tofu missing at " + repo + "@" + gh.Base}), nil, nil
	}
	out, rerr := helpersrender.Render(body, teamKey+"-main-production")
	if rerr != nil {
		return errResult(opError{Code: "source_parse_error", Message: repo + "/helpers.tofu: " + rerr.Error()}), nil, nil
	}
	return nil, wrap(out), nil
}
