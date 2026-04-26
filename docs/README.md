# pt-techne-mcp-server — internals

This is a tour for new contributors. Read it top to bottom before changing
code.

## Layout

```
cmd/pt-techne-mcp-server/   — main; wires SDK, registers tools, serves stdio
internal/spec/              — typed Team struct + JSON Schema validator
internal/render/            — canonical HCL emitter (pt-logos tfvars)
internal/tools/             — thin MCP adapters around spec+render
internal/schemadoc/         — generator for docs/schema.md
schema/team.schema.json     — single source of truth for the team spec
docs/                       — human-readable documentation (some generated)
```

Hard rules:

- **One concept per file.** No `helpers.go`/`util.go`/`common.go`.
- **No `pkg/`** — everything is `internal/`.
- **No interfaces** until there are two implementations.
- **No `init()` and no globals.** Wire dependencies in `main.go`.
- **Total non-test Go LOC under ~1500.** If a change pushes us past that,
  reconsider the design before merging.

## How the pieces fit

```
              ┌─────────────────────────┐
              │ schema/team.schema.json │  ← source of truth
              └────────────┬────────────┘
                           │ embed
                           ▼
            ┌──────────────────────────────┐
            │ internal/spec.Validator      │  ← validates raw JSON
            └──────────────────────────────┘
                           ▲
                           │ uses
            ┌──────────────────────────────┐
            │ internal/tools.{Validate,    │  ← MCP adapters
            │   Render}(server, validator) │
            └──────────┬───────────────────┘
                       │ on validate-ok, render
                       ▼
            ┌──────────────────────────────┐
            │ internal/render.Render       │  ← deterministic HCL emitter
            └──────────────────────────────┘
```

The schema is duplicated into `internal/spec/schema_embed.json` because
`//go:embed` cannot traverse parent directories. CI fails when the two
diverge; `make sync-schema` updates the copy.

## How to add a new MCP tool

Three files, no router abstraction:

1. **Pure function** in `internal/<package>/` for the actual behavior.
2. **Adapter** in `internal/tools/<tool_name>.go` that decodes input,
   calls the function, returns the typed output.
3. **Registration** in `cmd/pt-techne-mcp-server/main.go` — add one line:
   `tools.MyTool(server, deps...)`.

Use a typed input/output struct with `jsonschema:"…"` field tags. The MCP
SDK auto-derives the JSON Schema from the type — keep the struct shallow
(no `json.RawMessage`, no embedded interfaces) so the derived schema is
clean.

## Renderer

`internal/render/render.go` is a hand-written emitter, not a `text/template`.
The reason: the canonical pt-logos style requires per-block alignment of `=`
signs across heterogeneous keys, which is awkward in Go templates. The emitter
uses a single `bytes.Buffer` and a tiny `writer` helper. Maps are walked in
sorted key order so output is deterministic.

Field order inside a team body is enumerated explicitly by `emitTeamBody`.
That function is the contract: if you add a new top-level team field, add it
there in the right alphabetical position.

## Tests

- **`internal/render/testdata/parity/*.json`** — hand-authored JSON spec for
  each real pt-logos team. The test renders each one and compares to a
  golden `.tfvars`. **Run with `go test ./internal/render/... -update` to
  regenerate goldens after intentional output changes.**
- **`internal/spec/*_test.go`** (TBA) — table-driven validation cases.

The parity fixtures are the regression net. If you change the renderer, run
the parity test and inspect every golden diff.

## Releasing

Tag, push, done.

```sh
git tag v0.1.1
git push origin v0.1.1
```

The release workflow builds `linux/{amd64,arm64}` binaries, attaches them
to a GitHub release, and pushes a multi-arch image to
`ghcr.io/osinfra-io/pt-techne-mcp-server:<tag>` and `:latest`.
