// MCP tool: render_team_tfvars.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/render"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// RenderInput is the input for render_team_tfvars.
type RenderInput struct {
	Spec any `json:"spec" jsonschema:"the validated team spec to render"`
}

// RenderOutput is the structured result of render_team_tfvars.
type RenderOutput struct {
	Tfvars string `json:"tfvars"`
}

// Render registers the render_team_tfvars tool on s. It validates the spec
// first and returns a structured tool error if validation fails so the
// agent can surface the same per-field messages the validate tool would.
func Render(s *mcp.Server, v *spec.Validator) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_team_tfvars",
		Description: "Validate then render a team spec to canonical pt-logos tfvars bytes. Returns {tfvars}. On validation failure, returns an isError tool result whose structured content matches validate_team_spec.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in RenderInput) (*mcp.CallToolResult, *RenderOutput, error) {
		specMap, err := coerceSpec(in.Spec)
		if err != nil {
			return errResult(opError{Code: "invalid_input", Message: err.Error()}), nil, nil
		}
		if errs := v.Validate(specMap); len(errs) > 0 {
			body, _ := json.Marshal(ValidateOutput{Valid: false, Errors: errs})
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
			}, nil, nil
		}

		raw, err := json.Marshal(specMap)
		if err != nil {
			return nil, nil, fmt.Errorf("re-marshal spec: %w", err)
		}
		var team spec.Team
		if err := json.Unmarshal(raw, &team); err != nil {
			return nil, nil, fmt.Errorf("decode validated spec: %w", err)
		}
		out, err := render.Render(&team)
		if err != nil {
			return nil, nil, fmt.Errorf("render team: %w", err)
		}
		return nil, &RenderOutput{Tfvars: string(out)}, nil
	})
}
