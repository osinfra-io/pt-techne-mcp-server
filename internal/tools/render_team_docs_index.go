// MCP tool: render_team_docs_index.
//
// Renders the team's docs index page for pt-ekklesia-docs. Validation
// failures keep the same shape as validate_team_spec; the renderer also
// requires display_name_comment to be non-empty (it's the docs page's
// frontmatter description and body lede) and surfaces a structured
// docs_input_invalid op error otherwise.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/render/docs"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// RenderTeamDocsIndexInput is the input for render_team_docs_index.
type RenderTeamDocsIndexInput struct {
	Spec any `json:"spec" jsonschema:"the validated team spec to render docs for"`
}

// RenderTeamDocsIndexOutput is the structured result.
type RenderTeamDocsIndexOutput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// RenderTeamDocsIndex registers the render_team_docs_index tool.
func RenderTeamDocsIndex(s *mcp.Server, v *spec.Validator) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_team_docs_index",
		Description: "Validate then render a team spec to the deterministic docs/<section>/<team>/index.md page for osinfra-io/pt-ekklesia-docs. Returns {path, content}. On schema failure returns ValidateOutput; on docs-specific input failure returns docs_input_invalid.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Render team docs index",
			ReadOnlyHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in RenderTeamDocsIndexInput) (*mcp.CallToolResult, *RenderTeamDocsIndexOutput, error) {
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
		raw, err := json.Marshal(specMap)
		if err != nil {
			return nil, nil, fmt.Errorf("re-marshal spec: %w", err)
		}
		var team spec.Team
		if err := json.Unmarshal(raw, &team); err != nil {
			return nil, nil, fmt.Errorf("decode validated spec: %w", err)
		}
		res, err := docs.Render(&team)
		if err != nil {
			return errResult(opError{Code: "docs_input_invalid", Message: err.Error()}), nil, nil
		}
		return nil, &RenderTeamDocsIndexOutput{Path: res.Path, Content: string(res.Content)}, nil
	})
}
