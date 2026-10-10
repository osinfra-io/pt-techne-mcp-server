package tools

import (
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

func TestLookupAuthentikApplicationMembers(t *testing.T) {
	team := &spec.Team{
		TeamKey: "pt-pneuma",
		AuthentikGroups: map[string]spec.AuthentikGroup{
			"agentgateway-admins": {
				Label: "agentgateway Admins",
				Members: map[string][]string{
					"sandbox":    {"member@example.com"},
					"production": {},
				},
			},
		},
	}
	matches := scanTeam(team, "email", "MEMBER@example.com")
	if len(matches) != 1 || matches[0].System != "authentik" ||
		matches[0].Subject != "agentgateway-admins/sandbox" ||
		matches[0].Membership != "member" {
		t.Fatalf("unexpected application memberships: %#v", matches)
	}
	if matches := scanTeam(team, "github_username", "member"); len(matches) != 0 {
		t.Fatalf("GitHub team membership must not imply application access: %#v", matches)
	}
}
