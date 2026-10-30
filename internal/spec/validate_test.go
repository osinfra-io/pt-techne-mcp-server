package spec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// minimalValid returns the JSON for a syntactically minimal valid team. Tests
// derive variants from this and assert which pointer paths fail.
func minimalValid() string {
	return `{
		"team_key": "pt-example",
		"authentik_groups": {
			"openbao-secret-admins": {
				"description": "Manage OpenBao secrets",
				"label": "OpenBao Secret Admins",
				"members": {
					"non-production": ["a@b.com"],
					"production": ["a@b.com"],
					"sandbox": ["a@b.com"]
				}
			},
			"openbao-secret-readers": {"description": "Read OpenBao secrets", "label": "OpenBao Secret Readers"},
			"openbao-secret-writers": {"description": "Write OpenBao secrets", "label": "OpenBao Secret Writers"}
		},
		"datadog_team_memberships": {"admins": ["a@b.com"], "members": ["a@b.com"]},
		"display_name": "Example",
		"display_name_comment": "An example team used in tests.",
		"github_child_teams_memberships": {
			"non-production-approvers": {"maintainers": ["x"], "members": []},
			"production-approvers": {"maintainers": ["x"], "members": []},
			"repository-administrators": {"maintainers": ["x"], "members": []},
			"sandbox-approvers": {"maintainers": ["x"], "members": []}
		},
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

func TestValidateRequiresPneumaAuthentikAdminInEveryEnvironment(t *testing.T) {
	v := newValidator(t)
	for _, missingEnvironment := range []string{"sandbox", "non-production", "production"} {
		t.Run(missingEnvironment, func(t *testing.T) {
			var input map[string]any
			if err := json.Unmarshal([]byte(minimalValid()), &input); err != nil {
				t.Fatal(err)
			}
			input["team_key"] = "pt-pneuma"
			groups := input["authentik_groups"].(map[string]any)
			members := map[string]any{
				"sandbox":        []string{"brett@osinfra.io"},
				"non-production": []string{"brett@osinfra.io"},
				"production":     []string{"brett@osinfra.io"},
			}
			members[missingEnvironment] = []string{"other@example.com"}
			groups["authentik-admins"] = map[string]any{
				"description": "Authentik instance administration",
				"label":       "Authentik Admins",
				"members":     members,
			}
			groups["agentgateway-admins"] = map[string]any{
				"description": "agentgateway UI access",
				"label":       "agentgateway Admins",
			}

			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			errs, err := v.ValidateJSON(encoded)
			if err != nil {
				t.Fatalf("ValidateJSON: %v", err)
			}
			if len(errs) == 0 {
				t.Fatal("expected schema validation error when Brett is missing from Authentik Admins")
			}
		})
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
				"mesh_enabled": true,
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
			name: "valid-public",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"mode": "public"
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
			name: "valid-api-jwt",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"audiences": ["api://app"],
						"mode": "api-jwt",
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
			valid: true,
		},
		{
			name: "valid-shared-browser-requirements",
			namespace: `"admin": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"admin": {
						"required_groups": ["operators", "admins"],
						"required_roles": ["viewer"]
					}
				},
				"routes": {
					"admin": {
						"path": "/admin",
						"port": 8080,
						"service": "admin"
					}
				}
			},
			"reports": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"reports": {
						"required_groups": ["admins", "operators"],
						"required_roles": ["viewer"]
					}
				},
				"routes": {
					"reports": {
						"path": "/reports",
						"port": 8080,
						"service": "reports"
					}
				}
			}`,
			valid: true,
		},
		{
			name: "conflicting-shared-browser-requirements",
			namespace: `"admin": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"admin": {
						"required_groups": ["admins"]
					}
				},
				"routes": {
					"admin": {
						"path": "/admin",
						"port": 8080,
						"service": "admin"
					}
				}
			},
			"reports": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"reports": {
						"required_groups": ["reporters"]
					}
				},
				"routes": {
					"reports": {
						"path": "/reports",
						"port": 8080,
						"service": "reports"
					}
				}
			}`,
			wantPath:    "/namespaces/reports/route_auth_policies/reports",
			wantMessage: "identical required_groups and required_roles",
		},
		{
			name: "mesh-disabled",
			namespace: `"app": {
				"mesh_enabled": false,
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
				"mesh_enabled": true,
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
			name: "browser-without-principal",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"mode": "browser",
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
			name: "public-with-fields",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"mode": "public",
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
			wantPath:    "/route_auth_policies/app",
			wantMessage: "public",
		},
		{
			name: "public-with-public-path",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"mode": "public",
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
			wantMessage: "public",
		},
		{
			name: "browser-with-audience",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"audiences": ["api://app"],
						"mode": "browser",
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
			wantPath:    "/route_auth_policies/app/audiences",
			wantMessage: "browser",
		},
		{
			name: "api-jwt-without-audience",
			namespace: `"app": {
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"mode": "api-jwt"
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
			wantPath:    "/route_auth_policies/app/audiences",
			wantMessage: "api-jwt",
		},
		{
			name: "public-path-outside-route-prefix",
			namespace: `"app": {
				"mesh_enabled": true,
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
				"mesh_enabled": true,
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
				"mesh_enabled": true,
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
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"required_groups": [" bad group"]
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
				"mesh_enabled": true,
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
				"mesh_enabled": true,
				"route_auth_policies": {
					"app": {
						"required_roles": ["bad role "]
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
				"mesh_enabled": true,
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
				"artifact_registry_groups_memberships": {
					"readers": {"managers": [], "members": [], "owners": ["a@b.com"]},
					"writers": {"managers": [], "members": [], "owners": ["a@b.com"]}
				},
				"namespaces": {
					`+namespace+`
				}
			}
		},
		"team_type": "platform-team"`, 1)
}
