# Phase 1: Foundation — Data Layer, Config & Health - Context

**Gathered:** 2026-08-04
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers the service skeleton every later phase builds on: env-based config with fail-fast validation, structured (slog) JSON logging with request-ID correlation, a Postgres connection with golang-migrate schema migrations (bookkeeping only — no domain tables), sqlc wired up end-to-end via a trivial query, and a `/health` endpoint reporting DB connectivity. No watchlist, artist, or external-API logic yet — that starts in Phase 2/3.

</domain>

<decisions>
## Implementation Decisions

### Schema scope
- **D-01:** The initial migration creates only what golang-migrate itself needs (`schema_migrations` bookkeeping) — no domain tables (artists/watchlist) in Phase 1. Phase 2 owns the watchlist schema.
- **D-02:** sqlc is wired up in Phase 1 anyway — `sqlc.yaml` configured, `sql_package: "pgx/v5"`, a trivial query added (e.g. backing the health check) so `sqlc generate` produces real output, and the CI drift-check (`sqlc generate` + `git diff`) is added now rather than retrofitted in Phase 2.

### Health check depth
- **D-03:** `/health` does a DB ping only (pgx `Ping`) — no migration-version reporting, no liveness/readiness split. Response body: `{"status": "ok"|"degraded", "db": "up"|"down"}`.
- **D-04:** Returns `503 Service Unavailable` when the DB ping fails, `200 OK` when healthy — lets uptime monitors/load balancers gate on status code alone, not just body content.

### Config & startup failure behavior
- **D-05:** Config uses `caarlos0/env/v11` with `required` tags on critical settings (DB DSN). Missing/invalid required config → log a clear error listing what's missing → `exit(1)` immediately. Never boot in a half-configured state.
- **D-06:** `.env.example` and the `Config` struct stub settings for *future* phases too (Discord webhook URL, poll intervals, MusicBrainz/Deezer settings), not just Phase 1's own needs — **Reversibility:** reversible — adding/removing struct fields later is a cheap, local change.
- **D-07:** Stubbed future-phase settings are real `Config` struct fields with sane defaults (optional, not `required`) — the struct is the single source of truth from day one; later phases just start reading the field instead of adding it fresh.
- **D-08:** Defaults when unset: `LOG_LEVEL=info`, `LOG_FORMAT=json` (production-sane out of the box). Local dev overrides `LOG_FORMAT=text` explicitly via `.env`.

### Migration execution model
- **D-09:** Migrations auto-run on service startup — the service invokes golang-migrate's `Up()` against the configured DSN before it starts serving traffic. No separate manual `make migrate` step required for normal operation.
- **D-10:** If the DB is unreachable or a migration fails at startup, the service retries with backoff (handles the common case of the Postgres container still starting under docker-compose), then `exit(1)` if still failing after retries — consistent with the fail-fast philosophy in D-05.

### Claude's Discretion
- Exact project layout (`cmd/`, `internal/` package structure), the specific retry/backoff parameters (attempt count, delay curve) for D-10, and the precise request-ID header/log field naming for structured logging are left to planning/implementation — these are architecture details, not decisions the user needed to make.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level constraints (already locked)
- `.claude/CLAUDE.md` — locks the full tech stack for this project: chi router, sqlc + pgx/v5, golang-migrate, caarlos0/env/v11, log/slog + go-chi/httplog/v3, robfig/cron. Phase 1 must follow these choices exactly (see "Constraints" and "Recommended Stack" sections).
- `.planning/PROJECT.md` — Core Value, Constraints, and Key Decisions tables; single Go binary/service architecture, secrets via env vars only.
- `.planning/REQUIREMENTS.md` — OPS-01, OPS-02, OPS-03 are this phase's mapped requirements (see Traceability table).
- `.planning/ROADMAP.md` §Phase 1 — Goal and Success Criteria this phase must satisfy.

No phase-specific ADRs/specs exist yet beyond the above — this is the first phase of a greenfield repo.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
None — greenfield repo. Only `README.md`, `LICENSE`, `.gitignore`, and `.planning/` exist at present.

### Established Patterns
None yet — this phase establishes the first patterns (config loading, logging setup, migration/DB wiring, health handler) that later phases will follow.

### Integration Points
N/A — nothing to integrate with yet. This phase IS the integration point for everything that follows.

</code_context>

<specifics>
## Specific Ideas

- Health check response shape: `{"status": "ok"|"degraded", "db": "up"|"down"}`, `503` on DB-down, `200` on healthy (D-03/D-04).
- `.env.example` should read as a fairly complete picture of the app's eventual config surface, even though most fields are unused until later phases (D-06/D-07).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 1-Foundation — Data Layer, Config & Health*
*Context gathered: 2026-08-04*
