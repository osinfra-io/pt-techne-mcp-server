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
		{"valid", `{"reports-admins":{"description":"UI access","label":"Reports Admins","members":{"sandbox":["member@example.com"]}}}`, true},
		{"no-members", `{"reports-admins":{"description":"UI access","label":"Reports Admins"}}`, true},
		{"duplicate-label", `{"a":{"description":"A","label":"OpenBao Secret Admins"},"b":{"description":"B","label":"OpenBao Secret Admins"}}`, false},
		{"duplicate-member", `{"reports-admins":{"description":"A","label":"Reports Admins","members":{"sandbox":["a@example.com","a@example.com"]}}}`, false},
		{"uppercase-email", `{"reports-admins":{"description":"A","label":"Reports Admins","members":{"sandbox":["A@example.com"]}}}`, false},
		{"unknown-environment", `{"reports-admins":{"description":"A","label":"Reports Admins","members":{"prod":[]}}}`, false},
		{"delimiter", `{"reports-admins":{"description":"A","label":"Reports|Admins"}}`, false},
		{"leading-space", `{"reports-admins":{"description":"A","label":" Reports Admins"}}`, false},
		{"invalid-key", `{"Admin":{"description":"A","label":"Reports Admins"}}`, false},
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
			baseGroups := input["authentik_groups"].(map[string]any)
			for key, group := range groups.(map[string]any) {
				baseGroups[key] = group
			}
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
			{"pt-example: Reports Admins", true},
			{"pt-other: Reports Admins", false},
			{"pt-example: Reports Readers", false},
			{"pt-example: Reports Admin", false},
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
				baseGroups := input["authentik_groups"].(map[string]any)
				baseGroups["reports-admins"] = map[string]any{
					"description": "Reports access",
					"label":       "Reports Admins",
				}
				errs := v.Validate(input)
				if (len(errs) == 0) != tc.valid {
					t.Fatalf("valid=%v, errors=%v", tc.valid, errs)
				}
			})
		}
	}
}
