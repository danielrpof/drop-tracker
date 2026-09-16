# Phase 10: Event Retention Window - Research

**Researched:** 2026-08-13
**Domain:** Read-side retention filtering on an append-only Postgres event log (Go/chi/sqlc backend + React Router frontend), no new external dependencies
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Retention basis**
- **D-01:** The retention window is measured against `created_at` (when drop-tracker detected/recorded the event), not `release_date` (when the release actually came out). `release_date` is a nullable, free-text string sourced from MusicBrainz/Deezer (sometimes just a year, sometimes absent) — unreliable for date arithmetic. `created_at` is a real `timestamptz` and is already the codebase's effective ordering signal (`ListEvents` sorts by `id DESC`, a monotonic proxy for insertion time). A consequence worth carrying into planning: a freshly-seeded artist's entire back-catalog is inserted with `created_at = now()` at seed time regardless of how old the actual releases are (Phase 4 D-13), so a newly-watched artist's whole seeded history stays visible together for a full window from add-time, then ages out together — this is accepted, expected behavior, not a bug.

**Env var shape & edge cases (DATA-01)**
- **D-02:** New config field is `EVENT_RETENTION_DAYS`, a plain `int` with `envDefault:"90"` — not a `time.Duration` string. DATA-01's own requirement text says "defaulting to 90 days," and an integer day-count is the most operator-friendly unit for this specific setting, even though `PollInterval` elsewhere in `internal/config/config.go` uses `time.Duration`. Do not generalize this into a duration-everywhere rule — it's a per-field judgment call.
- **D-03:** `EVENT_RETENTION_DAYS <= 0` is invalid configuration and must fail fast at startup with a clear error — same posture as `DatabaseURL`'s `notEmpty` tag — rather than being silently interpreted as "show everything" or "hide everything." Avoids a typo (e.g. an unquoted negative number) producing a confusing, silent behavior change instead of an obvious boot failure. — **Reversibility:** reversible — a validation rule, easy to relax later if a real "disable retention" use case emerges.

**Near-boundary behavior**
- **D-04:** The cutoff comparison is inclusive on the "still visible" side: an event with `created_at` exactly at the window boundary (e.g. exactly 90 days old) is treated as still within the window and remains visible (`created_at >= cutoff`, not `>`). Matches the roadmap's literal wording — "older than the window" implies strictly older, not "at or older than."

**Empty-state copy (History UI)**
- **D-05:** Retention-caused emptiness gets its own, third empty-state message — distinct from `history.tsx`'s existing "No release activity yet" (truly empty, no events ever) and "No matching events" (user-applied artist/event-type filter, `isFiltered` branch). Neither existing message is accurate when retention is what emptied the feed: there IS release activity, and the user did not apply a filter. Exact copy is left to planning/UI work, but it must read as "your history isn't empty, it's just outside the retention window" rather than either existing message's implication.
- **D-06:** To select the correct one of the three empty-state messages precisely, the `GET /events` response needs a signal distinguishing "zero events ever for this scope" from "events exist but all are currently outside the retention window" — e.g. a `hasOlderEvents`-style boolean alongside the existing page/cursor response shape. A frontend-only heuristic (e.g. "watchlist is non-empty, so assume retention hid something") was explicitly rejected: it would misfire for a brand-new watchlist artist that has zero detections yet, showing the wrong empty-state message. This is new API surface beyond the existing `Page{Events, NextCursor}` shape (`internal/events/service.go`) — exact field name/shape is Claude's discretion.

**Design decision (locked at roadmap level, do not revisit during planning):** soft-delete/filter, not hard delete. Retention is a read-side filter on display/API queries; rows stay in the table permanently so dedup keys, deluxe-change baselines (`events.track_count`), and the per-source seed-mode signal all survive. The hard-delete variants explored in prior research — including the `release_group_baselines` migration needed to make hard delete safe — are rejected. Success criteria 3, 4, and 5 exist specifically to prove the three failure modes hard delete would have reintroduced.

### Claude's Discretion
- Exact SQL mechanism for the `created_at` cutoff filter in `queries/events.sql`'s `ListEvents` query — e.g. a Go-computed cutoff `timestamptz` parameter (`created_at >= sqlc.arg('cutoff')`) vs. an in-SQL interval expression. Not discussed; follows whichever is more idiomatic against the existing `sqlc.narg` filter pattern already in that query. **This research recommends the Go-computed parameter — see Pattern 1.**
- Exact field name/shape of the "are there older events" signal added to the events API response (D-06) — e.g. `hasOlderEvents: bool` on the existing page envelope, a separate lightweight endpoint, or a count. Behavior is locked (D-06); wire shape is not. **This research recommends `has_older_events: bool` on the existing envelope — see Open Question 1.**
- Exact `EVENT_RETENTION_DAYS` validation error message/mechanism (e.g. `env.Parse` custom validation vs. a manual post-parse check in `config.Load`) — implementation detail, follows whatever `caarlos0/env/v11` supports cleanly. **This research confirms no built-in validation tag exists — a manual post-`env.Parse` check is required, see Pattern 2.**
- Whether the cutoff is computed once per request or via a DB-side `now() - interval`, and any index needed on `events.created_at` for query performance at scale — implementation/performance detail, left to planning/research. **This research recommends once-per-request Go computation and no new index — see Pattern 1 and Open Question 2.**
- Exact empty-state copy text for the new third message (D-05) — behavior/placement is locked, wording is not.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|--------------|--------------------|
| DATA-01 | Event-retention window is configurable via environment variable, defaulting to 90 days | Pattern 2 (manual fail-fast validation, no built-in `caarlos0/env` numeric-minimum tag), Code Examples ("EVENT_RETENTION_DAYS config field + validation"), Validation Architecture (config test cases) |
| DATA-02 | History and API queries exclude events older than the retention window, while the underlying rows and detection state (dedup keys, deluxe-change baselines, seed-mode signal) are left untouched | Pattern 1 (Go-computed always-applied cutoff on `ListEvents` only), Pitfall 4 (do not touch `HasAnyEvent`/`ListExternalIDs`/`GroupTrackCountBaseline`/`ListUnnotified`), Architecture Diagram, Validation Architecture (integration test proving the four untouched queries stay unfiltered) |
</phase_requirements>

## Summary

This phase adds exactly one new, always-applied predicate to one existing sqlc query (`ListEvents`), one new required env-configured setting (`EVENT_RETENTION_DAYS`), and one new UI empty-state branch. Every fact needed to plan it precisely is already visible in the current codebase — this is not a "research an unfamiliar framework" phase, it is a "read the four files that change and get the mechanics exactly right" phase. No new third-party package is required anywhere in the implementation: `caarlos0/env/v11`, `pgx/v5`, `sqlc`, and the existing React Router/vitest frontend stack are all already dependencies.

The three load-bearing mechanics this research pins down precisely: (1) `EVENT_RETENTION_DAYS` needs a **manual post-`env.Parse` validation check** in `config.Load()` — `caarlos0/env/v11` has no built-in numeric-minimum tag, confirmed directly against its current struct-tag documentation; (2) the retention cutoff should be **Go-computed** (`time.Now().Add(-retentionDuration)`) and passed into `ListEvents` as a new required `sqlc.arg('cutoff')` parameter of Postgres type `timestamptz`, which sqlc's `pgx/v5` backend generates as `pgtype.Timestamptz` — the codebase already has a verified example of constructing this type by hand (`internal/detection/detector.go:97-102`); (3) widening `events.NewService`'s constructor to accept the retention duration touches exactly **two call sites** (`cmd/server/main.go:104`, `internal/httpserver/boot_e2e_test.go:54`), not the eleven-plus `httpserver.New(...)` call sites a naive grep might suggest — `httpserver.New` itself takes `events.Store`, an interface stub test doubles already implement directly, so it needs no change.

**Primary recommendation:** Add the cutoff as a Go-computed, always-applied `sqlc.arg` parameter on `ListEvents` (never `sqlc.narg` — it's not optional), validate `EVENT_RETENTION_DAYS > 0` with a manual check in `config.Load()` mirroring the existing aggregate-error style, and thread the retention duration through `events.NewService`'s constructor rather than computing the cutoff in the HTTP layer — this keeps the Go-side validation/clamping convention this codebase already uses for `PageSize` intact for the new cutoff logic too.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Retention window config (`EVENT_RETENTION_DAYS`) | API / Backend (`internal/config`) | — | Single source of truth for the whole process; no other tier reads env vars directly |
| Cutoff computation (`now - window`) | API / Backend (`internal/events.Service`) | — | Mirrors the existing `PageSize` clamp precedent — domain-service layer, not HTTP boundary, not SQL-embedded `now()` |
| Retention filter application | Database / Storage (`queries/events.sql` `ListEvents`) | API / Backend (passes the parameter) | The filter must apply to every `ListEvents` caller uniformly; embedding it in SQL prevents a future caller from bypassing it in Go |
| "Are there older events" signal (D-06) | API / Backend (`internal/httpserver/events.go` response envelope) | — | Frontend cannot answer this without a backend signal — a client-side heuristic was explicitly rejected in CONTEXT.md D-06 |
| Empty-state message selection | Browser / Client (`web/app/routes/history.tsx`) | — | Pure presentation logic branching on the API's `hasOlderEvents`-style signal; no business logic here |
| Detection-state integrity (dedup keys, baselines, seed-mode) | Database / Storage (unfiltered queries: `ListExternalIDs`, `HasAnyEvent`, `GroupTrackCountBaseline`, `ListUnnotified`) | — | Already correct by construction — these queries are explicitly NOT touched by this phase; the map entry exists to make that boundary explicit for planning |

## Standard Stack

No new libraries are introduced by this phase. Every dependency needed already exists in `go.mod` / `web/package.json`.

### Core (existing, reused)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/caarlos0/env/v11` | v11.4.1 [VERIFIED: go.mod:6] | Parses `EVENT_RETENTION_DAYS` from env | Already the project's sole config-parsing library (CLAUDE.md, `internal/config/config.go`) |
| `github.com/jackc/pgx/v5` | v5.10.0 [VERIFIED: go.mod:11] | `pgtype.Timestamptz` cutoff parameter | sqlc's `pgx/v5` backend already generates this type for every `timestamptz` column/param in this schema (`internal/db/sqlc/models.go:35`) |
| `sqlc` (CLI) | v1.31.1 [VERIFIED: Makefile:15, confirmed installed via `sqlc version`] | Regenerates `internal/db/sqlc/events.sql.go` after the new query parameter is added | Existing codegen pipeline (`make sqlc`, `make sqlc-check`) |

### Supporting
No new supporting libraries needed. `golang.org/x/time/rate`, `robfig/cron/v3`, chi, httplog — none of these are touched by this phase.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Go-computed cutoff parameter | SQL-side `now() - (sqlc.arg('days')::int * interval '1 day')` | Keeps `now()` evaluation inside Postgres (marginally simpler param plumbing) but makes the cutoff untestable without manipulating the DB clock — the codebase's existing tests (`internal/httpserver/events_test.go`) insert rows via raw SQL and assert against `time.Time` values in Go; a Go-computed cutoff is directly comparable and unit-testable without a live Postgres instance for the pure-logic parts |
| Manual post-`env.Parse` validation | Switch to `go-playground/validator` for a `min=1` tag | Adds a new dependency for one field; CLAUDE.md's existing `DatabaseURL`/`notEmpty` fail-fast pattern already establishes the manual-check idiom this codebase uses (see `internal/config/config_test.go:115-125`) |

**Installation:** None required — no `go get` / `npm install` needed for this phase.

**Version verification:** `caarlos0/env/v11` (v11.4.1), `pgx/v5` (v5.10.0), `sqlc` (v1.31.1) confirmed directly against this repo's `go.mod` and installed toolchain — no registry lookup needed since nothing new is added.

## Package Legitimacy Audit

**Not applicable.** This phase installs zero new external packages (Go or npm). No `go get`, no `npm install`, no new `import` of a third-party module anywhere in the plan. This section is intentionally empty — skip the gate protocol entirely for this phase's planning and execution.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────┐
                     │   Env: EVENT_RETENTION_DAYS  │
                     │   (default 90, must be > 0)  │
                     └───────────────┬─────────────┘
                                     │ parsed once at boot
                                     ▼
                     ┌─────────────────────────────┐
                     │   internal/config.Load()     │
                     │   fail-fast if <= 0           │
                     └───────────────┬─────────────┘
                                     │ cfg.EventRetentionDays
                                     ▼
                     ┌─────────────────────────────┐
                     │  cmd/server/main.go          │
                     │  events.NewService(q, days)  │
                     └───────────────┬─────────────┘
                                     │
  GET /events?artist_id&event_type&cursor&limit
        │                            │
        ▼                            ▼
┌───────────────────┐   ┌─────────────────────────────────┐
│ handleListEvents   │──▶│ events.Service.List              │
│ (HTTP boundary,     │  │  1. clamp PageSize (existing)     │
│  param validation)  │  │  2. compute cutoff = now - window │
└───────────────────┘   │  3. call q.ListEvents(..., cutoff)│
                          └───────────────┬───────────────────┘
                                          ▼
                          ┌─────────────────────────────────┐
                          │ ListEvents SQL (queries/events.sql)│
                          │ WHERE ... AND created_at >= cutoff │
                          │ ORDER BY id DESC LIMIT page_size   │
                          └───────────────┬───────────────────┘
                                          ▼
                          ┌─────────────────────────────────┐
                          │ events table (never deleted)      │
                          │ - dedup key intact                │
                          │ - track_count baseline intact      │
                          │ - full history for HasAnyEvent,    │
                          │   ListExternalIDs, ListUnnotified, │
                          │   GroupTrackCountBaseline           │
                          │   (all UNFILTERED, untouched)      │
                          └─────────────────────────────────┘
                                          │
                          Page{Events, NextCursor, hasOlderEvents}
                                          ▼
                          ┌─────────────────────────────────┐
                          │ web/app/routes/history.tsx        │
                          │  events.length === 0 branch:       │
                          │   - no filter, hasOlderEvents=false│
                          │       → "No release activity yet"  │
                          │   - filter applied                 │
                          │       → "No matching events"       │
                          │   - hasOlderEvents=true (NEW)       │
                          │       → third retention message    │
                          └─────────────────────────────────┘
```

### Recommended Project Structure

No new files/directories. Changes land inside existing files:
```
internal/
├── config/
│   └── config.go                # + EventRetentionDays field, + validation in Load()
├── events/
│   └── service.go                # + retentionDays field on Service, + NewService param,
│                                  #   + cutoff computation in List(), + hasOlderEvents on Page
├── httpserver/
│   └── events.go                 # + hasOlderEvents wired into eventsResponse
├── db/
│   ├── sqlc/
│   │   └── events.sql.go         # regenerated: ListEvents gets a new required cutoff param
│   └── migrations/                # NO new migration — no schema change, filter is query-only
queries/
└── events.sql                    # ListEvents: + AND created_at >= sqlc.arg('cutoff')::timestamptz
web/app/
├── routes/history.tsx            # + third empty-state branch
└── lib/api.ts                    # + has_older_events field on EventsPage
.env.example                      # + EVENT_RETENTION_DAYS=90 (parity test requires this)
```

### Pattern 1: Go-computed, always-applied cutoff parameter
**What:** Add `created_at >= sqlc.arg('cutoff')::timestamptz` as a new, non-optional predicate ANDed into `ListEvents`'s existing `WHERE` clause. Unlike `artist_id`/`event_type`/`cursor` (which use `sqlc.narg('x') IS NULL OR ...` because they're genuinely optional filters), the cutoff is never optional — it always applies — so it should NOT use the `narg`/`IS NULL OR` idiom. `events.Service.List` computes `cutoff := time.Now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour)` and converts it to `pgtype.Timestamptz{Time: cutoff, Valid: true}` before calling `s.q.ListEvents`.
**When to use:** Any time a filter must apply unconditionally to every call, contrasted with the existing `sqlc.narg` filters that are caller-optional.
**Example:**
```sql
-- Source: verified pattern, queries/events.sql (existing file, this session)
-- existing optional filters (sqlc.narg, "IS NULL OR" idiom):
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('cursor')::bigint IS NULL OR id < sqlc.narg('cursor')::bigint)
  -- NEW: always-applied retention cutoff, sqlc.arg (required), not sqlc.narg
  AND created_at >= sqlc.arg('cutoff')::timestamptz
ORDER BY id DESC
LIMIT sqlc.arg('page_size');
```
```go
// Source: verified pattern, internal/detection/detector.go:97-102 (existing file, this session)
// The codebase's own precedent for hand-building pgtype.Timestamptz from a
// Go time.Time -- reuse this exact shape for the cutoff parameter:
func seedNotifiedAt(seedMode bool) pgtype.Timestamptz {
	if !seedMode {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}
```

### Pattern 2: Manual fail-fast validation for a numeric env var
**What:** `caarlos0/env/v11` has no built-in "greater than zero" tag — confirmed against its current struct-tag documentation (`env`, `envDefault`, `envPrefix`, `envSeparator`, `envKeyValSeparator`, and the `env` tag's `expand`/`file`/`init`/`notEmpty`/`required`/`unset` options; no numeric-range option exists). `config.Load()` must add a manual post-`env.Parse` check that returns an error naming the field, matching the codebase's existing aggregate-error convention proven by `config_test.go:139-161` (`TestLoad_AggregatesAllMissing`).
**When to use:** Any numeric config field needing a range constraint beyond presence/emptiness.
**Example:**
```go
// Source: pattern verified against internal/config/config.go (existing file,
// this session) + caarlos0/env/v11 docs (pkg.go.dev, fetched this session --
// confirmed no numeric-minimum tag exists)
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if cfg.EventRetentionDays <= 0 {
		return nil, fmt.Errorf("EVENT_RETENTION_DAYS must be > 0, got %d", cfg.EventRetentionDays)
	}
	return cfg, nil
}
```

### Pattern 3: Widen a domain-service constructor, not the HTTP-layer constructor
**What:** `events.NewService(q sqlc.Querier)` becomes `events.NewService(q sqlc.Querier, retentionDays int)`. `httpserver.New(...)`'s signature is untouched because it depends on the narrow `events.Store` interface, not the concrete `*events.Service` — every stub test double (`stubEventsStore`) already implements `events.Store` directly and never calls `events.NewService`. This means the ripple is exactly two call sites, verified this session:
- `cmd/server/main.go:104` — `eventsStore := events.NewService(sqlc.New(pool))`
- `internal/httpserver/boot_e2e_test.go:54` — `eventsStore := events.NewService(sqlc.New(pool))`

Every other `httpserver.New(...)` call site (11 files, dozens of call sites — `watchlist_test.go`, `search_test.go`, `health_test.go`, `spa_test.go`, `server_test.go`) passes `stubEventsStore{}` and needs zero changes.
**When to use:** Whenever a new dependency is needed only by a concrete domain-service implementation, not by every consumer of its interface.

### Anti-Patterns to Avoid
- **Filtering `HasAnyEvent`, `ListExternalIDs`, `GroupTrackCountBaseline`, or `ListUnnotified` by `created_at`:** This is the exact regression success criteria 3–5 exist to prevent. These four queries are already commented in `queries/events.sql` explaining why they need full, unfiltered access (D-locked in CONTEXT.md) — do not "fix" them to also filter by retention.
- **Computing the cutoff with SQL's `now()` inside the query:** Makes the filter's boundary untestable in Go without manipulating the database clock, and inconsistent with this codebase's established pattern of doing time/clamp logic in the Go service layer (`PageSize` clamp precedent).
- **Reusing `sqlc.narg` for the cutoff:** `narg` means "nullable, caller-optional" — the retention filter is never optional, so using `narg` would silently create a code path where an unfiltered query is possible (a caller passing a nil cutoff), directly undermining DATA-02.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Env var numeric validation | A custom struct-tag parser or `go-playground/validator` integration | A single `if cfg.EventRetentionDays <= 0 { return error }` in `Load()` | One field, one constraint — a whole validation library is disproportionate; the codebase's `notEmpty` precedent shows this project already prefers manual checks for exactly this kind of single-field rule |
| "Are there older events" existence check | A second full-scan query counting all older rows | `SELECT EXISTS(SELECT 1 FROM events WHERE <same filters> AND created_at < cutoff LIMIT 1)` | `EXISTS` with `LIMIT 1` short-circuits on the first match — no need to count, mirrors the existing `HasAnyEvent` query's own `EXISTS(...)` idiom (`queries/events.sql:30-32`) |
| Retention-window date math | A hand-rolled day-to-duration conversion helper elsewhere in the codebase | `time.Duration(days) * 24 * time.Hour` inline in `events.Service.List` (or a tiny unexported helper in the same file) | Trivial arithmetic; no existing project convention for a shared "days to duration" helper, and none is needed for one call site |

**Key insight:** This entire phase is intentionally small in mechanism — the "don't hand-roll" risk here isn't about pulling in unnecessary libraries, it's about not over-engineering the cutoff/validation logic beyond what the existing codebase conventions already establish.

## Runtime State Inventory

Not applicable — this is not a rename/refactor/migration phase. No schema change, no renamed identifiers, no runtime state relocation. Explicitly confirmed:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no migration needed. `created_at` already exists as `TIMESTAMPTZ NOT NULL DEFAULT now()` [VERIFIED: internal/db/migrations/000003_events.up.sql:40, `created_at TIMESTAMPTZ NOT NULL DEFAULT now(),`] | None |
| Live service config | None — no external service config affected | None |
| OS-registered state | None | None |
| Secrets/env vars | `EVENT_RETENTION_DAYS` is a brand-new, non-secret env var — no existing key renamed | Add to `.env.example` only (required by `TestEnvExampleCompleteness`, see Pitfalls) |
| Build artifacts | None — sqlc regeneration only touches `internal/db/sqlc/events.sql.go`, a normal generated-code diff, not a stale-artifact problem | `make sqlc` + commit the regenerated file |

## Common Pitfalls

### Pitfall 1: `.env.example` parity test will fail silently if forgotten
**What goes wrong:** `TestEnvExampleCompleteness` [VERIFIED: internal/config/config_test.go:255-268] reflects over `Config`'s struct tags and diffs against `.env.example`'s keys — adding `EVENT_RETENTION_DAYS` to the `Config` struct without adding a corresponding line to `.env.example` fails this test with a clear message, but only when `go test ./internal/config/...` runs.
**Why it happens:** The env var is new; nothing about adding a struct field forces a `.env.example` edit.
**How to avoid:** Add `EVENT_RETENTION_DAYS=90` to `.env.example` in the same commit that adds the `Config` field.
**Warning signs:** `TestEnvExampleCompleteness` failure naming `EVENT_RETENTION_DAYS` in "Fields in Config with no documented .env.example key."

### Pitfall 2: Off-by-one on the inclusive boundary (D-04)
**What goes wrong:** Using `created_at > cutoff` instead of `created_at >= cutoff` silently excludes an event exactly 90 days old, contradicting D-04's locked "inclusive on the still-visible side" decision.
**Why it happens:** `>` reads more naturally as "strictly older than N days," but D-04 explicitly locks `>=` — this is a case where the intuitive SQL operator is the wrong one.
**How to avoid:** Write the predicate as `created_at >= cutoff`, and add a boundary test seeding an event at exactly the cutoff timestamp, asserting it IS returned.
**Warning signs:** A boundary-value test failing where an event created exactly `EVENT_RETENTION_DAYS` ago is missing from results.

### Pitfall 3: Widening `events.NewService` breaks compilation everywhere it's called, but the blast radius is smaller than a repo-wide grep suggests
**What goes wrong:** A naive `grep -r "httpserver.New("` returns 60+ matches across test files, making the change look far larger than it is.
**Why it happens:** `httpserver.New` takes the `events.Store` *interface*; test doubles (`stubEventsStore`) implement that interface directly and never call `events.NewService`. Only the two call sites that actually construct a real `*events.Service` need updating.
**How to avoid:** Grep specifically for `events.NewService(` (not `httpserver.New(`) to find the true blast radius: `cmd/server/main.go:104` and `internal/httpserver/boot_e2e_test.go:54`, verified this session.
**Warning signs:** Spending effort updating `stubEventsStore`-based test call sites that don't need any change.

### Pitfall 4: Seed-mode / dedup-key / baseline queries must NOT be touched
**What goes wrong:** A well-meaning "consistency" pass might add the same `created_at >= cutoff` filter to `HasAnyEvent`, `ListExternalIDs`, `GroupTrackCountBaseline`, or `ListUnnotified` — this directly reproduces the three regressions success criteria 3–5 exist to catch (dedup-key loss leading to re-notification, seed-mode reset causing back-catalogue re-announcement, deluxe baseline loss).
**Why it happens:** "Filter everywhere for consistency" is an easy instinct to over-apply once one query is changed.
**How to avoid:** Touch exactly one query (`ListEvents`). The other four already carry comments in `queries/events.sql` explaining why they need unfiltered access — extend that commenting convention on `ListEvents`'s own comment to explain why IT is filtered and the others aren't, so a future reader doesn't reverse the decision either direction.
**Warning signs:** A diff touching any file in `queries/events.sql` other than the `ListEvents` statement.

### Pitfall 5: `pgtype.Timestamptz{}` zero value silently means SQL NULL, not "far past"
**What goes wrong:** If the cutoff `pgtype.Timestamptz` is ever constructed without explicitly setting `Valid: true` (e.g., a bug leaves it as the zero value), the resulting query parameter is SQL `NULL`. Depending on how the predicate is written, `created_at >= NULL` evaluates to `NULL` (never true) in standard SQL, which would make `ListEvents` return **zero rows for every query** — a silent, total outage of the History feed, not a loud error.
**Why it happens:** `pgtype.Timestamptz{}` is a valid-looking Go value (a zero-valued struct) that doesn't obviously signal "you forgot to set Valid."
**How to avoid:** Always construct the cutoff via `pgtype.Timestamptz{Time: cutoff, Valid: true}` explicitly (matching the `seedNotifiedAt` precedent), and add a test asserting a full page of pre-existing events is still returned after the retention filter is wired in — a regression here fails loudly (empty result) rather than partially.
**Warning signs:** `TestHandleListEvents_HappyPathReturns200WithEnvelope`-style tests suddenly returning zero events despite seeded fixtures.

### Pitfall 6: `sqlc generate` must be re-run and the diff committed
**What goes wrong:** Hand-editing `internal/db/sqlc/events.sql.go` to add the new parameter instead of regenerating it drifts from `queries/events.sql`, and `make sqlc-check`'s `git diff --exit-code` CI gate will fail.
**Why it happens:** The generated file looks like normal Go code and is easy to hand-edit for a "just add one parameter" change.
**How to avoid:** Edit only `queries/events.sql`, then run `make sqlc` (which runs `sqlc-version-check` against the pinned `v1.31.1` first) and commit the regenerated `events.sql.go`.
**Warning signs:** `make sqlc-check` failing in CI with a non-empty diff under `internal/db/sqlc/`.

## Code Examples

### Full `ListEvents` query after the change
```sql
-- Source: derived from queries/events.sql (existing file, read this session) +
-- Pattern 1 above. This is the recommended shape, not yet applied to the repo.
SELECT id, artist_id, source, event_type, external_id, release_group_mbid,
       title, artist_name, release_date, cover_art_url, track_count,
       previous_track_count, release_type, notified_at, created_at
FROM events
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('cursor')::bigint IS NULL OR id < sqlc.narg('cursor')::bigint)
  AND created_at >= sqlc.arg('cutoff')::timestamptz
ORDER BY id DESC
LIMIT sqlc.arg('page_size');
```

### `events.Service.List` cutoff wiring
```go
// Source: derived from internal/events/service.go (existing file, read this
// session, lines 95-126) + Pattern 1/seedNotifiedAt precedent. Recommended
// shape, not yet applied.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	pageSize := p.PageSize
	switch {
	case pageSize <= 0:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}

	cutoff := pgtype.Timestamptz{
		Time:  time.Now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour),
		Valid: true,
	}

	rows, err := s.q.ListEvents(ctx, sqlc.ListEventsParams{
		ArtistID:  p.ArtistID,
		EventType: p.EventType,
		Cursor:    p.Cursor,
		Cutoff:    cutoff,
		PageSize:  pageSize,
	})
	// ... unchanged from here
}
```

### `EVENT_RETENTION_DAYS` config field + validation
```go
// Source: derived from internal/config/config.go (existing file, read this
// session, lines 17-46). Recommended shape, not yet applied.
type Config struct {
	// ... existing fields unchanged ...

	// Phase 10 — DATA-01: operator-configurable History/API retention window.
	// Days, not time.Duration, per DATA-01's own requirement wording ("90
	// days" is the natural operator unit here) -- do not generalize this into
	// a project-wide duration-vs-int rule.
	EventRetentionDays int `env:"EVENT_RETENTION_DAYS" envDefault:"90"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	// caarlos0/env/v11 has no built-in numeric-minimum tag (verified against
	// its struct-tag docs) -- manual fail-fast check, matching the
	// DatabaseURL/notEmpty posture (D-03): a bad value must not be silently
	// reinterpreted, it must abort startup.
	if cfg.EventRetentionDays <= 0 {
		return nil, fmt.Errorf("EVENT_RETENTION_DAYS must be > 0, got %d", cfg.EventRetentionDays)
	}
	return cfg, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| N/A | N/A | N/A | This phase does not touch code affected by any recent ecosystem shift — `caarlos0/env`, `pgx/v5`, `sqlc`, React Router are all already pinned at current stable versions in this repo, verified this session |

**Deprecated/outdated:** None relevant to this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | `sqlc` generates a required `timestamptz` query parameter as `pgtype.Timestamptz` (not `time.Time`) under this project's `sqlc.yaml` config (`sql_package: pgx/v5`, `emit_pointers_for_null_types: true`) | Code Examples, Pattern 1 | Low — this is inferred from `models.go`'s existing `CreatedAt pgtype.Timestamptz` and `NotifiedAt pgtype.Timestamptz` fields for the same column type on the same table, and is standard, well-documented sqlc `pgx/v5` behavior, but wasn't confirmed by actually running `sqlc generate` against the proposed new query in this research session. If wrong, `sqlc generate` will simply produce a `time.Time` field instead and the plan's code example needs a one-line adjustment — not a design change. |

**All other claims in this research were verified by reading the actual source files in this repository this session (config.go, service.go, events.go, events.sql, events.sql.go, models.go, history.tsx, EmptyState.tsx, main.go, boot_e2e_test.go, events_test.go, migrations, go.mod, sqlc.yaml, Makefile) or by fetching caarlos0/env's current official documentation directly** — no training-data-only claims about this codebase's own structure remain. A1 is the sole exception, flagged above.

## Open Questions

1. **Exact wire field name for D-06's "are there older events" signal**
   - What we know: CONTEXT.md leaves this as Claude's discretion; a natural candidate is `has_older_events: bool` added to `eventsResponse` (`internal/httpserver/events.go:22-25`) and `EventsPage` (`web/app/lib/api.ts:32-35`), computed via a second lightweight `EXISTS(...)` query scoped by the same artist_id/event_type filters but with `created_at < cutoff`.
   - What's unclear: Whether to compute this per-page (cheap, `EXISTS ... LIMIT 1`) or only on the first page (cheaper still, since it only matters when `events.length === 0`). Computing it unconditionally on every page is simplest and matches this codebase's preference for one obvious code path over conditional optimization; the query is a trivial `EXISTS` and this project's data volume is explicitly out of scope for partitioning concerns per REQUIREMENTS.md's Out of Scope table.
   - Recommendation: Compute `hasOlderEvents` on every call to `Service.List` via one added `EXISTS` query with the same filters, reusing `ListParams`' `ArtistID`/`EventType`. Name it `has_older_events` in the JSON envelope (snake_case matches every other field in `eventsResponse`).

2. **Index on `events.created_at` for query performance**
   - What we know: No index on `created_at` currently exists (`internal/db/migrations/000003_events.up.sql` — indexes are on `notified_at` (partial), `(artist_id, source)`, and `release_group_mbid` (partial) only). `ListEvents` already orders by `id DESC`, and `id` is the primary key (implicitly indexed).
   - What's unclear: Whether adding `AND created_at >= cutoff` to a query already ordered by `id DESC` benefits meaningfully from a new index at this project's expected scale. REQUIREMENTS.md's Out of Scope table explicitly excludes "Table partitioning for events retention" with the rationale "No realistic data volume at this project's scale justifies it."
   - Recommendation: Skip adding a new index in this phase. If a future phase's data volume genuinely warrants it, `CREATE INDEX events_created_at_idx ON events (created_at)` is a trivial additive migration — but speculatively adding it now contradicts the same "no realistic data volume" reasoning REQUIREMENTS.md already applied to partitioning.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | All backend code/tests | Yes [VERIFIED: `go version` this session] | go1.26.5 windows/amd64 | — |
| Node.js | Frontend build/tests | Yes [VERIFIED: `node --version` this session] | v22.21.1 | — |
| Docker | `make db-up` (Postgres for integration tests) | Yes [VERIFIED: `docker info` this session] | Docker Desktop 29.6.2 | — |
| `sqlc` CLI | `make sqlc` regeneration after query edit | Yes [VERIFIED: `sqlc version` this session] | v1.31.1 — matches `Makefile`'s pinned `SQLC_VERSION` | — |
| Postgres (via `docker compose`) | `make test-integration`, real-DB tests in `events_test.go` | Not checked as a running container this session, but `make db-up` brings it up on demand — existing, proven workflow | — | — |

No missing dependencies for this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go stdlib `testing` + real-Postgres integration tests via `testutil.NewTestPool` |
| Frontend framework | Vitest + React Testing Library [VERIFIED: web/package.json `"test": "vitest run"`] |
| Config file | `Makefile` (test targets), `sqlc.yaml`, `vitest` config implicit in `web/` (not read this session — pre-existing, proven working per Phase 8/9) |
| Quick run command (backend) | `go test ./... -short -race -count=1` (`make test-short`) |
| Full suite command (backend) | `make test-integration` (requires `make db-up` first) |
| Quick/full run command (frontend) | `cd web && pnpm run test` (vitest run) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|--------------|
| DATA-01 | `EVENT_RETENTION_DAYS` parses, defaults to 90, fails fast on `<= 0` | unit | `go test ./internal/config/... -run TestLoad -short` | ✅ `internal/config/config_test.go` exists — extend with new cases |
| DATA-01 | `.env.example` documents the new key | unit | `go test ./internal/config/... -run TestEnvExampleCompleteness -short` | ✅ existing test, no new file |
| DATA-02 | `ListEvents` excludes rows with `created_at < cutoff`, includes rows exactly at cutoff (D-04) | integration (real Postgres) | `make test-integration` then `go test ./internal/httpserver/... -run TestListEvents -race -count=1 -p 1` | ✅ `internal/httpserver/events_test.go` exists — extend with retention-boundary cases |
| DATA-02 | `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` remain unfiltered (success criteria 3-5) | integration | New test asserting an aged-out row still satisfies `HasAnyEvent`/`ListExternalIDs`/baseline lookup | ❌ Wave 0 — no existing test asserts this cross-query invariant; needs a new test seeding an old `created_at` row and calling the four untouched queries directly |
| DATA-02 | `GET /events` `has_older_events` (or chosen name) signal is correct in all three states (no events ever / all filtered by user filter / all filtered by retention) | integration + component | Backend: extend `TestHandleListEvents_*`; Frontend: extend `web/app/routes/history.test.tsx` | ❌ Wave 0 for both — new response field, new UI branch |
| DATA-02 | History UI shows the correct one of three empty-state messages | component (Vitest + RTL) | `cd web && pnpm run test -- history.test.tsx` | ✅ `web/app/routes/history.test.tsx` exists — extend with the retention-empty-state case |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/... ./internal/events/... -short -race -count=1` (backend) and `cd web && pnpm run test -- history.test.tsx` (frontend) for the touched packages
- **Per wave merge:** `make test-integration` (full backend suite against real Postgres) + `cd web && pnpm run test` (full frontend suite)
- **Phase gate:** Full suite green (`make test-integration` + `make coverage-gate` + frontend `pnpm run test`) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] New integration test in `internal/httpserver/events_test.go` proving `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` return an aged-out row's data unfiltered — this is the direct automated proof of success criteria 3-5, and no existing test covers it
- [ ] New unit test cases in `internal/config/config_test.go` for `EVENT_RETENTION_DAYS` default/override/`<=0`-rejection, mirroring the existing `DatabaseURL` fail-fast tests
- [ ] New boundary test in `internal/httpserver/events_test.go` asserting `created_at` exactly at the cutoff is included (D-04's inclusive-boundary rule) — requires inserting a row with an explicit `created_at` via raw SQL, following `insertTestEvent`'s existing raw-`pool.QueryRow` pattern (it does not go through the sqlc `InsertEvent` query, so setting an arbitrary `created_at` is straightforward: extend the raw INSERT statement to accept a `created_at` parameter)
- [ ] New Vitest case in `web/app/routes/history.test.tsx` for the third (retention) empty-state message, keyed off the new `has_older_events`-style API field

*Framework install: none — all frameworks already present and proven working (Phase 6, Phase 8 established the backend/frontend test conventions this phase extends).*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|-------------------|
| V2 Authentication | No | No auth in this project (single-operator deployable, per REQUIREMENTS.md Out of Scope) |
| V3 Session Management | No | N/A |
| V4 Access Control | No | No new access-control surface — `GET /events` remains unauthenticated, unchanged from Phase 6 |
| V5 Input Validation | Yes | `EVENT_RETENTION_DAYS` is server-side, boot-time config (not user input) — validated via the fail-fast `<= 0` check (Pattern 2); no new user-facing input is introduced by this phase (the retention cutoff is never derived from a request parameter) |
| V6 Cryptography | No | N/A — no secrets, no crypto touched |
| V12 File and Resources | No | N/A |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| DoS via unbounded query cost | Denial of Service | Already mitigated by `events.MaxPageSize` clamp (T-06-02, pre-existing) — the new cutoff parameter does not introduce a new unbounded-scan vector since it's a simple indexed-comparable range predicate, not a computed/recursive one |
| Misconfiguration silently disabling retention | Tampering / Information Disclosure (of history the user expected hidden) | Mitigated by D-03's fail-fast validation — `EVENT_RETENTION_DAYS<=0` cannot be silently misinterpreted as "show everything," it aborts boot |
| Accidental data loss via retention "cleanup" | (N/A — Repudiation avoidance) | Directly the reason D-locked "soft-delete/filter, never hard delete" — no delete/cleanup code path exists in this phase's scope; a future engineer adding one would need to re-derive the seed-mode/dedup-key/baseline safety argument this phase's research and CONTEXT.md already worked through |

## Sources

### Primary (HIGH confidence)
- This repository's own source, read directly this session: `queries/events.sql`, `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/models.go`, `internal/events/service.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/httpserver/events.go`, `internal/httpserver/events_test.go`, `internal/httpserver/server.go`, `internal/detection/detector.go`, `web/app/routes/history.tsx`, `web/app/routes/history.test.tsx`, `web/app/components/common/EmptyState.tsx`, `web/app/lib/api.ts`, `cmd/server/main.go`, `internal/httpserver/boot_e2e_test.go`, `internal/db/migrations/000003_events.up.sql`, `internal/db/migrations/000004_events_display_fields.up.sql`, `go.mod`, `sqlc.yaml`, `Makefile`, `web/package.json`
- `.planning/phases/10-event-retention-window/10-CONTEXT.md` — locked decisions D-01 through D-06
- `.planning/REQUIREMENTS.md` — DATA-01, DATA-02 exact text; Out of Scope table (table partitioning, `pg_cron` exclusions)
- `.planning/STATE.md` — project history and prior-phase decisions
- Installed toolchain, verified directly this session: `go version` (go1.26.5), `node --version` (v22.21.1), `docker info` (Docker Desktop 29.6.2), `sqlc version` (v1.31.1)

### Secondary (MEDIUM confidence)
- `pkg.go.dev/github.com/caarlos0/env/v11` — fetched directly this session via WebFetch, confirming the full list of supported struct tags and the absence of a numeric-minimum validation tag

### Tertiary (LOW confidence)
None — every claim in this research was either read directly from repository source this session or fetched from the library's own current documentation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every version/pattern verified against this repo's actual files
- Architecture: HIGH — every file/line reference in this document was read directly this session, not inferred
- Pitfalls: HIGH — derived from direct inspection of the exact queries, tests, and constructor call sites that change

**Research date:** 2026-08-13
**Valid until:** No expiry concern — this research is tied to this repository's current internal structure (verified directly), not to an external, versioned ecosystem that drifts over time. Re-verify only if `internal/events/service.go`, `queries/events.sql`, or `internal/config/config.go` change materially before planning begins.
