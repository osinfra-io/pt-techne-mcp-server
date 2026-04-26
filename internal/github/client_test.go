package github_test

import (
	"context"
	"testing"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
)

// TestNew_Smoke verifies the wrapper is constructible with a token.
// Network behavior is exercised by the open_team_pr fake-client tests;
// here we only confirm New returns a non-nil Client implementing the
// interface, and that the package-level constants stay aligned with
// pt-logos's hard-coded targets.
func TestNew_Smoke(t *testing.T) {
	c := gh.New(context.Background(), "fake-token")
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestRepoConstants(t *testing.T) {
	if gh.Owner != "osinfra-io" || gh.Repo != "pt-logos" || gh.Base != "main" {
		t.Fatalf("unexpected repo target: %s/%s @ %s", gh.Owner, gh.Repo, gh.Base)
	}
}
