// MCP tool: open_team_pr.
//
// Validates and renders the spec, then performs a deterministic
// branch/commit/PR transaction against osinfra-io/pt-logos. Idempotent
// on retry: identical input + identical repo state returns action=noop.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// OpenTeamPRInput is the input for open_team_pr.
type OpenTeamPRInput struct {
	Spec    any      `json:"spec" jsonschema:"the validated team spec to commit and PR"`
	Message string   `json:"message,omitempty" jsonschema:"optional commit/PR title override; defaults to 'Update teams/<team-key>.tfvars'"`
	Branch  string   `json:"branch,omitempty" jsonschema:"optional branch name override; defaults to 'team/<team-key>'"`
	Labels  []string `json:"labels,omitempty" jsonschema:"optional labels to apply to the PR"`
}

// OpenTeamPROutput is the structured result of open_team_pr.
type OpenTeamPROutput struct {
	PRURL     string `json:"pr_url"`
	PRNumber  int    `json:"pr_number"`
	Branch    string `json:"branch"`
	CommitSHA string `json:"commit_sha"`
	Action    string `json:"action"` // created|updated|noop
}

// opError is the structured body for operational failures (validation
// failures keep ValidateOutput's shape; see render_team_tfvars.go).
type opError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

const coAuthoredTrailer = "\n\nCo-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>\n"

// OpenTeamPR registers the open_team_pr tool. If c is nil, the tool
// still registers but every call returns a not_configured error so
// validate/render keep working without the GitHub token.
func OpenTeamPR(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "open_team_pr",
		Description: "Validate, render, and open-or-update a PR on osinfra-io/pt-logos for the given team spec. Idempotent: returns action=noop when the rendered tfvars already match the branch (with an open PR) or main. Requires NOMOS_GITHUB_TOKEN to be configured at server startup.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Open team PR",
			ReadOnlyHint: false,
		},
		// See open_team_docs_pr.go for why Out is `any` instead of *OpenTeamPROutput.
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenTeamPRInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return errResult(opError{
				Code:    "not_configured",
				Message: "open_team_pr requires NOMOS_GITHUB_TOKEN; see README Configuration",
			}), nil, nil
		}
		team, errRes, err := specToTeam(v, in.Spec)
		if errRes != nil || err != nil {
			return errRes, nil, err
		}
		rendered, err := render.Render(team)
		if err != nil {
			return nil, nil, fmt.Errorf("render team: %w", err)
		}

		branch := strings.TrimSpace(in.Branch)
		if branch == "" {
			branch = "team/" + team.TeamKey
		}
		if branch == gh.Base {
			return errResult(opError{Code: "invalid_input", Message: "branch must not be the base branch (\"" + gh.Base + "\")"}), nil, nil
		}
		path := "teams/" + team.TeamKey + ".tfvars"
		message := in.Message
		if message == "" {
			message = "Update " + path
		}
		message += coAuthoredTrailer

		out, opErr := openTeamPR(ctx, c, team, rendered, branch, path, message)
		if opErr != nil {
			return errResult(*opErr), nil, nil
		}
		if len(in.Labels) > 0 && out.PRNumber > 0 {
			_ = c.AddLabels(ctx, out.PRNumber, in.Labels)
		}
		return nil, out, nil
	})
}

// openTeamPR is the transaction. Returns either a result or a structured
// op error; never both, never an internal error (caller wraps real Go
// errors as github_api_error).
func openTeamPR(ctx context.Context, c gh.Client, team *spec.Team, rendered []byte, branch, path, message string) (*OpenTeamPROutput, *opError) {
	openPR, err := findOpenPRInRepo(ctx, c, gh.Repo, branch)
	if err != nil {
		return nil, apiError(err)
	}

	baseSHA, _, err := c.GetRefInRepo(ctx, gh.Repo, gh.Base)
	if err != nil {
		return nil, apiError(err)
	}

	if err := ensureBranchInRepo(ctx, c, gh.Repo, branch, baseSHA, openPR != nil); err != nil {
		return nil, err
	}

	branchBytes, blobSHA, _, err := c.GetFileInRepo(ctx, gh.Repo, path, branch)
	if err != nil {
		return nil, apiError(err)
	}
	mainBytes, _, _, err := c.GetFileInRepo(ctx, gh.Repo, path, gh.Base)
	if err != nil {
		return nil, apiError(err)
	}

	// Noop short-circuits — branch matches with an open PR, or main is
	// already shipped and no PR is needed.
	if bytes.Equal(branchBytes, rendered) && openPR != nil {
		return &OpenTeamPROutput{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, Action: "noop",
		}, nil
	}
	if bytes.Equal(mainBytes, rendered) && openPR == nil {
		return &OpenTeamPROutput{Branch: branch, Action: "noop"}, nil
	}

	commitSHA, opErr := commitWithRetryInRepo(ctx, c, gh.Repo, path, branch, blobSHA, rendered, message, openPR)
	if opErr != nil {
		return nil, opErr
	}
	if commitSHA == "" { // reconciled to noop during retry
		return &OpenTeamPROutput{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, Action: "noop",
		}, nil
	}

	if openPR != nil {
		return &OpenTeamPROutput{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, CommitSHA: commitSHA, Action: "updated",
		}, nil
	}
	pr, action, opErr := createPRWithReconcileInRepo(ctx, c, gh.Repo, branch, "Update teams/"+team.TeamKey+".tfvars", prBody(team))
	if opErr != nil {
		return nil, opErr
	}
	return &OpenTeamPROutput{
		PRURL: pr.URL, PRNumber: pr.Number,
		Branch: branch, CommitSHA: commitSHA, Action: action,
	}, nil
}

// prBody renders a deterministic high-level summary. The PR diff itself
// is the source of truth for what changed; this is just orientation.
func prBody(t *spec.Team) string {
	envCount := 0
	for _, r := range t.GitHubRepositories {
		envCount += len(r.Environments)
	}
	gh := t.GitHubParentTeamMemberships
	return fmt.Sprintf(
		"Automated update for team `%s` (`%s`).\n\n"+
			"- GitHub parent team: %d maintainers, %d members\n"+
			"- GitHub child teams: %d\n"+
			"- GitHub repositories: %d (%d environments total)\n"+
			"- Datadog: %d admins, %d members\n\n"+
			"Generated by `pt-techne-mcp-server` `open_team_pr`. Review the diff for the canonical change list.",
		t.TeamKey, t.DisplayName,
		len(gh.Maintainers), len(gh.Members),
		len(t.GitHubChildTeamsMemberships),
		len(t.GitHubRepositories), envCount,
		len(t.DatadogTeamMemberships.Admins), len(t.DatadogTeamMemberships.Members),
	)
}

// errResult wraps an opError as an MCP IsError result whose text body is
// the JSON-encoded structured error.
func errResult(e opError) *mcp.CallToolResult {
	body, err := json.Marshal(e)
	if err != nil {
		body = []byte(`{"code":"marshal_failed","message":"failed to encode error"}`)
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}
}

func apiError(err error) *opError {
	return &opError{Code: "github_api_error", Message: err.Error(), Retryable: true}
}
