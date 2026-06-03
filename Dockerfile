# Multi-stage build: cgo-disabled static Go binary in a scratch image.
# https://docs.docker.com/build/building/multi-stage/
#
# Runtime configuration (env vars; all optional):
#   GITHUB_TOKEN — enables open_team_pr (validate/render work without it).
#                  Any GH token works: PAT, gh auth token output, or an App
#                  installation token (e.g. from actions/create-github-app-token).
#                  See README "Configuration" for required permissions.

FROM golang:1.26.4-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/pt-techne-mcp-server \
      ./cmd/pt-techne-mcp-server

FROM scratch
WORKDIR /
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/pt-techne-mcp-server /pt-techne-mcp-server
USER 1000:1000
ENTRYPOINT ["/pt-techne-mcp-server"]
