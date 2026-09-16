# Phase 7: Containerization & CI/CD Pipeline - Pattern Map

**Mapped:** 2026-08-12
**Files analyzed:** 8 (2 new, 2 modified, 4 new/greenfield)
**Analogs found:** 4 in-repo analogs / 8 files (remaining 4 are greenfield infra with no in-repo analog — RESEARCH.md's Code Examples serve as their pattern source instead)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `docker-compose.yml` (`app:` service addition) | config | request-response (service definition, healthcheck-gated startup) | `docker-compose.yml`'s existing `postgres:` service (same file) | exact — same file, sibling service, same healthcheck/depends_on idiom |
| `.pre-commit-config.yaml` (golangci-lint hook addition) | config | event-driven (git pre-commit hook trigger) | `.pre-commit-config.yaml`'s existing `gitleaks` hook entry (same file) | exact — same file, sibling repo/hook block |
| `Dockerfile` (new) | config/build | file-I/O (multi-stage build, artifact copy between stages) | `Makefile`'s `web` target (build-then-embed convention) | role-match — Makefile target is the authoritative source of the exact build commands the Dockerfile's Node stage must reproduce |
| `.github/workflows/full-pipeline.yml` (new) | config/CI | event-driven (push/PR triggered pipeline) | `Makefile`'s `test-integration`, `sqlc-check` targets (commands to invoke, not reimplement) | role-match — CI steps call into these targets rather than duplicating their logic |
| `.dockerignore` (new) | config | file-I/O (build-context exclusion) | `.gitignore` (same repo, same exclusion intent) | role-match — mirrors what must never enter a build context/commit (`.env`, `node_modules/`, build artifacts) |
| `.golangci.yml` (new) | config | — (static config, no runtime data flow) | none in-repo (does not exist yet — Pitfall 1 in RESEARCH.md) | no analog — use RESEARCH.md's Code Examples v2-schema snippet |
| `.trivyignore` (new, conditional) | config | — (static allow-list config) | none in-repo (created only if/when a real unfixable finding requires it) | no analog — use RESEARCH.md's Code Examples format |
| `svu`/version bootstrap (`v0.1.0` git tag, D-04) | config/process | — (one-time manual git tag, not a file) | n/a | no analog — one-time `git tag v0.1.0 && git push --tags` step, not a file to pattern-match |

## Pattern Assignments

### `docker-compose.yml` (`app:` service addition)

**Analog:** the existing `postgres:` service in the same file (`C:/CodeProjects/drop-tracker/docker-compose.yml:1-23`)

**Full existing service (lines 1-23):**
```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: drop_tracker
      POSTGRES_PASSWORD: drop_tracker
      POSTGRES_DB: drop_tracker
    ports:
      # Published on 5433 rather than Postgres's default 5432 deliberately.
      # ...(see .planning/debug/resolved/notify-pass-hangs-forever.md)
      # Keep in lockstep with Makefile's TEST_DATABASE_URL and .env.example.
      - "5433:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U drop_tracker"]
      interval: 2s
      timeout: 5s
      retries: 10
```

**Pattern to copy:**
- **Comment style:** multi-line `#` comments directly above the field they explain, cross-referencing the Makefile/`.env.example`/debug doc when a value is non-obvious (port remap precedent) — D-12's `DATABASE_URL` override needs the same treatment, cross-referencing `.env.example` and this file's own port-remap comment.
- **Healthcheck idiom:** `test: [...]`, `interval`, `timeout`, `retries` keys — the `app` service's `depends_on: postgres: condition: service_healthy` gates on this exact healthcheck (no new pattern needed, just consume it).
- **Sibling-service indentation:** `app:` is added as a new top-level key under `services:`, same indentation level as `postgres:`.

**New service to add (per CONTEXT.md D-10/D-11/D-12, RESEARCH.md Code Examples):**
```yaml
  app:
    build: .
    env_file: .env
    environment:
      # .env's DATABASE_URL points at localhost:5433 (the host-mapped port `go run`
      # uses locally, per Makefile's TEST_DATABASE_URL comment). Inside the compose
      # network, the app must reach postgres via the service name and its internal
      # (unmapped) port — do NOT "fix" this back to match .env; see 07-CONTEXT.md D-12.
      DATABASE_URL: postgres://drop_tracker:drop_tracker@postgres:5432/drop_tracker?sslmode=disable
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
```

---

### `.pre-commit-config.yaml` (golangci-lint hook addition)

**Analog:** the existing `gitleaks` hook entry in the same file (`C:/CodeProjects/drop-tracker/.pre-commit-config.yaml:1-9`)

**Full existing file:**
```yaml
# Local secret scanning (CICD-10, gitleaks half). Mirrors the Phase 07 CI gitleaks job
# (CICD-02) so a secret is caught here, before it ever becomes a commit object, and again
# in CI as a non-bypassable backstop. The golangci-lint hook (the other half of CICD-10)
# is intentionally deferred to Phase 07 — its absence here is not an oversight.
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.30.1
    hooks:
      - id: gitleaks
```

**Pattern to copy:**
- `repos:` is a list — add a second `- repo:` entry, same indentation/structure as the existing gitleaks entry.
- `rev:` pins to an exact released tag (`v8.30.1` for gitleaks) — the new entry must pin `rev: v2.12.2` for golangci-lint, matching CLAUDE.md's locked version.
- The top-of-file comment block explicitly already anticipates this addition ("intentionally deferred to Phase 07") — update/remove that forward-reference comment once the hook is added, don't leave a stale "still deferred" note.
- Use hook id `golangci-lint` (changed-files-only, `--new-from-rev HEAD --fix`), not `golangci-lint-full` (whole repo) — matches the existing gitleaks hook's staged-diff-only scanning behavior (CONTEXT.md's discretion note + RESEARCH.md's Pitfall/Code Examples).

**Resulting file (per RESEARCH.md Code Examples):**
```yaml
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.30.1
    hooks:
      - id: gitleaks
  - repo: https://github.com/golangci/golangci-lint
    rev: v2.12.2
    hooks:
      - id: golangci-lint
```

---

### `Dockerfile` (new)

**Analog:** `Makefile`'s `web` target (`C:/CodeProjects/drop-tracker/Makefile:58-72`) — the authoritative sequence of build commands the Dockerfile's Node stage must reproduce exactly, and `Makefile`'s `test-integration`/`build` targets for the Go build convention.

**`web` target (lines 58-72) — the exact steps the Dockerfile's Node stage must mirror:**
```makefile
web:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm run build
	rm -rf internal/webassets/build/client
	mkdir -p internal/webassets/build
	cp -r web/build/client internal/webassets/build/client
```
Comment context (lines 58-66) explains *why*: `go:embed` needs `internal/webassets/build/client` populated before `go build`; Phase 7's Dockerfile "rebuilds this output in its own Node build stage rather than trusting the tree committed here."

**`build` target (line 24-25) — the exact Go build invocation to mirror (plain, non-static; Dockerfile adds `CGO_ENABLED=0`/`-trimpath`/`-ldflags` per D-01/D-03 needs):**
```makefile
build:
	go build -o ./bin/server ./cmd/server
```

**Pattern to copy:**
- Node stage: `pnpm install --frozen-lockfile` then `pnpm run build`, then copy `web/build/client` into `internal/webassets/build/client` — same relative paths as the Makefile target, just inside a Docker `COPY --from=` instead of `cp -r`.
- Go stage: build target is `./cmd/server`, same as `Makefile`'s `build` target — do not invent a different entrypoint path.
- No in-repo Dockerfile precedent exists (greenfield) — use RESEARCH.md's "Pattern 1: Node → Go → Alpine three-stage Dockerfile" code example directly (already verified against `go:embed` ordering requirements and this repo's actual Makefile/embed.go).
- D-02 non-root user syntax: `addgroup -g 10001 app && adduser -D -u 10001 -G app app` (fixed numeric UID/GID, alpine `addgroup`/`adduser` busybox applets).
- D-03 healthcheck target: hits the existing `/health` route (confirmed present at `internal/httpserver/health.go`) via `wget -qO-`.

---

### `.github/workflows/full-pipeline.yml` (new)

**Analog:** `Makefile`'s `test-integration` and `sqlc-check` targets — CI steps must invoke these, not reimplement their logic (per CONTEXT.md's "Established Patterns" and RESEARCH.md's Anti-Patterns).

**`test-integration` target (lines 39-40) — CI's test step must call this exact target, not bare `go test ./...`:**
```makefile
test-integration: db-up
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1 -p 1
```
The `-p 1` is load-bearing (comment lines 33-38 explain the shared-DB schema-reset race) — this is the folded-todo fix; CI must not regress it by running `go test ./...` directly.

**`sqlc-check` target (lines 54-56) — if CI needs a "no sqlc drift" gate, call this, don't hand-roll `sqlc generate && git diff`:**
```makefile
sqlc-check: sqlc-version-check
	sqlc generate
	git diff --exit-code -- internal/db/sqlc/
```

**Pattern to copy:**
- CI's `test` job step: `make test-integration` (which already runs `db-up` as a prerequisite — but note CI will likely run Postgres as a GitHub Actions service container instead of `docker compose up -d --wait postgres`; if so, either skip `db-up`'s dependency or ensure `TEST_DATABASE_URL` points at the CI-provided Postgres service, keeping the `-p 1` flag either way).
- CI's `lint` job step: no existing Makefile target for golangci-lint — use `golangci-lint-action@v9.3.0` directly per RESEARCH.md's Standard Stack table (SHA `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a`).
- No in-repo workflow file precedent exists (greenfield, `.github/workflows/` doesn't exist yet) — use RESEARCH.md's "Pattern 2: Build-once, scan, then conditionally push" code example as the structural template, including the exact pinned SHAs table (RESEARCH.md lines 144-157) and the `pull_request` vs `pull_request_target` guidance (Pitfall 3).

---

### `.dockerignore` (new)

**Analog:** `.gitignore` (`C:/CodeProjects/drop-tracker/.gitignore:1-33`) — same exclusion intent (never let build-irrelevant or secret-bearing paths into the context).

**Full existing `.gitignore`:**
```
# Go build artifacts
/bin/
/server
*.exe
*.test
*.out
coverage.out
coverage.html

# Go workspace files (local multi-module dev only; not committed)
go.work
go.work.sum

# Frontend build output (React + Vite, embedded via go:embed per CLAUDE.md)
node_modules/
dist/
web/build/
web/.react-router/

# Environment / secrets (CLAUDE.md: all secrets via env vars only, never committed)
.env
.envrc

# Editor / IDE
.vscode/
.idea/

# OS
.DS_Store
Thumbs.db

# GSD planning tool cache (local research API-response cache, not source)
.planning/research/.cache/
```

**Pattern to copy:**
- Mirror the same section groupings (build artifacts, workspace files, frontend output, secrets, editor/OS cruft) but adapt for Docker build-context semantics rather than git-tracking semantics.
- **Critical addition beyond `.gitignore`'s scope:** `.dockerignore` must exclude `.git/` (never needed in a build context, bloats context size) and must exclude `.env`/`.env.*` even more strictly than `.gitignore` does — RESEARCH.md's Anti-Patterns explicitly warns against `.env` leaking into an image layer (`internal/config/config.go`'s "process environment is the single source of truth" invariant). Also exclude `web/node_modules/` (matches `.gitignore`'s `node_modules/` but should be explicit for the Docker Node stage) and `bin/`.

---

## Shared Patterns

### Comment-heavy config discipline (repo-wide convention)
**Source:** `docker-compose.yml:9-16`, `Makefile:33-38,58-66`, `.pre-commit-config.yaml:1-4`
**Apply to:** All new/modified config files in this phase (docker-compose.yml, .pre-commit-config.yaml, Dockerfile, .golangci.yml, full-pipeline.yml)
This repo consistently explains *why* a non-obvious value exists directly above it, often cross-referencing another file (Makefile ↔ docker-compose.yml ↔ .env.example ↔ debug docs) rather than leaving tribal knowledge undocumented. New files in this phase should follow the same discipline — e.g., the `DATABASE_URL` override in `app:` needs the same treatment CONTEXT.md's D-12 already specifies in prose.

### Makefile as the single source of truth for build/test commands
**Source:** `Makefile:39-40` (`test-integration`), `Makefile:58-72` (`web`), `Makefile:54-56` (`sqlc-check`)
**Apply to:** `.github/workflows/full-pipeline.yml`, `Dockerfile`
CI steps and the Dockerfile's Node/Go stages must call into or exactly mirror these Makefile targets rather than reimplementing the logic inline — this is an explicit, repeated instruction in both CONTEXT.md ("Every prior phase's Makefile targets are the source of truth CI steps should call into (not reimplement)") and RESEARCH.md's Anti-Patterns section (bare `go test ./...` pitfall).

### Env-var-only secrets, never baked into an image or committed file
**Source:** `.gitignore:20-22` (`.env`/`.envrc` excluded), `docker-compose.yml`'s `app:` pattern (`env_file: .env`, D-11)
**Apply to:** `Dockerfile`, `.dockerignore`, `docker-compose.yml`
No Dockerfile `ENV`/`ARG` may hold a secret value; `.dockerignore` must exclude `.env*`; `docker-compose.yml`'s `app:` service loads secrets via `env_file: .env` (gitignored) with only the non-secret `DATABASE_URL` override committed as plain container-network DSN text (D-12) — not a credential leak since the same `drop_tracker:drop_tracker` dev-only creds already appear in `docker-compose.yml`'s existing `postgres:` service.

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md's Code Examples/Architecture Patterns instead):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.golangci.yml` | config | static | Does not exist anywhere in repo yet (RESEARCH.md Pitfall 1) — use RESEARCH.md's v2-schema minimal example (`version: "2"`, `linters: default: standard`, `run: timeout: 5m`) |
| `.github/workflows/full-pipeline.yml` | config/CI | event-driven | `.github/workflows/` directory doesn't exist yet — fully greenfield; use RESEARCH.md's Pattern 2 code example + pinned-SHA table as the structural source, since no in-repo CI workflow exists to pattern-match against |
| `.trivyignore` | config | static | Created only conditionally, if/when a real unfixable CRITICAL/HIGH finding requires it (D-08) — not pre-created; format given in RESEARCH.md's Code Examples section |
| `Dockerfile` | config/build | file-I/O | No Dockerfile exists anywhere in repo — closest available guidance is the Makefile's `web`/`build` targets (used above as partial analog for command sequence) combined with RESEARCH.md's fully worked Pattern 1 example |

## Metadata

**Analog search scope:** repo root (`docker-compose.yml`, `.pre-commit-config.yaml`, `.gitignore`), `Makefile`, `internal/httpserver/health.go` (confirmed `/health` route exists), `internal/webassets/embed.go` (referenced via RESEARCH.md, not re-read here — already verified there)
**Files scanned:** 7 (docker-compose.yml, .pre-commit-config.yaml, Makefile, .gitignore, .env.example [denied by permissions — content inferred from CONTEXT.md D-11/D-12 and Makefile comments], internal/httpserver/ directory listing, git status)
**Pattern extraction date:** 2026-08-12
