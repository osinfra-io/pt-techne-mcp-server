// MCP tool: lookup_user.
//
// Walks every team's role-bearing fields and returns every place a
// given GitHub username or email appears. Matching is case-insensitive
// and exact (GitHub usernames are case-insensitive; emails by
// convention treat the local-part case-insensitively).
package tools

import (
	"context"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// LookupUserInput accepts exactly one of github_username or email.
type LookupUserInput struct {
	GitHubUsername string `json:"github_username,omitempty" jsonschema:"the github username to look up; mutually exclusive with email"`
	Email          string `json:"email,omitempty" jsonschema:"the email address to look up; mutually exclusive with github_username"`
}

// LookupUserMatch is one place a user appears in a team.
//
// The fields are structured rather than encoded into a single role
// string so callers can filter or display them without re-parsing.
//
//	system     — "datadog" | "github" | "google" | "gke"
//	scope      — "team"        (datadog parent team)
//	             "team-parent" | "team-child" (github)
//	             "basic" | "browser" | "project-creator" | "xpn-admin" (google)
//	             "artifact-registry" (gke)
//	subject    — for team-child: the child team key (e.g. "production-approvers")
//	             for env-scoped google groups: "sandbox" | "non-production" | "production"
//	             for google basic: "admin" | "reader" | "writer"
//	             for gke artifact-registry: "readers" | "writers"
//	             empty for github team-parent and datadog team
//	membership — datadog: "admin" | "member"
//	             github : "maintainer" | "member"
//	             google : "owner" | "manager" | "member"
type LookupUserMatch struct {
	TeamKey    string `json:"team_key"`
	Via        string `json:"via"` // "github_username" | "email"
	System     string `json:"system"`
	Scope      string `json:"scope"`
	Subject    string `json:"subject,omitempty"`
	Membership string `json:"membership"`
}

// LookupUserOutput is the structured result of lookup_user.
type LookupUserOutput struct {
	Matches []LookupUserMatch `json:"matches"`
}

// LookupUser registers the lookup_user tool. Requires NOMOS_GITHUB_TOKEN.
func LookupUser(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "lookup_user",
		Description: "Find every team and role where a user appears across all teams in osinfra-io/pt-logos@main. Provide exactly one of github_username (matched case-insensitively against GitHub roles) or email (matched case-insensitively against Datadog and Google group roles). Returns {matches: []} (possibly empty) — empty is success, not an error. Requires NOMOS_GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Look up user",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in LookupUserInput) (*mcp.CallToolResult, any, error) {
		if c == nil {
			return notConfigured("lookup_user"), nil, nil
		}
		// Trim whitespace so " " is treated as empty for the XOR check
		// rather than passing validation and triggering a full repo scan.
		in.GitHubUsername = strings.TrimSpace(in.GitHubUsername)
		in.Email = strings.TrimSpace(in.Email)
		if (in.GitHubUsername == "") == (in.Email == "") {
			return errResult(opError{
				Code:    "invalid_input",
				Message: "lookup_user requires exactly one of github_username or email",
			}), nil, nil
		}

		var via, target string
		if in.GitHubUsername != "" {
			via, target = "github_username", strings.ToLower(in.GitHubUsername)
		} else {
			via, target = "email", strings.ToLower(in.Email)
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

		out := &LookupUserOutput{Matches: []LookupUserMatch{}}
		for _, t := range teams {
			out.Matches = append(out.Matches, scanTeam(t, via, target)...)
		}
		dedupSortMatches(out.Matches)
		out.Matches = dedupMatches(out.Matches)
		return nil, out, nil
	})
}

// scanTeam returns every match in a single team, in deterministic
// order. The caller is responsible for sorting across teams.
func scanTeam(t *spec.Team, via, target string) []LookupUserMatch {
	var out []LookupUserMatch
	add := func(m LookupUserMatch) {
		m.TeamKey = t.TeamKey
		m.Via = via
		out = append(out, m)
	}

	if via == "email" {
		// Datadog
		out = append(out, scanDatadogMemberships(t.TeamKey, via, target,
			[][]string{t.DatadogTeamMemberships.Admins, t.DatadogTeamMemberships.Members},
			[]string{"admin", "member"}, "datadog", "team", "")...)

		// Google basic groups
		for _, g := range []struct {
			subject string
			grp     spec.GoogleGroup
		}{
			{"admin", t.GoogleBasicGroupsMemberships.Admin},
			{"reader", t.GoogleBasicGroupsMemberships.Reader},
			{"writer", t.GoogleBasicGroupsMemberships.Writer},
		} {
			for _, m := range matchGoogleGroup(g.grp, target) {
				add(LookupUserMatch{System: "google", Scope: "basic", Subject: g.subject, Membership: m})
			}
		}

		// Env-scoped google groups
		envScoped := []struct {
			scope string
			grp   *spec.EnvScopedGoogleGroups
		}{
			{"browser", t.GoogleBrowserGroups},
			{"project-creator", t.GoogleProjectCreatorGroups},
			{"xpn-admin", t.GoogleXPNAdminGroups},
		}
		for _, es := range envScoped {
			if es.grp == nil {
				continue
			}
			for _, env := range []struct {
				name string
				grp  spec.GoogleGroup
			}{
				{"sandbox", es.grp.Sandbox},
				{"non-production", es.grp.NonProduction},
				{"production", es.grp.Production},
			} {
				for _, m := range matchGoogleGroup(env.grp, target) {
					add(LookupUserMatch{System: "google", Scope: es.scope, Subject: env.name, Membership: m})
				}
			}
		}

		// GKE artifact-registry groups
		if t.PlatformManagedProject != nil &&
			t.PlatformManagedProject.KubernetesEngine != nil &&
			t.PlatformManagedProject.KubernetesEngine.ArtifactRegistryGroups != nil {
			ar := t.PlatformManagedProject.KubernetesEngine.ArtifactRegistryGroups
			for _, g := range []struct {
				subject string
				grp     spec.GoogleGroup
			}{
				{"readers", ar.Readers},
				{"writers", ar.Writers},
			} {
				for _, m := range matchGoogleGroup(g.grp, target) {
					add(LookupUserMatch{System: "gke", Scope: "artifact-registry", Subject: g.subject, Membership: m})
				}
			}
		}
		return out
	}

	// via == "github_username"
	for _, m := range matchGitHubMembership(t.GitHubParentTeamMemberships, target) {
		add(LookupUserMatch{System: "github", Scope: "team-parent", Membership: m})
	}
	// Stable iteration over child team keys.
	childKeys := make([]string, 0, len(t.GitHubChildTeamsMemberships))
	for k := range t.GitHubChildTeamsMemberships {
		childKeys = append(childKeys, k)
	}
	sort.Strings(childKeys)
	for _, k := range childKeys {
		for _, m := range matchGitHubMembership(t.GitHubChildTeamsMemberships[k], target) {
			add(LookupUserMatch{System: "github", Scope: "team-child", Subject: k, Membership: m})
		}
	}
	return out
}

func matchGitHubMembership(g spec.GitHubMembership, target string) []string {
	var out []string
	if containsCI(g.Maintainers, target) {
		out = append(out, "maintainer")
	}
	if containsCI(g.Members, target) {
		out = append(out, "member")
	}
	return out
}

func matchGoogleGroup(g spec.GoogleGroup, target string) []string {
	var out []string
	if containsCI(g.Owners, target) {
		out = append(out, "owner")
	}
	if containsCI(g.Managers, target) {
		out = append(out, "manager")
	}
	if containsCI(g.Members, target) {
		out = append(out, "member")
	}
	return out
}

func containsCI(xs []string, target string) bool {
	for _, x := range xs {
		if strings.EqualFold(x, target) {
			return true
		}
	}
	return false
}

// scanDatadogMemberships checks parallel lists of membership strings
// against a target (case-insensitive) and returns structured matches.
// Used for Datadog team membership where admins and members are email
// lists rather than GitHub usernames.
func scanDatadogMemberships(teamKey, via, target string, lists [][]string, labels []string, system, scope, subject string) []LookupUserMatch {
	var out []LookupUserMatch
	for i, list := range lists {
		if containsCI(list, target) {
			out = append(out, LookupUserMatch{
				TeamKey: teamKey, Via: via,
				System: system, Scope: scope, Subject: subject, Membership: labels[i],
			})
		}
	}
	return out
}

func dedupSortMatches(ms []LookupUserMatch) {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if a.TeamKey != b.TeamKey {
			return a.TeamKey < b.TeamKey
		}
		if a.System != b.System {
			return a.System < b.System
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		return a.Membership < b.Membership
	})
}

// dedupMatches drops exact duplicates (same team/system/scope/subject/
// membership). A user listed twice in the same membership list does
// not appear twice in the output.
func dedupMatches(ms []LookupUserMatch) []LookupUserMatch {
	out := ms[:0]
	var prev LookupUserMatch
	for i, m := range ms {
		if i > 0 && m == prev {
			continue
		}
		out = append(out, m)
		prev = m
	}
	return out
}
