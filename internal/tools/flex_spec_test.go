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
