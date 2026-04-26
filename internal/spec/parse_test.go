package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseRoundTrip asserts that for every parity input, render -> parse
// reproduces the original Team value. This is the contract that makes
// "spec <-> tfvars round-trip" enforceable.
//
// The renderer lives in a sibling package (internal/render). We invoke it
// indirectly by reading the matching golden tfvars file rather than
// importing render here (which would create a cycle: render imports
// spec). Goldens are produced by the renderer's TestParity, so reading
// them is equivalent to "render the parity input".
func TestParseRoundTrip(t *testing.T) {
	parityDir := filepath.Join("..", "render", "testdata", "parity")
	goldenDir := filepath.Join("..", "render", "testdata", "golden")

	entries, err := os.ReadDir(parityDir)
	if err != nil {
		t.Fatalf("read parity dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			inputJSON, err := os.ReadFile(filepath.Join(parityDir, name))
			if err != nil {
				t.Fatalf("read parity input: %v", err)
			}
			var want Team
			if err := json.Unmarshal(inputJSON, &want); err != nil {
				t.Fatalf("decode parity input: %v", err)
			}
			tfvars, err := os.ReadFile(filepath.Join(goldenDir, name[:len(name)-len(".json")]+".tfvars"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			got, err := Parse(tfvars)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !reflect.DeepEqual(*got, want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(want, "", "  ")
				t.Fatalf("parsed team != expected\n--- got\n%s\n--- want\n%s", gotJSON, wantJSON)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"missing teams", `other = "x"`},
		{"empty teams", `teams = {}`},
		{"multiple teams", `teams = {
  a = { team_key = "a" }
  b = { team_key = "b" }
}`},
		{"syntax error", `teams = {`},
		{"extra top-level attr", `teams = {
  pt-arche = { team_type = "platform-team" }
}
extra = "x"`},
		{"team_key mismatch", `teams = {
  pt-arche = { team_key = "different", team_type = "platform-team" }
}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.src)); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
