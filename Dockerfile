# Multi-stage build: cgo-disabled static Go binary in a scratch image.
# https://docs.docker.com/build/building/multi-stage/

FROM golang:1.25.8-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/pt-techne-mcp-server \
      ./cmd/pt-techne-mcp-server

FROM scratch
COPY --from=build /out/pt-techne-mcp-server /pt-techne-mcp-server
USER 1000:1000
ENTRYPOINT ["/pt-techne-mcp-server"]
