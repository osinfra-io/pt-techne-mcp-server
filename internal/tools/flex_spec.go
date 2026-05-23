package tools

import (
	"encoding/json"
	"fmt"
)

// coerceSpec normalises a spec argument that may arrive as either a JSON
// object (map[string]any) or a JSON-encoded string — a common LLM
// serialisation quirk where parallel tool calls double-encode objects.
func coerceSpec(raw any) (map[string]any, error) {
	switch v := raw.(type) {
	case map[string]any:
		if v == nil {
			return nil, fmt.Errorf("spec is required")
		}
		return v, nil
	case string:
		var probe any
		if err := json.Unmarshal([]byte(v), &probe); err != nil {
			return nil, fmt.Errorf("spec string is not valid JSON: %w", err)
		}
		m, ok := probe.(map[string]any)
		if !ok || m == nil {
			return nil, fmt.Errorf("spec string must decode to a JSON object, got %T", probe)
		}
		return m, nil
	case nil:
		return nil, fmt.Errorf("spec is required")
	default:
		return nil, fmt.Errorf("spec must be a JSON object or a JSON string containing an object, got %T", raw)
	}
}
