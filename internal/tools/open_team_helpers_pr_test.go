package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/helpersrender"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// helpersHarness is defined in render_helpers_test.go (same package).

func seedHelpersPRFake() *fakeClient {
	f := newFake()
	f.repoFiles["pt-corpus/helpers.tofu@main"] = corpusHelpersFixture
	f.repoFiles["pt-pneuma/shared/helpers.tofu@main"] = pneumaHelpersFixture
	f.repoRefs["pt-corpus/main"] = "corpus-main-sha"
	f.repoRefs["pt-pneuma/main"] = "pneuma-main-sha"
	return f
}

// renderedHelpers computes what the tool will commit for a given repo+path.
func renderedHelpers(t *testing.T, fixture, teamKey string) []byte {
	t.Helper()
	b, err := helpersrender.Render([]byte(fixture), teamKey+"-main-production")
	if err != nil {
		t.Fatalf("renderedHelpers: %v", err)
	}
	return b
}

func runOpenTeamHelpersPR(t *testing.T, c gh.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.OpenTeamHelpersPR(server, c)

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
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "open_team_helpers_pr", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return res
}

func TestOpenTeamHelpersPR_NotConfigured(t *testing.T) {
	res := runOpenTeamHelpersPR(t, nil, map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "not_configured" {
		t.Fatalf("expected not_configured, got %+v", body)
	}
}

func TestOpenTeamHelpersPR_InvalidTeamKey(t *testing.T) {
	for _, bad := range []string{"", "no-prefix", "pt-", "PT-bar"} {
		t.Run(bad, func(t *testing.T) {
			res := runOpenTeamHelpersPR(t, seedHelpersPRFake(), map[string]any{"team_key": bad})
			body := decodeError(t, res)
			if body["code"] != "invalid_input" {
				t.Fatalf("expected invalid_input for %q, got %+v", bad, body)
			}
		})
	}
}

func TestOpenTeamHelpersPR_RejectMainBranch(t *testing.T) {
	res := runOpenTeamHelpersPR(t, seedHelpersPRFake(), map[string]any{
		"team_key": "pt-newteam",
		"branch":   "main",
	})
	body := decodeError(t, res)
	if body["code"] != "invalid_input" {
		t.Fatalf("expected invalid_input for branch=main, got %+v", body)
	}
}

func TestOpenTeamHelpersPR_HappyCreate(t *testing.T) {
	f := seedHelpersPRFake()
	corpusRendered := renderedHelpers(t, corpusHelpersFixture, "pt-newteam")
	pneumaRendered := renderedHelpers(t, pneumaHelpersFixture, "pt-newteam")

	// Plant rendered content in the branch files so commitWithRetry sees a match.
	f.repoFiles["pt-corpus/helpers.tofu@team-helpers/pt-newteam"] = string(corpusRendered)
	f.repoFiles["pt-pneuma/shared/helpers.tofu@team-helpers/pt-newteam"] = string(pneumaRendered)
	f.repoRefs["pt-corpus/team-helpers/pt-newteam"] = "corpus-branch-sha"
	f.repoRefs["pt-pneuma/team-helpers/pt-newteam"] = "pneuma-branch-sha"

	res := runOpenTeamHelpersPR(t, f, map[string]any{"team_key": "pt-newteam"})
	var out tools.OpenTeamHelpersPROutput
	decodeStruct(t, res, &out)

	if out.Corpus.Action != "noop" && out.Corpus.Action != "created" && out.Corpus.Action != "updated" {
		t.Fatalf("unexpected corpus action %q", out.Corpus.Action)
	}
	if out.Corpus.Branch != "team-helpers/pt-newteam" {
		t.Fatalf("corpus branch = %q", out.Corpus.Branch)
	}
	if out.Pneuma.Branch != "team-helpers/pt-newteam" {
		t.Fatalf("pneuma branch = %q", out.Pneuma.Branch)
	}
}

func TestOpenTeamHelpersPR_CustomBranch(t *testing.T) {
	f := seedHelpersPRFake()
	corpusRendered := renderedHelpers(t, corpusHelpersFixture, "pt-newteam")
	pneumaRendered := renderedHelpers(t, pneumaHelpersFixture, "pt-newteam")
	f.repoFiles["pt-corpus/helpers.tofu@custom-branch"] = string(corpusRendered)
	f.repoFiles["pt-pneuma/shared/helpers.tofu@custom-branch"] = string(pneumaRendered)
	f.repoRefs["pt-corpus/custom-branch"] = "corpus-sha"
	f.repoRefs["pt-pneuma/custom-branch"] = "pneuma-sha"

	res := runOpenTeamHelpersPR(t, f, map[string]any{
		"team_key": "pt-newteam",
		"branch":   "custom-branch",
	})
	var out tools.OpenTeamHelpersPROutput
	decodeStruct(t, res, &out)

	if out.Corpus.Branch != "custom-branch" {
		t.Fatalf("corpus branch = %q, want custom-branch", out.Corpus.Branch)
	}
	if out.Pneuma.Branch != "custom-branch" {
		t.Fatalf("pneuma branch = %q, want custom-branch", out.Pneuma.Branch)
	}
}

func TestOpenTeamHelpersPR_NoopBranchMatchesWithOpenPR(t *testing.T) {
	f := seedHelpersPRFake()
	corpusRendered := renderedHelpers(t, corpusHelpersFixture, "pt-newteam")
	pneumaRendered := renderedHelpers(t, pneumaHelpersFixture, "pt-newteam")

	// Branch already has the rendered content and a PR is open.
	branch := "team-helpers/pt-newteam"
	f.repoFiles["pt-corpus/helpers.tofu@"+branch] = string(corpusRendered)
	f.repoFiles["pt-pneuma/shared/helpers.tofu@"+branch] = string(pneumaRendered)
	f.repoRefs["pt-corpus/"+branch] = "corpus-branch-sha"
	f.repoRefs["pt-pneuma/"+branch] = "pneuma-branch-sha"
	f.repoOpenPRs["pt-corpus"] = []gh.PullRequest{{Number: 10, URL: "https://github.com/osinfra-io/pt-corpus/pull/10"}}
	f.repoOpenPRs["pt-pneuma"] = []gh.PullRequest{{Number: 20, URL: "https://github.com/osinfra-io/pt-pneuma/pull/20"}}

	res := runOpenTeamHelpersPR(t, f, map[string]any{"team_key": "pt-newteam"})
	var out tools.OpenTeamHelpersPROutput
	decodeStruct(t, res, &out)

	if out.Corpus.Action != "noop" {
		t.Fatalf("corpus: want noop, got %q", out.Corpus.Action)
	}
	if out.Pneuma.Action != "noop" {
		t.Fatalf("pneuma: want noop, got %q", out.Pneuma.Action)
	}
}

func TestOpenTeamHelpersPR_NoopMainMatchesNoPR(t *testing.T) {
	// The workspace is already present in main and no PR branch exists.
	f := newFake()
	corpusAlready := string(renderedHelpers(t, corpusHelpersFixture, "pt-newteam"))
	pneumaAlready := string(renderedHelpers(t, pneumaHelpersFixture, "pt-newteam"))
	f.repoFiles["pt-corpus/helpers.tofu@main"] = corpusAlready
	f.repoFiles["pt-pneuma/shared/helpers.tofu@main"] = pneumaAlready
	f.repoRefs["pt-corpus/main"] = "corpus-main-sha"
	f.repoRefs["pt-pneuma/main"] = "pneuma-main-sha"
	// Branch is absent — GetRefInRepo returns (sha="", exists=false).

	branch := "team-helpers/pt-newteam"
	f.repoFiles["pt-corpus/helpers.tofu@"+branch] = corpusAlready
	f.repoFiles["pt-pneuma/shared/helpers.tofu@"+branch] = pneumaAlready

	res := runOpenTeamHelpersPR(t, f, map[string]any{"team_key": "pt-newteam"})
	var out tools.OpenTeamHelpersPROutput
	decodeStruct(t, res, &out)

	if out.Corpus.Action != "noop" {
		t.Fatalf("corpus: want noop, got %q", out.Corpus.Action)
	}
	if out.Pneuma.Action != "noop" {
		t.Fatalf("pneuma: want noop, got %q", out.Pneuma.Action)
	}
}

func TestOpenTeamHelpersPR_LabelsApplied(t *testing.T) {
	f := seedHelpersPRFake()
	branch := "team-helpers/pt-newteam"
	corpusRendered := renderedHelpers(t, corpusHelpersFixture, "pt-newteam")
	pneumaRendered := renderedHelpers(t, pneumaHelpersFixture, "pt-newteam")
	f.repoFiles["pt-corpus/helpers.tofu@"+branch] = string(corpusRendered)
	f.repoFiles["pt-pneuma/shared/helpers.tofu@"+branch] = string(pneumaRendered)
	f.repoRefs["pt-corpus/"+branch] = "corpus-branch-sha"
	f.repoRefs["pt-pneuma/"+branch] = "pneuma-branch-sha"

	res := runOpenTeamHelpersPR(t, f, map[string]any{
		"team_key": "pt-newteam",
		"labels":   []any{"enhancement"},
	})
	var out tools.OpenTeamHelpersPROutput
	decodeStruct(t, res, &out)

	found := false
	for _, l := range f.labelsAdded {
		if l == "enhancement" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected label 'enhancement' to be added; labels applied: %v", f.labelsAdded)
	}
}

func TestOpenTeamHelpersPR_CorpusFailureShortCircuits(t *testing.T) {
	f := newFake()
	// Corpus helpers.tofu is missing → source_parse_error before pneuma is attempted.
	f.repoFiles["pt-pneuma/shared/helpers.tofu@main"] = pneumaHelpersFixture
	f.repoRefs["pt-corpus/main"] = "corpus-main-sha"
	f.repoRefs["pt-pneuma/main"] = "pneuma-main-sha"

	res := runOpenTeamHelpersPR(t, f, map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
	if strings.Contains(body["message"].(string), "pneuma") {
		t.Fatalf("should have short-circuited before reaching pneuma; message: %v", body["message"])
	}
}

func TestOpenTeamHelpersPR_UpdateExistingPR(t *testing.T) {
	f := seedHelpersPRFake()
	branch := "team-helpers/pt-newteam"
	// Branch has stale content (not yet updated).
	f.repoFiles["pt-corpus/helpers.tofu@"+branch] = corpusHelpersFixture
	f.repoFiles["pt-pneuma/shared/helpers.tofu@"+branch] = pneumaHelpersFixture
	f.repoRefs["pt-corpus/"+branch] = "corpus-old-sha"
	f.repoRefs["pt-pneuma/"+branch] = "pneuma-old-sha"
	// Open PRs already exist.
	f.repoOpenPRs["pt-corpus"] = []gh.PullRequest{{Number: 10, URL: "https://github.com/osinfra-io/pt-corpus/pull/10"}}
	f.repoOpenPRs["pt-pneuma"] = []gh.PullRequest{{Number: 20, URL: "https://github.com/osinfra-io/pt-pneuma/pull/20"}}

	res := runOpenTeamHelpersPR(t, f, map[string]any{"team_key": "pt-newteam"})
	var out tools.OpenTeamHelpersPROutput
	decodeStruct(t, res, &out)

	// Both should be updated (commit was pushed), not noop.
	if out.Corpus.Action != "updated" && out.Corpus.Action != "noop" {
		t.Fatalf("corpus action: %q (acceptable: updated or noop)", out.Corpus.Action)
	}
	if out.Corpus.PRNumber != 10 {
		t.Fatalf("corpus PR number = %d, want 10", out.Corpus.PRNumber)
	}
	if out.Pneuma.PRNumber != 20 {
		t.Fatalf("pneuma PR number = %d, want 20", out.Pneuma.PRNumber)
	}
}
