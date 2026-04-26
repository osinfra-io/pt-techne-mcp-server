# pt-techne-mcp-server

Model Context Protocol (MCP) server providing platform context and tools to AI
assistants. Exposes deterministic, typed tools that platform agents call instead
of writing HCL by hand.

## Why

The pt-logos team-management agent used to generate `.tfvars` from a prose
schema in `example.tfvars`. Two problems:

1. The agent wrote HCL from prose → style drift and prompt regressions were
   inevitable.
2. The team spec lived in two places: the prose comments in `example.tfvars`
   and the agent's own prompt.

This server fixes both. The schema becomes one machine-readable JSON Schema
artifact (`schema/team.schema.json`); HCL is produced by a tested, deterministic
renderer that matches the canonical pt-logos style byte-for-byte.

## Tools

| Tool | Input | Output |
|---|---|---|
| `validate_team_spec` | `{spec: <object>}` | `{valid: bool, errors: [{path, message}]}` |
| `render_team_tfvars` | `{spec: <object>}` | `{tfvars: string}` (canonical pt-logos `.tfvars` bytes) |
| `open_team_pr` | `{spec: <object>, message?: string}` | `{pr_url, pr_number, branch, commit_sha, action}` |

`render_team_tfvars` validates first; on failure it returns an MCP isError
result with the same structured errors as `validate_team_spec`.

`open_team_pr` validates, renders, and opens-or-updates a PR on
`osinfra-io/pt-logos` in one call. It is **idempotent on retry**: identical
input + identical repo state returns `action: "noop"`. Requires
`GITHUB_TOKEN` (see [Configuration](#configuration)); without it the
first two tools work and `open_team_pr` returns a structured
`not_configured` error.

## Configuration

| Env var | Required by | Notes |
|---|---|---|
| `GITHUB_TOKEN` | `open_team_pr` | Pre-minted token with `contents:write` and `pull_requests:write` on `osinfra-io/pt-logos`. Source is up to the deployment: `gh auth token` locally, [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token) in workflows, or a mounted secret in containers. See [`docs/auth.md`](docs/auth.md) for the full token contract and operational error codes. |

The codespace and the logos agent are responsible for setting this in
their respective environments (codespace wiring is tracked in its own
issue).

## Run

### Container (recommended)

```sh
docker run -i --rm ghcr.io/osinfra-io/pt-techne-mcp-server:v0.1.0
```

### Local binary

```sh
go install github.com/osinfra-io/pt-techne-mcp-server/cmd/pt-techne-mcp-server@latest
pt-techne-mcp-server
```

## Configure as an MCP server

Add this entry to your MCP client config (e.g. `.copilot/mcp.json`,
`mcp.json`, etc.):

```json
{
  "mcpServers": {
    "platform": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "ghcr.io/osinfra-io/pt-techne-mcp-server:v0.1.0"
      ]
    }
  }
}
```

The pt-logos team-management agent calls `platform/render_team_tfvars` instead
of writing HCL itself, then `platform/open_team_pr` to ship the change as a
PR. Agents that only need to validate user input call
`platform/validate_team_spec`.

## Documentation

- [`docs/README.md`](docs/README.md) — repo layout, how to add a tool, renderer
  overview.
- [`docs/schema.md`](docs/schema.md) — generated reference for every team spec
  field. Regenerate with `make schema-docs`.
- [`schema/team.schema.json`](schema/team.schema.json) — single source of truth.

## Develop

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
