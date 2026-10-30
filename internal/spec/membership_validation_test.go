package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmbeddedSchemaRequiresPneumaAdministrator(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	const valid = `{
		"team_key": "pt-pneuma",
		"team_type": "platform-team",
		"display_name": "Pneuma",
		"display_name_comment": "Pneuma platform team",
		"datadog_team_memberships": {"admins": ["admin@osinfra.io"], "members": []},
		"github_parent_team_memberships": {"maintainers": ["admin"], "members": []},
		"github_child_teams_memberships": {
			"sandbox-approvers": {"maintainers": ["admin"], "members": []},
			"non-production-approvers": {"maintainers": ["admin"], "members": []},
			"production-approvers": {"maintainers": ["admin"], "members": []},
			"repository-administrators": {"maintainers": ["admin"], "members": []}
		},
		"google_basic_groups_env_memberships": {
			"admin": {
				"sandbox": {"owners": ["admin@osinfra.io"], "managers": [], "members": []},
				"non-production": {"owners": ["admin@osinfra.io"], "managers": [], "members": []},
				"production": {"owners": ["admin@osinfra.io"], "managers": [], "members": []}
			},
			"reader": {
				"sandbox": {"owners": ["admin@osinfra.io"], "managers": [], "members": []},
				"non-production": {"owners": ["admin@osinfra.io"], "managers": [], "members": []},
				"production": {"owners": ["admin@osinfra.io"], "managers": [], "members": []}
			},
			"writer": {
				"sandbox": {"owners": ["admin@osinfra.io"], "managers": [], "members": []},
				"non-production": {"owners": ["admin@osinfra.io"], "managers": [], "members": []},
				"production": {"owners": ["admin@osinfra.io"], "managers": [], "members": []}
			}
		},
		"authentik_groups": {
			"openbao-secret-admins": {
				"description": "Manage team secrets", "label": "OpenBao Secret Admins",
				"members": {
					"sandbox": ["admin@osinfra.io"],
					"non-production": ["admin@osinfra.io"],
					"production": ["admin@osinfra.io"]
				}
			},
			"openbao-secret-readers": {"description": "Read team secrets", "label": "OpenBao Secret Readers"},
			"openbao-secret-writers": {"description": "Write team secrets", "label": "OpenBao Secret Writers"},
			"agentgateway-admins": {"description": "Manage agentgateway", "label": "agentgateway Admins"},
			"authentik-admins": {
				"description": "Manage authentik", "label": "authentik Admins",
				"members": {
					"sandbox": ["brett@osinfra.io", "other@osinfra.io"],
					"non-production": ["brett@osinfra.io", "other@osinfra.io"],
					"production": ["brett@osinfra.io", "other@osinfra.io"]
				}
			}
		}
	}`
	for _, environment := range []string{"valid", "sandbox", "non-production", "production"} {
		t.Run(environment, func(t *testing.T) {
			var input map[string]any
			if err := json.Unmarshal([]byte(valid), &input); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if environment != "valid" {
				groups := input["authentik_groups"].(map[string]any)
				admins := groups["authentik-admins"].(map[string]any)
				admins["members"].(map[string]any)[environment] = []any{"other@osinfra.io"}
			}
			// Bypass Go membership checks to exercise only the compiled embedded schema.
			err := validator.schema.Validate(input)
			if environment == "valid" {
				if err != nil {
					t.Fatalf("valid Pneuma spec rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("embedded schema accepted a missing designated administrator")
			}
			wantPath := "/authentik_groups/authentik-admins/members/" + environment
			if !strings.Contains(err.Error(), wantPath) || !strings.Contains(err.Error(), "contains") {
				t.Fatalf("expected contains failure at %s, got %v", wantPath, err)
			}
		})
	}
}

func TestValidateMembershipRequirements(t *testing.T) {
	tests := []struct {
		name string
		spec map[string]any
		want int
	}{
		{
			name: "ordinary team",
			spec: membershipSpec(),
		},
		{
			name: "pneuma requires both admin groups",
			spec: func() map[string]any {
				spec := membershipSpec()
				spec["team_key"] = "pt-pneuma"
				return spec
			}(),
			want: 2,
		},
		{
			name: "authentik admin must include brett in each environment",
			spec: func() map[string]any {
				spec := membershipSpec()
				spec["team_key"] = "pt-pneuma"
				groups := spec["authentik_groups"].(map[string]any)
				groups["authentik-admins"] = map[string]any{
					"members": map[string]any{
						"sandbox":        []string{"other@example.com"},
						"non-production": []string{"other@example.com"},
						"production":     []string{"other@example.com"},
					},
				}
				groups["agentgateway-admins"] = map[string]any{}
				return spec
			}(),
			want: 3,
		},
		{
			name: "ordinary team cannot declare pneuma admin groups",
			spec: func() map[string]any {
				spec := membershipSpec()
				spec["authentik_groups"].(map[string]any)["agentgateway-admins"] = map[string]any{}
				return spec
			}(),
			want: 1,
		},
		{
			name: "kubernetes requires artifact registry groups",
			spec: func() map[string]any {
				spec := membershipSpec()
				spec["platform_managed_project"] = map[string]any{
					"kubernetes_engine": map[string]any{
						"locations": map[string]any{"us-east1-b": map[string]any{}},
					},
				}
				return spec
			}(),
			want: 1,
		},
		{
			name: "artifact registry groups require owners",
			spec: func() map[string]any {
				spec := membershipSpec()
				spec["platform_managed_project"] = map[string]any{
					"kubernetes_engine": map[string]any{
						"locations": map[string]any{"us-east1-b": map[string]any{}},
						"artifact_registry_groups_memberships": map[string]any{
							"readers": map[string]any{"owners": []any{}},
							"writers": map[string]any{"owners": []any{"owner@example.com"}},
						},
					},
				}
				return spec
			}(),
			want: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateMembershipRequirements(tc.spec)
			if len(errs) != tc.want {
				t.Fatalf("got %d errors, want %d: %+v", len(errs), tc.want, errs)
			}
		})
	}
}

func membershipSpec() map[string]any {
	return map[string]any{
		"team_key": "pt-example",
		"authentik_groups": map[string]any{
			"openbao-secret-admins": map[string]any{
				"members": map[string]any{
					"sandbox":        []string{"brett@osinfra.io"},
					"non-production": []string{"brett@osinfra.io"},
					"production":     []string{"brett@osinfra.io"},
				},
			},
		},
	}
}
