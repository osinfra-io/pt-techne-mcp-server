package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// helpersHarness wires the two helpers-renderer tools against a fake
// gh.Client. Mirrors readToolHarness but isolated because these tools
// don't share the readToolHarness's pt-logos teams seeding.
type helpersHarness struct {
	t  *testing.T
	cs *mcp.ClientSession
}

func newHelpersHarness(t *testing.T, c gh.Client) *helpersHarness {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.RenderCorpusHelpers(server, c)
	tools.RenderPneumaHelpers(server, c)

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

func (h *helpersHarness) call(name string, args any) *mcp.CallToolResult {
	h.t.Helper()
	res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("CallTool %s: %v", name, err)
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
	f.repoFiles["pt-pneuma/helpers.tofu@main"] = pneumaHelpersFixture
	return f
}

func TestRenderCorpusHelpers_Insert(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())
	res := h.call("render_corpus_helpers", map[string]any{"team_key": "pt-newteam"})
	var out tools.RenderCorpusHelpersOutput
	decodeStruct(t, res, &out)
	if !strings.Contains(out.HelpersTofu, `"pt-newteam-main-production",`) {
		t.Fatalf("inserted line not found:\n%s", out.HelpersTofu)
	}
	// Sorted insert position: between pt-logos and pt-pneuma.
	wantOrder := []string{
		`"pt-corpus-main-production"`,
		`"pt-logos-main-production"`,
		`"pt-newteam-main-production"`,
		`"pt-pneuma-main-production"`,
	}
	prev := -1
	for _, s := range wantOrder {
		idx := strings.Index(out.HelpersTofu, s)
		if idx < 0 {
			t.Fatalf("missing %s in output:\n%s", s, out.HelpersTofu)
		}
		if idx <= prev {
			t.Fatalf("entries out of order; %s appeared before previous entry", s)
		}
		prev = idx
	}
}

func TestRenderPneumaHelpers_Insert(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())
	res := h.call("render_pneuma_helpers", map[string]any{"team_key": "pt-newteam"})
	var out tools.RenderPneumaHelpersOutput
	decodeStruct(t, res, &out)
	if !strings.Contains(out.HelpersTofu, `    "pt-newteam-main-production",`) {
		t.Fatalf("inserted line not found:\n%s", out.HelpersTofu)
	}
	if !strings.Contains(out.HelpersTofu, `    "pt-pneuma-main-production",`+"\n  ]") {
		t.Fatalf("trailing-comma style not preserved:\n%s", out.HelpersTofu)
	}
}

func TestRenderHelpers_Idempotent(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())

	res := h.call("render_corpus_helpers", map[string]any{"team_key": "pt-logos"})
	var corpusOut tools.RenderCorpusHelpersOutput
	decodeStruct(t, res, &corpusOut)
	if corpusOut.HelpersTofu != corpusHelpersFixture {
		t.Fatalf("noop must return byte-identical helpers.tofu\n--- got ---\n%s\n--- want ---\n%s", corpusOut.HelpersTofu, corpusHelpersFixture)
	}

	res = h.call("render_pneuma_helpers", map[string]any{"team_key": "pt-pneuma"})
	var pneumaOut tools.RenderPneumaHelpersOutput
	decodeStruct(t, res, &pneumaOut)
	if pneumaOut.HelpersTofu != pneumaHelpersFixture {
		t.Fatalf("noop must return byte-identical helpers.tofu\n--- got ---\n%s\n--- want ---\n%s", pneumaOut.HelpersTofu, pneumaHelpersFixture)
	}
}

func TestRenderHelpers_NotConfigured(t *testing.T) {
	h := newHelpersHarness(t, nil)
	for _, name := range []string{"render_corpus_helpers", "render_pneuma_helpers"} {
		t.Run(name, func(t *testing.T) {
			res := h.call(name, map[string]any{"team_key": "pt-newteam"})
			body := decodeError(t, res)
			if body["code"] != "not_configured" {
				t.Fatalf("expected not_configured, got %+v", body)
			}
		})
	}
}

func TestRenderHelpers_InvalidInput(t *testing.T) {
	h := newHelpersHarness(t, seedHelpersFake())
	for _, bad := range []string{"", "no-prefix", "pt-", "pt-Foo", "PT-bar"} {
		t.Run(bad, func(t *testing.T) {
			res := h.call("render_corpus_helpers", map[string]any{"team_key": bad})
			body := decodeError(t, res)
			if body["code"] != "invalid_input" {
				t.Fatalf("expected invalid_input for %q, got %+v", bad, body)
			}
		})
	}
}

func TestRenderHelpers_FileMissing(t *testing.T) {
	f := newFake() // no repoFiles seeded
	h := newHelpersHarness(t, f)
	res := h.call("render_corpus_helpers", map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
	if !strings.Contains(body["message"].(string), "missing") {
		t.Fatalf("expected 'missing' in message, got %v", body["message"])
	}
}

func TestRenderHelpers_MalformedSource(t *testing.T) {
	f := newFake()
	f.repoFiles["pt-corpus/helpers.tofu@main"] = `# no module block here
foo = "bar"
`
	h := newHelpersHarness(t, f)
	res := h.call("render_corpus_helpers", map[string]any{"team_key": "pt-newteam"})
	body := decodeError(t, res)
	if body["code"] != "source_parse_error" {
		t.Fatalf("expected source_parse_error, got %+v", body)
	}
}
