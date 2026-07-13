package tools

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// specToTeam coerces, validates, and decodes a raw spec argument into a *spec.Team.
// Returns (team, nil, nil) on success, (nil, callToolResult, nil) on input/schema
// errors, and (nil, nil, err) on internal errors.
func specToTeam(v *spec.Validator, raw any) (*spec.Team, *mcp.CallToolResult, error) {
	specMap, err := coerceSpec(raw)
	if err != nil {
		return nil, errResult(opError{Code: "invalid_input", Message: err.Error()}), nil
	}
	if errs := v.Validate(specMap); len(errs) > 0 {
		body, merr := json.Marshal(ValidateOutput{Valid: false, Errors: errs})
		if merr != nil {
			return nil, nil, fmt.Errorf("marshal validation errors: %w", merr)
		}
		return nil, &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil
	}
	b, err := json.Marshal(specMap)
	if err != nil {
		return nil, nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var team spec.Team
	if err := json.Unmarshal(b, &team); err != nil {
		return nil, nil, fmt.Errorf("decode validated spec: %w", err)
	}
	return &team, nil, nil
}
