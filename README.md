# <img align="left" width="45" height="45" src="https://user-images.githubusercontent.com/1610100/201473670-e0e6bdeb-742f-4be1-a47a-3506309620a3.png"> Techne MCP Server

[![Go Test](https://img.shields.io/github/actions/workflow/status/osinfra-io/pt-techne-mcp-server/go-test.yml?style=for-the-badge&logo=github&color=2088FF&label=Go%20Test)](https://github.com/osinfra-io/pt-techne-mcp-server/actions/workflows/go-test.yml)

Model Context Protocol (MCP) server providing platform context and tools to AI assistants. Exposes deterministic, typed tools so platform agents call a tested renderer instead of writing HCL by hand.

## Tools

| Tool | Input | Output |
|---|---|---|
| `validate_team_spec` | `{spec: <object>}` | `{valid: bool, errors: [{path, message}]}` |
| `render_team_tfvars` | `{spec: <object>}` | `{tfvars: string}` (canonical pt-logos `.tfvars` bytes) |
| `open_team_pr` | `{spec: <object>, message?: string}` | `{pr_url, pr_number, branch, commit_sha, action}` |

`render_team_tfvars` validates first; on failure it returns an MCP `isError` result with the same structured errors as `validate_team_spec`.

`open_team_pr` validates, renders, and opens-or-updates a PR on `osinfra-io/pt-logos` in one call. It is **idempotent on retry** — identical input + identical repo state returns `action: "noop"`.

## Configuration

| Env var | Required by | Notes |
|---|---|---|
| `GITHUB_TOKEN` | `open_team_pr` | Pre-minted token with `contents:write` and `pull_requests:write` on `osinfra-io/pt-logos`. Source is up to the deployment: `gh auth token` locally, [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token) in workflows, or a mounted secret in containers. See [`docs/auth.md`](docs/auth.md) for the full token contract and operational error codes. |

Without `GITHUB_TOKEN` the server still serves `validate_team_spec` and `render_team_tfvars`; `open_team_pr` returns a structured `not_configured` error.

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
- [`docs/auth.md`](docs/auth.md) — token contract, operational error codes, and rotation notes.
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
