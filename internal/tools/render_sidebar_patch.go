// MCP tool: render_team_sidebar_patch.
//
// Inserts the team's docs index entry into a provided pt-ekklesia-docs
// sidebars.js. Stateless — the caller supplies the current bytes — so
// the tool can be exercised against any sidebar shape (test fixtures,
// staged edits) without touching GitHub.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/docs"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/sidebar"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// RenderTeamSidebarPatchInput is the input for render_team_sidebar_patch.
type RenderTeamSidebarPatchInput struct {
	Spec              any    `json:"spec" jsonschema:"the validated team spec; section/team folder are derived from team_type and team_key"`
	CurrentSidebarsJS string `json:"current_sidebars_js" jsonschema:"current contents of pt-ekklesia-docs/sidebars.js"`
}

// RenderTeamSidebarPatchOutput is the structured result.
type RenderTeamSidebarPatchOutput struct {
	Content string `json:"content"`
}

// RenderSidebarPatch registers the render_team_sidebar_patch tool.
func RenderTeamSidebarPatch(s *mcp.Server, v *spec.Validator) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_team_sidebar_patch",
		Description: "Insert a team's docs index entry into the supplied pt-ekklesia-docs sidebars.js, returning the patched content. Byte-stable noop when the entry is already present. Returns source_parse_error when the // region: <section> / // endregion: <section> anchors are missing.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Render sidebar patch",
			ReadOnlyHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in RenderTeamSidebarPatchInput) (*mcp.CallToolResult, *RenderTeamSidebarPatchOutput, error) {
		specMap, err := coerceSpec(in.Spec)
		if err != nil {
			return errResult(opError{Code: "invalid_input", Message: err.Error()}), nil, nil
		}
		if errs := v.Validate(specMap); len(errs) > 0 {
			body, merr := json.Marshal(ValidateOutput{Valid: false, Errors: errs})
			if merr != nil {
				return nil, nil, fmt.Errorf("marshal validation errors: %w", merr)
			}
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil, nil
		}
		if in.CurrentSidebarsJS == "" {
			return errResult(opError{Code: "invalid_input", Message: "current_sidebars_js is required"}), nil, nil
		}

		raw, err := json.Marshal(specMap)
		if err != nil {
			return nil, nil, fmt.Errorf("re-marshal spec: %w", err)
		}
		var team spec.Team
		if err := json.Unmarshal(raw, &team); err != nil {
			return nil, nil, fmt.Errorf("decode validated spec: %w", err)
		}
		section, err := docs.SectionFor(team.TeamType)
		if err != nil {
			return errResult(opError{Code: "docs_input_invalid", Message: err.Error()}), nil, nil
		}
		folder, err := docs.TeamFolder(team.TeamKey)
		if err != nil {
			return errResult(opError{Code: "docs_input_invalid", Message: err.Error()}), nil, nil
		}

		out, err := sidebar.Render([]byte(in.CurrentSidebarsJS), section, folder, team.DisplayName)
		if err != nil {
			var anchorsErr *sidebar.ErrAnchorsMissing
			if errors.As(err, &anchorsErr) {
				return errResult(opError{Code: "source_parse_error", Message: err.Error()}), nil, nil
			}
			return nil, nil, fmt.Errorf("render sidebar: %w", err)
		}
		return nil, &RenderTeamSidebarPatchOutput{Content: string(out)}, nil
	})
}
