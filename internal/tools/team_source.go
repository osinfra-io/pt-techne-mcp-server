// Reading teams/*.tfvars from osinfra-io/pt-logos@main.
//
// The four read tools (list_teams, get_team, lookup_user, find_repo)
// share the same fetch shape: pin pt-logos main to a commit SHA so
// every read in the same request sees the same revision, list the
// teams/ directory, fan out a bounded number of GetFile calls, parse,
// validate. This file owns that shape; tools just call into it.
package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

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
			Message: teamsDir + "/ directory missing at " + ref,
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
	generic, err := asJSON(team)
	if err != nil {
		return nil, &opError{Code: "source_parse_error", Message: path + ": " + err.Error()}
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
