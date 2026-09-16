---
phase: 01
slug: foundation-data-layer-config-health
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-05
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| environment → process | Operator-supplied env vars (`DATABASE_URL`, `HTTP_PORT`, `LOG_LEVEL`, `LOG_FORMAT`, `DISCORD_WEBHOOK_URL`, `POLL_INTERVAL`, MusicBrainz/Deezer settings) cross into the process at boot; this is the only configuration ingress (no file/dotenv loader). | DSN with embedded password, webhook URL |
| client → HTTP API | Unauthenticated network clients reach `GET /health`; a caller-supplied `X-Request-Id` header propagates into logs. | Request metadata, correlation id |
| process → Postgres | The DSN (embeds password) crosses on every connection and every migration retry attempt; connection errors returned across this boundary routinely embed the raw DSN. | DSN / connection errors |
| working tree → git history | `.env` (real values) vs `.env.example` (placeholders) — only the latter may cross into version control. | Secrets vs placeholders |
| module proxy / sqlc CLI → build | Third-party Go modules and the `sqlc` dev-tool binary cross into the build artifact / generated source. | Supply-chain trust |
| SQL source → generated Go | `queries/*.sql` and `internal/db/migrations/*.sql` are compiled by `sqlc` into committed Go; drift means shipped code isn't reviewed code. | Generated source integrity |
| embedded migration files → database schema | `embed.FS` migration contents execute as SQL against the live database at boot. | Schema-modifying SQL |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Information Disclosure | `internal/db/pool.go`, `internal/db/migrate.go`, `internal/logging/logging.go` | medium | mitigate | DSN reduced to host/db description before touching slog or a returned error; `redactDSN`/`redactError` scrub connection-failure text. Verified by `TestRunMigrations_NeverLogsDSN` (+ keyword-value-form variant) and `TestNoDSNInLogs`. | closed |
| T-01-02 | Information Disclosure | `internal/httpserver/health.go` | medium | mitigate | Response body is a fixed `{status, db}` struct; the underlying Ping error goes only to the request log via `httplog.SetAttrs`. Verified by `TestHealth_Down`. | closed |
| T-01-03 | Denial of Service | `internal/httpserver/health.go` | medium | mitigate | `context.WithTimeout(r.Context(), 3*time.Second)` bounds every Ping; `/health` is the only registered route. Verified by `TestHealth_Concurrent`, `TestHealth_DownOnTimeout`. | closed |
| T-01-04 | Denial of Service | `internal/db/migrate.go` | low | mitigate | Retry loop bounded at 6 attempts / 8s delay cap, then terminates the process (D-10) rather than looping forever. Verified by `TestRunMigrations_RetriesThenFails`, `TestRunMigrations_HonoursContextCancellation`. | closed |
| T-01-05 | Spoofing | `GET /health` | low | accept | Intentionally unauthenticated (uptime monitors/load balancers); reveals only reachable/unreachable. See Accepted Risks Log. | closed |
| T-01-SC | Tampering | Go module supply chain | medium | mitigate | RESEARCH.md Package Legitimacy Audit verified every dependency directly against `proxy.golang.org` / GitHub Releases (zero SLOP/SUS/ASSUMED); versions pinned exactly in `go.mod`, `go.sum` present. | closed |
| T-01-06 | Spoofing | inbound `X-Request-Id` header | low | accept | Caller-supplied id never used for authz/authn or as a DB key; `/health` has no per-caller state. See Accepted Risks Log. | closed |
| T-01-07 | Repudiation | structured log sink | low | accept | Logs are operational, not evidentiary, at ASVS L1 single-operator scope. See Accepted Risks Log. | closed |
| T-01-08 | Information Disclosure | `.env` accidentally committed | high | mitigate | `.gitignore` exact-line `.env`; `git ls-files .env` empty; gitleaks pre-commit + CI backstop planned Phase 7. Verified by `TestDotEnvIsNotTracked`. | closed |
| T-01-09 | Information Disclosure | fail-fast config error on stderr | medium | mitigate | Original claim ("never echoes supplied values") was inaccurate — `caarlos0/env`'s type-conversion error does embed the raw invalid input via `%v`. Corrected and closed 2026-08-05: the only fields carrying secret material (`DatabaseURL`, `DiscordWebhookURL`) are plain unvalidated strings with no custom parser, so they structurally cannot fail type conversion and cannot have a value echoed this way. Verified by new `TestLoad_TypeErrorNeverEchoesSecretFields` (`internal/config/config_test.go`). | closed |
| T-01-10 | Tampering | configuration ingress | medium | mitigate | Environment-only ingress; no dotenv dependency in `go list -deps ./internal/config`, no file-read call in `internal/config/config.go`. | closed |
| T-01-11 | Denial of Service | malformed typed settings | low | mitigate | `caarlos0/env` type-conversion fails the parse on malformed input rather than silently defaulting to zero. Verified by `TestLoad_AggregatesAllMissing`. | closed |
| T-01-12 | Tampering | committed `internal/db/sqlc/` vs `queries/*.sql` | medium | mitigate | `make sqlc-check` regenerates and fails on any working-tree diff; Phase 7 CI (CICD-01) runs the same target on every push. | closed |
| T-01-13 | Tampering | `sqlc` CLI supply chain | medium | mitigate | Original claim (a "task precondition asserts `sqlc version`") left no durable artifact — `Makefile` invoked bare `sqlc` from PATH with no guard. Fixed 2026-08-05: added `sqlc-version-check` target enforcing exact `v1.31.1` (the version verified in 01-RESEARCH.md's Package Legitimacy Audit), wired as a prerequisite of both `sqlc` and `sqlc-check`. | closed |
| T-01-14 | Information Disclosure | `Makefile` recipes | low | mitigate | No recipe hardcodes a DSN; `run`/`test-integration` take `DATABASE_URL`/`TEST_DATABASE_URL` from the environment or an overridable throwaway default. | closed |
| T-01-15 | Injection (Tampering) | generated query surface | low | accept | sqlc emits parameterized queries; this phase's only query (`queries/health.sql`) takes no parameters. See Accepted Risks Log. | closed |
| T-01-16 | Tampering | embedded migration content | low | accept | `go:embed` compiles migration SQL into the binary; cannot be swapped at runtime without replacing the binary itself. See Accepted Risks Log. | closed |
| T-01-17 | Denial of Service | concurrent migrators | low | accept | Single service instance/migrator in Phase 1; golang-migrate's Postgres advisory lock is the backstop if a second instance ever starts. See Accepted Risks Log. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|--------------|------|
| AR-01 | T-01-05 | `/health` is intentionally unauthenticated so uptime monitors and load balancers can consume it without credentials; the response reveals only reachable/unreachable, no host/version/topology detail. ASVS L1 V4 not applicable — PROJECT.md scopes v1 to a single-operator deployment with no auth surface. `/health` is verified as the only registered route. | gsd-security-auditor (Phase 01 audit) | 2026-08-05 |
| AR-02 | T-01-06 | An inbound `X-Request-Id` is a caller-chosen correlation id, never used for authorization, authentication, or as a database key; `/health` carries no per-caller state to confuse. Revisit if a correlation id is ever persisted or trusted for access control. | gsd-security-auditor (Phase 01 audit) | 2026-08-05 |
| AR-03 | T-01-07 | A caller-supplied correlation id means a log line is not proof of origin. Accepted at ASVS L1: PROJECT.md scopes v1 to a single operator with no audit/evidentiary requirement on logs. | gsd-security-auditor (Phase 01 audit) | 2026-08-05 |
| AR-04 | T-01-15 | sqlc emits parameterized queries by construction, and this phase's only query takes no parameters, so there is no string-concatenated SQL surface. Phase 2 introduces the first parameterised domain queries and inherits sqlc's parameterisation. | gsd-security-auditor (Phase 01 audit) | 2026-08-05 |
| AR-05 | T-01-16 | Migration SQL is compiled into the binary via `go:embed`; it cannot be altered at runtime without replacing the binary, which requires a source change through code review. Binary integrity itself is CICD-04/CICD-05's concern (Trivy + SBOM, Phase 7), not an in-process control. | gsd-security-auditor (Phase 01 audit) | 2026-08-05 |
| AR-06 | T-01-17 | Phase 1 runs a single service instance and a single in-process migrator, so no contention exists today. golang-migrate takes a Postgres advisory lock during `Up` as the backstop if a second instance is ever started; scaling past one instance needs an explicit distributed-lock decision (already noted in CLAUDE.md), tracked for DTCT-05 in Phase 4. | gsd-security-auditor (Phase 01 audit) | 2026-08-05 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-05 | 18 | 10 | 8 (0 blocking) | gsd-security-auditor (opus) |
| 2026-08-05 | 18 | 18 | 0 | orchestrator (post-fix: T-01-09 test added, T-01-13 Makefile guard added; 6 accept-disposition threats transcribed to Accepted Risks Log) |

**Hardening observations (not registered threats, no action required this phase):**
- `internal/httpserver/health.go` passes the raw pgx error to `slog` on the DB-failure path without the same `redactError` treatment `migrate.go` applies; not a credential leak today (pgx's `ConnectError.Error()` doesn't include the password), but `TestNoDSNInLogs` only exercises the ping-success path — no redaction assertion covers DB-failure logging in `health.go`. Worth a follow-up test/hardening pass, not blocking.
- `internal/db/pool.go` wraps the raw pgx error with `%w` rather than applying `migrate.go`'s stronger `userInfoPattern` scrub. Low risk today; would close a future asymmetry if pgx's error formatting ever changes.
- Positive, unregistered: `cmd/server/main.go` adds full HTTP timeout coverage (WR-02, Slowloris hardening) and graceful shutdown (WR-03) — both DoS controls beyond what the threat register required.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-05
