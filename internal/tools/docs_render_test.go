package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

const sidebarsFixture = `// @ts-check
const sidebars = {
  docs: [
    { items: [
      // region: platform-grouping
      'platform-grouping/logos/index',
      // endregion: platform-grouping
    ]},
  ],
};
export default sidebars;
`

func runDocsTool(t *testing.T, name string, args any) *mcp.CallToolResult {
	t.Helper()
	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.RenderTeamDocsIndex(server, v)
	tools.RenderSidebarPatch(server, v)

	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "c"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func TestRenderTeamDocsIndex_Happy(t *testing.T) {
	res := runDocsTool(t, "render_team_docs_index", map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	var out tools.RenderTeamDocsIndexOutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Path != "docs/platform-grouping/example/index.md" {
		t.Errorf("path=%q", out.Path)
	}
	for _, want := range []string{"sidebar_label: Example", "description: An example team used in tests.", "# Example"} {
		if !strings.Contains(out.Content, want) {
			t.Errorf("content missing %q; got:\n%s", want, out.Content)
		}
	}
}

func TestRenderTeamDocsIndex_MissingDescription(t *testing.T) {
	s := validSpec()
	delete(s, "display_name_comment")
	res := runDocsTool(t, "render_team_docs_index", map[string]any{"spec": s})
	if !res.IsError {
		t.Fatalf("expected error")
	}
	var e map[string]any
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			_ = json.Unmarshal([]byte(tc.Text), &e)
			break
		}
	}
	if e["code"] != "docs_input_invalid" {
		t.Errorf("code=%v want docs_input_invalid", e["code"])
	}
}

func TestRenderTeamDocsIndex_ValidationFailure(t *testing.T) {
	s := validSpec()
	s["team_key"] = "xx-bad"
	res := runDocsTool(t, "render_team_docs_index", map[string]any{"spec": s})
	if !res.IsError {
		t.Fatalf("expected error")
	}
}

func TestRenderSidebarPatch_Insert(t *testing.T) {
	res := runDocsTool(t, "render_sidebar_patch", map[string]any{
		"spec":                validSpec(),
		"current_sidebars_js": sidebarsFixture,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	var out tools.RenderSidebarPatchOutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.Content, "'platform-grouping/example/index',") {
		t.Errorf("expected new entry; got:\n%s", out.Content)
	}
}

func TestRenderSidebarPatch_MissingAnchors(t *testing.T) {
	res := runDocsTool(t, "render_sidebar_patch", map[string]any{
		"spec":                validSpec(),
		"current_sidebars_js": "// no anchors here\n",
	})
	if !res.IsError {
		t.Fatalf("expected error")
	}
	var e map[string]any
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			_ = json.Unmarshal([]byte(tc.Text), &e)
			break
		}
	}
	if e["code"] != "source_parse_error" {
		t.Errorf("code=%v want source_parse_error", e["code"])
	}
}

func TestRenderSidebarPatch_RequiresCurrentBytes(t *testing.T) {
	res := runDocsTool(t, "render_sidebar_patch", map[string]any{"spec": validSpec()})
	if !res.IsError {
		t.Fatalf("expected error")
	}
}
