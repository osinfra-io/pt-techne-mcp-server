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
| `get_team` | `{team_key: string}` | `{spec: <object>, docs_pages: [string]}` (`spec` matches `validate_team_spec`; `docs_pages` lists existing pages under `docs/<section>/<team>/` in `pt-ekklesia-docs@main`, excluding `index.md`) |
| `lookup_user` | `{github_username: string} \| {email: string}` | `{matches: [{team_key, via, system, scope, subject, membership}]}` |
| `find_repo` | `{name: string}` | `{matches: [{team_key, repository: <object>}]}` |
| `render_corpus_helpers` | `{team_key: string}` | `{helpers_tofu: string}` (canonical `pt-corpus/helpers.tofu` bytes with `<team_key>-main-production` inserted) |
| `render_pneuma_helpers` | `{team_key: string}` | `{helpers_tofu: string}` (same, for `pt-pneuma/helpers.tofu`) |
| `render_team_docs_index` | `{spec: <object>}` | `{path, content}` (`docs/<section>/<team>/index.md` for pt-ekklesia-docs) |
| `render_sidebar_patch` | `{spec: <object>, current_sidebars_js: string}` | `{content}` (patched `pt-ekklesia-docs/sidebars.js`) |
| `open_team_docs_pr` | `{spec: <object>, message?: string}` | `{pr_url, pr_number, branch, commit_shas, action, index_path, sidebars_path}` |

`render_team_tfvars` validates first; on failure it returns an MCP `isError` result with the same structured errors as `validate_team_spec`.

`open_team_pr` validates, renders, and opens-or-updates a PR on `osinfra-io/pt-logos` in one call. It is **idempotent on retry** — identical input + identical repo state returns `action: "noop"`.

`list_teams`, `get_team`, `lookup_user`, and `find_repo` are pure reads against `osinfra-io/pt-logos@main` over the GitHub API. Each call fetches fresh — no in-process caching — so results always reflect the current branch state. Read tools require `GITHUB_TOKEN` for the same reason `open_team_pr` does (the repo is private to the GitHub API without authentication; even read access goes through it). Match semantics:

- `lookup_user` accepts exactly one of `github_username` or `email`. Matching is case-insensitive and exact. Returned `via` echoes which input matched. `system`/`scope`/`subject`/`membership` are structured (no embedded encoded role strings) so callers can filter without re-parsing.
- `find_repo` matches GitHub repository names case-sensitively. Empty `matches` is success, not an error.
- `get_team` returns `not_found` for an unknown `team_key`.

`render_corpus_helpers` and `render_pneuma_helpers` fetch `helpers.tofu` from `osinfra-io/pt-corpus@main` and `osinfra-io/pt-pneuma@main` respectively, insert `<team_key>-main-production` into the `logos_workspaces` list inside the `module "core_helpers"` block, and return the canonical updated file bytes. They are **idempotent**: when the workspace is already present, the input bytes are returned byte-identical. They preserve every other byte of the file (comments, indentation, line-ending style, trailing-comma convention) and refuse to rewrite a file whose `logos_workspaces` is anything other than a list of plain string literals (returns `source_parse_error`). These tools produce bytes only — they do not open PRs; the caller (or a follow-up tool) is responsible for landing the change.

`render_team_docs_index` and `render_sidebar_patch` produce the two artifacts a new team needs in `osinfra-io/pt-ekklesia-docs`: a `docs/<section>/<team>/index.md` page (frontmatter + heading + the team's `display_name_comment` as the lede) and a patched `sidebars.js` with the page registered under the right section. The sidebar patcher is **anchor-driven** — it relies on `// region: <section>` / `// endregion: <section>` markers in `sidebars.js` and returns `source_parse_error` when they are missing. Both renderers are byte-stable noops when the artifact already matches. Section is derived from `team_type`; team folder is derived from `team_key` (prefix stripped). The page body intentionally stays minimal — Bounded Context, Ubiquitous Language, ADRs, and so on remain human-authored.

`open_team_docs_pr` runs both docs renderers and lands them on `team-docs/<team_key>` in `osinfra-io/pt-ekklesia-docs` via the same idempotent transaction as `open_team_pr` (per-file commits, noop when the branch and PR already match).

## Configuration

### `GITHUB_TOKEN`

Required by `open_team_pr`, the four pt-logos read tools (`list_teams`, `get_team`, `lookup_user`, `find_repo`), the two helpers renderers (`render_corpus_helpers`, `render_pneuma_helpers`), and `open_team_docs_pr`. Without it the server still serves `validate_team_spec`, `render_team_tfvars`, `render_team_docs_index`, and `render_sidebar_patch`; the GitHub-backed tools return a structured `not_configured` error.

The token must be scoped to **`osinfra-io/pt-logos`**, **`osinfra-io/pt-corpus`**, **`osinfra-io/pt-pneuma`**, and **`osinfra-io/pt-ekklesia-docs`** with the following permissions:

- On `pt-logos`: **Contents — read and write**, **Pull requests — read and write** (used by `open_team_pr` plus the four read tools).
- On `pt-corpus` and `pt-pneuma`: **Contents — read** (used by `render_corpus_helpers` and `render_pneuma_helpers` to fetch `helpers.tofu`).
- On `pt-ekklesia-docs`: **Contents — read and write**, **Pull requests — read and write** (used by `open_team_docs_pr` to land the docs index page + sidebars patch, and by `get_team` to read existing docs pages).

How to express that depends on the token source:

- **GitHub App** (recommended for non-interactive deployments). In the App's settings, repository permissions → Contents: Read and write, Pull requests: Read and write; repository access → Only select repositories → `osinfra-io/pt-logos`, `osinfra-io/pt-corpus`, `osinfra-io/pt-pneuma`, `osinfra-io/pt-ekklesia-docs`. (Read-only on corpus/pneuma is enough — the App-permissions UI does not let you set per-repo permission levels, so the same Read/Write set applies to all selected repos; if least privilege matters, use a separate App for the helpers renderers.) Mint installation tokens with [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token) (already used elsewhere in the org) and pass the result as `GITHUB_TOKEN`.
- **Fine-grained PAT** (local development). At <https://github.com/settings/personal-access-tokens>, resource owner `osinfra-io`; repository access → Only select repositories → `osinfra-io/pt-logos`, `osinfra-io/pt-corpus`, `osinfra-io/pt-pneuma`, `osinfra-io/pt-ekklesia-docs`; repository permissions → Contents: Read and write, Pull requests: Read and write. (Same per-repo permission caveat as the App option above.)
- **`gh auth token`** works for local development as long as your account can push to a branch on `pt-logos` and `pt-ekklesia-docs` and open PRs against them, and read `pt-corpus` and `pt-pneuma`. Prefer a fine-grained PAT or App token for anything non-interactive.
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
  | `invalid_input` | false | `lookup_user` was called with neither or both of `github_username` and `email`; or `render_corpus_helpers`/`render_pneuma_helpers` was called with a `team_key` that does not match the team-spec regex; or `render_sidebar_patch` was called without `current_sidebars_js`. |
  | `docs_input_invalid` | false | `render_team_docs_index`, `render_sidebar_patch`, or `open_team_docs_pr` was given a spec the docs renderer can't use (missing `display_name_comment`, unknown `team_type`, `team_key` without a recognised prefix). |
  | `source_parse_error` | false | A `teams/*.tfvars` in `pt-logos@main`, `helpers.tofu` in `pt-corpus@main`/`pt-pneuma@main`, or `sidebars.js` in `pt-ekklesia-docs` failed to parse, validate, or matched an unsupported shape (e.g. missing `module "core_helpers"` block, `logos_workspaces` not a list of plain strings, missing `// region:` / `// endregion:` anchors). The source must be fixed; retrying will not help. |
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
