package render

import (
	"reflect"
	"strings"
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

func TestAuthentikGroupsRoundTrip(t *testing.T) {
	groups := map[string]spec.AuthentikGroup{
		"1-readers": {
			Description: "Numeric-leading RFC 1123 identifier",
			Label:       "Reports Readers",
		},
		"agentgateway-admins": {
			Description: "Access to the agentgateway administration UI",
			Label:       "agentgateway Admins",
			Members: map[string][]string{
				"non-production": {},
				"production":     {},
				"sandbox":        {"member@example.com"},
			},
		},
		"openbao-secret-readers": {},
	}
	team := &spec.Team{TeamKey: "pt-example", AuthentikGroups: groups}
	rendered, err := Render(team)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := spec.Parse(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed.AuthentikGroups, groups) {
		t.Fatalf("group round trip: got %#v, want %#v", parsed.AuthentikGroups, groups)
	}
	if !strings.Contains(string(rendered), `label       = "agentgateway Admins"`) {
		t.Fatal("renderer changed application branding")
	}
	if strings.Contains(string(rendered), `description = ""`) || strings.Contains(string(rendered), `label       = ""`) {
		t.Fatal("renderer emitted empty fixed OpenBao metadata")
	}
}
