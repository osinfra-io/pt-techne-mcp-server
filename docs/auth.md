# Authentication & operational errors for `open_team_pr`

This is a short ADR plus the operational error reference for the
`open_team_pr` tool.

## Decision: pre-minted token, not in-process App-token minting

`open_team_pr` accepts a single env var, `GITHUB_TOKEN`, and uses it as
a static `oauth2.TokenSource`. It does **not** mint installation tokens
itself.

### Why

- **Smaller surface.** No JWT, no installation cache, no mutex, no
  401-retry, no third-party auth dep — about ~110 fewer LOC. Helps us
  stay close to the ~1500 non-test LOC budget.
- **Token rotation is the deployment's job, not ours.** The right
  rotator depends on where the server runs:
  - GitHub Actions →
    [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token)
    (already in use elsewhere in `osinfra-io`, e.g. the
    `add-to-project` reusable workflow).
  - Local development → `gh auth token` (your personal credential, which
    you probably already have).
  - Long-running deployments → a sidecar / init container that mints
    and renews the token.
- **No secrets in the binary.** The server never sees a private key,
  app id, or installation id. If the host is compromised, the blast
  radius is one short-lived token, not a forgeable identity.

### Trade-off

Long-running deployments need an external rotator; we don't auto-renew.
If that becomes painful, we'll revisit and add an optional minter
behind the same `gh.Client` interface — both implementations would
satisfy the existing surface. Until that pain is real, we don't pay
for it.

## Required token capabilities

Whatever produces the token must scope it to `osinfra-io/pt-logos` with:

- `contents: write` — to commit the rendered tfvars to a branch.
- `pull_requests: write` — to open or update the PR.

Minimum viable scopes; the tool needs no organization-level permissions.

## Operational error codes

`open_team_pr` returns three flavors of MCP `IsError` result:

1. **Validation errors** — same JSON shape as `validate_team_spec`'s
   `{valid: false, errors: [...]}`. Don't change behavior on these
   without coordinating with the validate tool's consumers.
2. **Operational errors** — small structured body:

   ```json
   {"code": "...", "message": "...", "retryable": <bool>}
   ```

3. **Internal errors** — surfaced as plain MCP errors (the SDK's
   error path), reserved for things like JSON re-marshal failures that
   should never happen.

### Operational error codes

| `code` | `retryable` | Meaning | Action |
|---|---|---|---|
| `not_configured` | false | `GITHUB_TOKEN` was empty at server startup. | Configure the env var; restart. |
| `branch_diverged` | false | The team branch has diverged from `main` and an open PR exists. The tool refuses to rewrite history under a human's PR. | Rebase or close the PR, then retry. |
| `github_conflict` | true | A 409/422 from GitHub raced our write and didn't reconcile to a known good state. | Retry. |
| `github_api_error` | true | Any other GitHub API failure (network, 5xx, unexpected 4xx). | Retry; if persistent, check the GitHub status page and the token's scopes. |

Codes are an enumerated set; agents may switch on them. Add new codes
sparingly and never reuse old ones with new meanings.

## Rotating the token

There is nothing to rotate inside the server — restart with a fresh
`GITHUB_TOKEN`. Whatever produced the previous token is responsible for
revoking it (e.g. close the GitHub App installation, regenerate the
PAT, etc.).