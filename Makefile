# pt-techne-mcp-server developer Makefile.
# Targets: build, test, lint, run, schema-docs, sync-schema.

SHELL := /bin/bash
BIN   := pt-techne-mcp-server

.PHONY: all build test lint run schema-docs sync-schema clean

all: lint test build

build:
	go build -o bin/$(BIN) ./cmd/$(BIN)

test:
	go test -race ./...

lint:
	go vet ./...
	@command -v staticcheck >/dev/null || { echo "install staticcheck: go install honnef.co/go/tools/cmd/staticcheck@latest"; exit 1; }
	staticcheck ./...
	@diff -u <(echo -n) <(gofmt -l .) || { echo "gofmt: files need formatting (run 'gofmt -w .')"; exit 1; }

run: build
	./bin/$(BIN)

# Regenerate docs/schema.md from schema/team.schema.json.
schema-docs:
	go run ./internal/schemadoc schema/team.schema.json docs/schema.md

# The Go embed directive cannot traverse parent directories, so the schema
# is duplicated into internal/spec/schema_embed.json. This target keeps them
# in sync; CI calls this with --check to fail when they drift.
sync-schema:
	cp schema/team.schema.json internal/spec/schema_embed.json

clean:
	rm -rf bin/
