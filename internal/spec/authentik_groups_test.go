package spec_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestAuthentikGroups(t *testing.T) {
	v := newValidator(t)
	for _, tc := range []struct {
		name   string
		groups string
		valid  bool
	}{
		{"valid", `{"agentgateway-admins":{"description":"UI access","label":"agentgateway Admins","members":{"sandbox":["member@example.com"]}}}`, true},
		{"no-members", `{"agentgateway-admins":{"description":"UI access","label":"agentgateway Admins"}}`, true},
		{"duplicate-label", `{"a":{"description":"A","label":"agentgateway Admins"},"b":{"description":"B","label":"agentgateway Admins"}}`, false},
		{"duplicate-member", `{"a":{"description":"A","label":"agentgateway Admins","members":{"sandbox":["a@example.com","a@example.com"]}}}`, false},
		{"uppercase-email", `{"a":{"description":"A","label":"agentgateway Admins","members":{"sandbox":["A@example.com"]}}}`, false},
		{"unknown-environment", `{"a":{"description":"A","label":"agentgateway Admins","members":{"prod":[]}}}`, false},
		{"delimiter", `{"a":{"description":"A","label":"agentgateway|Admins"}}`, false},
		{"leading-space", `{"a":{"description":"A","label":" agentgateway Admins"}}`, false},
		{"invalid-key", `{"Admin":{"description":"A","label":"agentgateway Admins"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input map[string]any
			if err := json.Unmarshal([]byte(minimalValid()), &input); err != nil {
				t.Fatal(err)
			}

			var groups any
			if err := json.Unmarshal([]byte(tc.groups), &groups); err != nil {
				t.Fatal(err)
			}
			input["authentik_groups"] = groups
			errs := v.Validate(input)
			if (len(errs) == 0) != tc.valid {
				t.Fatalf("valid=%v, errors=%v", tc.valid, errs)
			}
		})
	}
}

func TestAuthentikGroupReferences(t *testing.T) {
	v := newValidator(t)
	for _, field := range []string{"required_groups", "required_roles"} {
		for _, tc := range []struct {
			name  string
			valid bool
		}{
			{"pt-example: agentgateway Admins", true},
			{"pt-other: agentgateway Admins", false},
			{"pt-example: agentgateway Readers", false},
			{"pt-example: Agentgateway Admins", false},
			{"authentik Admin", true},
			{"all", true},
			{"authentik,Admin", false},
			{"authentik|Admin", false},
			{"authentik  Admin", false},
			{"authentik\tAdmin", false},
			{" authentik Admin", false},
			{"authentik Admin ", false},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				inputJSON := withNamespace(fmt.Sprintf(`"app": {
					"mesh_enabled": true,
					"routes": {"app": {"path": "/app", "port": 8080, "service": "app"}},
					"route_auth_policies": {"app": {%q: [%q]}}
				}`, field, tc.name))
				var input map[string]any
				if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
					t.Fatal(err)
				}
				input["authentik_groups"] = map[string]any{
					"agentgateway-admins": map[string]any{
						"description": "UI access",
						"label":       "agentgateway Admins",
					},
				}
				errs := v.Validate(input)
				if (len(errs) == 0) != tc.valid {
					t.Fatalf("valid=%v, errors=%v", tc.valid, errs)
				}
			})
		}
	}
}
