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
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// OpenTeamPRInput is the input for open_team_pr.
type OpenTeamPRInput struct {
	Spec    map[string]any `json:"spec" jsonschema:"the validated team spec to commit and PR"`
	Message string         `json:"message,omitempty" jsonschema:"optional commit/PR title override; defaults to 'Update teams/<team-key>.tfvars'"`
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
		Description: "Validate, render, and open-or-update a PR on osinfra-io/pt-logos for the given team spec. Idempotent: returns action=noop when the rendered tfvars already match the branch (with an open PR) or main. Requires GITHUB_TOKEN to be configured at server startup.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenTeamPRInput) (*mcp.CallToolResult, *OpenTeamPROutput, error) {
		if c == nil {
			return errResult(opError{
				Code:    "not_configured",
				Message: "open_team_pr requires GITHUB_TOKEN; see README Configuration",
			}), nil, nil
		}
		if errs := v.Validate(in.Spec); len(errs) > 0 {
			body, _ := json.Marshal(ValidateOutput{Valid: false, Errors: errs})
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
			}, nil, nil
		}

		raw, err := json.Marshal(in.Spec)
		if err != nil {
			return nil, nil, fmt.Errorf("re-marshal spec: %w", err)
		}
		var team spec.Team
		if err := json.Unmarshal(raw, &team); err != nil {
			return nil, nil, fmt.Errorf("decode validated spec: %w", err)
		}
		rendered, err := render.Render(&team)
		if err != nil {
			return nil, nil, fmt.Errorf("render team: %w", err)
		}

		branch := "team/" + team.TeamKey
		path := "teams/" + team.TeamKey + ".tfvars"
		message := in.Message
		if message == "" {
			message = "Update " + path
		}
		message += coAuthoredTrailer

		out, opErr := openTeamPR(ctx, c, &team, rendered, branch, path, message)
		if opErr != nil {
			return errResult(*opErr), nil, nil
		}
		return nil, out, nil
	})
}

// openTeamPR is the transaction. Returns either a result or a structured
// op error; never both, never an internal error (caller wraps real Go
// errors as github_api_error).
func openTeamPR(ctx context.Context, c gh.Client, team *spec.Team, rendered []byte, branch, path, message string) (*OpenTeamPROutput, *opError) {
	openPR, err := findOpenPR(ctx, c, branch)
	if err != nil {
		return nil, apiError(err)
	}

	baseSHA, _, err := c.GetRef(ctx, gh.Base)
	if err != nil {
		return nil, apiError(err)
	}

	if err := ensureBranch(ctx, c, branch, baseSHA, openPR != nil); err != nil {
		return nil, err
	}

	branchBytes, blobSHA, _, err := c.GetFile(ctx, path, branch)
	if err != nil {
		return nil, apiError(err)
	}
	mainBytes, _, _, err := c.GetFile(ctx, path, gh.Base)
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

	commitSHA, opErr := commitWithRetry(ctx, c, path, branch, blobSHA, rendered, message, openPR)
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
	pr, action, opErr := createPRWithReconcile(ctx, c, branch, team)
	if opErr != nil {
		return nil, opErr
	}
	return &OpenTeamPROutput{
		PRURL: pr.URL, PRNumber: pr.Number,
		Branch: branch, CommitSHA: commitSHA, Action: action,
	}, nil
}

// findOpenPR returns the open PR for branch->main, or nil if none exists.
func findOpenPR(ctx context.Context, c gh.Client, branch string) (*gh.PullRequest, error) {
	prs, err := c.ListOpenPRs(ctx, branch, gh.Base)
	if err != nil {
		return nil, err
	}
	if len(prs) == 0 {
		return nil, nil
	}
	return &prs[0], nil
}

// ensureBranch resolves the four branch states. hasOpenPR controls the
// disposable-branch reset path: a diverged branch with no open PR is
// safe to recreate from main; with an open PR it requires human action.
func ensureBranch(ctx context.Context, c gh.Client, branch, baseSHA string, hasOpenPR bool) *opError {
	_, exists, err := c.GetRef(ctx, branch)
	if err != nil {
		return apiError(err)
	}
	if !exists {
		if err := c.CreateRef(ctx, branch, baseSHA); err != nil && !gh.IsConflict(err) {
			return apiError(err)
		}
		return nil
	}
	status, err := c.CompareCommits(ctx, gh.Base, branch)
	if err != nil {
		return apiError(err)
	}
	switch status {
	case gh.StatusIdentical, gh.StatusAhead:
		return nil
	case gh.StatusBehind:
		if err := c.UpdateRef(ctx, branch, baseSHA, false); err != nil {
			return apiError(err)
		}
		return nil
	case gh.StatusDiverged:
		if hasOpenPR {
			return &opError{
				Code:    "branch_diverged",
				Message: "branch " + branch + " has diverged from " + gh.Base + " and an open PR exists; rebase or close the PR before retrying",
			}
		}
		if err := c.DeleteRef(ctx, branch); err != nil {
			return apiError(err)
		}
		if err := c.CreateRef(ctx, branch, baseSHA); err != nil {
			return apiError(err)
		}
		return nil
	}
	return apiError(errors.New("unknown branch state"))
}

// commitWithRetry handles 409/422 reconciliation. Returns ("", nil) when
// a reconcile reveals the branch already matches (caller turns this into
// a noop).
func commitWithRetry(ctx context.Context, c gh.Client, path, branch, blobSHA string, rendered []byte, message string, openPR *gh.PullRequest) (string, *opError) {
	commitSHA, err := c.CreateOrUpdateFile(ctx, path, branch, blobSHA, rendered, message)
	if err == nil {
		return commitSHA, nil
	}
	if !gh.IsConflict(err) {
		return "", apiError(err)
	}
	current, freshSHA, _, gerr := c.GetFile(ctx, path, branch)
	if gerr != nil {
		return "", apiError(gerr)
	}
	if bytes.Equal(current, rendered) && openPR != nil {
		return "", nil
	}
	commitSHA, err = c.CreateOrUpdateFile(ctx, path, branch, freshSHA, rendered, message)
	if err != nil {
		return "", &opError{Code: "github_conflict", Message: err.Error(), Retryable: true}
	}
	return commitSHA, nil
}

// createPRWithReconcile opens a new PR; if a 422 race shows a PR now
// exists, returns it as "updated".
func createPRWithReconcile(ctx context.Context, c gh.Client, branch string, team *spec.Team) (gh.PullRequest, string, *opError) {
	title := "Update teams/" + team.TeamKey + ".tfvars"
	body := prBody(team)
	pr, err := c.CreatePR(ctx, branch, gh.Base, title, body)
	if err == nil {
		return pr, "created", nil
	}
	if !gh.IsConflict(err) {
		return gh.PullRequest{}, "", apiError(err)
	}
	prs, lerr := c.ListOpenPRs(ctx, branch, gh.Base)
	if lerr != nil {
		return gh.PullRequest{}, "", apiError(lerr)
	}
	if len(prs) > 0 {
		return prs[0], "updated", nil
	}
	return gh.PullRequest{}, "", apiError(err)
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
	body, _ := json.Marshal(e)
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}
}

func apiError(err error) *opError {
	return &opError{Code: "github_api_error", Message: err.Error(), Retryable: true}
}
