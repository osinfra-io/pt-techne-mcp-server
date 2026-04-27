package docs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// TestRenderGoldens drives the renderer against every input fixture and
// asserts the output matches a golden Markdown file byte-for-byte. Set
// RENDER_UPDATE=1 to rewrite goldens.
func TestRenderGoldens(t *testing.T) {
	updateGoldens := os.Getenv("RENDER_UPDATE") == "1"
	entries, err := os.ReadDir("testdata/input")
	if err != nil {
		t.Fatalf("read input dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata/input", name))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			var team spec.Team
			if err := json.Unmarshal(data, &team); err != nil {
				t.Fatalf("decode input: %v", err)
			}
			res, err := Render(&team)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			goldenPath := filepath.Join("testdata/golden", name[:len(name)-len(".json")]+".md")
			if updateGoldens {
				if err := os.WriteFile(goldenPath, res.Content, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v (re-run with RENDER_UPDATE=1 to create)", err)
			}
			if string(res.Content) != string(want) {
				t.Errorf("render mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, res.Content, want)
			}
		})
	}
}

func TestRenderPathDerivation(t *testing.T) {
	cases := []struct {
		teamKey, teamType, wantPath string
	}{
		{"pt-logos", "platform-team", "docs/platform-teams/logos/index.md"},
		{"st-ethos", "stream-aligned-team", "docs/stream-aligned-teams/ethos/index.md"},
		{"ct-mysterion", "complicated-subsystem-team", "docs/complicated-subsystem-teams/mysterion/index.md"},
		{"et-soteria", "enabling-team", "docs/enabling-teams/soteria/index.md"},
	}
	for _, tc := range cases {
		t.Run(tc.teamKey, func(t *testing.T) {
			res, err := Render(&spec.Team{
				TeamKey:            tc.teamKey,
				TeamType:           tc.teamType,
				DisplayName:        "Test",
				DisplayNameComment: "Test description.",
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if res.Path != tc.wantPath {
				t.Errorf("path: got %q want %q", res.Path, tc.wantPath)
			}
		})
	}
}

func TestRenderRequiredFields(t *testing.T) {
	base := spec.Team{
		TeamKey:            "pt-test",
		TeamType:           "platform-team",
		DisplayName:        "Test",
		DisplayNameComment: "Test description.",
	}
	cases := []struct {
		name string
		mut  func(*spec.Team)
		want string
	}{
		{"nil team", nil, "team is required"},
		{"empty team_key", func(t *spec.Team) { t.TeamKey = "" }, "team_key is required"},
		{"empty display_name", func(t *spec.Team) { t.DisplayName = "" }, "display_name is required"},
		{"empty display_name_comment", func(t *spec.Team) { t.DisplayNameComment = "" }, "display_name_comment is required"},
		{"unknown team_type", func(t *spec.Team) { t.TeamType = "unknown" }, "unknown team_type"},
		{"bad team_key prefix", func(t *spec.Team) { t.TeamKey = "xx-foo" }, "does not start with"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in *spec.Team
			if tc.mut != nil {
				cp := base
				in = &cp
				tc.mut(in)
			}
			_, err := Render(in)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want containing %q", err.Error(), tc.want)
			}
		})
	}
}
