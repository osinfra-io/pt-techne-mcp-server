// asJSON: typed Go value -> JSON object (map[string]any).
//
// get_team and find_repo hand back spec/repository sub-objects in the
// same JSON shape callers send to validate_team_spec and
// render_team_tfvars. The MCP SDK serializes structured output by
// marshaling the typed return value, so projecting through a
// map[string]any keeps the wire shape canonical and stable across
// Go-side struct field reordering.
package tools

import (
	"encoding/json"
	"fmt"
)

func asJSON(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("asJSON: marshal: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("asJSON: unmarshal to map: %w", err)
	}
	return out, nil
}
