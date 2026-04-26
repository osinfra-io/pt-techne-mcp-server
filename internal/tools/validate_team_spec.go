// Package tools wires the spec validator and renderer to MCP tool handlers.
//
// Each handler is a thin adapter: decode typed args, call the pure
// internal/spec or internal/render function, return a typed result. The MCP
// SDK wraps these into a standard tool/call response.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// ValidateInput is the input for the validate_team_spec tool.
//
// The spec is accepted as an arbitrary JSON object so callers can pass any
// shape; validation reports errors against the schema.
type ValidateInput struct {
	Spec map[string]any `json:"spec" jsonschema:"the team spec object to validate"`
}

// ValidateOutput is the structured result of validate_team_spec.
type ValidateOutput struct {
	Valid  bool                   `json:"valid"`
	Errors []spec.ValidationError `json:"errors"`
}

// Validate registers the validate_team_spec tool on s.
func Validate(s *mcp.Server, v *spec.Validator) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "validate_team_spec",
		Description: "Validate a team spec object against schema/team.schema.json. Returns {valid, errors[]} with structured per-field errors. Never throws.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ValidateInput) (*mcp.CallToolResult, *ValidateOutput, error) {
		errs := v.Validate(in.Spec)
		return nil, &ValidateOutput{
			Valid:  len(errs) == 0,
			Errors: errs,
		}, nil
	})
}
