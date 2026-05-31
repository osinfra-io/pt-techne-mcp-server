package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// helpersHarness wires the render_team_helpers tool against a fake gh.Client.
type helpersHarness struct {
	t  *testing.T
	cs *mcp.ClientSession
}

func newHelpersHarness(t *testing.T, c gh.Client) *helpersHarness {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.RenderTeamHelpers(server, c)

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
	return &helpersHarness{t: t, cs: cs}
}

func (h *helpersHarness) call(args any) *mcp.CallToolResult {
	h.t.Helper()
	res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "render_team_helpers", Arguments: args})
	if err != nil {
		h.t.Fatalf("CallTool render_team_helpers: %v", err)
	}
	return res
}

const corpusHelpersFixture = `module "core_helpers" {
  source = "github.com/osinfra-io/pt-arche-core-helpers//root?ref=70c1e4f4a11acdcfc36055bdd3aa16579d4203a1" # v0.3.1

  logos_workspaces = [
    "pt-corpus-main-production",
    "pt-logos-main-production",
    "pt-pneuma-main-production"
  ]

  repository = "pt-corpus"
  team       = "pt-corpus"
}
`

const pneumaHelpersFixture = `module "core_helpers" {
  logos_workspaces = [
    "pt-corpus-main-production",
    "pt-logos-main-production",
    "pt-pneuma-main-production",
  ]

  repository = "pt-pneuma"
  team       = "pt-pneuma"
}
`

func seedHelpersFake() *fakeClient {
	f := newFake()
	f.repoFiles["pt-corpus/helpers.tofu@main"] = corpusHelpersFixture
	f.repoFiles["pt-pneuma/shared/helpers.tofu@main"] = pneumaHelpersFixture
	return f
}

func TestRenderTeamHelpers_HappyPath(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())
	res := h.call(map[string]any{"team_key": "pt-newteam"})
	var out tools.RenderTeamHelpersOutput
	decodeStruct(t, res, &out)

	// Corpus: sorted insert position between pt-logos and pt-pneuma.
	wantCorpusOrder := []string{
		`"pt-corpus-main-production"`,
		`"pt-logos-main-production"`,
		`"pt-newteam-main-production"`,
		`"pt-pneuma-main-production"`,
	}
	prev := -1
	for _, s := range wantCorpusOrder {
		idx := strings.Index(out.Corpus.HelpersTofu, s)
		if idx < 0 {
			t.Fatalf("corpus: missing %s in output:\n%s", s, out.Corpus.HelpersTofu)
		}
		if idx <= prev {
			t.Fatalf("corpus: entries out of order; %s appeared before previous entry", s)
		}
		prev = idx
	}

	// Pneuma: inserted with trailing-comma style.
	if !strings.Contains(out.Pneuma.HelpersTofu, `    "pt-newteam-main-production",`) {
		t.Fatalf("pneuma: inserted line not found:\n%s", out.Pneuma.HelpersTofu)
	}
	if !strings.Contains(out.Pneuma.HelpersTofu, `    "pt-pneuma-main-production",`+"\n  ]") {
		t.Fatalf("pneuma: trailing-comma style not preserved:\n%s", out.Pneuma.HelpersTofu)
	}
}

func TestRenderTeamHelpers_IdempotentBoth(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())

	res := h.call(map[string]any{"team_key": "pt-logos"})
	var out tools.RenderTeamHelpersOutput
	decodeStruct(t, res, &out)
	if out.Corpus.HelpersTofu != corpusHelpersFixture {
		t.Fatalf("corpus noop must return byte-identical helpers.tofu\n--- got ---\n%s\n--- want ---\n%s",
			out.Corpus.HelpersTofu, corpusHelpersFixture)
	}

	// pt-pneuma is already present in pneumaHelpersFixture.
	res = h.call(map[string]any{"team_key": "pt-pneuma"})
	var out2 tools.RenderTeamHelpersOutput
	decodeStruct(t, res, &out2)
	if out2.Pneuma.HelpersTofu != pneumaHelpersFixture {
		t.Fatalf("pneuma noop must return byte-identical helpers.tofu\n--- got ---\n%s\n--- want ---\n%s",
			out2.Pneuma.HelpersTofu, pneumaHelpersFixture)
	}
}

func TestRenderTeamHelpers_NotConfigured(t *testing.T) {
	h := newHelpersHarness(t, nil)
	res := h.call(map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "not_configured" {
		t.Fatalf("expected not_configured, got %+v", body)
	}
}

func TestRenderTeamHelpers_InvalidInput(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())
	for _, bad := range []string{"", "no-prefix", "pt-", "pt-Foo", "PT-bar"} {
		t.Run(bad, func(t *testing.T) {
			res := h.call(map[string]any{"team_key": bad})
			body := decodeError(t, res)
			if body["code"] != "invalid_input" {
				t.Fatalf("expected invalid_input for %q, got %+v", bad, body)
			}
		})
	}
}

func TestRenderTeamHelpers_CorpusFileMissing(t *testing.T) {
	f := newFake()
	// Only seed pneuma; corpus is absent.
	f.repoFiles["pt-pneuma/shared/helpers.tofu@main"] = pneumaHelpersFixture
	h := newHelpersHarness(t, f)
	res := h.call(map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
	if !strings.Contains(body["message"].(string), "missing") {
		t.Fatalf("expected 'missing' in message, got %v", body["message"])
	}
}

func TestRenderTeamHelpers_PneumaFileMissing(t *testing.T) {
	f := newFake()
	// Only seed corpus; pneuma is absent.
	f.repoFiles["pt-corpus/helpers.tofu@main"] = corpusHelpersFixture
	h := newHelpersHarness(t, f)
	res := h.call(map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
	if !strings.Contains(body["message"].(string), "missing") {
		t.Fatalf("expected 'missing' in message, got %v", body["message"])
	}
}

func TestRenderTeamHelpers_MalformedCorpus(t *testing.T) {
	f := newFake()
	f.repoFiles["pt-corpus/helpers.tofu@main"] = `# no module block here
foo = "bar"
`
	f.repoFiles["pt-pneuma/shared/helpers.tofu@main"] = pneumaHelpersFixture
	h := newHelpersHarness(t, f)
	res := h.call(map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
}
