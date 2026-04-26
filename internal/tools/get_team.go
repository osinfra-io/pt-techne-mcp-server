// MCP tool: get_team.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// GetTeamInput is the input for get_team.
type GetTeamInput struct {
	TeamKey string `json:"team_key" jsonschema:"the team key, e.g. 'pt-arche' (no .tfvars suffix)"`
}

// GetTeamOutput returns the parsed spec as a JSON object — the same
// shape validate_team_spec and render_team_tfvars accept. Spec is
// omitempty so the typed-output schema validator does not reject the
// null body that accompanies an isError result.
type GetTeamOutput struct {
	Spec map[string]any `json:"spec,omitempty"`
}

// GetTeam registers the get_team tool. Requires GITHUB_TOKEN.
func GetTeam(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_team",
		Description: "Fetch one team's parsed spec from teams/<team_key>.tfvars in osinfra-io/pt-logos@main. Returns {spec: <object>} in the same JSON shape validate_team_spec and render_team_tfvars accept. Requires GITHUB_TOKEN.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetTeamInput) (*mcp.CallToolResult, *GetTeamOutput, error) {
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
		return nil, &GetTeamOutput{Spec: obj}, nil
	})
}
