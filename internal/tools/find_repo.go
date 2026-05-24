// MCP tool: find_repo.
package tools

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// FindRepoInput is the input for find_repo.
type FindRepoInput struct {
	Name string `json:"name" jsonschema:"the github repository name to look up, e.g. 'pt-arche-core-helpers'"`
}

// FindRepoMatch is one team's match for a repo name.
type FindRepoMatch struct {
	TeamKey    string         `json:"team_key"`
	Repository map[string]any `json:"repository"`
}

// FindRepoOutput is the structured result of find_repo.
type FindRepoOutput struct {
	Matches []FindRepoMatch `json:"matches"`
}

// FindRepo registers the find_repo tool. Requires GITHUB_TOKEN.
func FindRepo(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "find_repo",
		Description: "Find which team(s) own a github repository. Walks every team's github_repositories block in osinfra-io/pt-logos@main and returns matches by exact repository name (case-sensitive, matching GitHub). Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Find repository owner",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in FindRepoInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return notConfigured("find_repo"), nil, nil
		}
		if in.Name == "" {
			return errResult(opError{Code: "invalid_input", Message: "name is required"}), nil, nil
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
		out := &FindRepoOutput{Matches: []FindRepoMatch{}}
		for _, t := range teams {
			repo, ok := t.GitHubRepositories[in.Name]
			if !ok {
				continue
			}
			obj, err := asJSON(repo)
			if err != nil {
				return errResult(opError{Code: "marshal_failed", Message: fmt.Sprintf("marshal repository %q: %s", in.Name, err)}), nil, nil
			}
			out.Matches = append(out.Matches, FindRepoMatch{TeamKey: t.TeamKey, Repository: obj})
		}
		sort.Slice(out.Matches, func(i, j int) bool { return out.Matches[i].TeamKey < out.Matches[j].TeamKey })
		return nil, out, nil
	})
}
