# Techne MCP Server

[![Tests](https://img.shields.io/github/actions/workflow/status/osinfra-io/pt-techne-mcp-server/go-test.yml?style=for-the-badge&logo=go&color=00ADD8&label=Tests)](https://github.com/osinfra-io/pt-techne-mcp-server/actions/workflows/go-test.yml)

Model Context Protocol (MCP) server providing platform context and tools to AI assistants. Exposes deterministic, typed tools so platform agents call a tested renderer instead of writing HCL by hand.

## Tools

| Tool | Input | Output |
|---|---|---|
| `validate_team_spec` | `{spec: <object>}` | `{valid: bool, errors: [{path, message}]}` |
| `render_team_tfvars` | `{spec: <object>}` | `{tfvars: string}` (canonical pt-logos `.tfvars` bytes) |
| `open_team_pr` | `{spec: <object>, message?: string}` | `{pr_url, pr_number, branch, commit_sha, action}` |
| `list_teams` | `{}` | `{teams: [{team_key, display_name, team_type, member_count, repo_count, env_count}]}` |
| `get_team` | `{team_key: string}` | `{spec: <object>}` (same shape `validate_team_spec` accepts) |
| `lookup_user` | `{github_username: string} \| {email: string}` | `{matches: [{team_key, via, system, scope, subject, membership}]}` |
| `find_repo` | `{name: string}` | `{matches: [{team_key, repository: <object>}]}` |

`render_team_tfvars` validates first; on failure it returns an MCP `isError` result with the same structured errors as `validate_team_spec`.

`open_team_pr` validates, renders, and opens-or-updates a PR on `osinfra-io/pt-logos` in one call. It is **idempotent on retry** — identical input + identical repo state returns `action: "noop"`.

`list_teams`, `get_team`, `lookup_user`, and `find_repo` are pure reads against `osinfra-io/pt-logos@main` over the GitHub API. Each call fetches fresh — no in-process caching — so results always reflect the current branch state. Read tools require `GITHUB_TOKEN` for the same reason `open_team_pr` does (the repo is private to the GitHub API without authentication; even read access goes through it). Match semantics:

- `lookup_user` accepts exactly one of `github_username` or `email`. Matching is case-insensitive and exact. Returned `via` echoes which input matched. `system`/`scope`/`subject`/`membership` are structured (no embedded encoded role strings) so callers can filter without re-parsing.
- `find_repo` matches GitHub repository names case-sensitively. Empty `matches` is success, not an error.
- `get_team` returns `not_found` for an unknown `team_key`.

## Configuration

### `GITHUB_TOKEN`

Required by `open_team_pr` and the four read tools (`list_teams`, `get_team`, `lookup_user`, `find_repo`). Without it the server still serves `validate_team_spec` and `render_team_tfvars`; the GitHub-backed tools return a structured `not_configured` error.

The token must be scoped to `osinfra-io/pt-logos` with two write permissions:

- **Contents — read and write** (read commits, create branches, write blobs/trees, push commits).
- **Pull requests — read and write** (list, open, and update PRs).

How to express that depends on the token source:

- **GitHub App** (recommended for non-interactive deployments). In the App's settings, repository permissions → Contents: Read and write, Pull requests: Read and write; repository access → Only select repositories → `osinfra-io/pt-logos`. Mint installation tokens with [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token) (already used elsewhere in the org) and pass the result as `GITHUB_TOKEN`.
- **Fine-grained PAT** (local development). At <https://github.com/settings/personal-access-tokens>, resource owner `osinfra-io`; repository access → Only select repositories → `osinfra-io/pt-logos`; repository permissions → Contents: Read and write, Pull requests: Read and write.
- **`gh auth token`** works for local development as long as your account can push to a branch on `pt-logos` and open PRs against it. Prefer a fine-grained PAT or App token for anything non-interactive.
- **Classic PAT** is not recommended — the closest equivalent (`repo` scope) grants far more than the tool needs.

### Operational errors

`open_team_pr` and the read tools return structured MCP `isError` results:

- **Validation errors** — same `{valid: false, errors: [{path, message}]}` shape as `validate_team_spec` (write tools only).
- **Internal errors** — surfaced as plain MCP errors (the SDK's error path); reserved for things that should never happen.
- **Operational errors** — `{code, message, retryable}`:

  | `code` | `retryable` | Meaning |
  |---|---|---|
  | `not_configured` | false | `GITHUB_TOKEN` was empty at startup. Set it and restart. |
  | `not_found` | false | `get_team` was called with an unknown `team_key`. |
  | `invalid_input` | false | `lookup_user` was called with neither or both of `github_username` and `email`. |
  | `source_parse_error` | false | A `teams/*.tfvars` in `pt-logos@main` failed to parse or validate against the schema. The source must be fixed; retrying will not help. |
  | `branch_diverged` | false | The team branch has diverged from `main` and an open PR exists; the tool refuses to rewrite history under a human's PR. Rebase or close the PR, then retry. (write tools only) |
  | `github_conflict` | true | A 409/422 from GitHub raced our write and didn't auto-reconcile. Retry. (write tools only) |
  | `github_api_error` | true | Other GitHub API failure (network, 5xx, unexpected 4xx). Retry; if persistent, check the GitHub status page and the token's permissions. |

  Codes are an enumerated set; agents may switch on them. New codes are added sparingly and never reused with new meanings.

### Rotation

There is nothing to rotate inside the server — restart with a fresh `GITHUB_TOKEN`. Whatever produced the previous token is responsible for revoking it (regenerate the App installation, the PAT, etc.).

## Usage

Add this entry to your MCP client config (e.g. `.copilot/mcp.json`, `mcp.json`):

```json
{
  "mcpServers": {
    "pt-techne-mcp-server": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_TOKEN",
        "ghcr.io/osinfra-io/pt-techne-mcp-server:v0.1.0"
      ]
    }
  }
}
```

The pt-logos team-management agent calls `pt-techne-mcp-server/render_team_tfvars` instead of writing HCL itself, then `pt-techne-mcp-server/open_team_pr` to ship the change as a PR. Agents that only need to validate user input call `pt-techne-mcp-server/validate_team_spec`.

## Documentation

- [`docs/README.md`](docs/README.md) — repo layout, how to add a tool, renderer overview.
- [`docs/schema.md`](docs/schema.md) — generated reference for every team spec field. Regenerate with `make schema-docs`.
- [`schema/team.schema.json`](schema/team.schema.json) — single source of truth.

## Local development

Requires Go 1.25.8.

```sh
make build         # builds bin/pt-techne-mcp-server
make test          # go test -race ./...
make lint          # gofmt + go vet + staticcheck
make schema-docs   # regenerate docs/schema.md
make sync-schema   # mirror schema → internal/spec/schema_embed.json
```

## License

GPL-2.0. See [LICENSE](LICENSE).
