# pt-techne-mcp-server — internals

This is a tour for new contributors. Read it top to bottom before changing
code.

## Layout

```
cmd/pt-techne-mcp-server/   — main; wires SDK, registers tools, serves stdio
internal/spec/              — typed Team struct + JSON Schema validator
internal/render/            — canonical HCL emitter (pt-logos tfvars)
internal/github/            — narrow Client interface + go-github wrapper
internal/tools/             — thin MCP adapters around spec + render + github
internal/schemadoc/         — generator for docs/schema.md
schema/team.schema.json     — single source of truth for the team spec
docs/                       — human-readable documentation (some generated)
```

Hard rules:

- **One concept per file.** No `helpers.go`/`util.go`/`common.go`.
- **No `pkg/`** — everything is `internal/`.
- **Interfaces only with two implementations.** `internal/github.Client`
  is the first one in the repo: real go-github wrapper + in-memory
  fake under `internal/tools/open_team_pr_test.go`.
- **No `init()` and no globals.** Wire dependencies in `main.go`.
- **Total non-test Go LOC under ~1500 (soft); 1600 is the
  "stop and ask" threshold.**

## How the pieces fit

```
              ┌─────────────────────────┐
              │ schema/team.schema.json │  ← source of truth
              └────────────┬────────────┘
                           │ embed
                           ▼
            ┌──────────────────────────────┐
            │ internal/spec.Validator      │
            └──────────────────────────────┘
                           ▲
                           │ uses
            ┌──────────────────────────────┐         ┌──────────────────────────┐
            │ internal/tools.{Validate,    │ ──────► │ internal/render.Render   │
            │   Render, OpenTeamPR}        │         └──────────────────────────┘
            └──────────┬───────────────────┘
                       │ open_team_pr only
                       ▼
            ┌──────────────────────────────┐
            │ internal/github.Client       │  ← interface
            │   ├─ goClient (production)   │
            │   └─ fakeClient (tests)      │
            └──────────────────────────────┘
```

The schema is duplicated into `internal/spec/schema_embed.json` because
`//go:embed` cannot traverse parent directories. CI fails when the two
diverge; `make sync-schema` updates the copy.

## `open_team_pr` flow

`open_team_pr` is the only tool that mutates anything. It runs a
deterministic transaction:

1. Validate (shared validator).
2. Render (shared renderer) → bytes.
3. Find any open PR for `team/<team-key>` → `main`.
4. Resolve branch state via `CompareCommits`: missing → create from
   main; identical/ahead → reuse; behind → fast-forward; diverged with
   open PR → error; diverged without an open PR → reset (the branch is
   disposable when no human PR depends on it).
5. Read the file at the branch and at `main`. Two noop short-circuits:
   branch matches AND PR open; or main matches AND no PR.
6. Commit (with one 409/422 reconciliation retry that may collapse
   into a noop).
7. Reuse the open PR (no title/body edit — preserves human edits) or
   open a new one (with one 422 reconciliation that maps to
   `action: "updated"` if a parallel call won the race).

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
  golden `.tfvars`. **Run with `RENDER_UPDATE=1 go test ./internal/render/...`
  to regenerate goldens after intentional output changes.**
- **`internal/spec/*_test.go`** — table-driven validation cases.

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
