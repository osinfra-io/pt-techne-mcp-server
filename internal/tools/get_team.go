// MCP tool: get_team.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/docs"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// GetTeamInput is the input for get_team.
type GetTeamInput struct {
	TeamKey string `json:"team_key" jsonschema:"the team key, e.g. 'pt-arche' (no .tfvars suffix)"`
}

// GetTeamOutput returns the parsed spec as a JSON object — the same
// shape validate_team_spec and render_team_tfvars accept. DocsPages
// lists the team's existing docs pages on pt-ekklesia-docs@main (excluding
// index.md). Spec is omitempty so the typed-output schema validator does
// not reject the null body that accompanies an isError result.
type GetTeamOutput struct {
	Spec      map[string]any `json:"spec,omitempty"`
	DocsPages []string       `json:"docs_pages,omitempty"`
}

// GetTeam registers the get_team tool. Requires GITHUB_TOKEN.
func GetTeam(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_team",
		Description: "Fetch one team's parsed spec from teams/<team_key>.tfvars in osinfra-io/pt-logos@main. Returns {spec: <object>} in the same JSON shape validate_team_spec and render_team_tfvars accept. Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get team",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetTeamInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return notConfigured("get_team"), nil, nil
		}
		if in.TeamKey == "" {
			return errResult(opError{Code: "invalid_input", Message: "team_key is required"}), nil, nil
		}
		ref, oe := resolveBaseRef(ctx, c)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		team, oe := fetchTeam(ctx, c, v, in.TeamKey, ref)
		if oe != nil {
			if oe.Code == "not_found" {
				return notFound("team " + in.TeamKey + " not found"), nil, nil
			}
			return errResult(*oe), nil, nil
		}
		obj, err := asJSON(team)
		if err != nil {
			return nil, nil, err
		}
		// docs_pages: list the team's existing pages on pt-ekklesia-docs@main,
		// excluding the auto-rendered index.md. Best-effort: any error
		// (including unknown team_type or unprefixed team_key) leaves the
		// field empty so the primary spec response still ships.
		var pages []string
		if section, derr := docs.SectionFor(team.TeamType); derr == nil {
			if folder, derr := docs.TeamFolder(team.TeamKey); derr == nil {
				dir := "docs/" + section + "/" + folder
				names, exists, lerr := c.ListDirInRepo(ctx, gh.RepoEkklesiaDocs, dir, gh.Base)
				if lerr == nil && exists {
					for _, n := range names {
						if n == "index.md" {
							continue
						}
						pages = append(pages, n)
					}
				}
			}
		}
		return nil, &GetTeamOutput{Spec: obj, DocsPages: pages}, nil
	})
}
