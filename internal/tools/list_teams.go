// MCP tool: list_teams.
//
// Returns one summary row per team in pt-logos@main. Counts are derived
// from the parsed spec so a parse/validate failure on any team surfaces
// as a source_parse_error rather than producing a partial listing.
package tools

import (
	"context"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// ListTeamsInput is intentionally empty.
type ListTeamsInput struct{}

// TeamSummary is one row in ListTeamsOutput.
type TeamSummary struct {
	TeamKey     string `json:"team_key"`
	DisplayName string `json:"display_name"`
	TeamType    string `json:"team_type"`
	// MemberCount is the unique GitHub-username count across the parent
	// team's maintainers and members. Child teams are not included so
	// the number reflects "how many humans on the team itself".
	MemberCount int `json:"member_count"`
	RepoCount   int `json:"repo_count"`
	EnvCount    int `json:"env_count"`
}

// ListTeamsOutput is the structured result of list_teams.
type ListTeamsOutput struct {
	Teams []TeamSummary `json:"teams"`
}

// ListTeams registers the list_teams tool. Requires GITHUB_TOKEN.
func ListTeams(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_teams",
		Description: "List every team defined under teams/ in osinfra-io/pt-logos@main, with one summary row per team. Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List teams",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListTeamsInput) (*mcp.CallToolResult, *ListTeamsOutput, error) {
		if c == nil {
			return notConfigured("list_teams"), nil, nil
		}
		ref, oe := resolveBaseRef(ctx, c)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		keys, oe := listTeamFiles(ctx, c, ref)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		teams, oe := fetchAllTeams(ctx, c, v, keys, ref)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		out := &ListTeamsOutput{Teams: make([]TeamSummary, 0, len(teams))}
		for _, t := range teams {
			out.Teams = append(out.Teams, TeamSummary{
				TeamKey:     t.TeamKey,
				DisplayName: t.DisplayName,
				TeamType:    t.TeamType,
				MemberCount: uniqueCount(t.GitHubParentTeamMemberships.Maintainers, t.GitHubParentTeamMemberships.Members),
				RepoCount:   len(t.GitHubRepositories),
				EnvCount:    envCount(t),
			})
		}
		sort.Slice(out.Teams, func(i, j int) bool { return out.Teams[i].TeamKey < out.Teams[j].TeamKey })
		return nil, out, nil
	})
}

// uniqueCount counts the union of multiple lists, normalizing case so
// usernames and emails dedupe regardless of inconsistent casing.
func uniqueCount(lists ...[]string) int {
	seen := make(map[string]struct{})
	for _, l := range lists {
		for _, s := range l {
			seen[strings.ToLower(s)] = struct{}{}
		}
	}
	return len(seen)
}

func envCount(t *spec.Team) int {
	n := 0
	for _, r := range t.GitHubRepositories {
		n += len(r.Environments)
	}
	return n
}
