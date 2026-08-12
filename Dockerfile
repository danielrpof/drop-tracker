# syntax=docker/dockerfile:1
#
# Three-stage build: Node (SPA) -> Go (static binary) -> Alpine (runtime).
#
# This Dockerfile is the single build authority for drop-tracker's
# deployable image (API + scheduler + notifier + embedded SPA, PROJECT.md's
# single-binary/service architecture). It deliberately builds web/ itself
# rather than trusting the committed internal/webassets/build/client/ tree
# (07-CONTEXT.md D-10) — that tree exists solely so a Node-less clone can
# still `go build`/`go vet`/`go test ./...` (see Makefile's `web` target
# comment), it is excluded from this build's context by .dockerignore, and
# a broken Node/Go stage here fails loudly at `go build` instead of
# silently embedding stale committed assets.
#
# No ENV or ARG instruction anywhere in this file may carry a configuration
# value: internal/config/config.go documents the process environment as the
# single source of truth at runtime, and a baked value here would both
# break that invariant and risk a secret landing in a committed image layer
# (T-07-01).

# ---- Stage 1: build the SPA ----
FROM node:26-alpine3.24@sha256:aadf416b2cdce311a8811ba3f0608a61b77dbf997500e2eafe781b51f6a0b019 AS web-build
WORKDIR /src/web

# Node 26 no longer bundles corepack (removed starting with Node 25) —
# install pnpm explicitly, pinned to the version this repo's
# lockfileVersion: '9.0' lock file is actually verified against
# (07-RESEARCH.md Standard Stack / Pitfall "corepack removed").
RUN npm install -g pnpm@11.8.0

# Copy only the three lockfile-relevant inputs first so the dependency
# layer caches independently of application source changes, mirroring the
# Makefile `web` target's own build sequence.
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build

# ---- Stage 2: build the Go binary ----
FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS go-build
WORKDIR /src

# Copy module files first so `go mod download` caches independently of
# source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The SPA build output must exist here BEFORE `go build`, because
# //go:embed all:build/client (internal/webassets/embed.go) resolves at
# compile time, not at runtime. rm -rf + mkdir -p mirrors the Makefile
# `web` target's own reset step, so no stale hashed asset filenames from
# any previously-committed tree survive into the embedded FS.
RUN rm -rf ./internal/webassets/build/client && \
    mkdir -p ./internal/webassets/build
COPY --from=web-build /src/web/build/client ./internal/webassets/build/client

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
    -o /out/server ./cmd/server

# ---- Stage 3: minimal non-root runtime ----
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

# Mandatory, not optional: the CGO_ENABLED=0 binary reads the system
# certificate pool, and plain alpine ships none. Without this, every
# outbound HTTPS call to MusicBrainz, Deezer and the Discord webhook fails
# the TLS handshake at runtime even though /health still reports healthy
# (T-07-03). Never work around a certificate failure by disabling TLS
# verification.
RUN apk add --no-cache ca-certificates

# Fixed numeric UID/GID (07-CONTEXT.md D-02) — deterministic across
# rebuilds, referenceable later if a VPS/orchestrator SecurityContext is
# added. Not an auto-assigned UID (T-07-02).
RUN addgroup -g 10001 app && adduser -D -u 10001 -G app app

COPY --from=go-build /out/server /usr/local/bin/server

USER 10001:10001
EXPOSE 8080

# Shell form so the container's shell expands HTTP_PORT at runtime
# (07-CONTEXT.md D-03). busybox wget exits non-zero on the /health
# handler's 503 response, which is exactly what makes container health
# track real database health (internal/httpserver/health.go).
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${HTTP_PORT:-8080}/health" || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
