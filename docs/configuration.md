# Configuration

## `GITHUB_TOKEN`

Required by `open_team_pr`, the four pt-logos read tools (`list_teams`, `get_team`, `lookup_user`, `find_repo`), the two helpers renderers (`render_corpus_helpers`, `render_pneuma_helpers`), and `open_team_docs_pr`. Without it the server still serves `validate_team_spec`, `render_team_tfvars`, `render_team_docs_index`, and `render_sidebar_patch`; the GitHub-backed tools return a structured `not_configured` error.

The token must be scoped to **`osinfra-io/pt-logos`**, **`osinfra-io/pt-corpus`**, **`osinfra-io/pt-pneuma`**, and **`osinfra-io/pt-ekklesia-docs`** with the following permissions:

- On `pt-logos`: **Contents — read and write**, **Pull requests — read and write** (used by `open_team_pr` plus the four read tools).
- On `pt-corpus` and `pt-pneuma`: **Contents — read** (used by `render_corpus_helpers` and `render_pneuma_helpers` to fetch `helpers.tofu`).
- On `pt-ekklesia-docs`: **Contents — read and write**, **Pull requests — read and write** (used by `open_team_docs_pr` to land the docs index page + sidebars patch, and by `get_team` to read existing docs pages).

### Token sources

- **GitHub App** (recommended for non-interactive deployments). In the App's settings, repository permissions → Contents: Read and write, Pull requests: Read and write; repository access → Only select repositories → `osinfra-io/pt-logos`, `osinfra-io/pt-corpus`, `osinfra-io/pt-pneuma`, `osinfra-io/pt-ekklesia-docs`. Mint installation tokens with [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token) and pass the result as `GITHUB_TOKEN`.
- **Fine-grained PAT** (local development). At <https://github.com/settings/personal-access-tokens>, resource owner `osinfra-io`; repository access → Only select repositories → the four repos above; repository permissions → Contents: Read and write, Pull requests: Read and write.
- **`gh auth token`** works for local development as long as your account can push to a branch on `pt-logos` and `pt-ekklesia-docs` and open PRs against them, and read `pt-corpus` and `pt-pneuma`.
- **Classic PAT** is not recommended — the closest equivalent (`repo` scope) grants far more than the tool needs.

### Rotation

There is nothing to rotate inside the server — restart with a fresh `GITHUB_TOKEN`. Whatever produced the previous token is responsible for revoking it.

## Operational errors

All tools return structured MCP `isError` results:

- **Validation errors** — same `{valid: false, errors: [{path, message}]}` shape as `validate_team_spec` (write tools only).
- **Internal errors** — surfaced as plain MCP errors (the SDK's error path); reserved for things that should never happen.
- **Operational errors** — `{code, message, retryable}`:

  | `code` | `retryable` | Meaning |
  |---|---|---|
  | `not_configured` | false | `GITHUB_TOKEN` was empty at startup. Set it and restart. |
  | `not_found` | false | `get_team` was called with an unknown `team_key`. |
  | `invalid_input` | false | `lookup_user` was called with neither or both of `github_username` and `email`; or `render_corpus_helpers`/`render_pneuma_helpers` was called with a `team_key` that does not match the team-spec regex; or `render_sidebar_patch` was called without `current_sidebars_js`; or `find_repo` was called with an empty `name`. |
  | `docs_input_invalid` | false | `render_team_docs_index`, `render_sidebar_patch`, or `open_team_docs_pr` was given a spec the docs renderer can't use (missing `display_name_comment`, unknown `team_type`, `team_key` without a recognised prefix). |
  | `source_parse_error` | false | A `teams/*.tfvars` in `pt-logos@main`, `helpers.tofu` in `pt-corpus@main`/`pt-pneuma@main`, or `sidebars.js` in `pt-ekklesia-docs` failed to parse, validate, or matched an unsupported shape. The source must be fixed; retrying will not help. |
  | `render_failed` | false | A renderer produced an error (e.g. sidebar anchors missing, template execution failure). Check the message for details. |
  | `marshal_failed` | false | Internal JSON serialization failure. Should not occur under normal operation. |
  | `branch_diverged` | false | The team branch has diverged from `main` and an open PR exists; the tool refuses to rewrite history under a human's PR. Rebase or close the PR, then retry. (write tools only) |
  | `github_conflict` | true | A 409/422 from GitHub raced our write and didn't auto-reconcile. Retry. (write tools only) |
  | `github_api_error` | true | Other GitHub API failure (network, 5xx, unexpected 4xx). Retry; if persistent, check the GitHub status page and the token's permissions. |

  Codes are an enumerated set; agents may switch on them.

## Tool semantics

### Spec parameter handling

All tools that accept a `spec` parameter (`validate_team_spec`, `render_team_tfvars`, `open_team_pr`, `open_team_docs_pr`, `render_team_docs_index`, `render_sidebar_patch`) accept it as either a JSON object **or** a JSON-encoded string. This handles LLM serialization quirks where the spec is double-encoded during parallel tool calls.

### Read tools

`list_teams`, `get_team`, `lookup_user`, and `find_repo` are pure reads against `osinfra-io/pt-logos@main` over the GitHub API. Each call fetches fresh — no in-process caching — so results always reflect the current branch state.

Match semantics:

- `lookup_user` accepts exactly one of `github_username` or `email`. Matching is case-insensitive and exact. Returned `via` echoes which input matched. `system`/`scope`/`subject`/`membership` are structured so callers can filter without re-parsing.
- `find_repo` matches GitHub repository names case-sensitively. Empty `matches` is success, not an error.
- `get_team` returns `not_found` for an unknown `team_key`.

### Write tools

`open_team_pr` validates, renders, and opens-or-updates a PR on `osinfra-io/pt-logos` in one call. It is **idempotent on retry** — identical input + identical repo state returns `action: "noop"`.

`open_team_docs_pr` runs both docs renderers and lands them on `team-docs/<team_key>` in `osinfra-io/pt-ekklesia-docs` via the same idempotent transaction.

### Helpers renderers

`render_corpus_helpers` and `render_pneuma_helpers` fetch `helpers.tofu` from `osinfra-io/pt-corpus@main` and `osinfra-io/pt-pneuma@main` respectively, insert `<team_key>-main-production` into `logos_workspaces`, and return the canonical updated file bytes. They are **idempotent** and **byte-stable** on noop.

### Docs renderers

`render_team_docs_index` and `render_sidebar_patch` produce the two artifacts a new team needs in `osinfra-io/pt-ekklesia-docs`: a `docs/<section>/<team>/index.md` page and a patched `sidebars.js`. Both are byte-stable noops when the artifact already matches.
