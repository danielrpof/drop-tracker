---
phase: 10
slug: event-retention-window
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-17
---

# Phase 10 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator env → process | `EVENT_RETENTION_DAYS` is read once at boot from the process environment; a malformed value must not produce a silently degraded runtime | config int |
| client → `GET /events` | Unauthenticated HTTP request crosses into the events domain service; the retention cutoff is server-derived and must never be influenced by request input | HTTP query params |
| service → Postgres | The cutoff crosses into SQL as a bound query parameter, never as interpolated text | timestamptz |
| API JSON → React state | `has_older_events` crosses into the SPA and selects a rendered message | boolean |
| static copy → DOM | New empty-state strings are rendered as text nodes | string literal |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-10-01 | Tampering | `config.Load()` / `EVENT_RETENTION_DAYS` | medium | mitigate | Manual post-`env.Parse` check rejects `<= 0` naming the variable; `TestLoad_EventRetentionDaysRejectsNonPositive` pins all three invalid cases (0/-1/-90). Confirmed present in 10-01-SUMMARY.md verification results. | closed |
| T-10-02 | Denial of Service | `events.Service.List` cutoff construction | medium | mitigate | Cutoff `pgtype.Timestamptz` always built with `Valid: true` explicit, preventing a zero-value struct from binding as SQL NULL and silently emptying the feed. `TestListEvents_RetentionExcludesAgedOutRows` confirmed passing. | closed |
| T-10-03 | Information Disclosure | `queries/events.sql` `ListEvents` | high | mitigate | Cutoff uses `sqlc.arg` (required), never `sqlc.narg` — no path returns unfiltered, out-of-window rows on a null cutoff. Acceptance criteria (absence of `sqlc.narg('cutoff')`) confirmed verified. | closed |
| T-10-04 | Repudiation | events table integrity | high | mitigate | Retention is read-side only; no row-removal/mutating statement introduced. `TestRetention_DetectionStateQueriesStayUnfiltered` (5 subtests) confirmed passing in 10-01-SUMMARY.md. | closed |
| T-10-05 | Denial of Service | `GET /events` query cost | low | accept | Simple range comparison on an existing indexed timestamptz column; result size already bounded by `events.MaxPageSize`. No new index warranted at documented data volume. | closed |
| T-10-06 | Elevation of Privilege | request-derived retention window | low | accept | Cutoff derived solely from boot-time config and `time.Now()`; `handleListEvents` parses only `artist_id`/`event_type`/`cursor`/`limit`, unchanged by this phase. | closed |
| T-10-07 | Information Disclosure | `has_older_events` response field | low | accept | Flag reveals only that hidden events exist, never their content; single-operator deployment with no auth surface by design (REQUIREMENTS.md Out of Scope). | closed |
| T-10-08 | Denial of Service | `recordingQuerier` nil embedded `sqlc.Querier` | medium | mitigate | Explicit `HasOlderEvents` stub method added to `recordingQuerier`; pre-existing "limit clamped" subtest confirmed still passing (proves no nil-dereference panic). Confirmed in 10-02-SUMMARY.md. | closed |
| T-10-09 | Denial of Service | second per-request DB round trip | low | accept | `EXISTS` short-circuits on first match, reuses same filters/cutoff as primary query, matching `HasAnyEvent`'s existing cost profile. | closed |
| T-10-10 | Tampering | new empty-state copy rendering | low | mitigate | Copy is a static string literal through the existing `EmptyState` component; `grep -rc dangerouslySetInnerHTML web/app/` confirmed 0 matches (Phase 6 invariant preserved). | closed |
| T-10-11 | Information Disclosure | retention window value in UI copy | low | mitigate | Copy names neither `EVENT_RETENTION_DAYS` nor a day count; API exposes only a boolean. Exact-copy acceptance criteria confirmed verified in 10-02-SUMMARY.md. | closed |
| T-10-SC | Tampering | npm/pip/cargo/go installs | low | accept | Zero new dependencies in either plan — 10-RESEARCH.md's Package Legitimacy Audit explicitly "not applicable". | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-10-01 | T-10-05, T-10-06, T-10-07, T-10-09, T-10-SC | Low-severity, single-operator deployment with no auth surface; no new dependencies introduced; existing query-cost/index precedents already cover the added query shape. | plan-time (10-01-PLAN.md, 10-02-PLAN.md threat models) | 2026-08-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-17 | 12 | 12 | 0 | orchestrator (L1 grep-depth, register authored at plan time — asvs_level 1 short-circuit per secure-phase.md Step 3) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-17
