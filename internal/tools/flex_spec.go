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
		return v, nil
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("spec string is not valid JSON: %w", err)
		}
		return m, nil
	case nil:
		return nil, fmt.Errorf("spec is required")
	default:
		return nil, fmt.Errorf("spec must be a JSON object or a JSON string containing an object, got %T", raw)
	}
}
