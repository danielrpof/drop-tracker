---
status: diagnosed
trigger: "Phase 14 added an instance passphrase gate. With INSTANCE_PASSPHRASE configured, opening the app should show a passphrase form and block all access until unlocked. Instead, the app loads straight to the watchlist with no passphrase form and everything is accessible."
created: 2026-08-31T00:00:00Z
updated: 2026-08-31T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — the `app` container boots with an empty `INSTANCE_PASSPHRASE`, so the gate takes its intentional inert path (GATE-07). The operator's `.env` file (the only channel `docker-compose.yml` uses to inject app env, via `env_file: .env`) has no `INSTANCE_PASSPHRASE` line, and the compose file neither passes the host shell variable through nor interpolates `${INSTANCE_PASSPHRASE}` anywhere.
test: (1) dumped the key names in `.env` — 12 keys, none is INSTANCE_PASSPHRASE/TRUST_PROXY_HEADERS/NOTIFY_MAX_RELEASE_AGE_DAYS. (2) read docker-compose.yml — app service uses `env_file: .env` + an `environment:` block that only hardcodes DATABASE_URL. (3) read config.go + server.go — empty passphrase => `WithAuthGate("")` => `gate == nil` => `registerDataRoutes(r, s)` flat, no auth middleware, no /session routes; SPA served publicly and returns 200s so the SPA's 401 interceptor never fires.
expecting: n/a — root cause established
next_action: none (goal: find_root_cause_only) — report ROOT CAUSE FOUND

## Symptoms

expected: With INSTANCE_PASSPHRASE set, opening http://localhost:8080 renders the passphrase form and blocks all routes/UI until the correct passphrase is entered. A dt_session cookie is set on unlock.
actual: User set INSTANCE_PASSPHRASE, ran `docker compose up --build`, opened localhost — the React watchlist UI renders immediately, no passphrase form appears, all routes/data are accessible. The gate does nothing.
errors: None reported.
reproduction: Test 1 in .planning/phases/14-instance-passphrase-gate/14-UAT.md — set INSTANCE_PASSPHRASE, `docker compose up --build`, open localhost.
started: Phase 14 UAT (implementation reported complete across plans 14-01..14-04)

## Eliminated

- hypothesis: The gate middleware is not registered on the chi router, or is registered in the wrong order relative to the SPA / API routes.
  evidence: internal/httpserver/server.go:114-188 registers `gate.Authenticate` + `gate.RequireCSRFHeader` on an `r.Group` that wraps `registerDataRoutes`, but only when `cfg.gatePassphrase != ""`. The SPA `r.NotFound` fallback being public is deliberate (D-04, so the passphrase form can load). 14-VERIFICATION.md truth 1 + `TestGate_*` prove the ordering is correct. The wiring is sound; it just never activates because the passphrase is empty.
  timestamp: 2026-08-31T00:00:00Z

- hypothesis: The embedded SPA bundle is stale (predates the phase-14 PassphraseScreen / 401 interceptor), or the Docker build embeds a stale `go:embed dist`.
  evidence: Does not fit the primary symptom. If the server-side gate were active, GET /watchlist would return 401 and the watchlist would render empty/errored — but the user sees the watchlist populated ("the watchlist is there") and "everything is accessible", which requires the API to return 200. That only happens on the inert (gate-disabled) path. Also `docker compose up --build` rebuilds the SPA fresh in Dockerfile stage 1 regardless of the committed tree. Frontend 401 handling (api.ts + authStore + PassphraseScreen + root.tsx) is present and test-covered per 14-03/14-VERIFICATION.
  timestamp: 2026-08-31T00:00:00Z

- hypothesis: A bug in the authgate code makes it fall open when the passphrase looks empty.
  evidence: The inert-on-empty-passphrase behavior is the intended GATE-07 contract (config.go:58-75, server.go:113-117 + else-branch), covered by TestInertPath_FiveArgConstructor, TestInertPath_EmptyPassphraseIsIndistinguishable, TestGate_NoOptionsIsUngated. The code is behaving exactly as designed given an empty input. The defect is upstream: the container never receives a non-empty value.
  timestamp: 2026-08-31T00:00:00Z

## Evidence

- timestamp: 2026-08-31T00:00:00Z
  checked: Knowledge base (.planning/debug/knowledge-base.md) + MemPalace fallback
  found: No matching pattern. The two prior entries (guest-feature-label-missing, backlog-songs-trigger-discord) are data/migration bugs, unrelated to middleware wiring or docker env.
  implication: No known-pattern shortcut. Open investigation.

- timestamp: 2026-08-31T00:00:00Z
  checked: `.env` file at repo root (the file docker-compose.yml feeds the app container via `env_file: .env`) — extracted key names only
  found: 12 keys present — DATABASE_URL, HTTP_PORT, LOG_LEVEL, LOG_FORMAT, DISCORD_WEBHOOK_URL, POLL_INTERVAL, MUSICBRAINZ_USER_AGENT, MUSICBRAINZ_RATE_LIMIT_PER_SEC, DEEZER_RATE_LIMIT_PER_5S, EVENT_RETENTION_DAYS, MUSICBRAINZ_POLL_WORKERS, DEEZER_POLL_WORKERS. An explicit `grep -nE '^INSTANCE_PASSPHRASE=' .env` returned nothing. NOTIFY_MAX_RELEASE_AGE_DAYS (added 2026-08-26) and TRUST_PROXY_HEADERS (Phase 14) are also absent.
  implication: The operator's live `.env` predates Phase 14 (and the late-Phase-5 NOTIFY var). `INSTANCE_PASSPHRASE` is never delivered to the container. This is the root cause.

- timestamp: 2026-08-31T00:00:00Z
  checked: docker-compose.yml — `app` service env injection
  found: `app` service has `env_file: .env` and an `environment:` block that only sets `DATABASE_URL` to a hardcoded literal. Nothing lists `INSTANCE_PASSPHRASE` (e.g. `- INSTANCE_PASSPHRASE`) for host pass-through, and the compose file contains no `${INSTANCE_PASSPHRASE}` interpolation anywhere.
  implication: Setting `INSTANCE_PASSPHRASE` in the host shell (`export` / `$env:`) before `docker compose up` has zero effect on the container — Compose does not forward unreferenced host vars. The ONLY working channel is a `KEY=VALUE` line in `.env`.

- timestamp: 2026-08-31T00:00:00Z
  checked: .env.example (via `git show HEAD:.env.example`)
  found: Ships `INSTANCE_PASSPHRASE=caliber` (a self-warning placeholder) and `TRUST_PROXY_HEADERS=false`, added/finalized during Phase 14 by the operator (agent tools are denied read/write on `.env*` in this sandbox — documented Phase 11.1-04 / WINDOWS.md limitation).
  implication: `.env.example` looks like it already wires the gate, which invites the mistake of editing the example (or assuming `.env` inherits it). Compose reads `.env`, never `.env.example`.

- timestamp: 2026-08-31T00:00:00Z
  checked: internal/config/config.go:58-75, internal/httpserver/server.go:106-204, cmd/server/main.go:197-214
  found: `cfg.InstancePassphrase` is `env:"INSTANCE_PASSPHRASE"` with no default and no Load() validation (empty is legal by design). main.go passes it into `httpserver.WithAuthGate(cfg.InstancePassphrase, ...)`. In `New`, `if cfg.gatePassphrase != "" { gate = authgate.NewManager(...) }` — empty leaves `gate == nil`, so the else-branch calls `registerDataRoutes(r, s)` directly: no `Authenticate`, no `RequireCSRFHeader`, no `/session` routes, no `middleware.RealIP`. The SPA `r.NotFound(webassets.Handler)` serves publicly in both branches.
  implication: Empty `INSTANCE_PASSPHRASE` => byte-for-byte v1.2 behavior. API returns 200, SPA never sees a 401, `authStore` stays optimistically `authed=true`, `PassphraseScreen` never renders. Every reported symptom (watchlist loads, no form, all data accessible, no errors) is explained exactly.

## Resolution

root_cause: |
  The `app` container starts with an empty `INSTANCE_PASSPHRASE`, so `httpserver.New` builds `gate == nil` and registers the data routes flat with no auth middleware — the intentional, fully-tested GATE-07 "inert" path. The gate code, router wiring, and SPA 401-handling are all correct; they simply never engage.

  Why the container sees an empty value: `docker-compose.yml`'s `app` service injects environment only through `env_file: .env`, and the operator's `.env` at the repo root has no `INSTANCE_PASSPHRASE` line (it predates Phase 14 — it is also missing `TRUST_PROXY_HEADERS` and `NOTIFY_MAX_RELEASE_AGE_DAYS`). Phase 14 updated `.env.example` (which now ships `INSTANCE_PASSPHRASE=caliber`) but nothing reconciled the operator's gitignored `.env`, and agent tooling is blocked from touching `.env*` in this sandbox. Compounding it: the compose file does not pass a host-shell `INSTANCE_PASSPHRASE` through to the container and never interpolates `${INSTANCE_PASSPHRASE}`, so "I set the instance passphrase" in the shell (or by editing `.env.example`) silently does nothing.

  Category: config/data (primary — missing `.env` entry) with a process/tooling contributing factor (no reconciliation of the live `.env`; compose surfaces no host-env fallback or gate-active log line). Not an AND-gate: the missing `.env` line alone is sufficient and is the direct mechanism.
fix: |
  Not applied (goal: find_root_cause_only). Direction:
  1. Operator adds `INSTANCE_PASSPHRASE=<random 24+ chars>` (and `TRUST_PROXY_HEADERS=false`, plus the missing `NOTIFY_MAX_RELEASE_AGE_DAYS=7`) to the repo-root `.env`, then `docker compose up --build`. This alone fixes the reported bug.
  2. Optional hardening so this cannot recur silently:
     - docker-compose.yml `app.environment:` add `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}` and `TRUST_PROXY_HEADERS: ${TRUST_PROXY_HEADERS:-false}` so a host-shell value also works and the coupling is visible in the compose file.
     - cmd/server/main.go: emit one Info line at boot stating whether the instance gate is ACTIVE or INERT, so "the gate does nothing" is observable in logs instead of silent.
     - 14-UAT.md Test 1: add an explicit precondition step — "add the line to `.env` (not `.env.example`, not your shell), confirm with `docker compose run --rm app env | grep INSTANCE_PASSPHRASE`".
     - config_test.go already has `TestEnvExampleCompleteness` (struct<->`.env.example` parity); consider an analogous check or a documented `.env` sync step for local `.env`.
verification: n/a (diagnosis only)
files_changed: []
