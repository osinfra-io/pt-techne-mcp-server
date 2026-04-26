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
		"github_parent_team_memberships": {"maintainers": ["x"], "members": []},
		"google_basic_groups_memberships": {
			"admin":  {"managers": [], "members": [], "owners": ["a@b.com"]},
			"reader": {"managers": [], "members": [], "owners": ["a@b.com"]},
			"writer": {"managers": [], "members": [], "owners": ["a@b.com"]}
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

func errPathContains(errs []spec.ValidationError, want string) bool {
	for _, e := range errs {
		if strings.Contains(e.Path, want) {
			return true
		}
	}
	return false
}
