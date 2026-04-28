package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/docs"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

const docsBranch = "team-docs/pt-example"

// runOpenDocsPR mirrors runOpenPR but for open_team_docs_pr.
func runOpenDocsPR(t *testing.T, c gh.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.OpenTeamDocsPR(server, v, c)

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
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "open_team_docs_pr", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return res
}

// docsFakeWithSidebars sets up a fakeClient pre-populated with a
// pt-ekklesia-docs main branch and a sidebars.js with anchors. Subtests
// flip on/off the index page and add open PRs.
func docsFakeWithSidebars() *fakeClient {
	f := newFake()
	f.repoRefs[gh.RepoEkklesiaDocs+"/main"] = "main-sha"
	// Branch starts present so ensureBranch hits the StatusIdentical
	// branch (compareRes default) without create/delete churn.
	f.repoRefs[gh.RepoEkklesiaDocs+"/"+docsBranch] = "main-sha"
	f.repoFiles[gh.RepoEkklesiaDocs+"/"+sidebarsBareFixture+"@main"] = sidebarsFixture
	f.repoFiles[gh.RepoEkklesiaDocs+"/"+sidebarsBareFixture+"@"+docsBranch] = sidebarsFixture
	return f
}

const sidebarsBareFixture = "sidebars.js"

func TestOpenDocsPR_NotConfigured(t *testing.T) {
	res := runOpenDocsPR(t, nil, map[string]any{"spec": validSpec()})
	e := decodeOpError(t, res)
	if e["code"] != "not_configured" {
		t.Fatalf("code=%v", e["code"])
	}
}

func TestOpenDocsPR_HappyCreate(t *testing.T) {
	f := docsFakeWithSidebars()
	res := runOpenDocsPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	var out tools.OpenTeamDocsPROutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Action != "created" {
		t.Errorf("action=%q want created", out.Action)
	}
	if out.IndexPath != "docs/platform-grouping/example/index.md" {
		t.Errorf("index_path=%q", out.IndexPath)
	}
	if out.SidebarsPath != "sidebars.js" {
		t.Errorf("sidebars_path=%q", out.SidebarsPath)
	}
	// Two commits expected: index + sidebars.
	if f.committed != 2 || f.prsCreated != 1 {
		t.Errorf("committed=%d prsCreated=%d want 2/1", f.committed, f.prsCreated)
	}
	if len(out.CommitSHAs) != 2 {
		t.Errorf("commit_shas=%v", out.CommitSHAs)
	}
}

func TestOpenDocsPR_NoopWhenAlreadyApplied(t *testing.T) {
	f := docsFakeWithSidebars()
	// Pre-render and pre-apply both files on main with no open PR.
	rendered, err := docs.Render(specToTeamForTest(t, validSpec()))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	indexPath := rendered.Path
	f.repoFiles[gh.RepoEkklesiaDocs+"/"+indexPath+"@main"] = string(rendered.Content)
	f.repoFiles[gh.RepoEkklesiaDocs+"/"+indexPath+"@"+docsBranch] = string(rendered.Content)
	// Pre-apply the patched sidebars on main + branch so all four
	// comparisons hit equality.
	patchedSidebars := patchedSidebarsForTest(t)
	f.repoFiles[gh.RepoEkklesiaDocs+"/sidebars.js@main"] = patchedSidebars
	f.repoFiles[gh.RepoEkklesiaDocs+"/sidebars.js@"+docsBranch] = patchedSidebars

	res := runOpenDocsPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	var out tools.OpenTeamDocsPROutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Action != "noop" {
		t.Errorf("action=%q want noop", out.Action)
	}
	if f.committed != 0 || f.prsCreated != 0 {
		t.Errorf("noop should not commit/PR; got committed=%d prsCreated=%d", f.committed, f.prsCreated)
	}
}

func TestOpenDocsPR_MissingAnchorsSurfacedAsParseError(t *testing.T) {
	f := newFake()
	f.repoRefs[gh.RepoEkklesiaDocs+"/main"] = "main-sha"
	f.repoRefs[gh.RepoEkklesiaDocs+"/"+docsBranch] = "main-sha"
	f.repoFiles[gh.RepoEkklesiaDocs+"/sidebars.js@main"] = "// no anchors\n"
	f.repoFiles[gh.RepoEkklesiaDocs+"/sidebars.js@"+docsBranch] = "// no anchors\n"
	res := runOpenDocsPR(t, f, map[string]any{"spec": validSpec()})
	e := decodeOpError(t, res)
	if e["code"] != "source_parse_error" {
		t.Errorf("code=%v want source_parse_error", e["code"])
	}
}

// ---- helpers ----

func specToTeamForTest(t *testing.T, m map[string]any) *spec.Team {
	t.Helper()
	raw, _ := json.Marshal(m)
	var team spec.Team
	if err := json.Unmarshal(raw, &team); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	return &team
}

func patchedSidebarsForTest(t *testing.T) string {
	t.Helper()
	// Mirror what the renderer would produce for validSpec on the
	// fixture: append the entry before the platform-grouping endregion.
	return `// @ts-check
const sidebars = {
  docs: [
    { items: [
      // region: platform-grouping
      'platform-grouping/logos/index',
      'platform-grouping/example/index',
      // endregion: platform-grouping
    ]},
  ],
};
export default sidebars;
`
}
