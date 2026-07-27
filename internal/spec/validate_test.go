package spec_test

import (
	"strings"
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// minimalValid returns the JSON for a syntactically minimal valid team. Tests
// derive variants from this and assert which pointer paths fail.
func minimalValid() string {
	return `{
		"team_key": "pt-example",
		"datadog_team_memberships": {"admins": ["a@b.com"], "members": []},
		"display_name": "Example",
		"display_name_comment": "An example team used in tests.",
		"github_parent_team_memberships": {"maintainers": ["x"], "members": []},
		"google_basic_groups_env_memberships": {
			"admin": {
				"non-production": {"managers": [], "members": [], "owners": ["a@b.com"]},
				"production":     {"managers": [], "members": [], "owners": ["a@b.com"]},
				"sandbox":        {"managers": [], "members": [], "owners": ["a@b.com"]}
			},
			"reader": {
				"non-production": {"managers": [], "members": [], "owners": ["a@b.com"]},
				"production":     {"managers": [], "members": [], "owners": ["a@b.com"]},
				"sandbox":        {"managers": [], "members": [], "owners": ["a@b.com"]}
			},
			"writer": {
				"non-production": {"managers": [], "members": [], "owners": ["a@b.com"]},
				"production":     {"managers": [], "members": [], "owners": ["a@b.com"]},
				"sandbox":        {"managers": [], "members": [], "owners": ["a@b.com"]}
			}
		},
		"team_type": "platform-team"
	}`
}

func newValidator(t *testing.T) *spec.Validator {
	t.Helper()
	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func TestValidate(t *testing.T) {
	v := newValidator(t)

	tests := []struct {
		name     string
		spec     string
		valid    bool
		wantPath string // expected path substring on first error (if !valid)
	}{
		{"minimal-valid", minimalValid(), true, ""},
		{
			"missing-team-key",
			strings.Replace(minimalValid(), `"team_key": "pt-example",`, ``, 1),
			false, "",
		},
		{
			"bad-team-key-prefix",
			strings.Replace(minimalValid(), `"pt-example"`, `"xx-bogus"`, 1),
			false, "/team_key",
		},
		{
			"bad-team-type",
			strings.Replace(minimalValid(), `"platform-team"`, `"unknown-type"`, 1),
			false, "/team_type",
		},
		{
			"bad-email",
			strings.Replace(minimalValid(), `"a@b.com"`, `"not-an-email"`, 1),
			false, "",
		},
		{
			"prefix-type-mismatch",
			// st- prefix with platform-team type is forbidden by the schema's
			// allOf rules.
			strings.Replace(minimalValid(), `"pt-example"`, `"st-example"`, 1),
			false, "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs, err := v.ValidateJSON([]byte(tc.spec))
			if err != nil {
				t.Fatalf("ValidateJSON: %v", err)
			}
			if tc.valid && len(errs) > 0 {
				t.Fatalf("expected valid, got errors: %+v", errs)
			}
			if !tc.valid && len(errs) == 0 {
				t.Fatalf("expected invalid, got no errors")
			}
			if tc.wantPath != "" && !errPathContains(errs, tc.wantPath) {
				t.Fatalf("expected error path containing %q, got %+v", tc.wantPath, errs)
			}
		})
	}
}

func TestValidateJSON_BadJSON(t *testing.T) {
	v := newValidator(t)
	if _, err := v.ValidateJSON([]byte("not-json")); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidateRouteAuthPolicies(t *testing.T) {
	v := newValidator(t)

	tests := []struct {
		name        string
		namespace   string
		valid       bool
		wantPath    string
		wantMessage string
	}{
		{
			name: "valid-required-group",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"public_paths": ["/app/healthz"],
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			valid: true,
		},
		{
			name: "valid-disabled",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"enforced": false
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			valid: true,
		},
		{
			name: "mesh-disabled",
			namespace: `"app": {
				"istio_injection": "disabled",
				"route_auth_policies": {
					"app": {
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/route_auth_policies",
			wantMessage: "enabled",
		},
		{
			name: "unknown-route-key",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"missing": {
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/route_auth_policies/missing",
			wantMessage: "existing route",
		},
		{
			name: "enforced-without-principal",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"public_paths": ["/app/healthz"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/route_auth_policies/app",
			wantMessage: "required_groups or required_roles",
		},
		{
			name: "disabled-with-fields",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"enforced": false,
						"required_roles": ["admin"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/route_auth_policies/app/required_roles",
			wantMessage: "disabled",
		},
		{
			name: "public-path-outside-route-prefix",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"public_paths": ["/other/healthz"],
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/route_auth_policies/app/public_paths/0",
			wantMessage: "route path prefix",
		},
		{
			name: "non-rfc1123-key",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"App": {
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "",
			wantMessage: "pattern",
		},
		{
			name: "duplicate-public-paths",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"public_paths": ["/app/healthz", "/app/healthz"],
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/public_paths",
			wantMessage: "equal",
		},
		{
			name: "whitespace-required-group",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"required_groups": ["bad group"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/required_groups",
			wantMessage: "pattern",
		},
		{
			name: "duplicate-required-roles",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"required_roles": ["admin", "admin"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/required_roles",
			wantMessage: "equal",
		},
		{
			name: "whitespace-required-role",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"required_roles": ["bad role"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/required_roles",
			wantMessage: "pattern",
		},
		{
			name: "bad-public-path",
			namespace: `"app": {
				"istio_injection": "enabled",
				"route_auth_policies": {
					"app": {
						"public_paths": ["/"],
						"required_groups": ["group@example.com"]
					}
				},
				"routes": {
					"app": {
						"path": "/app",
						"port": 8080,
						"service": "app"
					}
				}
			}`,
			wantPath:    "/public_paths",
			wantMessage: "not",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs, err := v.ValidateJSON([]byte(withNamespace(tc.namespace)))
			if err != nil {
				t.Fatalf("ValidateJSON: %v", err)
			}
			if tc.valid && len(errs) > 0 {
				t.Fatalf("expected valid, got errors: %+v", errs)
			}
			if !tc.valid && len(errs) == 0 {
				t.Fatalf("expected invalid, got no errors")
			}
			if tc.wantPath != "" && !errPathContains(errs, tc.wantPath) {
				t.Fatalf("expected error path containing %q, got %+v", tc.wantPath, errs)
			}
			if tc.wantMessage != "" && !errMessageContains(errs, tc.wantMessage) {
				t.Fatalf("expected error message containing %q, got %+v", tc.wantMessage, errs)
			}
		})
	}
}

func errPathContains(errs []spec.ValidationError, want string) bool {
	for _, e := range errs {
		if strings.Contains(e.Path, want) {
			return true
		}
	}
	return false
}

func errMessageContains(errs []spec.ValidationError, want string) bool {
	for _, e := range errs {
		if strings.Contains(e.Message, want) {
			return true
		}
	}
	return false
}

func withNamespace(namespace string) string {
	return strings.Replace(minimalValid(), `"team_type": "platform-team"`, `"platform_managed_project": {
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
					`+namespace+`
				}
			}
		},
		"team_type": "platform-team"`, 1)
}
