// Shared helpers for the read tools (lookup_user, list_teams, get_team,
// find_repo). They all share the same shape: pin pt-logos main to a
// commit SHA so every read in the request sees the same revision, list
// teams/, fan out a bounded number of GetFile calls, parse, validate.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// teamsDir is the canonical directory under pt-logos that holds one
// tfvars file per team. Hard-coded to match the rest of the server.
const teamsDir = "teams"

// readFanoutLimit is the maximum number of in-flight GetFile calls
// against the GitHub API. Empirically a small number is plenty — the
// team count is in single digits — and keeps us well clear of secondary
// rate limits if the count grows.
const readFanoutLimit = 8

// notConfigured returns the standard not_configured opError result.
func notConfigured(toolName string) *mcp.CallToolResult {
	return errResult(opError{
		Code:    "not_configured",
		Message: toolName + " requires GITHUB_TOKEN; see README Configuration",
	})
}

// notFound returns a not_found opError result.
func notFound(message string) *mcp.CallToolResult {
	return errResult(opError{Code: "not_found", Message: message})
}

// resolveBaseRef pins pt-logos@main to a commit SHA so every subsequent
// read in the same request sees the same revision. Without this a
// concurrent merge to main could let ListDir and GetFile observe
// different commits and produce a partial or mixed answer.
func resolveBaseRef(ctx context.Context, c gh.Client) (string, *opError) {
	sha, exists, err := c.GetRef(ctx, gh.Base)
	if err != nil {
		return "", apiError(err)
	}
	if !exists {
		return "", &opError{
			Code:    "source_parse_error",
			Message: gh.Base + " branch not found on " + gh.Owner + "/" + gh.Repo,
		}
	}
	return sha, nil
}

// listTeamFiles lists every .tfvars file under teams/ at the given ref.
// Returned names are bare keys (no extension), sorted, deduplicated.
// Treats a missing teams/ directory as an error rather than as an empty
// list so layout drift surfaces instead of silently returning no
// matches.
func listTeamFiles(ctx context.Context, c gh.Client, ref string) ([]string, *opError) {
	names, exists, err := c.ListDir(ctx, teamsDir, ref)
	if err != nil {
		return nil, apiError(err)
	}
	if !exists {
		return nil, &opError{
			Code:    "source_parse_error",
			Message: teamsDir + "/ directory missing on " + gh.Base,
		}
	}
	keys := make([]string, 0, len(names))
	for _, n := range names {
		const ext = ".tfvars"
		if !strings.HasSuffix(n, ext) {
			continue
		}
		keys = append(keys, strings.TrimSuffix(n, ext))
	}
	sort.Strings(keys)
	return keys, nil
}

// fetchTeam fetches and parses a single team at the given ref.
// Validates against the schema as well so a corrupt source surfaces as
// source_parse_error rather than as malformed downstream output.
func fetchTeam(ctx context.Context, c gh.Client, v *spec.Validator, key, ref string) (*spec.Team, *opError) {
	path := teamsDir + "/" + key + ".tfvars"
	body, _, exists, err := c.GetFile(ctx, path, ref)
	if err != nil {
		return nil, apiError(err)
	}
	if !exists {
		return nil, &opError{Code: "not_found", Message: "team " + key + " not found"}
	}
	team, err := spec.Parse(body)
	if err != nil {
		return nil, &opError{Code: "source_parse_error", Message: path + ": " + err.Error()}
	}
	// Round-trip through JSON so the validator sees the same shape it
	// would for an agent-supplied spec.
	raw, err := json.Marshal(team)
	if err != nil {
		return nil, &opError{Code: "source_parse_error", Message: path + ": marshal: " + err.Error()}
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, &opError{Code: "source_parse_error", Message: path + ": remarshal: " + err.Error()}
	}
	if errs := v.Validate(generic); len(errs) > 0 {
		return nil, &opError{
			Code:    "source_parse_error",
			Message: fmt.Sprintf("%s: %d schema violation(s); first: %s: %s", path, len(errs), errs[0].Path, errs[0].Message),
		}
	}
	return team, nil
}

// fetchAllTeams fans out fetchTeam across every key with bounded
// concurrency. The returned slice is ordered by team_key so downstream
// output is deterministic regardless of which fetch finished first.
func fetchAllTeams(ctx context.Context, c gh.Client, v *spec.Validator, keys []string, ref string) ([]*spec.Team, *opError) {
	teams := make([]*spec.Team, len(keys))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(readFanoutLimit)
	var mu sync.Mutex
	var firstOpErr *opError
	for i, k := range keys {
		i, k := i, k
		g.Go(func() error {
			t, oe := fetchTeam(gctx, c, v, k, ref)
			if oe != nil {
				mu.Lock()
				if firstOpErr == nil {
					firstOpErr = oe
				}
				mu.Unlock()
				return fmt.Errorf("%s", oe.Message)
			}
			teams[i] = t
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		mu.Lock()
		oe := firstOpErr
		mu.Unlock()
		if oe != nil {
			return nil, oe
		}
		return nil, apiError(err)
	}
	return teams, nil
}

// asJSON marshals v to a JSON map for embedding in tool output. Used by
// get_team and find_repo to hand back spec/repository sub-objects in
// the same shape callers send to validate/render.
func asJSON(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
