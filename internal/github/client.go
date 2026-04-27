// Package github wraps go-github with the narrow surface open_team_pr
// needs. The interface exists because there are two implementations: the
// real go-github wrapper and the fake used in tools tests.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"

	gh "github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// Repo is the hard-coded write target for tools that target pt-logos
// (open_team_pr, the four read tools, etc). Tools that write to other
// repos under the same org pass repo as a parameter via the *InRepo
// methods below.
const (
	Owner = "osinfra-io"
	Repo  = "pt-logos"
	Base  = "main"
)

// Repos for cross-repo helpers; kept as named constants so callers do
// not hard-code repository names at use sites.
const (
	RepoCorpus       = "pt-corpus"
	RepoPneuma       = "pt-pneuma"
	RepoEkklesiaDocs = "pt-ekklesia-docs"
)

// CompareStatus is the result of comparing a branch against the base.
// Mirrors the GitHub Compare API status string.
type CompareStatus string

const (
	StatusIdentical CompareStatus = "identical"
	StatusAhead     CompareStatus = "ahead"
	StatusBehind    CompareStatus = "behind"
	StatusDiverged  CompareStatus = "diverged"
)

// PullRequest is the minimal shape open_team_pr needs from a PR.
type PullRequest struct {
	Number int
	URL    string
}

// Client is the narrow GitHub surface open_team_pr depends on. Two
// implementations exist (this is what justifies the interface under the
// "no interfaces until two impls" rule): goClient (production, wraps
// go-github) and the in-memory fake in open_team_pr_test.go.
type Client interface {
	GetRef(ctx context.Context, branch string) (sha string, exists bool, err error)
	CreateRef(ctx context.Context, branch, fromSHA string) error
	UpdateRef(ctx context.Context, branch, toSHA string, force bool) error
	DeleteRef(ctx context.Context, branch string) error
	CompareCommits(ctx context.Context, base, head string) (CompareStatus, error)
	GetFile(ctx context.Context, path, ref string) (content []byte, blobSHA string, exists bool, err error)
	ListDir(ctx context.Context, path, ref string) (names []string, exists bool, err error)
	// GetFileInRepo reads a single file from a sibling repo under
	// osinfra-io. Used by the helpers renderers (render_corpus_helpers,
	// render_pneuma_helpers) which fetch helpers.tofu from pt-corpus and
	// pt-pneuma respectively, and by open_team_docs_pr / get_team's
	// docs_pages probe against pt-ekklesia-docs.
	GetFileInRepo(ctx context.Context, repo, path, ref string) (content []byte, blobSHA string, exists bool, err error)
	// ListDirInRepo lists files in a directory of a sibling repo under
	// osinfra-io. Used by get_team to enumerate a team's docs pages.
	ListDirInRepo(ctx context.Context, repo, path, ref string) (names []string, exists bool, err error)
	// Cross-repo write surface used by open_team_docs_pr (writes to
	// pt-ekklesia-docs). Mirrors the pt-logos-only methods above so the
	// same transaction shape works for any repo under osinfra-io.
	GetRefInRepo(ctx context.Context, repo, branch string) (sha string, exists bool, err error)
	CreateRefInRepo(ctx context.Context, repo, branch, fromSHA string) error
	UpdateRefInRepo(ctx context.Context, repo, branch, toSHA string, force bool) error
	DeleteRefInRepo(ctx context.Context, repo, branch string) error
	CompareCommitsInRepo(ctx context.Context, repo, base, head string) (CompareStatus, error)
	CreateOrUpdateFileInRepo(ctx context.Context, repo, path, branch, blobSHA string, content []byte, message string) (commitSHA string, err error)
	ListOpenPRsInRepo(ctx context.Context, repo, head, base string) ([]PullRequest, error)
	CreatePRInRepo(ctx context.Context, repo, head, base, title, body string) (PullRequest, error)
	CreateOrUpdateFile(ctx context.Context, path, branch, blobSHA string, content []byte, message string) (commitSHA string, err error)
	ListOpenPRs(ctx context.Context, head, base string) ([]PullRequest, error)
	CreatePR(ctx context.Context, head, base, title, body string) (PullRequest, error)
}

// New returns a Client backed by go-github authenticated with the given
// pre-minted token. The token may be a PAT, an installation token, or
// the output of `gh auth token` — the server doesn't care; how it's
// minted is the deployment's responsibility (see the README's
// Configuration section for required permissions).
func New(ctx context.Context, token string) Client {
	tc := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	return &goClient{api: gh.NewClient(tc)}
}

type goClient struct {
	api *gh.Client
}

func (c *goClient) GetRef(ctx context.Context, branch string) (string, bool, error) {
	ref, resp, err := c.api.Git.GetRef(ctx, Owner, Repo, "refs/heads/"+branch)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return ref.GetObject().GetSHA(), true, nil
}

func (c *goClient) CreateRef(ctx context.Context, branch, fromSHA string) error {
	full := "refs/heads/" + branch
	_, _, err := c.api.Git.CreateRef(ctx, Owner, Repo, &gh.Reference{
		Ref:    &full,
		Object: &gh.GitObject{SHA: &fromSHA},
	})
	return err
}

func (c *goClient) UpdateRef(ctx context.Context, branch, toSHA string, force bool) error {
	full := "refs/heads/" + branch
	_, _, err := c.api.Git.UpdateRef(ctx, Owner, Repo, &gh.Reference{
		Ref:    &full,
		Object: &gh.GitObject{SHA: &toSHA},
	}, force)
	return err
}

func (c *goClient) DeleteRef(ctx context.Context, branch string) error {
	_, err := c.api.Git.DeleteRef(ctx, Owner, Repo, "refs/heads/"+branch)
	return err
}

func (c *goClient) CompareCommits(ctx context.Context, base, head string) (CompareStatus, error) {
	cmp, _, err := c.api.Repositories.CompareCommits(ctx, Owner, Repo, base, head, nil)
	if err != nil {
		return "", err
	}
	switch cmp.GetStatus() {
	case "identical":
		return StatusIdentical, nil
	case "ahead":
		return StatusAhead, nil
	case "behind":
		return StatusBehind, nil
	case "diverged":
		return StatusDiverged, nil
	default:
		return "", errors.New("unknown compare status: " + cmp.GetStatus())
	}
}

func (c *goClient) GetFile(ctx context.Context, path, ref string) ([]byte, string, bool, error) {
	file, _, resp, err := c.api.Repositories.GetContents(ctx, Owner, Repo, path, &gh.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, "", false, nil
		}
		return nil, "", false, err
	}
	body, err := file.GetContent()
	if err != nil {
		return nil, "", false, err
	}
	return []byte(body), file.GetSHA(), true, nil
}

func (c *goClient) GetFileInRepo(ctx context.Context, repo, path, ref string) ([]byte, string, bool, error) {
	file, _, resp, err := c.api.Repositories.GetContents(ctx, Owner, repo, path, &gh.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("get %s/%s@%s: %w", repo, path, ref, err)
	}
	body, err := file.GetContent()
	if err != nil {
		return nil, "", false, fmt.Errorf("decode %s/%s@%s: %w", repo, path, ref, err)
	}
	return []byte(body), file.GetSHA(), true, nil
}

func (c *goClient) ListDir(ctx context.Context, path, ref string) ([]string, bool, error) {
	return c.listDir(ctx, Repo, path, ref)
}

func (c *goClient) ListDirInRepo(ctx context.Context, repo, path, ref string) ([]string, bool, error) {
	return c.listDir(ctx, repo, path, ref)
}

func (c *goClient) listDir(ctx context.Context, repo, path, ref string) ([]string, bool, error) {
	_, dir, resp, err := c.api.Repositories.GetContents(ctx, Owner, repo, path, &gh.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	out := make([]string, 0, len(dir))
	for _, e := range dir {
		if e.GetType() == "file" {
			out = append(out, e.GetName())
		}
	}
	sort.Strings(out)
	return out, true, nil
}

// Cross-repo write methods. These mirror the pt-logos-only methods
// above but accept an explicit repo so open_team_docs_pr can target
// pt-ekklesia-docs. Both sets share the same goClient receiver.

func (c *goClient) GetRefInRepo(ctx context.Context, repo, branch string) (string, bool, error) {
	ref, resp, err := c.api.Git.GetRef(ctx, Owner, repo, "refs/heads/"+branch)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return ref.GetObject().GetSHA(), true, nil
}

func (c *goClient) CreateRefInRepo(ctx context.Context, repo, branch, fromSHA string) error {
	full := "refs/heads/" + branch
	_, _, err := c.api.Git.CreateRef(ctx, Owner, repo, &gh.Reference{
		Ref:    &full,
		Object: &gh.GitObject{SHA: &fromSHA},
	})
	return err
}

func (c *goClient) UpdateRefInRepo(ctx context.Context, repo, branch, toSHA string, force bool) error {
	full := "refs/heads/" + branch
	_, _, err := c.api.Git.UpdateRef(ctx, Owner, repo, &gh.Reference{
		Ref:    &full,
		Object: &gh.GitObject{SHA: &toSHA},
	}, force)
	return err
}

func (c *goClient) DeleteRefInRepo(ctx context.Context, repo, branch string) error {
	_, err := c.api.Git.DeleteRef(ctx, Owner, repo, "refs/heads/"+branch)
	return err
}

func (c *goClient) CompareCommitsInRepo(ctx context.Context, repo, base, head string) (CompareStatus, error) {
	cmp, _, err := c.api.Repositories.CompareCommits(ctx, Owner, repo, base, head, nil)
	if err != nil {
		return "", err
	}
	switch cmp.GetStatus() {
	case "identical":
		return StatusIdentical, nil
	case "ahead":
		return StatusAhead, nil
	case "behind":
		return StatusBehind, nil
	case "diverged":
		return StatusDiverged, nil
	default:
		return "", errors.New("unknown compare status: " + cmp.GetStatus())
	}
}

func (c *goClient) CreateOrUpdateFileInRepo(ctx context.Context, repo, path, branch, blobSHA string, content []byte, message string) (string, error) {
	opts := &gh.RepositoryContentFileOptions{
		Branch:  &branch,
		Content: content,
		Message: &message,
	}
	if blobSHA != "" {
		opts.SHA = &blobSHA
	}
	res, _, err := c.api.Repositories.UpdateFile(ctx, Owner, repo, path, opts)
	if err != nil {
		return "", err
	}
	return res.GetSHA(), nil
}

func (c *goClient) ListOpenPRsInRepo(ctx context.Context, repo, head, base string) ([]PullRequest, error) {
	prs, _, err := c.api.PullRequests.List(ctx, Owner, repo, &gh.PullRequestListOptions{
		State: "open",
		Head:  Owner + ":" + head,
		Base:  base,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PullRequest, 0, len(prs))
	for _, p := range prs {
		out = append(out, PullRequest{Number: p.GetNumber(), URL: p.GetHTMLURL()})
	}
	return out, nil
}

func (c *goClient) CreatePRInRepo(ctx context.Context, repo, head, base, title, body string) (PullRequest, error) {
	pr, _, err := c.api.PullRequests.Create(ctx, Owner, repo, &gh.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
	})
	if err != nil {
		return PullRequest{}, err
	}
	return PullRequest{Number: pr.GetNumber(), URL: pr.GetHTMLURL()}, nil
}

func (c *goClient) CreateOrUpdateFile(ctx context.Context, path, branch, blobSHA string, content []byte, message string) (string, error) {
	opts := &gh.RepositoryContentFileOptions{
		Branch:  &branch,
		Content: content,
		Message: &message,
	}
	if blobSHA != "" {
		opts.SHA = &blobSHA
	}
	res, _, err := c.api.Repositories.UpdateFile(ctx, Owner, Repo, path, opts)
	if err != nil {
		return "", err
	}
	return res.GetSHA(), nil
}

func (c *goClient) ListOpenPRs(ctx context.Context, head, base string) ([]PullRequest, error) {
	prs, _, err := c.api.PullRequests.List(ctx, Owner, Repo, &gh.PullRequestListOptions{
		State: "open",
		Head:  Owner + ":" + head,
		Base:  base,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PullRequest, 0, len(prs))
	for _, p := range prs {
		out = append(out, PullRequest{Number: p.GetNumber(), URL: p.GetHTMLURL()})
	}
	return out, nil
}

func (c *goClient) CreatePR(ctx context.Context, head, base, title, body string) (PullRequest, error) {
	pr, _, err := c.api.PullRequests.Create(ctx, Owner, Repo, &gh.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
	})
	if err != nil {
		return PullRequest{}, err
	}
	return PullRequest{Number: pr.GetNumber(), URL: pr.GetHTMLURL()}, nil
}

// IsConflict reports whether err is a 409 or 422 response from the GitHub
// API, indicating a concurrent-write reconcile point.
func IsConflict(err error) bool {
	var er *gh.ErrorResponse
	if !errors.As(err, &er) || er.Response == nil {
		return false
	}
	return er.Response.StatusCode == http.StatusConflict ||
		er.Response.StatusCode == http.StatusUnprocessableEntity
}
