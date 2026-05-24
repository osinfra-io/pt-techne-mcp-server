# Repository instructions for pt-techne-mcp-server

This file applies to every change in this repo. Read it together with the
platform- and techne-team instructions, both of which are loaded by Copilot
automatically:

- `pt-ai-context/.github/instructions/team.instructions.md` — platform-wide
  conventions (commits, PRs, labels, full-SHA action pins, "keep it simple").
- `techne/pt-techne-ai-context/.github/instructions/techne-team.instructions.md` —
  techne team conventions.

## What this repo is

A Go MCP server (stdio transport) that provides deterministic tools for
platform team operations. Distribution: static binary via GitHub releases,
GHCR container image.

### Tools

| Tool | Purpose | Mutating |
|------|---------|----------|
| `validate_team_spec` | JSON Schema validation, structured errors | No |
| `render_team_tfvars` | Validate then render canonical pt-logos `.tfvars` | No |
| `render_corpus_helpers` | Render pt-corpus helpers `.tfvars` | No |
| `render_pneuma_helpers` | Render pt-pneuma helpers `.tfvars` | No |
| `render_team_docs_index` | Render Docusaurus docs index page | No |
| `render_sidebar_patch` | Render sidebars.js category patch | No |
| `list_teams` | List all teams from pt-logos | No |
| `get_team` | Get a single team's spec | No |
| `find_repo` | Find repository owner/team | No |
| `lookup_user` | Look up user across GitHub + Datadog | No |
| `open_team_pr` | Open/update a PR on pt-logos (idempotent) | Yes |
| `open_team_docs_pr` | Open/update a PR on pt-ekklesia-docs | Yes |

All tools require `GITHUB_TOKEN`. Without it, read tools return
`not_configured`; write tools do the same.

## Hard rules

- **Layout is fixed.** `cmd/pt-techne-mcp-server/`,
  `internal/{spec,render,github,tools,helpersrender,schemadoc}/`,
  `schema/`, `docs/`. **No `pkg/`** — everything internal.
- **One concept per file.** No `helpers.go`/`util.go`/`common.go`.
- **Interfaces only with two+ implementations.** Today: just
  `internal/github.Client` (production wrapper + in-memory test fake).
- **No `init()`, no globals.** Dependencies passed explicitly through
  `main.go`.
- **Errors are values.** Wrap with `fmt.Errorf("…: %w", err)`. No panics in
  library code; `main.go` may `log.Fatal`.
- **Tests use stdlib only** — no testify, no mocking framework.

## Dependencies

The allowed list is fixed. Adding a dependency requires explicit discussion.

- `github.com/modelcontextprotocol/go-sdk` — MCP transport + tool SDK
- `github.com/santhosh-tekuri/jsonschema/v6` — JSON Schema validation
- `github.com/google/go-github/v68` — GitHub API client
- `golang.org/x/oauth2` — static `TokenSource` for `GITHUB_TOKEN`
- `github.com/hashicorp/hcl/v2` + `github.com/zclconf/go-cty` — HCL2
  round-trip parser for `.tfvars`
- `golang.org/x/sync` — `errgroup` for bounded concurrent fetches

## Schema

`internal/spec/schema_embed.json` is the canonical schema (used via
`//go:embed`). `schema/team.schema.json` is a symlink to it for
repo-level discoverability. The Go structs in `internal/spec/spec.go`
mirror the schema; if they diverge, **the schema wins**.

After editing the schema, `pre-commit` auto-regenerates `docs/schema.md`.

## Renderers

- **`internal/render/render.go`** — tfvars emitter. Output must be
  byte-identical to canonical pt-logos team files. Regenerate goldens:
  `RENDER_UPDATE=1 go test ./internal/render/...`
- **`internal/render/docs/`** — Docusaurus markdown emitter for team
  docs index pages. Goldens: `DOCS_RENDER_UPDATE=1 go test ./internal/render/docs/...`
- **`internal/render/sidebar/`** — sidebars.js patcher. Uses
  `json.Marshal` for JS-safe category block values.
- **`internal/helpersrender/`** — corpus/pneuma helpers tfvars emitter.

## Pre-commit hooks

All validation runs locally via `pre-commit run -a`:

- Standard checks (yaml, whitespace, symlinks)
- `golangci-lint-full` — same linter config and version as CI
- `schemadoc` — regenerates `docs/schema.md` when schema changes
- `go-test` — runs `go test ./...`

## CI

- **go-test.yml** — lint (golangci-lint via goinstall SHA pin), build,
  test with race detector + coverage
- **release.yml** — on tag push: build multi-arch binary, push GHCR image
