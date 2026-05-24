# Techne MCP Server

[![Tests](https://img.shields.io/github/actions/workflow/status/osinfra-io/pt-techne-mcp-server/go-test.yml?style=for-the-badge&logo=go&color=00ADD8&label=Tests)](https://github.com/osinfra-io/pt-techne-mcp-server/actions/workflows/go-test.yml)

Model Context Protocol (MCP) server providing platform context and tools to AI assistants. Exposes deterministic, typed tools so platform agents call a tested renderer instead of writing HCL by hand.

## Usage

### Docker (recommended)

Add this entry to your MCP client config (e.g. `.copilot/mcp.json`, `mcp.json`):

```json
{
  "mcpServers": {
    "pt-techne-mcp-server": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_TOKEN",
        "ghcr.io/osinfra-io/pt-techne-mcp-server:latest"
      ]
    }
  }
}
```

### Download binary

Pre-built binaries for Linux (amd64, arm64) are attached to each [GitHub release](https://github.com/osinfra-io/pt-techne-mcp-server/releases):

```sh
# Download the latest release for your architecture
gh release download --repo osinfra-io/pt-techne-mcp-server --pattern '*linux-amd64'
chmod +x pt-techne-mcp-server-linux-amd64
```

Then configure your MCP client:

```json
{
  "mcpServers": {
    "pt-techne-mcp-server": {
      "command": "/path/to/pt-techne-mcp-server-linux-amd64",
      "env": {
        "GITHUB_TOKEN": "<YOUR_TOKEN>"
      }
    }
  }
}
```

### Build from source

```sh
go install github.com/osinfra-io/pt-techne-mcp-server/cmd/pt-techne-mcp-server@latest
```

```json
{
  "mcpServers": {
    "pt-techne-mcp-server": {
      "command": "pt-techne-mcp-server",
      "env": {
        "GITHUB_TOKEN": "<YOUR_TOKEN>"
      }
    }
  }
}
```

### Configuration

Set `GITHUB_TOKEN` with access to `osinfra-io/pt-logos`, `osinfra-io/pt-corpus`, `osinfra-io/pt-pneuma`, and `osinfra-io/pt-ekklesia-docs`. Without it, GitHub-backed tools return `not_configured`; offline tools (`validate_team_spec`, `render_team_tfvars`, `render_team_docs_index`, `render_sidebar_patch`) still work.

For local development, `gh auth token` is the simplest option. See [docs/configuration.md](docs/configuration.md) for full details on token scopes, sources, and operational error codes.

## Tools

| Tool | Description |
|---|---|
| `validate_team_spec` | Validate a team spec against the JSON Schema |
| `render_team_tfvars` | Render a spec to canonical pt-logos `.tfvars` bytes |
| `open_team_pr` | Validate, render, and open/update a PR on pt-logos |
| `list_teams` | List all teams from pt-logos@main |
| `get_team` | Get a single team's spec and docs pages |
| `lookup_user` | Find every team/role where a user appears |
| `find_repo` | Find which team(s) own a repository |
| `render_corpus_helpers` | Insert a workspace into pt-corpus `helpers.tofu` |
| `render_pneuma_helpers` | Insert a workspace into pt-pneuma `helpers.tofu` |
| `render_team_docs_index` | Render a team's docs index page |
| `render_sidebar_patch` | Patch pt-ekklesia-docs `sidebars.js` with a team entry |
| `open_team_docs_pr` | Render docs + sidebar and open/update a PR on pt-ekklesia-docs |

All spec-accepting tools handle both JSON objects and JSON-encoded strings (for LLM double-encoding quirks). Write tools are idempotent — identical input returns `action: "noop"`.

## Local development

Requires Go 1.26.3.

```sh
go build ./cmd/pt-techne-mcp-server            # build the binary
go test -race ./...                             # run tests
golangci-lint run                               # lint (uses .golangci.yml)
```

Schema maintenance (run after editing `schema/team.schema.json`):

```sh
cp schema/team.schema.json internal/spec/schema_embed.json   # sync embedded copy
go run ./internal/schemadoc schema/team.schema.json docs/schema.md  # regenerate docs
```

## Documentation

| Document | Description |
|---|---|
| [`docs/contributing.md`](docs/contributing.md) | Repo layout, architecture, how to add a tool |
| [`docs/configuration.md`](docs/configuration.md) | Token setup, operational error codes, tool semantics |
| [`docs/schema.md`](docs/schema.md) | Generated reference for every team spec field |
| [`schema/team.schema.json`](schema/team.schema.json) | Single source of truth for the team spec |

## License

GPL-2.0. See [LICENSE](LICENSE).
