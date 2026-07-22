package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

func TestParity(t *testing.T) {
	// Set RENDER_UPDATE=1 to rewrite golden files from the current
	// renderer output instead of asserting against them. Scoped to this
	// test — no package-level state.
	updateGoldens := os.Getenv("RENDER_UPDATE") == "1"

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
			if updateGoldens {
				if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v (re-run with RENDER_UPDATE=1 to create)", err)
			}
			if string(got) != string(want) {
				t.Errorf("render mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}

func TestRenderIncludesKubernetesNamespaces(t *testing.T) {
	var team spec.Team
	input := []byte(`{
  "team_key": "pt-kryptos",
  "datadog_team_memberships": { "admins": [], "members": [] },
  "display_name": "Pt Kryptos",
  "github_parent_team_memberships": { "maintainers": [], "members": [] },
  "google_basic_groups_env_memberships": {
    "admin":  { "non-production": {"managers": [], "members": [], "owners": []}, "production": {"managers": [], "members": [], "owners": []}, "sandbox": {"managers": [], "members": [], "owners": []} },
    "reader": { "non-production": {"managers": [], "members": [], "owners": []}, "production": {"managers": [], "members": [], "owners": []}, "sandbox": {"managers": [], "members": [], "owners": []} },
    "writer": { "non-production": {"managers": [], "members": [], "owners": []}, "production": {"managers": [], "members": [], "owners": []}, "sandbox": {"managers": [], "members": [], "owners": []} }
  },
  "platform_managed_project": {
    "kubernetes_engine": {
      "locations": {
        "us-east1-b": {
          "node_pools": {
            "default-pool": {
              "machine_type": "e2-standard-2",
              "max_node_count": 3,
              "min_node_count": 1
            }
          },
          "subnet": {
            "ip_cidr_range": "10.60.96.0/20",
            "master_ipv4_cidr_block": "10.63.192.96/28",
            "pod_ip_cidr_range": "10.12.0.0/15",
            "services_ip_cidr_range": "10.62.64.0/20"
          }
        }
      },
      "namespaces": {
        "istio-test": {
          "istio_injection": "enabled",
          "routes": {
            "istio-test": {
              "path": "/istio-test",
              "port": 8080,
              "service": "istio-test"
            }
          }
        },
        "openbao": {
          "istio_injection": "disabled"
        }
      }
    }
  },
  "team_type": "platform-team"
}`)

	if err := json.Unmarshal(input, &team); err != nil {
		t.Fatalf("decode input: %v", err)
	}

	got, err := Render(&team)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := string(got)
	if !strings.Contains(out, "namespaces = {") {
		t.Fatalf("render output missing namespaces block:\n%s", out)
	}
	if !strings.Contains(out, `"openbao" = {`) {
		t.Fatalf("render output missing quoted namespace entry:\n%s", out)
	}
	if !strings.Contains(out, `istio_injection = "disabled"`) {
		t.Fatalf("render output missing istio_injection field:\n%s", out)
	}
	if !strings.Contains(out, "routes = {") {
		t.Fatalf("render output missing routes block:\n%s", out)
	}
	if !strings.Contains(out, `path = "/istio-test"`) {
		t.Fatalf("render output missing route path field:\n%s", out)
	}
	if !strings.Contains(out, "port = 8080") {
		t.Fatalf("render output missing route port field:\n%s", out)
	}
	if !strings.Contains(out, `service = "istio-test"`) {
		t.Fatalf("render output missing route service field:\n%s", out)
	}
}

func TestRenderNamespaceKeysAreQuoted(t *testing.T) {
	var team spec.Team
	input := []byte(`{
  "team_key": "pt-kryptos",
  "datadog_team_memberships": { "admins": [], "members": [] },
  "display_name": "Pt Kryptos",
  "github_parent_team_memberships": { "maintainers": [], "members": [] },
  "google_basic_groups_env_memberships": {
    "admin":  { "non-production": {"managers": [], "members": [], "owners": []}, "production": {"managers": [], "members": [], "owners": []}, "sandbox": {"managers": [], "members": [], "owners": []} },
    "reader": { "non-production": {"managers": [], "members": [], "owners": []}, "production": {"managers": [], "members": [], "owners": []}, "sandbox": {"managers": [], "members": [], "owners": []} },
    "writer": { "non-production": {"managers": [], "members": [], "owners": []}, "production": {"managers": [], "members": [], "owners": []}, "sandbox": {"managers": [], "members": [], "owners": []} }
  },
  "platform_managed_project": {
    "kubernetes_engine": {
      "locations": {
        "us-east1-b": {
          "node_pools": {
            "default-pool": {
              "machine_type": "e2-standard-2",
              "max_node_count": 3,
              "min_node_count": 1
            }
          },
          "subnet": {
            "ip_cidr_range": "10.60.96.0/20",
            "master_ipv4_cidr_block": "10.63.192.96/28",
            "pod_ip_cidr_range": "10.12.0.0/15",
            "services_ip_cidr_range": "10.62.64.0/20"
          }
        }
      },
      "namespaces": {
        "istio-system": { "istio_injection": "enabled" },
        "kube-system":  { "istio_injection": "disabled" }
      }
    }
  },
  "team_type": "platform-team"
}`)

	if err := json.Unmarshal(input, &team); err != nil {
		t.Fatalf("decode input: %v", err)
	}

	got, err := Render(&team)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := string(got)
	for _, ns := range []string{"istio-system", "kube-system"} {
		want := `"` + ns + `" = {`
		if !strings.Contains(out, want) {
			t.Fatalf("render output: expected quoted key %q, got:\n%s", want, out)
		}
		bare := ns + " = {"
		if strings.Contains(out, bare) {
			t.Fatalf("render output: found unquoted key %q (invalid HCL):\n%s", bare, out)
		}
	}
}
