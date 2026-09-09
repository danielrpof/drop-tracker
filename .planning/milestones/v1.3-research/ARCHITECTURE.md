# Architecture Research: v1.3 Continuous Deployment

**Domain:** Integration architecture for 3 features (VPS SSH deploy + rollback, passphrase gate middleware, coverage-diff PR comment) into an existing single-binary Go + React Router SPA release tracker shipped through a GitHub Actions pipeline
**Researched:** 2026-08-27
**Confidence:** HIGH for repo integration points (verified directly against current source); MEDIUM for external tool semantics (golang-migrate `Up()` on an ahead-of-source schema, GitHub Actions cache eviction timing)

This is **integration analysis, not greenfield ecosystem research.** Each section maps a v1.3 feature onto the exact files/patterns it touches, states what is genuinely NEW vs. a small extension of an existing pattern, and calls out the ordering/data-integrity constraints specific to this codebase.

---

## System Overview (current, pre-milestone)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     GitHub Actions: full-pipeline.yml                    │
│  push + pull_request                                                     │
│                                                                         │
│   [vet] [lint] [test] [gitleaks] [trivy-fs] [frontend-test] [pr-title]   │
│        └────────────────────┬───────────────────────┘                    │
│                        needs: ▼  (parallel gate tier)                    │
│                     [build-scan]  ── build image, Trivy image scan,      │
│                          │           upload scanned-image tar (main only)│
│                     needs: ▼                                             │
│                      [release]   ── svu next → docker push               │
│                                     ghcr.io/danielrpof/drop-tracker:vX,  │
│                                     syft SBOM, git tag                   │
│                       (if: push && ref == refs/heads/main)               │
└─────────────────────────────────────────────────────────────────────────┘
                                    │  (nothing consumes the published image yet)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                cmd/server/main.go — single process (one image)           │
│  run(ctx):                                                               │
│   config.Load()  →  logging.New  →  db.RunMigrations(ctx)  BOOT-TIME,     │
│                                     bounded retry, ErrNoChange = success  │
│   db.NewPool(ctx, dsn, mbWorkers+dzWorkers)                              │
│   ┌───────────────────────────┬───────────────────────────────────────┐  │
│   │ internal/httpserver        │ internal/poller (robfig/cron, 2 entries)│  │
│   │ chi router:                │ CAS overlap guards, bounded worker pool │  │
│   │  RequestID → echoRequestID │  → detection → notifier → Discord      │  │
│   │  → httplog → Recoverer     │                                        │  │
│   │  routes: /health /search   │ internal/artistart.Backfill goroutine  │  │
│   │  /watchlist /events        │                                        │  │
│   │  NotFound → webassets SPA   │                                        │  │
│   └───────────────────────────┴───────────────────────────────────────┘  │
│   internal/db/sqlc over pgx/v5 pool → Postgres (watchlist, artists,      │
│                                                  events seen-store)      │
└─────────────────────────────────────────────────────────────────────────┘

Local dev only: docker-compose.yml runs `app` (build: .) + `postgres:16`.
No production deploy target exists today.
```

### Component Responsibilities (relevant to v1.3)

| Component | Owns today | v1.3 change |
|-----------|-----------|-------------|
| `.github/workflows/full-pipeline.yml` | Whole CI/CD graph via `needs:` | **NEW** `deploy` job (`needs: release`); **NEW** `coverage-comment` job (`needs: [test, frontend-test]`); **MOD** `release` gains a `version` output; **MOD** `test` + `frontend-test` publish coverage artifacts / cache |
| `internal/httpserver/server.go` | chi router + 4-middleware chain + route registration | **MOD** add gate to a protected route `Group`; register `POST /session`; `New(...)` gains an auth option |
| `internal/config/config.go` | Env-only config struct (`caarlos0/env`) | **MOD** add `INSTANCE_PASSPHRASE` (+ optional `SESSION_TTL`) |
| `cmd/server/main.go` | Boot sequencing incl. `db.RunMigrations` | **MOD** one line: pass `httpserver.WithAuthGate(cfg.InstancePassphrase)` |
| `internal/db/migrate.go` + `migrations/*.sql` | Boot-time `migrate.Up()`, embedded | **NO code change**; new migrations must follow expand/contract discipline (see §Deploy) |
| `web/app/` (React SPA) | Watchlist / search / history UI, API client `lib/api.ts` | **MOD** handle `401`, render passphrase screen, `POST /session` |
| VPS (new host) | — | **NEW** holds prod compose file, real `.env`, the pinned image tag |

---

## Feature 1 — VPS SSH deploy job with `/health`-gated rollback

### Where it sits in the `needs:` graph

```
build-scan ──▶ release ──▶ deploy
                            (if: github.event_name == 'push' && github.ref == 'refs/heads/main')
```

- **`needs: [release]`** — deploy consumes the versioned image `release` just pushed to ghcr.io. It cannot start earlier: before `release` there is no published tag to pull.
- **Branch/event guard:** identical to `release` — `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`. Never runs on PRs, never on tag pushes (the `release` job's own `git push origin "$VERSION"` would otherwise re-trigger the workflow on the tag ref; that run's `deploy` is filtered out by the `refs/heads/main` check).
- **`concurrency:`** — **NEW dedicated group**, mirroring `release`'s pattern:
  ```yaml
  concurrency:
    group: deploy-${{ github.ref }}
    cancel-in-progress: false   # queue, never cancel
  ```
  `cancel-in-progress: false` is mandatory here: a deploy interrupted mid-`up -d` or mid-rollback leaves the VPS in an unknown state. Two rapid merges (v1.3.0 then v1.3.1) queue — v1.3.1 deploys strictly after v1.3.0 finishes (or rolls back). The top-level workflow `concurrency` block already does **not** cancel on `push`, so this only adds job-level serialization.
- **GitHub Environment:** **NEW** `environment: production`. This is the right home for the SSH secrets (`VPS_SSH_HOST`, `VPS_SSH_USER`, `VPS_SSH_KEY`, `VPS_SSH_KNOWN_HOSTS`) rather than repo secrets, because:
  - environment protection rules can restrict deployment to `main` and optionally require manual approval / a wait timer;
  - the GitHub UI gets a real deployment history + rollback visibility;
  - secrets are only exposed to jobs that name the environment.
- **`permissions:`** — the job needs **no** elevated `GITHUB_TOKEN` scope. The drop-tracker package is **public and pullable without auth** (PROJECT.md), so the VPS `docker compose pull` needs no registry login. Set `permissions: {}` (or `contents: read` only if the job checks out the repo for the compose file — see below). If the package is ever made private, add a long-lived GHCR read PAT to the VPS `.env` and `docker login ghcr.io` once during host bootstrap — not to the workflow.
- **Passing the version:** `release` currently writes `VERSION` only to `$GITHUB_ENV` (job-local). **MOD:** also emit it as a job output:
  ```yaml
  release:
    outputs:
      version: ${{ steps.svu.outputs.version }}
  ```
  and in the svu step `echo "version=$NEXT" >> "$GITHUB_OUTPUT"`. `deploy` then pins `needs.release.outputs.version`. (Alternative: `deploy` runs `svu current` after checkout since the tag is now pushed — but reading the upstream output is race-free and explicit.)

### What lives ON the VPS vs. in the repo

| Artifact | Lives where | Notes |
|----------|-------------|-------|
| Production compose file (`deploy/compose.prod.yaml`) | **Repo**, rsync'd to the VPS each deploy | The existing `docker-compose.yml` is dev-only (`build: .`). Prod file uses `image: ghcr.io/danielrpof/drop-tracker:${DROP_TRACKER_TAG:?}`, a `postgres:16` service with a named volume, `restart: unless-stopped`, and **no** `build:` / no host port for Postgres. Shipping it from the repo keeps compose changes versioned with code. |
| Real `.env` (secrets) | **VPS only**, `chmod 600`, owned by deploy user | `DATABASE_URL` (real password), `DISCORD_WEBHOOK_URL`, `INSTANCE_PASSPHRASE`, `MUSICBRAINZ_USER_AGENT` (real contact). Never in the repo, never rsync'd, never echoed. gitleaks already backstops accidental commits. |
| The pinned image tag | **VPS only**, in its own one-line file | Keep it **out of** the secret `.env` so the rollback rewrite touches a file with nothing sensitive in it. E.g. `deploy/current-tag` containing `v1.3.0`, consumed via `docker compose --env-file <(echo DROP_TRACKER_TAG=$(cat current-tag)) ...` or a dedicated `deploy.env` with `env_file: [.env, deploy.env]` in the compose file. |
| Previous tag + image digest | **VPS only**, written by the deploy script | `deploy/previous-tag`; also capture the running container's resolved image digest (`docker inspect`) so rollback targets an immutable digest, not a mutable tag. |
| Reverse proxy / TLS (Caddy, nginx, or Traefik) | **VPS only**, configured once | The session cookie must be `Secure` → the public origin must be HTTPS. This is a host-bootstrap prerequisite, not part of the deploy job. |

### How the deploy step updates the pinned tag atomically

Write-to-temp-then-rename (rename is atomic within a filesystem):

```sh
# inside the SSH session, cwd = deploy dir on the VPS
NEW_TAG="$1"
PREV_TAG="$(cat current-tag 2>/dev/null || true)"
PREV_DIGEST="$(docker inspect --format '{{.Image}}' drop-tracker-app 2>/dev/null || true)"

printf '%s\n' "$NEW_TAG" > current-tag.next && mv current-tag.next current-tag
printf '%s\n' "$PREV_TAG" > previous-tag
```

The `mv` is the commit point. If the script dies before it, the old tag file is untouched and the running container is unchanged.

### How `/health`-gated rollback works with a single compose service

There is one rollable service (`app`); `postgres` is never touched by a deploy.

```sh
docker compose pull app                       # fails fast if the tag doesn't exist in ghcr.io
docker compose up -d app                       # recreate ONLY app; postgres stays up

ok=""
for _ in $(seq 1 30); do
  sleep 2
  if curl -fsS "http://127.0.0.1:8080/health" | grep -q '"status":"ok"'; then ok=1; break; fi
done

if [ -z "$ok" ]; then
  # roll the tag file back and recreate from the previous image (still in local cache)
  printf '%s\n' "$PREV_TAG" > current-tag.next && mv current-tag.next current-tag
  docker compose up -d app
  # re-poll /health to confirm the previous image recovered; alert if not
  exit 1
fi
```

Mechanics and limits:
- `docker compose up -d app` with a changed tag recreates just that container; Compose leaves the previous image in the local store, so **rollback needs no re-pull** (do not run `docker image prune` in the deploy path).
- **Poll from inside the SSH session** (`127.0.0.1`), not from the GitHub runner — the app port is typically firewalled to localhost + the reverse proxy.
- **What `/health` actually gates:** `internal/httpserver/health.go` returns `503` iff `db.Ping` fails; `200 {"status":"ok","db":"up"}` otherwise. So the gate reliably catches:
  - a container that crash-loops before binding the listener (a **failed boot-time migration** returns a non-nil error from `run()` → `os.Exit(1)` → container never answers → curl fails → rollback);
  - Postgres unreachable from the new container.
  It does **not** catch a migration that *succeeds* but ships a logic regression — `/health` still returns `200`. This is an accepted coarse gate for v1.3; a deeper readiness check or a `/version` assertion is a later refinement.
- **Digest vs. tag on rollback:** prefer recreating from `PREV_DIGEST` (pin `image: ...@sha256:...` transiently) rather than `PREV_TAG`, in case a tag was force-repushed. Tag-based rollback is acceptable given `release` only ever pushes immutable `vX.Y.Z` tags today.
- **First deploy:** `previous-tag` / `PREV_DIGEST` are empty → script skips rollback and just `exit 1` on health failure. Seed `current-tag` during host bootstrap.

### Boot-time migrations × rollback — the expand/contract constraint

This is the sharpest data-integrity constraint of the milestone.

- Migrations run at **boot**, embedded via `//go:embed migrations/*.sql`, `migrate.Up()` only (never `Down`).
- `RunMigrations` already treats `migrate.ErrNoChange` as success.
- **On rollback, the older image boots against the newer schema.** Its embedded `migrations/` set stops at version N; the DB is at N+1. `migrate.Up()` asks the source for the migration after the current DB version, the source driver returns `os.ErrNotExist` (N+1 isn't in the older image's index at all), and `Up()` returns `ErrNoChange` — **not** an error. (The `"no migration found for version X"` error people hit is on the `Down`/`Steps(-n)` path, which drop-tracker never runs.) So the older binary starts cleanly *as long as it can still operate the schema it finds.*
- Therefore **every v1.3+ migration must be backward-compatible for at least one release**:
  - additive only within a release: new **nullable** columns (or `DEFAULT`ed), new tables, new indexes;
  - **no** `DROP COLUMN`, `RENAME`, type narrowing, or `ADD COLUMN ... NOT NULL` without default in the same release that the code starts depending on it;
  - destructive changes are split: **expand** in vN (add new column, backfill, dual-read/dual-write), **contract** in vN+M (drop the old column) — and the contract release ships only once vN is confirmed stable, because the rollback target must be ≥ the expand release.
  - Precedent already in the tree: `000007_backfill_events_watched_artist_name` is an expand-style data backfill.
- The deploy job's `needs: release` position means this discipline starts with the **first v1.3 migration**, regardless of which phase lands the deploy job — once auto-rollback exists, a non-expand migration is a latent outage.
- **Recommended phase test:** apply migrations `1..N+1`, then call the `N`-migration build's `RunMigrations` against that DB and assert `nil` — locks the "older binary tolerates newer schema" guarantee.

### NEW vs. MODIFIED — Feature 1

| New | Modified |
|-----|----------|
| `deploy/compose.prod.yaml` (repo) | `.github/workflows/full-pipeline.yml` — `deploy` job; `release` gains `outputs.version` |
| `deploy/rollout.sh` (repo) — the rsync'd rollback script | `.env.example` — document `INSTANCE_PASSPHRASE` + any prod-only vars |
| GitHub Environment `production` + 4 SSH secrets | *(no application Go code changes)* |
| VPS host bootstrap (Docker, deploy user, SSH key, `.env`, reverse proxy, `current-tag` seed) | |

---

## Feature 2 — Passphrase gate middleware

### Exact position in the chi chain

Current chain in `internal/httpserver/server.go` (`New`):

```
r.Use(middleware.RequestID)     // 1. correlation id into context
r.Use(echoRequestID)            // 2. echo id as X-Request-Id response header
r.Use(httplog.RequestLogger)    // 3. structured JSON access log (+ RecoverPanics)
r.Use(middleware.Recoverer)     // 4. panic → 500
// routes registered here
```

**The gate goes AFTER all four**, applied to a protected-route `Group` — not as a fifth top-level `r.Use`:

```go
r.Get("/health", s.handleHealth)          // exempt — outside the group
r.Post("/session", s.handleCreateSession)  // login — outside the group
r.Group(func(pr chi.Router) {
    pr.Use(s.authGate)                     // 5. runs after 1–4 (chi runs parent Use then group Use)
    pr.Get("/search", s.handleSearch)
    pr.Post("/watchlist", s.handleAddWatchlist)
    pr.Get("/watchlist", s.handleListWatchlist)
    pr.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
    pr.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
    pr.Get("/events", s.handleListEvents)
})
r.NotFound(webassets.Handler().ServeHTTP)  // SPA shell — outside the group
```

Rationale for "after 1–4":
- **After `RequestID` + `httplog`:** rejected `401`s are logged *with a request id*. You want failed auth attempts in the access log (brute-force visibility, lockout diagnosis). Placing the gate before `httplog` would silently drop them.
- **After `Recoverer`:** a panic in cookie decoding becomes a `500`, not a process crash.
- **Group, not top-level `r.Use`:** avoids in-middleware URL-path string matching entirely. `/health`, `/session`, and the SPA `NotFound` handler are simply registered outside the group. chi guarantees group middleware runs after the parent chain, so ordering relative to 1–4 is automatic.

### Exempting `/health` and the SPA static assets

- **`/health`:** registered outside the protected group. The VPS deploy health-poll (Feature 1) and Docker's `HEALTHCHECK` both hit `/health` unauthenticated — this exemption is load-bearing for the deploy gate.
- **SPA static assets / shell (`NotFound` → `webassets.Handler`):** also outside the group. **Deliberate interpretation of "all routes except `/health`":** the *data API* (`/search`, `/watchlist`, `/events`) is gated; the *static SPA shell* (`index.html`, hashed JS/CSS under `/assets/`) is public. Justification:
  - the browser must load the SPA bundle to render the passphrase form at all;
  - the SPA source is open — serving the empty shell to the world leaks nothing; the watchlist data and the Discord webhook (the actual PROJECT.md threat model) only ever move through the gated API;
  - keeps the gate a pure API concern with no second HTML-rendering path in Go.
  - The SPA calls `lib/api.ts`, sees `401`, and renders the passphrase screen.
- **Stricter alternative (flag for discuss-phase):** move `NotFound` *inside* the group and add a Go-served `/login` page + its assets to the exempt set. Gates the SPA shell too, at the cost of a second static-serving path. Recommend the API-gate interpretation unless the operator explicitly wants the shell hidden.

### Session store: in-memory map vs. signed stateless cookie

**Recommendation: signed stateless cookie.** HMAC-SHA256 over `expiry-timestamp || random-nonce`, key derived from the passphrase itself (`key = SHA256(INSTANCE_PASSPHRASE)`) so there is exactly one new secret.

| | In-memory map (`map[token]expiry` + mutex) | **Signed stateless cookie (recommended)** |
|---|---|---|
| Container restart | **All sessions wiped.** This milestone restarts the container on *every merge to main* → everyone re-enters the passphrase after every deploy. | Survives restarts and deploys — no server state. |
| Multi-replica later | Broken without sticky sessions or a shared store (Redis/Postgres). | Any replica validates any cookie. Aligns with the latent "could run 2 instances behind a LB" direction (already gated by robfig/cron needing leader election — same class of constraint). |
| Revocation | Can revoke a single session immediately. | Cannot revoke one session before expiry. Mitigation: rotate the passphrase (invalidates all — the derived key changes) + a modest `Max-Age` (7–30 days). |
| Code | ~40 lines + a sweeper goroutine. | ~30 lines HMAC sign/verify, no goroutine. Matches the project's hand-roll-small-things ethos (Discord notifier, MB/Deezer clients). |
| Dependency | none | none (stdlib `crypto/hmac`, `crypto/sha256`, `encoding/base64`). `gorilla/securecookie` is an acceptable swap if AEAD/rotation helpers are wanted. |

Cookie attributes: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, `Max-Age` from `SESSION_TTL` (default e.g. `720h`). No PII in the payload — it is just a signed "authenticated until T".

Passphrase check in `POST /session`: `subtle.ConstantTimeCompare` against `cfg.InstancePassphrase`. Ensure the passphrase never reaches a log line — `httplog` logs request metadata, not bodies, but verify no handler logs the decoded body.

### Config and gate-disable behavior

`internal/config/config.go` — new field:
```go
InstancePassphrase string `env:"INSTANCE_PASSPHRASE"`  // empty = gate disabled
```
**Optional, empty disables the gate** (log a `WARN` at boot when empty). This keeps local `docker-compose`, `make run`, and the ~8 `httpserver.New(...)` test call sites working without every one of them setting a passphrase. Prod `.env` sets it. Optionally add `SESSION_TTL time.Duration env:"SESSION_TTL" envDefault:"720h"`.

### NEW vs. MODIFIED — Feature 2

| New | Modified |
|-----|----------|
| `internal/authgate/` — `gate.go` (middleware, exemption via Group so none needed inline), `session.go` (HMAC sign/verify) + tests | `internal/httpserver/server.go` — protected `Group`, `POST /session` route, `New(...)` gains `WithAuthGate(passphrase)` functional option (matches existing `detection.With*` / `poller.With*` / `watchlist.With*` idiom; keeps test call sites unchanged when omitted) |
| `internal/httpserver/session.go` — `handleCreateSession` (+ optional `handleDeleteSession`) | `internal/config/config.go` — `InstancePassphrase` (+ optional `SESSION_TTL`) |
| `web/app/` — passphrase screen component + a route guard | `cmd/server/main.go` — one line: `httpserver.WithAuthGate(cfg.InstancePassphrase)` into `httpserver.New` |
| | `web/app/lib/api.ts` — on `401`, surface a "needs auth" state |
| | `web/app/lib/api.ts` test mocks + affected component tests |
| | `.env.example`; `docker-compose.yml` inherits via `env_file: .env` (add to `.env.example` only) |

---

## Feature 3 — Coverage-diff PR comment

### Which job produces the artifacts

| Language | Producing job | Current state | v1.3 change |
|----------|---------------|---------------|-------------|
| Backend | `test` — `make test-integration` writes `coverage.out`, `make coverage-gate` parses `go tool cover -func` for the `total:` line | `coverage.out` is **not** uploaded (Makefile: "no HTML report and no artifact upload") | **MOD** add `actions/upload-artifact` of `coverage.out` (or a computed `%`); reuse `coverage-gate`'s exact parse so the comment and the 80% gate never disagree — extract it into a `make coverage-report` target that prints just the number |
| Frontend | `frontend-test` — `pnpm test` (Vitest) with `coverage.thresholds` in `web/vitest.config.ts` | Coverage runs for the 70% gate; reporter is likely `text` only | **MOD** add `json-summary` to `coverage.reporter` in `web/vitest.config.ts`; upload `web/coverage/coverage-summary.json` |

### Does the comment job need `needs:` on both test jobs

**Yes — `needs: [test, frontend-test]`, and nothing else.**
- It needs both coverage artifacts, so both jobs must complete.
- It must **not** `needs: build-scan` / `release` — there is no reason to wait for the image build to post a coverage comment, and narrower `needs` = faster PR feedback. (`build-scan` already `needs` both test jobs, so the graph stays consistent regardless.)
- Default behavior (skip if an upstream job fails) is correct — a coverage number from a failed test run is meaningless.

### Guards

```yaml
coverage-comment:
  needs: [test, frontend-test]
  if: github.event_name == 'pull_request'
  permissions:
    contents: read
    pull-requests: write          # NEW — workflow default is contents: read only
  concurrency:
    group: coverage-comment-${{ github.ref }}
    cancel-in-progress: true       # a re-push supersedes the in-flight comment
```

Use an upsert-by-marker comment (`peter-evans/create-or-update-comment` or `marocchino/sticky-pull-request-comment`, SHA-pinned per repo convention) keyed on a hidden HTML marker so re-pushes edit one comment instead of stacking.

### How it reads the main-branch baseline

**Recommendation: GitHub Actions Cache**, with a graceful "no baseline → post absolute numbers only" fallback.

- On **push to `main`**: after `test` / `frontend-test`, `actions/cache/save` the two coverage numbers under a key like `coverage-baseline-main-${{ github.sha }}` plus the stable prefix `coverage-baseline-main-`.
- On **pull_request**: `actions/cache/restore` with `restore-keys: coverage-baseline-main-` pulls the most recent main baseline. Diff against it.
- Trade-off: the 7-day cache eviction means an idle `main` loses the baseline → the job reports absolute coverage only that run. Acceptable.
- **More durable alternative** (if eviction becomes annoying): commit the numbers to an orphan `coverage-baseline` branch on each `main` build, or write a repo variable via `gh api .../actions/variables`. Both cost more moving parts; start with cache.

Reject: checking out `main` and re-running the whole suite in the PR job — doubles CI time for a reporting nicety.

### NEW vs. MODIFIED — Feature 3

| New | Modified |
|-----|----------|
| `coverage-comment` job in `full-pipeline.yml` | `full-pipeline.yml` — `test` + `frontend-test` gain coverage upload / cache-save steps |
| `make coverage-report` target (prints the number `coverage-gate` already computes) | `web/vitest.config.ts` — add `json-summary` reporter |
| | *(no application code changes)* |

---

## Data-flow changes introduced by v1.3

1. **Deploy flow (new):** merge to main → `release` publishes `ghcr.io/...:vX` + `outputs.version` → `deploy` SSHes to VPS → rsync `deploy/` → rewrite `current-tag` (atomic `mv`) → `docker compose pull app` → `up -d app` → poll `127.0.0.1:8080/health` → on failure: restore `current-tag`, `up -d app` from cached previous image, `exit 1`.
2. **Request flow (changed):** `RequestID → echoRequestID → httplog → Recoverer` **→ `authGate` (protected group only)** → handler. Unauthenticated API request → `401` (logged with request id) → SPA renders passphrase screen → `POST /session` (outside gate) → HMAC-signed cookie → subsequent API requests carry the cookie → gate verifies signature + expiry.
3. **Schema evolution (constrained):** boot-time `migrate.Up()` unchanged, but every new migration is now on a one-release-backward-compatibility contract because rollback boots the prior image against the newer schema.
4. **CI reporting (new):** `test` / `frontend-test` coverage numbers → artifact + main-baseline cache → `coverage-comment` job → sticky PR comment with the delta vs. main.

---

## Suggested build order

Ordered by dependency and prerequisite weight; each item ships observable value independently.

1. **Passphrase gate (backend + frontend).** Zero external infra, testable entirely under `docker-compose`. Constrains nothing downstream. Should exist *before* the first public deploy so the instance is never briefly public-unprotected.
   1a. `config.InstancePassphrase` (+ boot WARN when empty).
   1b. `internal/authgate` package — TDD the HMAC sign/verify, the middleware, the disabled-when-empty path.
   1c. Wire into `httpserver.New` via `WithAuthGate` option; add protected `Group` + `POST /session`.
   1d. Frontend: `401` handling in `lib/api.ts` + passphrase screen + route guard.
   1e. `.env.example`.
2. **Coverage-diff PR comment.** Independent of the other two; touches only CI YAML + `vitest.config.ts` + a Makefile target. Low risk. First PR after merge shows "no baseline" until `main` runs once.
3. **VPS deploy job + rollback.** Last — the heaviest prerequisites:
   - a provisioned, reachable VPS with Docker + Compose, a deploy user + SSH key, a reverse proxy terminating TLS (the `Secure` cookie needs HTTPS);
   - GitHub Environment `production` + 4 SSH secrets;
   - `deploy/compose.prod.yaml` + `deploy/rollout.sh` in the repo; real `.env` + seeded `current-tag` on the VPS;
   - the passphrase gate already merged (don't expose an unprotected instance);
   - **expand/contract migration discipline in force from the first v1.3 migration** — a cross-cutting rule that starts before this job lands, not when it lands.
   3a. Repo: prod compose file + tag-file scheme + rollout script.
   3b. Host bootstrap (wizard-style — install Docker, create user + key, place `.env`, reverse proxy, first manual `compose up`, seed `current-tag`).
   3c. GitHub Environment + secrets; `release` gains `outputs.version`.
   3d. `deploy` job: rsync + ssh + health-poll + rollback.
   3e. Verify with a real merge; then a deliberate bad-deploy (e.g. point at a nonexistent tag) to prove rollback fires.

Independent tracks: (1) and (2) can proceed in parallel. (3) depends on (1) being merged and on the migration-discipline decision being recorded.

---

## Anti-Patterns to avoid

### Storing the pinned tag inside the secret `.env`
Rollback has to rewrite the tag file; doing that to the file that also holds `DATABASE_URL`'s password widens the blast radius of a botched `sed`/write and makes the atomic-rename harder. Keep the tag in its own file / `deploy.env`.

### Polling `/health` from the GitHub runner instead of over SSH
The app port is normally firewalled to localhost + the reverse proxy. Run the health loop inside the SSH session against `127.0.0.1`.

### Treating `/health` `200` as "the deploy is good"
`/health` only pings Postgres. It catches a crash-looping container and an unreachable DB; it does not catch a semantic regression behind a successful migration. Fine as a coarse gate for v1.3 — just don't over-trust it.

### Shipping a destructive migration in the same release the code depends on it
Once auto-rollback exists, the prior image boots against the new schema. `DROP COLUMN` / `RENAME` / `ADD NOT NULL` (no default) in release N → rollback to N-1 breaks. Split expand (N) and contract (N+M).

### Putting the gate before `httplog` to "reduce log noise"
You lose request-id-correlated `401`s — exactly the lines you need to diagnose a lockout or spot a brute-force attempt.

### In-memory session map under continuous deployment
Every merge to main recreates the container and wipes the map — the operator re-authenticates after every deploy. Use a signed stateless cookie.

### Adding a param to `httpserver.New(...)` instead of a functional option
`New` is called from ~8 test files. A positional param ripples through all of them. The codebase already standardizes on `With*` options for exactly this reason.

### Re-running the suite on `main` inside the PR job to get a baseline
Doubles CI minutes. Cache (or an orphan branch / repo variable) the number from the `main` run instead.

---

## Integration Points

### External services

| Service | Integration pattern | Gotchas |
|---------|---------------------|---------|
| VPS (SSH) | `deploy` job → SSH → rsync `deploy/` + run `rollout.sh` | Secrets in GitHub Environment `production`, not repo secrets. `known_hosts` pinned. Non-cancelable `concurrency` group. |
| ghcr.io (from VPS) | `docker compose pull` | Package is public → no login needed. If made private, PAT in VPS `.env` + one-time `docker login`. |
| GitHub PR API | `coverage-comment` job → sticky comment | Needs job-scoped `pull-requests: write`; `if: pull_request`; upsert by hidden marker. |
| Reverse proxy / ACME (Caddy/nginx/Traefik) | Host bootstrap, terminates TLS | Required because the session cookie is `Secure`. Not part of the deploy job. |

### Internal boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `authgate` ↔ `httpserver` | `httpserver.New` imports `authgate`, mounts `authGate` on a route `Group` | Gate disabled (pass-through) when `InstancePassphrase == ""` |
| `authgate` ↔ `config` | `cmd/server/main.go` reads `cfg.InstancePassphrase`, passes via `WithAuthGate` | HMAC key derived from the passphrase → no second secret |
| SPA ↔ API | `401` from `lib/api.ts` → passphrase screen → `POST /session` → cookie | SPA shell served unauthenticated (deliberate) |
| `deploy` job ↔ `release` job | `needs: release`, reads `needs.release.outputs.version` | `release` must be modified to expose the output |
| boot migrations ↔ rollback | Older image runs `migrate.Up()` → `ErrNoChange` against ahead-of-source schema | Safe only under expand/contract; add a regression test |

---

## Sources

- Repo source, verified directly (confidence HIGH): `.github/workflows/full-pipeline.yml`, `docker-compose.yml`, `Dockerfile`, `cmd/server/main.go`, `internal/httpserver/server.go`, `internal/httpserver/health.go`, `internal/config/config.go`, `internal/db/migrate.go`, `internal/db/pool.go`, `internal/webassets/embed.go`, `Makefile`, `.planning/PROJECT.md`, `.planning/ROADMAP.md`, `internal/db/migrations/*.sql`
- golang-migrate `Up()` semantics when the DB version is ahead of / unknown to the embedded source — reasoned from the library's `readUp`/source-driver `Next` behavior and corroborated by community usage; the `"no migration found for version X"` error is a `Down`/`Steps` path, not `Up` (confidence MEDIUM, recommend a phase regression test): [pkg.go.dev golang-migrate/migrate/v4](https://pkg.go.dev/github.com/golang-migrate/migrate/v4), [golang-migrate/migrate README](https://github.com/golang-migrate/migrate), [issue #702](https://github.com/golang-migrate/migrate/issues/702), [issue #1100](https://github.com/golang-migrate/migrate/issues/1100), [Better Stack: Database migrations in Go with golang-migrate](https://betterstack.com/community/guides/scaling-go/golang-migrate/)
- GitHub Actions cache eviction (7 days / 10 GB) and `restore-keys` prefix matching for the coverage baseline pattern (confidence MEDIUM — standard Actions behavior, not re-verified this session)
- chi middleware execution order (parent `Use` chain then group `Use`) — chi v5 documented behavior, consistent with the existing chain in `server.go` (confidence HIGH)

---
*Integration research for: v1.3 Continuous Deployment*
*Researched: 2026-08-27*
