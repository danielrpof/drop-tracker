---
phase: 10-event-retention-window
reviewed: 2026-08-13T00:00:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - .env.example
  - cmd/server/main.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/db/sqlc/events.sql.go
  - internal/db/sqlc/querier.go
  - internal/events/service.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/events.go
  - internal/httpserver/events_test.go
  - queries/events.sql
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/routes/history.test.tsx
  - web/app/routes/history.tsx
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-08-13T00:00:00Z
**Depth:** standard
**Files Reviewed:** 15
**Status:** issues_found

## Summary

This phase adds a read-side retention window to the `GET /events` history feed: `EVENT_RETENTION_DAYS` config plumbing, a `created_at >= cutoff` predicate on `ListEvents`, a new `HasOlderEvents` query/handler field, and matching frontend empty-state copy. The SQL predicates are correct and well-tested (boundary inclusivity, pagination non-repetition, and — importantly — the four detection-state queries that must stay unfiltered are all covered by an explicit regression test). No Critical/BLOCKER-severity defects were found: no injection, no secret leakage (`.env.example` contains only local-dev placeholder credentials matching CLAUDE.md's "nothing real ever committed" rule, verified via `git show HEAD:.env.example`), no crash paths, no data loss (the filter never deletes rows, confirmed by `TestListEvents_RetentionExcludesAgedOutRows`'s row-count assertion).

Two Warning-level robustness concerns and two Info-level quality notes are below — none block shipping, but the first Warning (new query failure widens the blast radius of `GET /events`) is worth a deliberate decision rather than an accidental byproduct of the added query.

## Warnings

### WR-01: `HasOlderEvents` failure now fails the entire `GET /events` response, even when the primary event list succeeded

**File:** `internal/events/service.go:143-150`
**Issue:** `Service.List` now issues two independent queries per call — `ListEvents` and the new `HasOlderEvents` — and returns an error if *either* fails:
```go
rows, err := s.q.ListEvents(ctx, sqlc.ListEventsParams{...})
if err != nil {
    return Page{}, fmt.Errorf("list events: %w", err)
}
...
hasOlderEvents, err := s.q.HasOlderEvents(ctx, sqlc.HasOlderEventsParams{...})
if err != nil {
    return Page{}, fmt.Errorf("has older events: %w", err)
}
```
`HasOlderEvents` only feeds a supplementary empty-state hint (D-06) — it is not required to render the actual event list. Before this phase, a transient failure on this second query path did not exist; now a `HasOlderEvents`-specific failure (e.g. a lock/timeout hitting the second round-trip after the first succeeded) turns a page that *did* successfully fetch data into a full 500, and the frontend shows "Couldn't load release history" even though the events themselves were available. This is a strict availability regression for the primary feature relative to a secondary nicety.
**Fix:** Consider treating a `HasOlderEvents` error as non-fatal — log it and default `HasOlderEvents` to `false` (the "safe" value: it never claims retained-but-hidden history that may not exist) — rather than discarding a successfully-fetched `Page`:
```go
hasOlderEvents, err := s.q.HasOlderEvents(ctx, sqlc.HasOlderEventsParams{...})
if err != nil {
    // Non-fatal: HasOlderEvents only drives an empty-state hint; a
    // successfully-fetched event list must not be discarded because of it.
    hasOlderEvents = false
}
```
If the team prefers today's fail-together behavior for consistency guarantees, that's a legitimate call too — but it should be an explicit decision, not something a reviewer has to point out.

### WR-02: `ListEvents`'s `Cutoff` parameter silently returns an empty result set instead of erroring if left unset by a direct caller

**File:** `internal/db/sqlc/events.sql.go:158-169`, `internal/events/service.go:117-125`
**Issue:** `queries/events.sql`'s own comment documents the exact footgun (T-10-02): `Cutoff` is `sqlc.arg`, not `sqlc.narg`, and a zero-value `pgtype.Timestamptz{}` (i.e. `Valid: false`) marshals to SQL `NULL`. `created_at >= NULL` evaluates to `NULL` (never true) for every row, so any caller of `sqlc.Queries.ListEvents` that forgets to set `Cutoff.Valid = true` gets a **silent empty result** for the whole table, not an error. Today `events.Service.List` is the only production call site and it always sets `Valid: true`, but `TestListEvents_RetentionBoundaryIsInclusive` in `events_test.go` already calls `sqlc.Queries.ListEvents` directly with an explicit `Cutoff` — proving this is a real, reachable code path, not a hypothetical one. There is no runtime guard anywhere in the reviewed files that would catch a future caller (a new admin/debug endpoint, a script, a different service) constructing `ListEventsParams{}` with a zero-value `Cutoff` and silently getting `[]` back instead of a clear error.
**Fix:** This can't be fixed in the generated file (`DO NOTHING` per its header), but `events.Service.List` — the one hand-written call site — could defensively assert `cutoffParam.Valid` is true before calling, or a lightweight helper could construct `Cutoff` in one place with a name that makes "never construct this directly" explicit, e.g.:
```go
// mustCutoff panics if constructed with a zero time, so a caller can never
// accidentally pass Valid: false and get a silent empty-table result
// (T-10-02) instead of a loud failure.
func mustCutoff(t time.Time) pgtype.Timestamptz {
    if t.IsZero() {
        panic("events: cutoff time must not be zero")
    }
    return pgtype.Timestamptz{Time: t, Valid: true}
}
```
Lower-cost alternative: add a short comment at the top of `ListEventsParams` in `events.sql.go`-adjacent code (or in `querier.go`'s `Querier` interface doc) explicitly warning future callers, since the existing warning currently lives only in `queries/events.sql`'s comment on `ListEvents`, not on the `Cutoff` field itself.

## Info

### IN-01: `parseOptionalPageSize`'s `int` → `int32` conversion can wrap around for very large `limit` values, with no test covering the boundary

**File:** `internal/httpserver/events.go:56-66`
**Issue:** `strconv.Atoi` parses `limit` into a 64-bit `int`, and the function then does `int32(v)` with a `//nolint:gosec` comment reasoning that "every reachable output is either <=0 or >MaxPageSize ... or lands directly in the already-valid range." That's true for the negative/zero and in-range cases, but there's an unexamined middle case: a `limit` value whose low 32 bits land in `[1, MaxPageSize]` after wraparound (e.g. `limit=2147483747` wraps to `99`) would be silently accepted as a small, "valid-looking" page size rather than being clamped to `MaxPageSize` or rejected — the user asked for a huge page and got a small one for no visible reason. It's bounded and harmless (no DoS, no crash), but it's an unintended behavior with zero test coverage; `events_test.go`'s existing `limit=100000` case doesn't exercise the `int32` boundary at all.
**Fix:** Reject values above `MaxPageSize` (or above `math.MaxInt32`) explicitly before the cast, rather than relying on wraparound producing an incidentally-safe result:
```go
if v > events.MaxPageSize {
    v = events.MaxPageSize // or: return 0, false, per team preference
}
return int32(v), true
```

### IN-02: `config.Load()`'s two-stage validation can hide a second boot-time misconfiguration

**File:** `internal/config/config.go:50-64`
**Issue:** `env.Parse` aggregates all missing/invalid *tagged* fields into one error, but the manual `EventRetentionDays <= 0` check runs only after `env.Parse` succeeds. If an operator has both `DATABASE_URL` unset *and* `EVENT_RETENTION_DAYS=0` at the same time, only the `DATABASE_URL` error surfaces; fixing it and restarting is required before the `EVENT_RETENTION_DAYS` problem is even reported. This is explicitly called out and accepted in the code's own comment ("placing it after preserves `TestLoad_AggregatesAllMissing`'s existing aggregate-error behavior"), so it's a deliberate tradeoff, not an oversight — flagging for visibility only, since it does mean two avoidable restart cycles in a real multi-error misconfiguration.
**Fix:** Optional: collect the manual check into the same aggregate error path (e.g. via `errors.Join`) so both classes of problems are reported in one boot attempt:
```go
var errs []error
if err := env.Parse(cfg); err != nil {
    errs = append(errs, err)
}
if cfg.EventRetentionDays <= 0 {
    errs = append(errs, fmt.Errorf("EVENT_RETENTION_DAYS must be a positive integer, got %d", cfg.EventRetentionDays))
}
if len(errs) > 0 {
    return nil, errors.Join(errs...)
}
```
Note this would need `cfg.EventRetentionDays` to still hold its parsed/defaulted value even when `env.Parse` itself errors elsewhere — verify that's true for `caarlos0/env/v11` before making this change, since partial-parse-then-error semantics vary by library.

---

_Reviewed: 2026-08-13T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
