package render

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// updateGoldens, when set via -update, rewrites golden files instead of
// asserting against them. Scoped to the test binary; no package globals.
var updateGoldens = flag.Bool("update", false, "rewrite golden tfvars from current renderer output")

func TestParity(t *testing.T) {
	// Discover every parity input and assert the renderer reproduces the
	// canonical golden tfvars. The golden files are the canonical source of
	// truth — pt-logos files will be regenerated to match.
	entries, err := os.ReadDir("testdata/parity")
	if err != nil {
		t.Fatalf("read parity dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata/parity", name))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			var team spec.Team
			if err := json.Unmarshal(data, &team); err != nil {
				t.Fatalf("decode input: %v", err)
			}
			got, err := Render(&team)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			goldenPath := filepath.Join("testdata/golden", name[:len(name)-len(".json")]+".tfvars")
			if *updateGoldens {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v (re-run with -update to create)", err)
			}
			if string(got) != string(want) {
				t.Errorf("render mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}
