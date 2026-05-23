package tools

import (
	"testing"
)

func TestCoerceSpec_Object(t *testing.T) {
	in := map[string]any{"team_key": "pt-test", "display_name": "Test"}
	out, err := coerceSpec(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["team_key"] != "pt-test" {
		t.Fatalf("got team_key=%v, want pt-test", out["team_key"])
	}
}

func TestCoerceSpec_String(t *testing.T) {
	in := `{"team_key":"pt-test","display_name":"Test"}`
	out, err := coerceSpec(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["team_key"] != "pt-test" {
		t.Fatalf("got team_key=%v, want pt-test", out["team_key"])
	}
}

func TestCoerceSpec_InvalidString(t *testing.T) {
	in := "not json at all"
	_, err := coerceSpec(in)
	if err == nil {
		t.Fatal("expected error for invalid JSON string")
	}
}

func TestCoerceSpec_Nil(t *testing.T) {
	_, err := coerceSpec(nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
}

func TestCoerceSpec_TypedNilMap(t *testing.T) {
	var m map[string]any
	_, err := coerceSpec(m)
	if err == nil {
		t.Fatal("expected error for typed nil map")
	}
}

func TestCoerceSpec_NullString(t *testing.T) {
	_, err := coerceSpec("null")
	if err == nil {
		t.Fatal("expected error for JSON null string")
	}
}

func TestCoerceSpec_ArrayString(t *testing.T) {
	_, err := coerceSpec(`[1, 2, 3]`)
	if err == nil {
		t.Fatal("expected error for JSON array string")
	}
	if got := err.Error(); got != `spec string must decode to a JSON object, got []interface {}` {
		t.Fatalf("unexpected error message: %s", got)
	}
}

func TestCoerceSpec_NumberString(t *testing.T) {
	_, err := coerceSpec("42")
	if err == nil {
		t.Fatal("expected error for JSON number string")
	}
}

func TestCoerceSpec_BoolString(t *testing.T) {
	_, err := coerceSpec("true")
	if err == nil {
		t.Fatal("expected error for JSON bool string")
	}
}
