# Feature Research — v1.3 Continuous Deployment

**Domain:** CI/CD delivery + operator access control for an already-shipped single-binary Go + React service (drop-tracker v1.2)
**Researched:** 2026-08-27
**Confidence:** HIGH (features are well-trodden DevOps patterns; verified against current repo `full-pipeline.yml`, PROJECT.md constraints, and 2026-dated ecosystem sources)

This is **integration research for a subsequent milestone**, not greenfield ecosystem work. Three new capabilities land on top of an existing pipeline that already publishes a scanned, SBOM'd, semver-tagged image to `ghcr.io/danielrpof/drop-tracker`:

1. **DPLY-01** — automated deploy to one self-hosted VPS on merge to main, after the image is published: pull pinned tag, swap the running container, verify `/health`, auto-rollback to the previous image on failure.
2. **Passphrase gate** — one shared operator-set passphrase (env var) + session cookie, protecting every route except `/health`.
3. **CICD-13** — PR comment showing backend + frontend coverage delta vs. the main baseline.

Existing capabilities these build on (do NOT re-research): watchlist CRUD, MB/Deezer search + polling, detection engine, Discord notifier, React SPA embedded via `go:embed`, `full-pipeline.yml` through `release`, `/health` endpoint, boot-time golang-migrate migrations with bounded retry, `signal.NotifyContext` graceful shutdown + bounded `httpSrv.Shutdown`, `make coverage-gate` (80% Go) + Vitest `coverage.thresholds` (70% frontend).

---

## Feature 1 — Automated VPS Deploy (DPLY-01)

### What a good single-VPS continuous-deployment flow looks like (end to end)

```
merge to main
  └─> full-pipeline.yml: vet/lint/test/gitleaks/trivy-fs/frontend-test
        └─> build-scan (Trivy image scan, saves scanned image artifact)
              └─> release (svu next → tag → push ghcr.io/…:X.Y.Z + SBOM + git tag)
                    └─> deploy  [NEW]  needs: [release]
                         environment: production        # GitHub Environment → deployment record
                         concurrency: group deploy-production, cancel-in-progress: false
                         1. SSH to VPS (deploy user, pinned known_hosts)
                         2. capture currently-running tag  (docker inspect → PREVIOUS)
                         3. write new tag into compose .env  (IMAGE_TAG=X.Y.Z, never `latest`)
                         4. docker compose pull && docker compose up -d
                         5. poll https://host/health (or /ready) — N attempts, backoff
                         6a. healthy  → prune old image, record deployment success
                         6b. unhealthy/timeout/pull-fail → set IMAGE_TAG=PREVIOUS,
                             up -d again, re-verify, mark run FAILED + Discord alert
```

### Table Stakes (expected of any real auto-deploy)

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Deploy gated on `release` success, not raw push | Deploying an unbuilt/unscanned commit is the classic footgun | LOW | `needs: [release]`; `release` already runs only on `push` to `main`. Deploy the exact `svu`-computed semver tag the release job just pushed — pass it via job output. |
| Deploy a **pinned tag**, never `latest` | Rollback target must be unambiguous; `latest` races and hides what's running | LOW | Compose `.env` on the VPS holds `IMAGE_TAG=X.Y.Z`. `docker inspect` the running container for the current tag before swapping. |
| Concurrency control — one deploy at a time, queue don't cancel | Two merges landing seconds apart must not run `docker compose up` simultaneously (half-swapped state, lost rollback baseline) | LOW | `concurrency: { group: deploy-production, cancel-in-progress: false }`. Mirror of the existing `release` job's `cancel-in-progress: false` rationale in `full-pipeline.yml`. **Never** `cancel-in-progress: true` on a deploy — a cancelled mid-swap deploy leaves the box in an unknown state. |
| GitHub Environment + deployment record | `environment: production` auto-creates a deployment object; shows "deployed to production" in the commit/PR timeline and the repo Deployments tab; scopes SSH key + host + passphrase as **environment secrets** | LOW | Also unlocks optional protection rules (required reviewer, wait timer) later without rework. Set `environment.url` to the public URL so the record links out. |
| Health-check gating before declaring success | The whole point — a container that starts but crash-loops or fails migrations must not count as "deployed" | MEDIUM | Poll `/health` (or a dedicated `/ready` — see dependency note) from the workflow over SSH or via the public URL: ~10–20 attempts, 2–5s apart, expect HTTP 200. Also give the Compose service its own `healthcheck:` + `stop_grace_period` so Docker itself knows. |
| Auto-rollback to previous image on failure | Unattended deploy without rollback = every bad merge is an outage until a human wakes up | MEDIUM | Rollback = rewrite `IMAGE_TAG` to the captured previous tag, `up -d`, re-verify `/health`. If **rollback also fails** → fail the workflow red + fire a Discord alert (reuse the existing `internal/discord` webhook path or a raw curl). Never leave the loop silently. |
| Rollback trigger conditions defined explicitly | Ambiguity here causes both false rollbacks and missed ones | LOW | Trigger on: image pull failure; container exits / restart-loops within grace window; `/health` non-200 after all attempts; migration failure (container exits non-zero at boot). Do **not** trigger on a single transient 5xx — that's what the retry budget is for. |
| Deploy status surfaced in ≥1 human-visible place | Operator needs to know a deploy happened and whether it stuck | LOW | Free: GitHub deployment record + workflow run badge + commit status. Cheap add: Discord message ("deployed X.Y.Z ✅" / "rolled back to X.Y.(Z-1) ⚠️") via the existing webhook. |
| Secrets via GitHub Environment secrets only | Matches PROJECT.md "secrets via env vars only, nothing real committed" | LOW | SSH private key, VPS host, deploy user, `INSTANCE_PASSPHRASE`, any `SESSION_SECRET`. Pin `known_hosts` (don't `StrictHostKeyChecking=no`). |
| Dedicated low-privilege deploy user on the VPS | Blast-radius control; a leaked Actions secret shouldn't be root | LOW | User in the `docker` group, no sudo, key restricted to the deploy command if practical. |
| Production compose file lives in the repo | The box should be reproducible from git, not hand-edited | LOW | `deploy/docker-compose.prod.yml` + templated `.env`; `scp` or `git pull` it onto the box as part of deploy. |
| Backward-compatible (expand/contract) migrations | A rollback runs the **old** image against the **new** schema — it must still work | MEDIUM | Already called out in PROJECT.md. This is a *discipline constraint on every future migration*, not code. Add a checklist / ADR. Boot-time migration + `/health` gate is the safety net for a migration that breaks startup. |

### Differentiators (nice, on-brand for a DevOps portfolio piece)

| Feature | Value Proposition | Complexity | Notes |
|---|---|---|---|
| Discord deploy notifications | Closes the loop in the same channel that already gets release alerts; demoable | LOW | Reuse `internal/discord`. Distinct embed colour for success vs rollback. |
| Dedicated `/ready` readiness probe (DB reachable + migrations applied) vs. `/health` liveness | Deploy gate should verify the app can actually serve, not just that the process is up | LOW–MEDIUM | PROJECT.md currently frames `/health` as "liveness/readiness". Splitting them makes the deploy gate meaningful and keeps `/health` cheap for uptime pings. If not split, ensure `/health` checks the DB. |
| Reverse proxy with automatic TLS (Caddy) **or** Cloudflare Tunnel as a compose service | A public URL needs HTTPS; Tunnel also removes the need to open :80/:443/:22 to the world | MEDIUM | Not in the DPLY-01 line item but a *de facto dependency of "public URL"*. Caddy = one extra service, auto-cert. Cloudflare Tunnel = no inbound ports, strong portfolio talking point, no cert management. **Decision needed before the first live deploy.** |
| Capture + store last-known-good tag on the VPS (file or `.env` `IMAGE_TAG_PREVIOUS`) | Makes rollback a one-liner even outside the workflow | LOW | Belt-and-suspenders with the `docker inspect` capture. |
| GitHub Environment protection rule: manual approve or 5-min wait timer | Human circuit-breaker for a bad Friday merge | LOW | Config-only once the Environment exists. Arguably an anti-feature for *continuous* deploy — list as opt-in. |
| Post-deploy smoke check beyond `/health` (hit `/watchlist` with the passphrase cookie) | Catches "process up, routing broken" | LOW–MEDIUM | Needs a test credential in the deploy job. |

### Anti-Features (out of scope — do not build)

| Feature | Why Requested | Why Problematic Here | Alternative |
|---|---|---|---|
| Blue-green / rolling multi-replica / zero-downtime orchestration | "Real deploys have no downtime" | One container on one host. True zero-downtime needs 2+ replicas + a proxy doing connection draining — disproportionate for a hobby tracker where a **2–5s gap during `up -d` is fine** | Graceful shutdown (already implemented) + Docker `healthcheck` + `stop_grace_period`; a front proxy (Caddy) naturally retries the brief gap. Accept near-zero, not zero. |
| Kubernetes / Nomad / Docker Swarm | "Industry standard" | Explicitly deferred in PROJECT.md; enormous surface for one node | Plain `docker compose` over SSH. |
| Terraform / Ansible / IaC provisioning of the VPS | "Infra as code" | Deferred past current ceiling in PROJECT.md; one hand-provisioned box is fine for now | Document the box setup in a runbook / `wizard`-style script. Revisit later. |
| Canary / percentage traffic shifting | "Safe rollout" | Needs a load balancer + metrics; no traffic to speak of | Health-gated all-or-nothing swap + fast rollback. |
| Separate staging environment in this milestone | "Test before prod" | Only one VPS; doubles infra work now | GitHub Environments make adding `staging` later a config change, not a redesign. |
| `docker build` on the VPS | Simpler mental model | Turns the runtime box into a build box; drifts from the scanned `ghcr.io` artifact | VPS only `pull`s the exact tag `build-scan`/`release` produced. |
| Pushing/pulling `latest` | "Convenient" | Destroys rollback determinism, races between deploys | Pinned semver tag in compose `.env`. |
| Registry credentials on the VPS | Assumed necessary | The `ghcr.io` package is already **public and pullable without auth** (PROJECT.md) | No `docker login` on the box unless the package is later made private. |
| Deploy triggered directly by `push` (parallel to `release`) | Fewer job hops | Could deploy a tag that failed to publish, or race the tag push | `needs: [release]`, consume the released version as a job output. |

### Dependencies (on existing capabilities)

- **`/health` endpoint** (exists, Phase 01) → the deploy gate and the Compose `healthcheck` both consume it. Consider `/ready` (DB + migrations) for the gate specifically.
- **Boot-time migrations with bounded retry** (exists, Phase 01) → deploy relies on the new container self-migrating at startup; a failed migration surfaces as an unhealthy container → rollback.
- **Expand/contract migration discipline** (NEW constraint) → required so rollback-to-previous-image is schema-safe.
- **Graceful shutdown / SIGTERM + bounded `Shutdown`** (exists, Phase 01) → makes `docker compose up -d` container replacement drop in-flight requests cleanly, minimising the swap gap.
- **Single compose service** (exists, Phase 07) → keeps the swap a single `up -d`; also *why* zero-downtime is out of scope.
- **`release` job publishing a pinned semver tag to `ghcr.io`** (exists, Phase 07) → the deploy input. Needs a small change: expose `VERSION` as a job output.
- **`internal/discord` webhook** (exists, Phase 05) → optional deploy/rollback notifications.
- **`concurrency` pattern already in `full-pipeline.yml`** (`release` job) → copy the queue-don't-cancel idiom.

### Complexity: **HIGH** (milestone's main risk)
New: VPS provisioning + hardening, SSH key management + `known_hosts` pinning, GitHub Environment + secrets, production compose file + `.env` templating, TLS/proxy or Tunnel decision, health-poll + rollback control flow, first-ever live deploy, and the ongoing expand/contract migration constraint. The Go/JS code change is tiny (maybe `/ready`); the operational surface is large.

---

## Feature 2 — Instance Passphrase Gate

Single shared operator-set passphrase in an env var (`INSTANCE_PASSPHRASE`), session cookie so a device stays authenticated, chi middleware, everything gated except `/health`. **Not** multi-user auth (explicitly rejected in PROJECT.md).

### Expected UX

| Choice | Verdict | Why |
|---|---|---|
| **Custom login form (single password field), served by the SPA** | ✅ **Recommended** | Styleable, mobile-friendly, supports an explicit logout, credential sent once (not on every request), plays with the SPA's existing 401-handling. |
| HTTP Basic Auth (browser dialog) | ❌ | Ugly native prompt, re-prompts unpredictably, **no logout** (must close the browser), credentials replayed every request, poor mobile UX, can't show a friendly "wrong passphrase" state. |
| Token page (paste a long token) | ❌ | Worse UX than a password field for zero security gain at this scale; operator has to store the token somewhere. |

**Flow:** SPA calls `/api/…` → server returns `401` (JSON) when the cookie is missing/invalid → SPA renders the login screen → operator submits passphrase → `POST /auth/login` → server does a **constant-time compare** against `INSTANCE_PASSPHRASE` → on success sets the session cookie and the SPA re-fetches → on failure returns `401` + increments the throttle counter.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| chi middleware gating all routes except the allowlist | The core ask; one composition point | LOW | Allowlist: `/health` (+ `/ready` if added), `POST /auth/login`, and the static login assets (see below). Everything else → `401` if unauthenticated. |
| `/health` (and `/ready`) **stay open** | The deploy health-gate, the Compose `healthcheck`, and any uptime monitor all hit it unauthenticated — gate it and **every deploy fails** and monitoring goes blind | LOW | Hard requirement, not a preference. Keep `/health` payload minimal — no version/build/DSN leakage. |
| Signed session cookie: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/` | Standard cookie hardening; `Lax` so the first navigation to the site still sends it | LOW | Stateless HMAC-signed token (`issued-at` + expiry, signed with `SESSION_SECRET` or a key derived from the passphrase) avoids a sessions table. Server-side sessions only buy revocation, which a single-operator instance rarely needs. |
| Cookie lifetime long enough that a device stays logged in | "Session cookie so a device stays authenticated" — re-login on every browser restart would be the wrong behaviour | LOW | ~30-day expiry, persistent cookie (not session-scoped). Optional **sliding renewal**: re-issue when >50% elapsed so active devices never get logged out; keep an absolute cap (~90 days). |
| Explicit logout | Operators expect a way out, especially on a shared/public URL | LOW | `POST /auth/logout` → `Set-Cookie` with `Max-Age=0`. Stateless cookie ⇒ logout is client-local only; acceptable at this scale. |
| API returns `401` JSON (not a redirect) for unauthenticated calls | Lets the SPA detect and show the login screen cleanly | LOW | HTML navigations can 302 to the login route; XHR/fetch should get `401` + a JSON body. |
| Per-IP login throttling | A single shared secret on a public URL **is** a brute-force target | LOW–MEDIUM | Token-bucket per IP (reuse `golang.org/x/time/rate`, the project already uses it): ~5 attempts/min then exponential backoff; add a small fixed delay (~250ms–1s) per attempt; log failures via `slog`. Constant-time compare regardless. |
| Constant-time passphrase comparison | Timing-attack hygiene | LOW | `crypto/subtle.ConstantTimeCompare` (compare hashes of equal length, not raw strings of differing length). |
| Config via env var, documented in `.env.example` | Matches PROJECT.md secret-handling constraint | LOW | `INSTANCE_PASSPHRASE` (+ optional `SESSION_SECRET`). Recommend the operator set a 24+ char random value; document that. Fail closed at boot if unset in production mode. |

### What "protected" should mean

| Surface | Protected? | Rationale |
|---|---|---|
| **API / data routes** (`/watchlist`, `/search`, `/events`, `/auth/logout`, …) | **Yes** — `401` without a valid cookie | This is the actual secret-bearing surface (watchlist contents, Discord webhook config). |
| **SPA shell + JS/CSS bundle** (`index.html`, hashed assets) | **No** (serve freely) | The bundle contains no secrets — it's public client code. Gating it means you can't even render a login form. Serve the shell; let it hit a `401` and show the login screen. |
| **SPA client-side routes** (`/history`, `/watchlist` in the router) | **Not individually** | They're just JS state. Enforcement is at the API boundary; the SPA route guard is UX only (redirect to login when a fetch 401s). |
| **`/health`, `/ready`** | **No** — always open | Deploy gate, container healthcheck, uptime monitors. Non-negotiable. |
| **`POST /auth/login`** | **No** (but throttled) | Bootstrap endpoint. |

### Differentiators

| Feature | Value | Complexity | Notes |
|---|---|---|---|
| Sliding cookie renewal | Active devices never get surprise-logged-out | LOW | Re-issue cookie past the halfway mark. |
| Passphrase change invalidates existing sessions | Rotating the secret should actually kick everyone | LOW | Derive the signing key from the passphrase, or include a key-version byte. |
| Global failed-attempt backoff (not just per-IP) | Defends against distributed guessing of one shared secret | LOW–MEDIUM | A global counter that widens delay after N failures across all IPs. |
| Structured audit log of auth events | "who/when" visibility; portfolio-friendly | LOW | `slog` lines for login success/failure/logout with source IP. |

### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---|---|---|---|
| Multi-user accounts / OAuth / RBAC | "Proper auth" | Explicitly rejected in PROJECT.md; orthogonal complexity that doesn't serve the CI/CD goal | One shared passphrase + per-instance self-host. |
| CAPTCHA on the login form | Bot defense | Third-party JS/service dependency, UX tax, overkill for a single long random secret behind rate-limiting | Per-IP + global throttle, long passphrase, `slog` alerting. |
| TOTP / email 2FA on the passphrase | "Extra factor" | No user directory, no mailer; contradicts "single shared secret" | N/A — out of scope. |
| Server-side session store in Postgres | "Revocable sessions" | Adds a table, migrations, cleanup job for a single operator who can just rotate the passphrase | Stateless signed cookie; rotate secret to revoke. |
| Gating the static bundle / `index.html` | "Lock everything down" | Breaks the ability to render a login form; no security benefit (no secrets in the bundle) | Gate the API; serve the shell. |
| Storing the passphrase hashed in a DB with bcrypt/argon2 per-login | "Password best practice" | It's one operator-set value from env, not a user-chosen password in a breach-prone table; per-request KDF is just latency | Constant-time compare against the env value (optionally pre-hashed at boot). |
| Putting the passphrase in a query param or the SPA bundle | "Simpler" | Leaks to logs, referrers, browser history | `POST` body over TLS → `HttpOnly` cookie. |

### Dependencies

- **chi router + middleware stack** (exists) → the gate is one more `r.Use(...)` / sub-router.
- **`go:embed` SPA + chi `NotFound` → `index.html` fallback** (exists, Phase 06) → login screen is a normal SPA route; the fallback must stay outside the gate.
- **`/health`** (exists) → must be in the middleware allowlist; ties directly to DPLY-01's health gate.
- **`golang.org/x/time/rate`** (already a dependency) → reuse for login throttling.
- **`slog` JSON logging + DSN redaction pattern** (exists, Phase 01) → auth event logging; make sure the passphrase never lands in a log line.
- **`caarlos0/env` config struct** (per STACK.md) → add `INSTANCE_PASSPHRASE` / `SESSION_SECRET`, `required` in prod.
- **Ordering vs DPLY-01:** the gate should land **before or with** the first public deploy so the instance is never briefly public-and-open. Recommend gate first, then deploy.

### Complexity: **MEDIUM**
Self-contained, well-trodden. New: one middleware, cookie signing/verification, a login form + route guard in the SPA, login throttling, two config vars. Low risk, small code, no infra.

---

## Feature 3 — PR Coverage-Delta Comment (CICD-13)

A PR comment showing backend (Go) + frontend (Vitest) coverage vs. the `main` baseline. **Report-only** — the existing `make coverage-gate` (80%) and Vitest `thresholds` (70%) remain the actual merge blockers.

### Sticky vs. per-push comment

**Single sticky comment, updated in place.** A new comment per push buries the PR in noise. Implementations (e.g. `marocchino/sticky-pull-request-comment`, or the coverage actions' built-in sticky mode) find their prior comment via a hidden HTML marker and edit it. One always-current comment per PR.

### What numbers matter

| Metric | Include? | Why |
|---|---|---|
| **Total coverage % (backend, frontend) + Δ vs main baseline** (percentage points) | ✅ Core | The headline the milestone asks for. Show absolute + signed delta. |
| **Patch / diff coverage** — % of lines *added or changed in this PR* that are covered | ✅ Strongly recommended | The most actionable signal; total-project Δ is noisy (a large unrelated file drops the average). Codecov calls this "patch coverage". |
| **Uncovered new lines** (list or count) | ✅ if cheap | Points the author straight at what to test. Several Go/JS actions emit this. |
| **Per-package / per-file table for changed files** | ⚠️ Differentiator | Useful on big PRs; collapse behind a `<details>` to keep the comment small. |
| Full per-package table for the whole repo | ❌ | Wall of text; that's what the coverage report artifact is for. |

### Baseline source (no external SaaS — matches project ethos)

- On **push to `main`**: a job computes backend + frontend coverage and stores it — as a workflow **artifact**, in the Actions **cache** keyed by a stable key, or committed to an orphan `coverage-badges`/`gh-pages` branch.
- On **`pull_request`**: the comment job restores that baseline and diffs.
- Fallback if no baseline is found (first run): post absolute numbers only, no delta, don't fail.
- Ready-made options that do the baseline bookkeeping: `fgrosse/go-coverage-report` (Go, diff vs main, sticky comment, artifact baseline), `davelosert/vitest-coverage-report-action` or `ArtiomTr/jest-coverage-report-action` (frontend, reads `coverage-summary.json`, sticky). One comment each, or merge into a single comment with a custom step.

### Behaviour on PRs from forks

| Approach | Verdict | Notes |
|---|---|---|
| Plain `pull_request` trigger, `permissions: pull-requests: write`, skip/degrade on forks | ✅ **Recommended for this repo** | Solo portfolio project — every PR is from a branch in the same repo, so `GITHUB_TOKEN` has write and the comment just works. For a fork PR the token is read-only: detect it (`github.event.pull_request.head.repo.fork`) and **skip the comment step** (tests + gate still run). No security exposure. |
| `pull_request_target` | ❌ Avoid | Runs in the base-repo context *with secrets*; if it checks out and runs PR code you've created an RCE-with-secrets hole. Only safe if it never executes untrusted code. |
| Split: `pull_request` runs tests → uploads coverage artifact; separate `workflow_run` (trusted context) downloads it and posts the comment | ⚠️ Upgrade path | The correct fork-safe pattern. More moving parts; adopt only if external contributors actually show up. |

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Single sticky comment, backend + frontend, total % + Δ vs main | The literal ask | LOW–MEDIUM | Reuse existing coverage runs; just add reporters + a baseline + a comment step. |
| Reuse existing coverage invocations | `make test-integration` already writes `coverage.out`; `pnpm test` already runs v8 coverage | LOW | Add `go tool cover` / `-coverprofile` funcs and a Vitest `json-summary` + `lcov` reporter. Do **not** add a second test run. |
| Report-only — never a merge gate | The gate already exists (`coverage-gate`, Vitest thresholds); a second gate on delta double-jeopardies unrelated small drops | LOW | Comment job has no `needs:` dependents. |
| Graceful on missing baseline / fork PR | First run and external PRs shouldn't error the pipeline | LOW | Degrade to absolute-only; skip comment where the token can't write. |

### Differentiators

| Feature | Value | Complexity | Notes |
|---|---|---|---|
| Patch/diff coverage | Directs the author to untested new code | MEDIUM | `fgrosse/go-coverage-report` does this for Go; frontend needs lcov + a diff-cover style step. |
| Per-changed-file table behind `<details>` | Detail without clutter | LOW | Most actions emit this. |
| Coverage badge in README from the main-branch job | Portfolio polish | LOW | Same baseline job writes a shields.io endpoint JSON. |
| Fail-soft annotation on new uncovered lines | Inline PR annotations | MEDIUM | `octocov` / `dorny/test-reporter` style. |

### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---|---|---|---|
| Making the delta a hard merge gate | "Enforce coverage" | Duplicates `coverage-gate`; blocks PRs for noise (deleting a well-tested file *raises* delta, adding a small trivial file *lowers* it) | Keep the absolute-threshold gate; comment is informational. |
| New comment per push | "See history" | Buries the PR; reviewers lose the thread | Sticky comment; git history + run logs hold the trail. |
| `pull_request_target` to make fork comments work | "Comments on all PRs" | Secrets + untrusted code = RCE risk | Skip on forks now; `workflow_run` split later. |
| Codecov / Coveralls SaaS | Batteries-included | External account, token as a secret, third-party data egress — against the "hand-rolled, minimal deps, secrets-tight" project ethos | Self-hosted baseline via artifact/cache + a sticky-comment action. |
| Whole-repo per-package table in the comment | "Full picture" | Unreadable; scrolls forever | Link the HTML coverage artifact; table only for changed files. |

### Dependencies

- **`make test-integration` → `coverage.out`** (exists, Phase 09) → backend numbers; add a func/summary extraction.
- **`make coverage-gate`** (exists) → stays the backend gate; comment is separate.
- **`pnpm test` with Vitest v8 coverage + `coverage.thresholds`** (exists, Phase 08/09) → frontend numbers; add `json-summary`/`lcov` reporters.
- **`full-pipeline.yml`** → new job(s); **note:** DPLY-01 also edits this file — sequence the two phases or expect a merge touch-up (the repo has a documented history of `frontend-test` being added report-only in one phase and wired in the next; same pattern applies).
- **`GITHUB_TOKEN` with `pull-requests: write`** → the workflow currently sets `permissions: contents: read` at top level; the comment job needs its own `permissions:` block.

### Complexity: **LOW–MEDIUM**
No product code. Fiddly bits are baseline storage and fork handling, both low-risk and report-only.

---

## Feature Dependency Graph

```
release job (ghcr.io pinned tag)  ── exists
      └──required by──> DPLY-01 deploy job
                             ├──requires──> /health (exists)   [enhanced by ──> /ready readiness probe]
                             ├──requires──> boot-time migrations (exists)
                             ├──requires──> expand/contract migration discipline (NEW constraint)
                             ├──requires──> graceful shutdown / SIGTERM (exists)
                             ├──requires──> GitHub Environment + secrets (NEW)
                             ├──requires──> VPS + deploy user + known_hosts (NEW infra)
                             └──de-facto-requires──> TLS proxy (Caddy) OR Cloudflare Tunnel (NEW, decide first)

Passphrase gate ──should land before──> first public DPLY-01 deploy
      ├──requires──> chi middleware stack (exists)
      ├──requires──> go:embed SPA + NotFound fallback (exists)
      ├──requires──> /health stays in the allowlist  <── shared constraint with DPLY-01
      └──reuses──> golang.org/x/time/rate (exists)

CICD-13 coverage comment ── independent of the other two
      ├──reuses──> make test-integration / coverage.out (exists)
      ├──reuses──> pnpm test Vitest coverage (exists)
      └──edits──> full-pipeline.yml   <── also edited by DPLY-01 (sequence or merge-resolve)
```

### Dependency notes

- **DPLY-01 requires the `release` job** — deploy consumes the semver tag `release` publishes; expose it as a job output.
- **DPLY-01 + passphrase gate share the `/health` open-route constraint** — the gate's allowlist and the deploy health check must agree; if `/ready` is added, allowlist it too.
- **Passphrase gate should precede the first live deploy** — otherwise the instance is briefly public and unauthenticated.
- **DPLY-01 and CICD-13 both edit `full-pipeline.yml`** — order the phases, or expect a small merge reconciliation (documented repo pattern).
- **TLS/ingress decision blocks the first deploy** — Caddy vs Cloudflare Tunnel isn't in the DPLY-01 line item but "deploy to a public URL" can't complete without it.
- **Expand/contract migrations become a permanent constraint** once auto-rollback exists — every future phase that adds a migration inherits it.

---

## MVP Definition

### Launch With (v1.3)

**DPLY-01**
- [ ] `deploy` job, `needs: [release]`, `environment: production`, `concurrency: deploy-production` (cancel-in-progress: false)
- [ ] SSH to VPS, capture running tag, pull the pinned semver tag, `docker compose up -d`
- [ ] Health-gate: poll `/health` (or `/ready`) with a retry budget
- [ ] Auto-rollback to the captured previous tag on pull failure / unhealthy / migration failure; fail red + Discord alert if rollback also fails
- [ ] Deployment record (via Environment) + workflow status as the surfaced state
- [ ] Production compose file + templated `.env` in the repo
- [ ] TLS ingress: Caddy service **or** Cloudflare Tunnel (pick one)
- [ ] Expand/contract migration checklist / ADR

**Passphrase gate**
- [ ] chi middleware gating everything except `/health` (+ `/ready`), `POST /auth/login`, static shell
- [ ] `POST /auth/login` — constant-time compare vs `INSTANCE_PASSPHRASE`, sets `HttpOnly`/`Secure`/`SameSite=Lax` signed cookie (~30d)
- [ ] `POST /auth/logout` — clears the cookie
- [ ] SPA login screen + 401-driven route guard
- [ ] Per-IP login throttle (`x/time/rate`) + `slog` audit lines
- [ ] `.env.example` documents `INSTANCE_PASSPHRASE` (+ `SESSION_SECRET`), fail-closed at boot in prod

**CICD-13**
- [ ] Main-branch job stores backend + frontend coverage baseline (artifact or cache)
- [ ] PR job: single sticky comment with total % + Δ vs baseline for both languages
- [ ] Degrade gracefully: absolute-only when no baseline; skip comment on fork PRs
- [ ] Report-only (no `needs:` dependents)

### Add After Validation (v1.4+)

- [ ] `/ready` split from `/health` (if not done in v1.3)
- [ ] Discord deploy/rollback notifications (if not done in v1.3)
- [ ] GitHub Environment protection rule (manual approve / wait timer)
- [ ] Sliding cookie renewal + passphrase-rotation session invalidation
- [ ] Patch/diff coverage + per-changed-file table in the comment
- [ ] Post-deploy smoke check beyond `/health`
- [ ] `staging` environment (second Environment, same job template)

### Future Consideration (v2+)

- [ ] Blue-green via proxy + 2 replicas (only if "zero downtime" becomes a real requirement)
- [ ] Fork-safe `workflow_run` coverage-comment split (only with external contributors)
- [ ] IaC provisioning of the VPS (Terraform/Ansible)
- [ ] Metrics/alerting on deploy frequency & failure rate (pairs with the deferred Prometheus/Grafana tier)

---

## Feature Prioritization Matrix

| Feature | Operator Value | Implementation Cost | Priority |
|---|---|---|---|
| DPLY-01 deploy + health gate + rollback | HIGH | HIGH | P1 |
| TLS ingress (Caddy / Cloudflare Tunnel) | HIGH (blocks public URL) | MEDIUM | P1 |
| Passphrase middleware + login/logout + cookie | HIGH (protects a public instance) | MEDIUM | P1 |
| Login throttling | MEDIUM | LOW | P1 |
| GitHub Environment + deployment record | MEDIUM | LOW | P1 |
| Expand/contract migration discipline | HIGH (rollback safety) | LOW (process) | P1 |
| Sticky coverage comment (total + Δ) | MEDIUM | LOW–MEDIUM | P1 |
| `/ready` readiness probe | MEDIUM | LOW–MEDIUM | P2 |
| Discord deploy notifications | MEDIUM | LOW | P2 |
| Sliding cookie renewal | LOW–MEDIUM | LOW | P2 |
| Patch/diff coverage in comment | MEDIUM | MEDIUM | P2 |
| Environment protection rule (manual approve) | LOW–MEDIUM | LOW | P3 |
| Staging environment | MEDIUM | MEDIUM | P3 |
| Blue-green / zero-downtime | LOW (here) | HIGH | P3 |

---

## Sources

- [Deploying with GitHub Actions — GitHub Docs (Environments, deployment records, concurrency)](https://docs.github.com/en/enterprise-cloud@latest/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments)
- [How to Control Concurrency in GitHub Actions — oneuptime](https://oneuptime.com/blog/post/2026-01-25-github-actions-concurrency-control/view)
- [How to prevent concurrent deployments in GitHub Actions — serverlessfirst.com](https://serverlessfirst.com/emails/how-to-prevent-concurrent-deployments-of-serverless-stacks-in-github-actions/)
- [From GitHub Push to VPS Deployment: a Simple CI/CD Pipeline with Docker — Medium](https://medium.com/@mrzahidxy/from-github-push-to-vps-deployment-building-a-simple-ci-cd-pipeline-with-docker-d70e03d9bf10)
- [Deploy docker containers in VPS with GitHub Actions — DEV Community](https://dev.to/ikurotime/deploy-docker-containers-in-vps-with-github-actions-2e28)
- [CI/CD to VPS with GitHub Actions: Automated Deploy Guide (2026) — Server.HK](https://server.hk/blog/ci-cd-to-hong-kong-vps-with-github-actions-automated-deploy-guide-2026/)
- [docker-deploy-action-go (health check + rollback + cleanup) — GitHub](https://github.com/alcharra/docker-deploy-action-go)
- [dockercompose-health-action — GitHub](https://github.com/thegabriele97/dockercompose-health-action)
- [OWASP Authentication Cheat Sheet (throttling, session cookies, Basic Auth weaknesses)](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [PortSwigger Web Security Academy — Vulnerabilities in password-based login](https://portswigger.net/web-security/authentication/password-based)
- [Password, Session, Cookie, Token, JWT — ByteByteGo](https://blog.bytebytego.com/p/password-session-cookie-token-jwt)
- [Codecov — Pull Request Comments (sticky comment, patch vs project coverage)](https://docs.codecov.com/docs/pull-request-comments)
- [ArtiomTr/jest-coverage-report-action (sticky comment, fork/pull_request_target handling) — GitHub](https://github.com/ArtiomTr/jest-coverage-report-action)
- [target/pull-request-code-coverage (Go PR coverage) — pkg.go.dev](https://pkg.go.dev/github.com/target/pull-request-code-coverage)
- [Code Coverage Report Difference — GitHub Marketplace](https://github.com/marketplace/actions/code-coverage-report-difference)
- [GitHub Code Quality: code coverage directly in pull requests (baseline vs default branch)](https://itacademy.com.ua/en/articles/2026-05-29/github-code-quality-pr-coverage-cobertura/)
- Repo files verified directly: `.github/workflows/full-pipeline.yml`, `.planning/PROJECT.md`, `.claude/CLAUDE.md` (confidence HIGH)

---
*Feature research for: v1.3 Continuous Deployment (automated VPS deploy + passphrase gate + coverage-diff PR comment)*
*Researched: 2026-08-27*
