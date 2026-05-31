// MCP tool: open_team_helpers_pr.
//
// Renders helpers.tofu for both osinfra-io/pt-corpus and
// osinfra-io/pt-pneuma, then performs a deterministic branch/commit/PR
// transaction against each. Idempotent on retry: identical input +
// identical repo state returns action=noop per repo. Fail-fast: if the
// corpus operation fails, pneuma is not attempted.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/helpersrender"
)

// OpenTeamHelpersPRInput is the input for open_team_helpers_pr.
type OpenTeamHelpersPRInput struct {
	TeamKey string   `json:"team_key" jsonschema:"the team key, e.g. 'pt-arche'. Must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"`
	Branch  string   `json:"branch,omitempty" jsonschema:"optional branch name override; defaults to 'team-helpers/<team-key>'"`
	Message string   `json:"message,omitempty" jsonschema:"optional commit/PR title override; defaults to 'Add <team-key>-main-production to pt-corpus and pt-pneuma helpers.tofu'"`
	Labels  []string `json:"labels,omitempty" jsonschema:"optional labels to apply to the PRs"`
}

// OpenTeamHelpersPROutput is the structured result of open_team_helpers_pr.
type OpenTeamHelpersPROutput struct {
	Corpus TeamHelpersPRResult `json:"corpus"`
	Pneuma TeamHelpersPRResult `json:"pneuma"`
}

// TeamHelpersPRResult holds the PR transaction result for one repo.
type TeamHelpersPRResult struct {
	PRURL     string `json:"pr_url"`
	PRNumber  int    `json:"pr_number"`
	Branch    string `json:"branch"`
	CommitSHA string `json:"commit_sha"`
	Action    string `json:"action"` // created|updated|noop
}

// OpenTeamHelpersPR registers the open_team_helpers_pr tool.
// Requires GITHUB_TOKEN with write access to osinfra-io/pt-corpus and
// osinfra-io/pt-pneuma.
func OpenTeamHelpersPR(s *mcp.Server, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "open_team_helpers_pr",
		Description: "Render helpers.tofu for both osinfra-io/pt-corpus and osinfra-io/pt-pneuma and open-or-update a PR in each with '<team_key>-main-production' inserted into logos_workspaces. Idempotent: returns action=noop per repo when the rendered content already matches the branch (with an open PR) or main. Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Open team helpers PR",
			ReadOnlyHint: false,
		},
		// Out is intentionally typed as `any` (not *OpenTeamHelpersPROutput).
		// See open_team_docs_pr.go (issue #21).
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenTeamHelpersPRInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return notConfigured("open_team_helpers_pr"), nil, nil
		}
		if !teamKeyRe.MatchString(in.TeamKey) {
			return errResult(opError{Code: "invalid_input", Message: "team_key must match ^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$"}), nil, nil
		}

		branch := strings.TrimSpace(in.Branch)
		if branch == "" {
			branch = "team-helpers/" + in.TeamKey
		}
		if branch == gh.Base {
			return errResult(opError{Code: "invalid_input", Message: "branch must not be the base branch (\"" + gh.Base + "\")"}), nil, nil
		}

		workspace := in.TeamKey + "-main-production"
		title := strings.TrimSpace(in.Message)
		if title == "" {
			title = "Add " + workspace + " to pt-corpus and pt-pneuma helpers.tofu"
		}
		message := title + coAuthoredTrailer

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

		corpusResult, opErr := openHelpersPRInRepo(ctx, c, gh.RepoCorpus, "helpers.tofu", branch, title, message, corpusRendered)
		if opErr != nil {
			return errResult(*opErr), nil, nil
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

		pneumaResult, opErr := openHelpersPRInRepo(ctx, c, gh.RepoPneuma, "shared/helpers.tofu", branch, title, message, pneumaRendered)
		if opErr != nil {
			return errResult(*opErr), nil, nil
		}

		out := &OpenTeamHelpersPROutput{
			Corpus: corpusResult,
			Pneuma: pneumaResult,
		}
		if len(in.Labels) > 0 {
			if corpusResult.PRNumber > 0 {
				_ = c.AddLabelsInRepo(ctx, gh.RepoCorpus, corpusResult.PRNumber, in.Labels)
			}
			if pneumaResult.PRNumber > 0 {
				_ = c.AddLabelsInRepo(ctx, gh.RepoPneuma, pneumaResult.PRNumber, in.Labels)
			}
		}
		return nil, out, nil
	})
}

func openHelpersPRInRepo(ctx context.Context, c gh.Client, repo, path, branch, title, message string, rendered []byte) (TeamHelpersPRResult, *opError) {
	openPR, err := findOpenPRInRepo(ctx, c, repo, branch)
	if err != nil {
		return TeamHelpersPRResult{}, apiError(err)
	}

	baseSHA, _, err := c.GetRefInRepo(ctx, repo, gh.Base)
	if err != nil {
		return TeamHelpersPRResult{}, apiError(err)
	}

	if opErr := ensureBranchInRepo(ctx, c, repo, branch, baseSHA, openPR != nil); opErr != nil {
		return TeamHelpersPRResult{}, opErr
	}

	branchBytes, blobSHA, _, err := c.GetFileInRepo(ctx, repo, path, branch)
	if err != nil {
		return TeamHelpersPRResult{}, apiError(err)
	}
	mainBytes, _, _, err := c.GetFileInRepo(ctx, repo, path, gh.Base)
	if err != nil {
		return TeamHelpersPRResult{}, apiError(err)
	}

	if bytes.Equal(branchBytes, rendered) && openPR != nil {
		return TeamHelpersPRResult{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, Action: "noop",
		}, nil
	}
	if bytes.Equal(mainBytes, rendered) && openPR == nil {
		return TeamHelpersPRResult{Branch: branch, Action: "noop"}, nil
	}

	commitSHA, opErr := commitWithRetryInRepo(ctx, c, repo, path, branch, blobSHA, rendered, message, openPR)
	if opErr != nil {
		return TeamHelpersPRResult{}, opErr
	}
	if commitSHA == "" {
		return TeamHelpersPRResult{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, Action: "noop",
		}, nil
	}

	if openPR != nil {
		return TeamHelpersPRResult{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, CommitSHA: commitSHA, Action: "updated",
		}, nil
	}

	pr, action, opErr := createHelpersPRInRepo(ctx, c, repo, branch, title)
	if opErr != nil {
		return TeamHelpersPRResult{}, opErr
	}
	return TeamHelpersPRResult{
		PRURL: pr.URL, PRNumber: pr.Number,
		Branch: branch, CommitSHA: commitSHA, Action: action,
	}, nil
}

func createHelpersPRInRepo(ctx context.Context, c gh.Client, repo, branch, title string) (gh.PullRequest, string, *opError) {
	body := helpersPRBody(repo)
	pr, err := c.CreatePRInRepo(ctx, repo, branch, gh.Base, title, body)
	if err == nil {
		return pr, "created", nil
	}
	if !gh.IsConflict(err) {
		return gh.PullRequest{}, "", apiError(err)
	}
	prs, lerr := c.ListOpenPRsInRepo(ctx, repo, branch, gh.Base)
	if lerr != nil {
		return gh.PullRequest{}, "", apiError(lerr)
	}
	if len(prs) > 0 {
		return prs[0], "updated", nil
	}
	return gh.PullRequest{}, "", apiError(err)
}

func helpersPRBody(repo string) string {
	return fmt.Sprintf(
		"Automated update for `%s/helpers.tofu`.\n\n"+
			"Inserts the team workspace into `logos_workspaces`.\n\n"+
			"Generated by `pt-techne-mcp-server` `open_team_helpers_pr`.",
		repo,
	)
}
