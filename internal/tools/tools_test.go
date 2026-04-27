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

func validSpec() map[string]any {
	return map[string]any{
		"team_key": "pt-example",
		"datadog_team_memberships": map[string]any{
			"admins": []string{"a@b.com"}, "members": []string{},
		},
		"display_name":         "Example",
		"display_name_comment": "An example team used in tests.",
		"github_parent_team_memberships": map[string]any{
			"maintainers": []string{"x"}, "members": []string{},
		},
		"google_basic_groups_memberships": map[string]any{
			"admin":  map[string]any{"managers": []string{}, "members": []string{}, "owners": []string{"a@b.com"}},
			"reader": map[string]any{"managers": []string{}, "members": []string{}, "owners": []string{"a@b.com"}},
			"writer": map[string]any{"managers": []string{}, "members": []string{}, "owners": []string{"a@b.com"}},
		},
		"team_type": "platform-team",
	}
}

// roundTrip starts the server in-process, calls a tool, returns the result.
func roundTrip(t *testing.T, name string, args any) *mcp.CallToolResult {
	t.Helper()

	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.Validate(server, v)
	tools.Render(server, v)

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

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func TestValidateTool_Valid(t *testing.T) {
	res := roundTrip(t, "validate_team_spec", map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}
	var out tools.ValidateOutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Valid || len(out.Errors) != 0 {
		t.Fatalf("expected valid=true errors=[], got %+v", out)
	}
}

func TestValidateTool_Invalid(t *testing.T) {
	bad := validSpec()
	bad["team_key"] = "xx-bogus"
	res := roundTrip(t, "validate_team_spec", map[string]any{"spec": bad})

	var out tools.ValidateOutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid || len(out.Errors) == 0 {
		t.Fatalf("expected valid=false with errors, got %+v", out)
	}
}

func TestRenderTool_Valid(t *testing.T) {
	res := roundTrip(t, "render_team_tfvars", map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}
	var out tools.RenderOutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.HasPrefix(out.Tfvars, "teams = {\n  pt-example = {") {
		t.Fatalf("rendered tfvars unexpected: %q", out.Tfvars[:min(80, len(out.Tfvars))])
	}
}

func TestRenderTool_InvalidReturnsIsError(t *testing.T) {
	bad := validSpec()
	bad["team_key"] = "xx-bogus"
	res := roundTrip(t, "render_team_tfvars", map[string]any{"spec": bad})
	if !res.IsError {
		t.Fatalf("expected IsError on invalid spec, got success: %+v", res)
	}
}

// structuredOrText returns the JSON body of a tool call, preferring
// structured content (StructuredContent) over the first text block.
func structuredOrText(t *testing.T, res *mcp.CallToolResult) []byte {
	t.Helper()
	if res.StructuredContent != nil {
		switch sc := res.StructuredContent.(type) {
		case []byte:
			return sc
		case json.RawMessage:
			return sc
		default:
			b, err := json.Marshal(sc)
			if err != nil {
				t.Fatalf("marshal structured content: %v", err)
			}
			return b
		}
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return []byte(tc.Text)
		}
	}
	t.Fatal("tool result had neither structured nor text content")
	return nil
}
