---
phase: "18"
slug: "backend-readiness-poll-run-history-status-api"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-10"
---

# Phase 18 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Register authored at plan time (all four `18-0N-PLAN.md` carry a `<threat_model>`
> block). Verified at ASVS L1 (grep-depth) per `verify:post` — the short-circuit
> path (`threats_open: 0`, register authored at plan time, L1) applies, so no
> deeper auditor pass was required.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| unauthenticated client → `/ready` | Uptime monitor / orchestrator probe reaches the handler with no credential, on a gated instance too | migration version numbers (low sensitivity) |
| authenticated client → `/status` | Session-bearing operator (incl. one past a weak passphrase — boot only warns) reads an aggregate of internal state | run counts, timestamps, watchlist size, schema version, app version |
| handler → Postgres | One (`/ready`) or two (`/status`) reads on the shared pool; a connection failure yields an error whose text embeds the DSN + password | driver error strings (must not reach the body) |
| poll cycle → `pollruns.Store` → response encoder | Values composed in the poll cycle (driver errors, upstream bodies, webhook URLs in scope) cross into a structure `/status` serializes to an authed client | run summaries — must be store-composed from counts only |
| two cron goroutines → one `Store` | MusicBrainz + Deezer cycles fire on the same `@every` spec, can record near-simultaneously | concurrent map/slice writes |
| CI workflow expression → shell; build arg → image layer; built image → registry | Actions expression-injection surface; build-arg persistence in a public image; scanned-image identity | commit SHA (public), no secrets |
| `queries/` → generated sqlc package | Only drift gate is local (`make sqlc-check`); a forgotten regen ships green through CI | generated Go from hand SQL |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-18-01 | Information Disclosure | `handleReady` 503 path | medium | mitigate | Raw cause → `httplog.SetAttrs` (`ready_db_error` / `ready_schema_error`); no error-derived response field. `TestReady_NoLeak` green. | closed |
| T-18-02 | Denial of Service | `handleReady` DB check | medium | mitigate | `readyCheckTimeout` (3s) bounds ping + schema read together. `TestReady_Timeout` green. | closed |
| T-18-03 | Elevation of Privilege / Access Control | `/ready` route registration | medium | mitigate | Registered on the root router at `server.go:215`, before the `if gate != nil` branch — structural exemption, not a middleware path allowlist. `TestReady_GatedNo401`, `TestReady_InertBranch` green. | closed |
| T-18-04 | Information Disclosure | schema version served unauthenticated | low | accept | A migration count is not sensitive and the deferred Phase 17 deploy gate needs it. Documented. | closed (accepted) |
| T-18-05 | Tampering | `/ready` side effects | low | mitigate | Handler does one `SELECT` + one `Ping`, writes nothing; seam exposes no write method. `grep -nE 'sql\.Open\|pgxpool\.New\|migrate\.New' internal/httpserver/ready.go` empty. | closed |
| T-18-06 | Spoofing / Integrity | `applied` compared by equality vs at-or-above | high | mitigate | D-01. `ready.go:68` is `case applied < s.expectedSchema:` — no `==`. `TestReady_AheadOfSource` green. | closed |
| T-18-07 | Information Disclosure | `RunResult` fields | high | mitigate | No free-text error field; `RecordRun` recomposes `Summary` from counts + normalized outcome. `TestStore_SummaryAlwaysComposed` (feeds a password-bearing DSN) green. | closed |
| T-18-08 | Tampering | `Outcome` value | medium | mitigate | `RecordRun` normalizes anything outside the three constants to `error`. `TestStore_OutcomeNormalized` green. | closed |
| T-18-09 | Tampering / Integrity | concurrent `RecordRun` from two cron goroutines | high | mitigate | One `sync.Mutex` (`pollruns.go:62`) guards every map/slice access. `TestStore_TwoSourceConcurrent` (1000-iteration two-source exact-equality) green. | closed |
| T-18-10 | Tampering / Integrity | `Snapshot` returning an aliasing slice | high | mitigate | Entries copied into a fresh slice inside the critical section. `TestStore_SnapshotDoesNotAlias` green. | closed |
| T-18-11 | Denial of Service | unbounded history growth | medium | mitigate | `N = 50` per source enforced inside `RecordRun`; a skipped tick creates no entry. `TestStore_RingBound`, `TestStore_Skip` green. | closed |
| T-18-12 | Denial of Service | recorder call blocking the poll cycle | low | accept | Mutex-guarded in-memory append, microseconds, no I/O. Call-site log-and-swallow is Phase 18.1's to build. Documented. | closed (accepted) |
| T-18-13 | Tampering / Integrity | pipeline image identity | high | mitigate | Change confined to `build-scan`; `release` job unedited, contains no image build, still `docker load`s the scanned tarball then tag+push. Verified live: CI run 34435266885 + UAT Test 1. | closed |
| T-18-14 | Spoofing / Integrity of provenance | `-X` link-flag import path | high | mitigate | Fully-qualified path `github.com/danielrpof/drop-tracker/internal/buildinfo.Version` at `Dockerfile:70`; CI `build-scan` extracts the binary and greps it for the commit SHA. Verified live: run 34435266885 ("build provenance OK: binary carries 886f936…"). | closed |
| T-18-15 | Information Disclosure | build-argument content | low | accept | The only build arg is the public repo's own commit SHA; `ARG VERSION` is bare with no default, no secret ever passed. Documented. | closed (accepted) |
| T-18-16 | Injection | workflow expression spliced into a shell body | medium | mitigate | Provenance step binds the SHA through an `env:` key and references `"$COMMIT_SHA"` quoted, never interpolating the `${{ }}` expression into the script. | closed |
| T-18-17 | Tampering | Dockerfile invariant erosion | medium | mitigate | Exactly one bare `ARG VERSION` line (`Dockerfile:68`), no default; header comment amended to say why it is not configuration. | closed |
| T-18-18 | Information Disclosure | `/status` failure paths | high | mitigate | Both DB-failure paths send the raw cause to `httplog.SetAttrs` (`status_watchlist_error` / `status_schema_error`) and write a fixed body via the shared helper. `TestStatus_NoLeak` (password DSN + webhook URL through both seams) green. | closed |
| T-18-19 | Information Disclosure | response envelope shape | high | mitigate | No `Error` / `Detail` / `Message` / `LastError` field at any depth — grep on `status.go` empty; run objects carry only counts, timestamps, closed enum, store-composed summary. `TestStatus_EmptyErrorMessage` green. | closed |
| T-18-20 | Elevation of Privilege / Access Control | `/status` route placement | high | mitigate | Registered inside `registerDataRoutes`, called under `pr.Use(gate.Authenticate)` (`server.go:224-229`); carries `X-Instance-Gated` on the gated path like `/events`. `TestStatus_Gated401` green; verified live (401 no session, 200 + header with session). | closed |
| T-18-21 | Denial of Service | schema + count reads per request | medium | mitigate | Both are bounded single-row queries on the shared pool; snapshot is an in-memory copy. Phase 19's fetch-on-mount + manual-refresh (no polling) constraint recorded in ROADMAP. | closed |
| T-18-22 | Denial of Service | a DB blip blanking the operator panel | low | mitigate | Schema-read failure degrades to `schema_applied: null` with 200; a counter failure is 500. Asymmetry documented in a test comment. `TestStatus_SchemaErrorStillTwoHundred` green. | closed |
| T-18-23 | Tampering (supply chain) | `CountWatchlist` sqlc drift | medium | mitigate | `make sqlc-check` (version-pinned generator, fails on any diff) named in the DoD gate + an acceptance criterion. Run clean this cycle. | closed |
| T-18-24 | Tampering | two run stores instead of one | medium | mitigate | `pollruns.NewStore()` appears exactly once, in `cmd/server/main.go:248`. `TestWithRunRecorder_WiresRealStore` green. | closed |
| T-18-SC | Tampering (supply chain) | npm/pip/cargo installs | n/a | accept | All four phase-18 plans install zero packages — every import is stdlib or already-audited `go.mod`. See note below re: the out-of-band js-yaml CVE fix. | closed (accepted) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Only open threats at or above `high` count toward `threats_open`.*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-18-01 | T-18-04 | Migration version numbers served unauthenticated on `/ready` — not sensitive; the deferred Phase 17 deploy gate consumes them to confirm a rollback. | daniel.rf2766 | 2026-09-10 |
| AR-18-02 | T-18-12 | A `RunRecorder` call blocking the poll cycle — the append is a microsecond mutex-guarded in-memory op; the call-site's log-and-swallow guarantee is Phase 18.1's scope (seam is inert this phase). | daniel.rf2766 | 2026-09-10 |
| AR-18-03 | T-18-15 | Commit SHA passed as a Docker build arg — it is the public repo's own SHA, already implicit; `ARG VERSION` is bare, no secret ever transits a build arg. | daniel.rf2766 | 2026-09-10 |
| AR-18-04 | T-18-SC | Phase-18 plans add zero third-party packages. | daniel.rf2766 | 2026-09-10 |

*Note (out-of-band): during `/gsd-verify-work 18`, commit `886f936` added a pnpm
`overrides` entry pinning `js-yaml` to `4.3.2` (HIGH CVE-2026-84375, transitive via
`shadcn → cosmiconfig`) and deleted the stale, unmaintained `web/package-lock.json`.
This lowered supply-chain exposure and removed a second drifting scan input; it added
no new dependency. `trivy-fs` is green on the resulting `main` (run 34435266885).*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-10 | 24 | 24 | 0 | verify:post short-circuit (ASVS L1, register authored at plan time) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-10
