package spec

import "testing"

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
