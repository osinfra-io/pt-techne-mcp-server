// Package github wraps go-github with the narrow surface open_team_pr
// needs. The interface exists because there are two implementations: the
// real go-github wrapper and the fake used in tools tests.
package github

import (
	"context"
	"errors"
	"net/http"
	"sort"

	gh "github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// Repo is the hard-coded target. v0 deliberately has no repo parameter.
const (
	Owner = "osinfra-io"
	Repo  = "pt-logos"
	Base  = "main"
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

func (c *goClient) ListDir(ctx context.Context, path, ref string) ([]string, bool, error) {
	_, dir, resp, err := c.api.Repositories.GetContents(ctx, Owner, Repo, path, &gh.RepositoryContentGetOptions{Ref: ref})
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
