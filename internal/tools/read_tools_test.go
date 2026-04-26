package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// readToolHarness wires the read-side tools against a fake gh.Client
// and returns a function that runs a single tool call. Mirrors the
// roundTrip helper for write-side tools but isolates the fake state
// per test.
type readToolHarness struct {
	t  *testing.T
	cs *mcp.ClientSession
}

func newReadHarness(t *testing.T, c gh.Client) *readToolHarness {
	t.Helper()
	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.ListTeams(server, v, c)
	tools.GetTeam(server, v, c)
	tools.LookupUser(server, v, c)
	tools.FindRepo(server, v, c)

	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return &readToolHarness{t: t, cs: cs}
}

func (h *readToolHarness) call(name string, args any) *mcp.CallToolResult {
	h.t.Helper()
	res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

// seedFakeFromGoldens loads every golden tfvars file into the fake
// client at teams/<key>.tfvars@main, mirroring the production layout.
// Using the goldens means the parser exercises the same byte stream
// the production renderer produces.
func seedFakeFromGoldens(t *testing.T) *fakeClient {
	t.Helper()
	f := newFake()
	dir := filepath.Join("..", "render", "testdata", "golden")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read goldens: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".tfvars" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		f.files["teams/"+e.Name()+"@main-sha"] = string(body)
	}
	return f
}

func decodeStruct(t *testing.T, res *mcp.CallToolResult, into any) {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected IsError: %+v", res)
	}
	if err := json.Unmarshal(structuredOrText(t, res), into); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func decodeError(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	if !res.IsError {
		t.Fatalf("expected IsError, got success: %+v", res)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			var m map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &m); err != nil {
				t.Fatalf("unmarshal err body: %v", err)
			}
			return m
		}
	}
	t.Fatalf("no text content in error result")
	return nil
}

// --- not_configured ----------------------------------------------------

func TestReadTools_NotConfigured(t *testing.T) {
	h := newReadHarness(t, nil)

	for _, c := range []struct {
		name string
		args any
	}{
		{"list_teams", map[string]any{}},
		{"get_team", map[string]any{"team_key": "pt-arche"}},
		{"lookup_user", map[string]any{"github_username": "brettcurtis"}},
		{"find_repo", map[string]any{"name": "pt-arche-core-helpers"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := h.call(c.name, c.args)
			body := decodeError(t, res)
			if body["code"] != "not_configured" {
				t.Fatalf("expected not_configured, got %+v", body)
			}
		})
	}
}

// --- list_teams --------------------------------------------------------

func TestListTeams_Goldens(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)

	res := h.call("list_teams", map[string]any{})
	var out tools.ListTeamsOutput
	decodeStruct(t, res, &out)

	if len(out.Teams) != 8 {
		t.Fatalf("expected 8 teams, got %d: %+v", len(out.Teams), out.Teams)
	}
	for i := 1; i < len(out.Teams); i++ {
		if out.Teams[i-1].TeamKey >= out.Teams[i].TeamKey {
			t.Fatalf("teams not sorted: %s >= %s", out.Teams[i-1].TeamKey, out.Teams[i].TeamKey)
		}
	}
	// Spot-check pt-corpus has the expected counts.
	for _, s := range out.Teams {
		if s.TeamKey != "pt-corpus" {
			continue
		}
		if s.DisplayName != "Corpus" || s.TeamType != "platform-team" {
			t.Fatalf("pt-corpus identity unexpected: %+v", s)
		}
		if s.RepoCount != 2 || s.MemberCount != 1 || s.EnvCount != 9 {
			t.Fatalf("pt-corpus counts unexpected: %+v (want repos=2 members=1 envs=9)", s)
		}
	}
}

func TestListTeams_DirMissing(t *testing.T) {
	f := newFake() // no teams files seeded
	h := newReadHarness(t, f)
	res := h.call("list_teams", map[string]any{})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
}

// --- get_team ----------------------------------------------------------

func TestGetTeam_RoundTrip(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)
	res := h.call("get_team", map[string]any{"team_key": "pt-arche"})
	var out tools.GetTeamOutput
	decodeStruct(t, res, &out)

	if got := out.Spec["team_key"]; got != "pt-arche" {
		t.Fatalf("expected team_key=pt-arche, got %v", got)
	}
	// Re-validate the returned spec to prove the shape matches what
	// validate_team_spec expects.
	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	if errs := v.Validate(out.Spec); len(errs) > 0 {
		t.Fatalf("returned spec failed validation: %+v", errs)
	}
}

func TestGetTeam_NotFound(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)
	res := h.call("get_team", map[string]any{"team_key": "no-such-team"})
	body := decodeError(t, res)
	if body["code"] != "not_found" {
		t.Fatalf("expected not_found, got %+v", body)
	}
}

func TestGetTeam_SourceParseError(t *testing.T) {
	f := seedFakeFromGoldens(t)
	// Corrupt one team's tfvars with an unknown attribute. Strict
	// decoding in spec.Parse should reject it.
	f.files["teams/pt-arche.tfvars@main-sha"] = `teams = {
  pt-arche = {
    team_type = "platform-team"
    surprise  = "not-in-schema"
  }
}`
	h := newReadHarness(t, f)
	res := h.call("get_team", map[string]any{"team_key": "pt-arche"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
}

// --- lookup_user -------------------------------------------------------

func TestLookupUser_GitHubUsername_Goldens(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)

	// brettcurtis is the maintainer on every team's parent + child
	// teams. Expect at least one team-parent maintainer match per team
	// and several team-child maintainer matches.
	res := h.call("lookup_user", map[string]any{"github_username": "BrettCurtis"}) // case-insensitive
	var out tools.LookupUserOutput
	decodeStruct(t, res, &out)

	if len(out.Matches) == 0 {
		t.Fatalf("expected matches for brettcurtis across all teams, got 0")
	}
	parentSeen := map[string]bool{}
	for _, m := range out.Matches {
		if m.Via != "github_username" {
			t.Fatalf("expected via=github_username, got %+v", m)
		}
		if m.System != "github" {
			t.Fatalf("expected system=github, got %+v", m)
		}
		if m.Scope == "team-parent" && m.Membership == "maintainer" {
			parentSeen[m.TeamKey] = true
		}
	}
	if len(parentSeen) < 7 {
		t.Fatalf("expected brettcurtis as parent maintainer on most teams, got %d: %v", len(parentSeen), parentSeen)
	}
	// Sorted output.
	for i := 1; i < len(out.Matches); i++ {
		a, b := out.Matches[i-1], out.Matches[i]
		if a.TeamKey > b.TeamKey {
			t.Fatalf("matches not sorted by team_key: %+v then %+v", a, b)
		}
	}
}

func TestLookupUser_Email_Goldens(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)
	res := h.call("lookup_user", map[string]any{"email": "Brett@OSInfra.io"}) // case-insensitive
	var out tools.LookupUserOutput
	decodeStruct(t, res, &out)

	if len(out.Matches) == 0 {
		t.Fatalf("expected email matches, got 0")
	}
	systems := map[string]int{}
	for _, m := range out.Matches {
		if m.Via != "email" {
			t.Fatalf("expected via=email, got %+v", m)
		}
		systems[m.System]++
	}
	if systems["datadog"] == 0 || systems["google"] == 0 {
		t.Fatalf("expected matches in datadog and google, got %+v", systems)
	}
}

func TestLookupUser_NoMatch(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)
	res := h.call("lookup_user", map[string]any{"github_username": "no-such-user"})
	var out tools.LookupUserOutput
	decodeStruct(t, res, &out)
	if len(out.Matches) != 0 {
		t.Fatalf("expected no matches, got %+v", out.Matches)
	}
}

func TestLookupUser_InvalidInput(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)

	// Both empty.
	res := h.call("lookup_user", map[string]any{})
	body := decodeError(t, res)
	if body["code"] != "invalid_input" {
		t.Fatalf("expected invalid_input, got %+v", body)
	}
	// Both set.
	res = h.call("lookup_user", map[string]any{"github_username": "x", "email": "y@z"})
	body = decodeError(t, res)
	if body["code"] != "invalid_input" {
		t.Fatalf("expected invalid_input, got %+v", body)
	}
}

// --- find_repo ---------------------------------------------------------

func TestFindRepo_Hit(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)
	res := h.call("find_repo", map[string]any{"name": "pt-arche-core-helpers"})
	var out tools.FindRepoOutput
	decodeStruct(t, res, &out)

	if len(out.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %+v", len(out.Matches), out.Matches)
	}
	if out.Matches[0].TeamKey != "pt-arche" {
		t.Fatalf("expected team_key=pt-arche, got %s", out.Matches[0].TeamKey)
	}
	desc, _ := out.Matches[0].Repository["description"].(string)
	if !strings.Contains(desc, "helpers") {
		t.Fatalf("expected description containing 'helpers', got %q", desc)
	}
}

func TestFindRepo_Miss(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)
	res := h.call("find_repo", map[string]any{"name": "no-such-repo"})
	var out tools.FindRepoOutput
	decodeStruct(t, res, &out)
	if len(out.Matches) != 0 {
		t.Fatalf("expected no matches, got %+v", out.Matches)
	}
}
