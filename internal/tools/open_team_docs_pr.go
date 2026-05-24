// MCP tool: open_team_docs_pr.
//
// Validates the spec, renders the team's docs/<section>/<team>/index.md,
// patches pt-ekklesia-docs/sidebars.js, and lands both files on
// branch team-docs/<team_key> via a deterministic transaction. Same
// idempotence semantics as open_team_pr: identical input + identical
// repo state returns action=noop. Two files = up to two commits per call.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/docs"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/sidebar"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// OpenTeamDocsPRInput is the input for open_team_docs_pr.
type OpenTeamDocsPRInput struct {
	Spec    any      `json:"spec" jsonschema:"the validated team spec to commit and PR"`
	Message string   `json:"message,omitempty" jsonschema:"optional commit/PR title override; defaults to 'Add docs for <team-key>'"`
	Branch  string   `json:"branch,omitempty" jsonschema:"optional branch name override; defaults to 'team-docs/<team-key>'"`
	Labels  []string `json:"labels,omitempty" jsonschema:"optional labels to apply to the PR"`
}

// OpenTeamDocsPROutput mirrors OpenTeamPROutput. CommitSHAs holds the
// per-file commit SHAs (index, then sidebars) when commits happened —
// either may be empty when that file was already up to date.
type OpenTeamDocsPROutput struct {
	PRURL        string   `json:"pr_url"`
	PRNumber     int      `json:"pr_number"`
	Branch       string   `json:"branch"`
	CommitSHAs   []string `json:"commit_shas,omitempty"`
	Action       string   `json:"action"` // created|updated|noop
	IndexPath    string   `json:"index_path"`
	SidebarsPath string   `json:"sidebars_path"`
}

const sidebarsPath = "sidebars.js"

// OpenTeamDocsPR registers the open_team_docs_pr tool. Like OpenTeamPR,
// when c is nil the tool registers but every call returns not_configured.
func OpenTeamDocsPR(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "open_team_docs_pr",
		Description: "Validate, render index.md, patch sidebars.js, and open-or-update a PR on osinfra-io/pt-ekklesia-docs for the given team spec. Idempotent. Requires GITHUB_TOKEN with write access to pt-ekklesia-docs.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Open team docs PR",
			ReadOnlyHint: false,
		},
		// Out is intentionally typed as `any` (not *OpenTeamDocsPROutput) so the
		// MCP go-sdk does not substitute a zero-value struct into
		// StructuredContent on error paths (server.go:362-369), which would
		// otherwise shadow our errResult body for clients that read structured
		// content. See issue #21.
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenTeamDocsPRInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return notConfigured("open_team_docs_pr"), nil, nil
		}
		specMap, err := coerceSpec(in.Spec)
		if err != nil {
			return errResult(opError{Code: "invalid_input", Message: err.Error()}), nil, nil
		}
		if errs := v.Validate(specMap); len(errs) > 0 {
			body, merr := json.Marshal(ValidateOutput{Valid: false, Errors: errs})
			if merr != nil {
				return nil, nil, fmt.Errorf("marshal validation errors: %w", merr)
			}
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil, nil
		}

		raw, err := json.Marshal(specMap)
		if err != nil {
			return nil, nil, fmt.Errorf("re-marshal spec: %w", err)
		}
		var team spec.Team
		if err := json.Unmarshal(raw, &team); err != nil {
			return nil, nil, fmt.Errorf("decode validated spec: %w", err)
		}

		indexRes, err := docs.Render(&team)
		if err != nil {
			return errResult(opError{Code: "docs_input_invalid", Message: err.Error()}), nil, nil
		}
		section, err := docs.SectionFor(team.TeamType)
		if err != nil {
			return errResult(opError{Code: "docs_input_invalid", Message: err.Error()}), nil, nil
		}
		folder, err := docs.TeamFolder(team.TeamKey)
		if err != nil {
			return errResult(opError{Code: "docs_input_invalid", Message: err.Error()}), nil, nil
		}

		branch := strings.TrimSpace(in.Branch)
		if branch == "" {
			branch = "team-docs/" + team.TeamKey
		}
		if branch == gh.Base {
			return errResult(opError{Code: "invalid_input", Message: "branch must not be the base branch (\"" + gh.Base + "\")"}), nil, nil
		}
		title := in.Message
		if title == "" {
			title = "Add docs for " + team.TeamKey
		}
		message := title + coAuthoredTrailer

		out, opErr := openTeamDocsPR(ctx, c, &team, indexRes, section, folder, branch, title, message)
		if opErr != nil {
			return errResult(*opErr), nil, nil
		}
		if len(in.Labels) > 0 && out.PRNumber > 0 {
			_ = c.AddLabelsInRepo(ctx, gh.RepoEkklesiaDocs, out.PRNumber, in.Labels)
		}
		return nil, out, nil
	})
}

func openTeamDocsPR(ctx context.Context, c gh.Client, team *spec.Team, indexRes *docs.Result, section, folder, branch, title, message string) (*OpenTeamDocsPROutput, *opError) {
	repo := gh.RepoEkklesiaDocs

	openPR, err := findOpenPRInRepo(ctx, c, repo, branch)
	if err != nil {
		return nil, apiError(err)
	}

	baseSHA, _, err := c.GetRefInRepo(ctx, repo, gh.Base)
	if err != nil {
		return nil, apiError(err)
	}
	if err := ensureBranchInRepo(ctx, c, repo, branch, baseSHA, openPR != nil); err != nil {
		return nil, err
	}

	// Sidebars: derive the patched bytes from the *branch* state so the
	// transaction stays consistent across retries; falling back to main
	// when the branch doesn't yet have a divergent copy.
	currentSidebars, sidebarsBlobSHA, sidebarsExists, err := c.GetFileInRepo(ctx, repo, sidebarsPath, branch)
	if err != nil {
		return nil, apiError(err)
	}
	branchHadSidebars := sidebarsExists
	if !sidebarsExists {
		// Branch doesn't have sidebars.js yet — fall back to main.
		currentSidebars, sidebarsBlobSHA, sidebarsExists, err = c.GetFileInRepo(ctx, repo, sidebarsPath, gh.Base)
		if err != nil {
			return nil, apiError(err)
		}
		if !sidebarsExists {
			return nil, &opError{Code: "source_parse_error", Message: "sidebars.js not found in " + repo + "@" + gh.Base}
		}
	}
	patchedSidebars, err := sidebar.Render(currentSidebars, section, folder, team.DisplayName)
	if err != nil {
		var anchorsErr *sidebar.ErrAnchorsMissing
		if errors.As(err, &anchorsErr) {
			return nil, &opError{Code: "source_parse_error", Message: err.Error()}
		}
		return nil, &opError{Code: "render_failed", Message: err.Error()}
	}

	// Index page: get current state on branch (may be absent).
	currentIndex, indexBlobSHA, _, err := c.GetFileInRepo(ctx, repo, indexRes.Path, branch)
	if err != nil {
		return nil, apiError(err)
	}

	// Compare main too — when the branch perfectly mirrors main and no
	// PR exists, the change is already shipped.
	mainIndex, _, _, err := c.GetFileInRepo(ctx, repo, indexRes.Path, gh.Base)
	if err != nil {
		return nil, apiError(err)
	}
	mainSidebars, _, _, err := c.GetFileInRepo(ctx, repo, sidebarsPath, gh.Base)
	if err != nil {
		return nil, apiError(err)
	}

	indexUpToDate := bytes.Equal(currentIndex, indexRes.Content)
	// If the branch didn't have sidebars.js, the file always needs committing
	// even when the patched content equals main (the branch lacks it).
	sidebarsUpToDate := branchHadSidebars && bytes.Equal(currentSidebars, patchedSidebars)

	if indexUpToDate && sidebarsUpToDate && openPR != nil {
		return &OpenTeamDocsPROutput{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, Action: "noop",
			IndexPath: indexRes.Path, SidebarsPath: sidebarsPath,
		}, nil
	}
	if openPR == nil &&
		bytes.Equal(mainIndex, indexRes.Content) &&
		bytes.Equal(mainSidebars, patchedSidebars) {
		return &OpenTeamDocsPROutput{
			Branch: branch, Action: "noop",
			IndexPath: indexRes.Path, SidebarsPath: sidebarsPath,
		}, nil
	}

	var commits []string

	if !indexUpToDate {
		sha, opErr := commitWithRetryInRepo(ctx, c, repo, indexRes.Path, branch, indexBlobSHA, indexRes.Content, message, openPR)
		if opErr != nil {
			return nil, opErr
		}
		if sha != "" {
			commits = append(commits, sha)
		}
	}
	if !sidebarsUpToDate {
		// Re-fetch the sidebars blob SHA after the index commit so the
		// next write doesn't 409 on a stale sha.
		_, freshSHA, _, gerr := c.GetFileInRepo(ctx, repo, sidebarsPath, branch)
		if gerr != nil {
			return nil, apiError(gerr)
		}
		sha, opErr := commitWithRetryInRepo(ctx, c, repo, sidebarsPath, branch, freshSHA, patchedSidebars, message, openPR)
		if opErr != nil {
			return nil, opErr
		}
		if sha != "" {
			commits = append(commits, sha)
		}
		_ = sidebarsBlobSHA // intentionally unused; freshSHA replaces it
	}

	if openPR != nil {
		return &OpenTeamDocsPROutput{
			PRURL: openPR.URL, PRNumber: openPR.Number,
			Branch: branch, CommitSHAs: commits, Action: "updated",
			IndexPath: indexRes.Path, SidebarsPath: sidebarsPath,
		}, nil
	}
	pr, action, opErr := createDocsPRWithReconcile(ctx, c, repo, branch, title, team)
	if opErr != nil {
		return nil, opErr
	}
	return &OpenTeamDocsPROutput{
		PRURL: pr.URL, PRNumber: pr.Number,
		Branch: branch, CommitSHAs: commits, Action: action,
		IndexPath: indexRes.Path, SidebarsPath: sidebarsPath,
	}, nil
}

// The remaining helpers mirror open_team_pr.go's findOpenPR /
// ensureBranch / commitWithRetry / createPRWithReconcile but accept a
// repo parameter and use the *InRepo client surface.

func findOpenPRInRepo(ctx context.Context, c gh.Client, repo, branch string) (*gh.PullRequest, error) {
	prs, err := c.ListOpenPRsInRepo(ctx, repo, branch, gh.Base)
	if err != nil {
		return nil, err
	}
	if len(prs) == 0 {
		return nil, nil
	}
	return &prs[0], nil
}

func ensureBranchInRepo(ctx context.Context, c gh.Client, repo, branch, baseSHA string, hasOpenPR bool) *opError {
	_, exists, err := c.GetRefInRepo(ctx, repo, branch)
	if err != nil {
		return apiError(err)
	}
	if !exists {
		if err := c.CreateRefInRepo(ctx, repo, branch, baseSHA); err != nil && !gh.IsConflict(err) {
			return apiError(err)
		}
		return nil
	}
	status, err := c.CompareCommitsInRepo(ctx, repo, gh.Base, branch)
	if err != nil {
		return apiError(err)
	}
	switch status {
	case gh.StatusIdentical, gh.StatusAhead:
		return nil
	case gh.StatusBehind:
		if err := c.UpdateRefInRepo(ctx, repo, branch, baseSHA, false); err != nil {
			return apiError(err)
		}
		return nil
	case gh.StatusDiverged:
		if hasOpenPR {
			return &opError{
				Code:    "branch_diverged",
				Message: "branch " + branch + " on " + repo + " has diverged from " + gh.Base + " and an open PR exists; rebase or close the PR before retrying",
			}
		}
		if err := c.DeleteRefInRepo(ctx, repo, branch); err != nil {
			return apiError(err)
		}
		if err := c.CreateRefInRepo(ctx, repo, branch, baseSHA); err != nil {
			return apiError(err)
		}
		return nil
	}
	return apiError(errors.New("unknown branch state"))
}

func commitWithRetryInRepo(ctx context.Context, c gh.Client, repo, path, branch, blobSHA string, rendered []byte, message string, openPR *gh.PullRequest) (string, *opError) {
	commitSHA, err := c.CreateOrUpdateFileInRepo(ctx, repo, path, branch, blobSHA, rendered, message)
	if err == nil {
		return commitSHA, nil
	}
	if !gh.IsConflict(err) {
		return "", apiError(err)
	}
	current, freshSHA, _, gerr := c.GetFileInRepo(ctx, repo, path, branch)
	if gerr != nil {
		return "", apiError(gerr)
	}
	if bytes.Equal(current, rendered) && openPR != nil {
		return "", nil
	}
	commitSHA, err = c.CreateOrUpdateFileInRepo(ctx, repo, path, branch, freshSHA, rendered, message)
	if err != nil {
		return "", &opError{Code: "github_conflict", Message: err.Error(), Retryable: true}
	}
	return commitSHA, nil
}

func createDocsPRWithReconcile(ctx context.Context, c gh.Client, repo, branch, title string, team *spec.Team) (gh.PullRequest, string, *opError) {
	body := docsPRBody(team)
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

func docsPRBody(t *spec.Team) string {
	return fmt.Sprintf(
		"Automated docs scaffold for team `%s` (`%s`).\n\n"+
			"- Adds `docs/<section>/<team>/index.md`\n"+
			"- Inserts the entry into `sidebars.js` (anchor-driven)\n\n"+
			"Generated by `pt-techne-mcp-server` `open_team_docs_pr`. The page is a stub — extend it with Context, Glossary, ADRs, etc., as that understanding lands.",
		t.TeamKey, t.DisplayName,
	)
}
