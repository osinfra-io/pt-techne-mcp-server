# Repository instructions for pt-techne-mcp-server

This file applies to every change in this repo. Read it together with the
platform- and techne-team instructions, both of which are loaded by Copilot
automatically:

- `pt-ai-context/.github/instructions/team.instructions.md` — platform-wide
  conventions (commits, PRs, labels, full-SHA action pins, "keep it simple").
- `techne/pt-techne-ai-context/.github/instructions/team.instructions.md` —
  techne team conventions.

## What this repo is

A small Go MCP server providing two deterministic tools:

- `validate_team_spec(spec)` — JSON Schema validation, structured errors.
- `render_team_tfvars(spec)` — validate then render canonical pt-logos
  `.tfvars` bytes.

Transport is stdio only. Distribution: static Go binary → GHCR container
image (public) plus GitHub release artifacts.

## Hard rules (specific to this repo)

- **Layout is fixed.** `cmd/pt-techne-mcp-server/`, `internal/{spec,render,tools,schemadoc}/`,
  `schema/team.schema.json`, `docs/`. **No `pkg/`** — everything internal.
- **One concept per file.** Reject `helpers.go`/`util.go`/`common.go`.
- **No new abstractions for "future flexibility."** Solve today's problem.
  Phase 2 tools will be added by appending one registration line in
  `main.go`, not by introducing a router or registry.
- **Total non-test Go LOC under ~1500.** Re-examine the design when nearing
  this.
- **No interfaces** until two implementations exist.
- **No `init()`, no globals.** Dependencies are passed explicitly through
  `main.go`.
- **Errors are values.** Wrap with `fmt.Errorf("…: %w", err)`. No panics in
  library code; `main.go` may `log.Fatal`.

## Dependencies

The allowed list is fixed. Adding a dependency requires explicit discussion.

- `github.com/modelcontextprotocol/go-sdk` — MCP transport + tool SDK.
- `github.com/santhosh-tekuri/jsonschema/v6` — JSON Schema validation.
- (phase 2) `github.com/google/go-github/v68` — GitHub PR creation.

Tests use the standard library only — no testify, no mocking framework.

## The schema is the contract

`schema/team.schema.json` is the single source of truth. The Go structs in
`internal/spec/spec.go` mirror it; if they diverge, **the schema wins** and
the structs follow.

A copy lives at `internal/spec/schema_embed.json` because `//go:embed` can't
traverse parent directories. CI fails when they drift; `make sync-schema`
updates the copy before committing.

## Renderer

`internal/render/render.go` is a hand-written emitter. Output must remain
byte-identical to the canonical pt-logos team files. Any intentional change
to formatting requires regenerating every golden under
`internal/render/testdata/golden/` (run `go test ./internal/render/... -update`)
and reviewing each diff manually.
