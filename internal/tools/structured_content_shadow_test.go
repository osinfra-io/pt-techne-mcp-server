// Regression tests for issue #21.
//
// The MCP go-sdk (server.go:362-369 in v1.6.1) substitutes a zero-value
// element when a typed-pointer Out handler returns nil for output. That
// zero value was being marshalled into CallToolResult.StructuredContent
// on error paths, shadowing the opError JSON body that errResult/notConfigured
// emit via Content. MCP clients that read structured content (most do)
// therefore saw `{"action":"","branch":"",...}` and never the actual error.
//
// The fix is to declare Out as `any` on every tool handler that uses
// errResult/notConfigured. With Out=any the SDK skips the zero-value
// substitution and StructuredContent stays unset. These tests pin that
// behaviour: if anyone re-introduces a typed pointer Out on these tools,
// StructuredContent will start coming back non-nil and these tests fail.
package tools_test

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func assertNoStructuredShadow(t *testing.T, name string, res *mcp.CallToolResult) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("%s: expected IsError result, got success: %+v", name, res)
	}
	if res.StructuredContent != nil {
		t.Errorf("%s: StructuredContent must be nil on error paths so the "+
			"opError JSON in Content is not shadowed for MCP clients; got %T(%v)",
			name, res.StructuredContent, res.StructuredContent)
	}
	// Sanity-check: the error body is still reachable via Content.
	var sawText bool
	for _, c := range res.Content {
		if _, ok := c.(*mcp.TextContent); ok {
			sawText = true
			break
		}
	}
	if !sawText {
		t.Errorf("%s: error result must carry a TextContent body", name)
	}
}

func TestNoStructuredShadow_OpenDocsPR_NotConfigured(t *testing.T) {
	res := runOpenDocsPR(t, nil, map[string]any{"spec": validSpec()})
	assertNoStructuredShadow(t, "open_team_docs_pr/not_configured", res)
}

func TestNoStructuredShadow_OpenDocsPR_SourceParseError(t *testing.T) {
	// docsFakeWithSidebars + sidebars without anchors triggers source_parse_error.
	f := docsFakeWithSidebars()
	f.repoFiles["pt-ekklesia-docs/sidebars.js@main"] = "// no anchors\n"
	f.repoFiles["pt-ekklesia-docs/sidebars.js@"+docsBranch] = "// no anchors\n"
	res := runOpenDocsPR(t, f, map[string]any{"spec": validSpec()})
	assertNoStructuredShadow(t, "open_team_docs_pr/source_parse_error", res)
}

func TestNoStructuredShadow_OpenPR_NotConfigured(t *testing.T) {
	res := runOpenPR(t, nil, map[string]any{"spec": validSpec()})
	assertNoStructuredShadow(t, "open_team_pr/not_configured", res)
}
