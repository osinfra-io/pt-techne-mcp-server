package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gogh "github.com/google/go-github/v68/github"

	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/render"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

// fakeClient is the second implementation of gh.Client (the first is the
// production go-github wrapper). Each field tracks calls so tests can
// assert "no GitHub calls happened" and which paths fired.
type fakeClient struct {
	refs       map[string]string // branch -> SHA
	files      map[string]string // path@ref -> content (blob SHA == content here for simplicity)
	openPRs    []gh.PullRequest
	compareRes gh.CompareStatus

	// fault injection
	createPRConflictThenSucceed bool
	commitConflictThenSucceed   bool
	commitConflictMatchesAfter  bool

	// call counters
	created, deleted, updated, committed, prsCreated int
}

func newFake() *fakeClient {
	return &fakeClient{
		refs:    map[string]string{"main": "main-sha"},
		files:   map[string]string{},
		openPRs: nil,
	}
}

func (f *fakeClient) GetRef(_ context.Context, branch string) (string, bool, error) {
	sha, ok := f.refs[branch]
	return sha, ok, nil
}

func (f *fakeClient) CreateRef(_ context.Context, branch, fromSHA string) error {
	f.created++
	f.refs[branch] = fromSHA
	return nil
}

func (f *fakeClient) UpdateRef(_ context.Context, branch, toSHA string, _ bool) error {
	f.updated++
	f.refs[branch] = toSHA
	return nil
}

func (f *fakeClient) DeleteRef(_ context.Context, branch string) error {
	f.deleted++
	delete(f.refs, branch)
	return nil
}

func (f *fakeClient) CompareCommits(_ context.Context, _, _ string) (gh.CompareStatus, error) {
	if f.compareRes == "" {
		return gh.StatusIdentical, nil
	}
	return f.compareRes, nil
}

func (f *fakeClient) GetFile(_ context.Context, path, ref string) ([]byte, string, bool, error) {
	v, ok := f.files[path+"@"+ref]
	if !ok {
		return nil, "", false, nil
	}
	return []byte(v), "blob-" + v, true, nil
}

func (f *fakeClient) CreateOrUpdateFile(_ context.Context, path, branch, _ string, content []byte, _ string) (string, error) {
	if f.commitConflictThenSucceed {
		f.commitConflictThenSucceed = false
		if f.commitConflictMatchesAfter {
			f.files[path+"@"+branch] = string(content) // pretend a parallel writer wrote the same bytes
		}
		return "", fakeConflict()
	}
	f.committed++
	f.files[path+"@"+branch] = string(content)
	return "commit-sha-" + branch, nil
}

func (f *fakeClient) ListOpenPRs(_ context.Context, _, _ string) ([]gh.PullRequest, error) {
	return f.openPRs, nil
}

func (f *fakeClient) CreatePR(_ context.Context, _, _, _, _ string) (gh.PullRequest, error) {
	if f.createPRConflictThenSucceed {
		f.createPRConflictThenSucceed = false
		f.openPRs = []gh.PullRequest{{Number: 99, URL: "https://github.com/osinfra-io/pt-logos/pull/99"}}
		return gh.PullRequest{}, fakeConflict()
	}
	f.prsCreated++
	pr := gh.PullRequest{Number: 100, URL: "https://github.com/osinfra-io/pt-logos/pull/100"}
	f.openPRs = []gh.PullRequest{pr}
	return pr, nil
}

func fakeConflict() error {
	return &gogh.ErrorResponse{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}}
}

// runOpenPR drives the tool against a fake client and returns the raw
// CallToolResult plus the fake (for call-count assertions).
func runOpenPR(t *testing.T, c gh.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	v, err := spec.NewValidator()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	tools.OpenTeamPR(server, v, c)

	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "open_team_pr", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return res
}

func decodeOutput(t *testing.T, res *mcp.CallToolResult) tools.OpenTeamPROutput {
	t.Helper()
	var out tools.OpenTeamPROutput
	if err := json.Unmarshal(structuredOrText(t, res), &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return out
}

func decodeOpError(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	if !res.IsError {
		t.Fatalf("expected IsError, got success: %+v", res)
	}
	// Always read explicit text content here; the SDK may also attach an
	// empty StructuredContent for the (nil) typed output, which would
	// shadow our op-error JSON body.
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			var m map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &m); err != nil {
				t.Fatalf("decode op error: %v", err)
			}
			return m
		}
	}
	t.Fatal("op error result had no text content")
	return nil
}

func TestOpenPR_NotConfigured(t *testing.T) {
	res := runOpenPR(t, nil, map[string]any{"spec": validSpec()})
	e := decodeOpError(t, res)
	if e["code"] != "not_configured" {
		t.Fatalf("expected code=not_configured, got %v", e)
	}
}

func TestOpenPR_ValidationFailureNoGitHubCalls(t *testing.T) {
	bad := validSpec()
	bad["team_key"] = "xx-bogus"
	f := newFake()
	res := runOpenPR(t, f, map[string]any{"spec": bad})
	if !res.IsError {
		t.Fatal("expected IsError")
	}
	if f.created+f.deleted+f.updated+f.committed+f.prsCreated > 0 {
		t.Fatalf("validation failure should not call GitHub: %+v", f)
	}
}

func TestOpenPR_HappyCreate(t *testing.T) {
	f := newFake()
	f.compareRes = gh.StatusIdentical
	delete(f.refs, "team/pt-example") // branch missing
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	out := decodeOutput(t, res)
	if out.Action != "created" {
		t.Fatalf("action=%q want created", out.Action)
	}
	if f.created != 1 || f.committed != 1 || f.prsCreated != 1 {
		t.Fatalf("expected 1 ref-create + 1 commit + 1 PR-create, got %+v", f)
	}
	if out.PRURL == "" || out.PRNumber == 0 {
		t.Fatalf("missing PR URL/number: %+v", out)
	}
}

func TestOpenPR_UpdateExistingPR(t *testing.T) {
	f := newFake()
	f.refs["team/pt-example"] = "stale-sha"
	f.files["teams/pt-example.tfvars@team/pt-example"] = "old content"
	f.compareRes = gh.StatusAhead
	f.openPRs = []gh.PullRequest{{Number: 42, URL: "https://github.com/osinfra-io/pt-logos/pull/42"}}
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	out := decodeOutput(t, res)
	if out.Action != "updated" || out.PRNumber != 42 {
		t.Fatalf("unexpected: %+v", out)
	}
	if f.committed != 1 || f.prsCreated != 0 {
		t.Fatalf("expected 1 commit, 0 PR creates, got %+v", f)
	}
	if f.created != 0 || f.deleted != 0 || f.updated != 0 {
		t.Fatalf("ahead state must not move ref: %+v", f)
	}
}

func TestOpenPR_NoopBranchMatchesWithOpenPR(t *testing.T) {
	f := newFake()
	rendered := renderFor(t, validSpec())
	f.refs["team/pt-example"] = "branch-sha"
	f.files["teams/pt-example.tfvars@team/pt-example"] = string(rendered)
	f.compareRes = gh.StatusAhead
	f.openPRs = []gh.PullRequest{{Number: 7, URL: "url-7"}}
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	out := decodeOutput(t, res)
	if out.Action != "noop" || out.PRNumber != 7 {
		t.Fatalf("unexpected: %+v", out)
	}
	if f.committed != 0 || f.prsCreated != 0 {
		t.Fatalf("noop must not write: %+v", f)
	}
}

func TestOpenPR_NoopMainMatchesNoPR(t *testing.T) {
	f := newFake()
	rendered := renderFor(t, validSpec())
	f.files["teams/pt-example.tfvars@main"] = string(rendered)
	delete(f.refs, "team/pt-example")
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	out := decodeOutput(t, res)
	if out.Action != "noop" || out.PRURL != "" {
		t.Fatalf("expected noop with empty PR URL, got %+v", out)
	}
	// branch is created (ensureBranch ran before we know main matches), but
	// no commit/PR. Acceptable: leaves a no-op branch we'll reuse next time.
	if f.committed != 0 || f.prsCreated != 0 {
		t.Fatalf("noop must not commit or open PR: %+v", f)
	}
}

func TestOpenPR_BranchBehindFastForward(t *testing.T) {
	f := newFake()
	f.refs["team/pt-example"] = "old-branch-sha"
	f.compareRes = gh.StatusBehind
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	if f.updated != 1 {
		t.Fatalf("expected fast-forward UpdateRef, got updated=%d", f.updated)
	}
	if f.refs["team/pt-example"] != "main-sha" {
		t.Fatalf("expected branch fast-forwarded to main-sha, got %q", f.refs["team/pt-example"])
	}
}

func TestOpenPR_DivergedWithOpenPR(t *testing.T) {
	f := newFake()
	f.refs["team/pt-example"] = "diverged-sha"
	f.compareRes = gh.StatusDiverged
	f.openPRs = []gh.PullRequest{{Number: 1, URL: "u"}}
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	e := decodeOpError(t, res)
	if e["code"] != "branch_diverged" {
		t.Fatalf("expected branch_diverged, got %v", e)
	}
	if f.committed+f.deleted+f.updated+f.created+f.prsCreated > 0 {
		t.Fatalf("diverged-with-PR must not write: %+v", f)
	}
}

func TestOpenPR_DivergedNoPRResetAndCreate(t *testing.T) {
	f := newFake()
	f.refs["team/pt-example"] = "stale-diverged-sha"
	f.compareRes = gh.StatusDiverged
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	out := decodeOutput(t, res)
	if out.Action != "created" {
		t.Fatalf("expected created after reset, got %q", out.Action)
	}
	if f.deleted != 1 || f.created != 1 {
		t.Fatalf("expected 1 delete + 1 create on reset, got %+v", f)
	}
}

func TestOpenPR_ClosedPRStaleMatchingBranchRebuilds(t *testing.T) {
	// Branch has matching bytes but no open PR - must NOT noop. The
	// closed-PR-with-stale-branch case: previous PR was closed unmerged
	// and the branch still carries those bytes.
	f := newFake()
	rendered := renderFor(t, validSpec())
	f.refs["team/pt-example"] = "stale-branch-sha"
	f.files["teams/pt-example.tfvars@team/pt-example"] = string(rendered)
	f.compareRes = gh.StatusDiverged // closed-but-not-merged often shows as diverged after squash
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	out := decodeOutput(t, res)
	if out.Action == "noop" {
		t.Fatalf("must NOT noop when no open PR exists: %+v", out)
	}
	if out.Action != "created" {
		t.Fatalf("expected created (after reset), got %q", out.Action)
	}
}

func TestOpenPR_CreatePRConflictReconciles(t *testing.T) {
	f := newFake()
	delete(f.refs, "team/pt-example")
	f.createPRConflictThenSucceed = true
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	out := decodeOutput(t, res)
	if out.Action != "updated" || out.PRNumber != 99 {
		t.Fatalf("expected reconciled to existing PR 99, got %+v", out)
	}
}

func TestOpenPR_CommitConflictReconcilesToNoop(t *testing.T) {
	// Commit returns 422; on re-read, branch already has the rendered
	// bytes AND a PR is open → noop.
	f := newFake()
	f.refs["team/pt-example"] = "branch-sha"
	f.compareRes = gh.StatusAhead
	f.openPRs = []gh.PullRequest{{Number: 5, URL: "u-5"}}
	f.files["teams/pt-example.tfvars@team/pt-example"] = "old"
	f.commitConflictThenSucceed = true
	f.commitConflictMatchesAfter = true
	res := runOpenPR(t, f, map[string]any{"spec": validSpec()})
	out := decodeOutput(t, res)
	if out.Action != "noop" || out.PRNumber != 5 {
		t.Fatalf("expected noop after race-converged commit, got %+v", out)
	}
}

// renderFor reproduces what the tool will commit so tests can plant
// matching bytes in the fake.
func renderFor(t *testing.T, in map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var team spec.Team
	if err := json.Unmarshal(raw, &team); err != nil {
		t.Fatal(err)
	}
	out, err := render.Render(&team)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
