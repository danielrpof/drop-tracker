# Phase 10: Event Retention Window - Context

**Gathered:** 2026-08-13
**Status:** Ready for planning

<domain>
## Phase Boundary

The History UI and the `GET /events` API stop showing events older than a configurable retention window (default 90 days) — every display path (feed, artist/type filters, pagination) consistently excludes them. Nothing is ever deleted: every event row and every piece of detection state derived from the events table (dedup keys via `(event_type, source, external_id)`, deluxe-change `track_count` baselines, per-source seed-mode via `HasAnyEvent`) stays fully intact and unfiltered, so an old row aging out of view can never cause a stale release to re-notify, an artist's seed mode to reset, or a deluxe baseline to be lost. This is a read-side filter added to the existing `ListEvents` query only — `ListExternalIDs`, `HasAnyEvent`, `GroupTrackCountBaseline`, and `ListUnnotified` are untouched. No new detection logic (Phase 4), no changes to Discord notification behavior (Phase 5), no bounded concurrent polling (Phase 11).

</domain>

<decisions>
## Implementation Decisions

### Retention basis
- **D-01:** The retention window is measured against `created_at` (when drop-tracker detected/recorded the event), not `release_date` (when the release actually came out). `release_date` is a nullable, free-text string sourced from MusicBrainz/Deezer (sometimes just a year, sometimes absent) — unreliable for date arithmetic. `created_at` is a real `timestamptz` and is already the codebase's effective ordering signal (`ListEvents` sorts by `id DESC`, a monotonic proxy for insertion time). A consequence worth carrying into planning: a freshly-seeded artist's entire back-catalog is inserted with `created_at = now()` at seed time regardless of how old the actual releases are (Phase 4 D-13), so a newly-watched artist's whole seeded history stays visible together for a full window from add-time, then ages out together — this is accepted, expected behavior, not a bug.

### Env var shape & edge cases (DATA-01)
- **D-02:** New config field is `EVENT_RETENTION_DAYS`, a plain `int` with `envDefault:"90"` — not a `time.Duration` string. DATA-01's own requirement text says "defaulting to 90 days," and an integer day-count is the most operator-friendly unit for this specific setting, even though `PollInterval` elsewhere in `internal/config/config.go` uses `time.Duration`. Do not generalize this into a duration-everywhere rule — it's a per-field judgment call.
- **D-03:** `EVENT_RETENTION_DAYS <= 0` is invalid configuration and must fail fast at startup with a clear error — same posture as `DatabaseURL`'s `notEmpty` tag — rather than being silently interpreted as "show everything" or "hide everything." Avoids a typo (e.g. an unquoted negative number) producing a confusing, silent behavior change instead of an obvious boot failure. — **Reversibility:** reversible — a validation rule, easy to relax later if a real "disable retention" use case emerges.

### Near-boundary behavior
- **D-04:** The cutoff comparison is inclusive on the "still visible" side: an event with `created_at` exactly at the window boundary (e.g. exactly 90 days old) is treated as still within the window and remains visible (`created_at >= cutoff`, not `>`). Matches the roadmap's literal wording — "older than the window" implies strictly older, not "at or older than."

### Empty-state copy (History UI)
- **D-05:** Retention-caused emptiness gets its own, third empty-state message — distinct from `history.tsx`'s existing "No release activity yet" (truly empty, no events ever) and "No matching events" (user-applied artist/event-type filter, `isFiltered` branch). Neither existing message is accurate when retention is what emptied the feed: there IS release activity, and the user did not apply a filter. Exact copy is left to planning/UI work, but it must read as "your history isn't empty, it's just outside the retention window" rather than either existing message's implication.
- **D-06:** To select the correct one of the three empty-state messages precisely, the `GET /events` response needs a signal distinguishing "zero events ever for this scope" from "events exist but all are currently outside the retention window" — e.g. a `hasOlderEvents`-style boolean alongside the existing page/cursor response shape. A frontend-only heuristic (e.g. "watchlist is non-empty, so assume retention hid something") was explicitly rejected: it would misfire for a brand-new watchlist artist that has zero detections yet, showing the wrong empty-state message. This is new API surface beyond the existing `Page{Events, NextCursor}` shape (`internal/events/service.go`) — exact field name/shape is Claude's discretion.

### Claude's Discretion
- Exact SQL mechanism for the `created_at` cutoff filter in `queries/events.sql`'s `ListEvents` query — e.g. a Go-computed cutoff `timestamptz` parameter (`created_at >= sqlc.arg('cutoff')`) vs. an in-SQL interval expression. Not discussed; follows whichever is more idiomatic against the existing `sqlc.narg` filter pattern already in that query.
- Exact field name/shape of the "are there older events" signal added to the events API response (D-06) — e.g. `hasOlderEvents: bool` on the existing page envelope, a separate lightweight endpoint, or a count. Behavior is locked (D-06); wire shape is not.
- Exact `EVENT_RETENTION_DAYS` validation error message/mechanism (e.g. `env.Parse` custom validation vs. a manual post-parse check in `config.Load`) — implementation detail, follows whatever `caarlos0/env/v11` supports cleanly.
- Whether the cutoff is computed once per request or via a DB-side `now() - interval`, and any index needed on `events.created_at` for query performance at scale — implementation/performance detail, left to planning/research.
- Exact empty-state copy text for the new third message (D-05) — behavior/placement is locked, wording is not.

### Reviewed Todos (not folded)
- **Fix flaky tests under parallel `go test ./...`** (`.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`) — matched Phase 10 by a low-confidence keyword score (0.6, generic terms "phase"/"during"). Its own frontmatter locks `resolves_phase: 9` and it was already folded into Phase 9's context (`09-CONTEXT.md` Folded Todos) — not re-raised here; it does not belong to retention/history scope. (Note: the todo file is still present under `.planning/todos/pending/` despite being folded into a completed Phase 9 — likely a cleanup gap from that phase's closeout, unrelated to Phase 10's own scope.)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` (§ Data Retention, lines 204-207) — DATA-01, DATA-02 exact requirement text; § Out of Scope table's "Table partitioning for events retention" and "`pg_cron` extension for retention scheduling" exclusions already locked
- `.planning/ROADMAP.md` (§ Phase 10: Event Retention Window, lines 305-324) — goal, 5 success criteria, and the locked design decision (soft-delete/filter, never hard delete; success criteria 3-5 exist specifically to prove the three regressions hard delete would reintroduce)
- `.planning/PROJECT.md` (§ Current Milestone, § Key Decisions) — v1.1 milestone framing; "Events table retention: soft-delete/filter" target feature description

### Prior phase decisions this phase builds directly on
- `.planning/milestones/v1.0-phases/04-detection-engine/04-CONTEXT.md` — D-09/D-10 (events table is both seen-store and event log; dedup key is per-event-type external ID), D-13/D-14 (seed-mode first-run behavior — directly shapes D-01's "seeded catalog ages out together" consequence), D-20 (`ON CONFLICT DO NOTHING`, write-once snapshot columns)
- `.planning/milestones/v1.0-phases/06-frontend-release-history/06-CONTEXT.md` — D-05/D-06/D-07 (global chronological feed, artist/event-type filtering, cursor-based "load more" pagination — the display paths D-01/D-04 must apply consistently across), D-16 (empty states get friendly styled messages, not bare placeholder text — the pattern D-05 extends with a third message)

### Existing code (Phase 1-9)
- `queries/events.sql` — `ListEvents` (the one query this phase's filter is added to), `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified`/`MarkNotified` (the queries that must stay untouched — each is commented with exactly why it needs full, unfiltered table access)
- `internal/events/service.go` — `Service.List`, `ListParams`, `Page{Events, NextCursor}` — the Go-side surface D-06's API signal extends; `DefaultPageSize`/`MaxPageSize` clamp already lives here, not at the HTTP boundary
- `internal/config/config.go` — `Config` struct, `Load()` — where `EVENT_RETENTION_DAYS` (D-02/D-03) is added; existing `notEmpty`/fail-fast pattern (`DatabaseURL`) is the template for D-03's validation
- `internal/httpserver/events.go` (referenced in `ARCHITECTURE.md`, not yet read in full) — `handleListEvents`, where D-06's new response field would be wired if it lands on the existing `/events` response rather than a new endpoint
- `web/app/routes/history.tsx` — existing `isFiltered` branch and two current `EmptyState` calls ("Couldn't load release history" / "No release activity yet" vs "No matching events") — D-05's new third message slots in alongside these
- `web/app/components/common/EmptyState.tsx` — shared empty-state component; its own header comment already anticipates a "filtered-to-zero History" case, confirming this was a known future need
- `internal/db/sqlc/models.go` — `Event` struct, confirms `CreatedAt` is a real `timestamptz` field (`pgtype.Timestamptz` under the hood) suitable for cutoff comparison, and `ReleaseDate` is `*string`

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `queries/events.sql`'s `ListEvents` query — already uses the `sqlc.narg('x') IS NULL OR ...` filter idiom for `artist_id`/`event_type`/`cursor`; the new `created_at` cutoff filter follows the exact same pattern, keeping one statically-checkable SQL string.
- `web/app/components/common/EmptyState.tsx` — generic `{heading, body, action}` component already used for 3 distinct empty states in `history.tsx`/`watchlist.tsx`; D-05's new message is a fourth call to this same component, no new component needed.
- `internal/config/config.go`'s existing `notEmpty`-tag pattern (`DatabaseURL`) — the template for D-03's fail-fast validation, though `caarlos0/env/v11` does not have a built-in ">0" numeric validator, so this likely needs a manual post-`env.Parse` check in `Load()`.

### Established Patterns
- Every domain service (`watchlist.Service`, `events.Service`) clamps/validates inputs in the Go service layer, not at the HTTP boundary or in raw SQL (`events.Service.List`'s `PageSize` clamp is the direct precedent) — the retention cutoff computation likely belongs in `events.Service.List` (or a helper it calls), passed down to the sqlc query as a parameter.
- sqlc queries that must see the full, unfiltered table are already explicitly commented with why (`ListUnnotified`, `HasAnyEvent`, `GroupTrackCountBaseline` in `queries/events.sql`) — any new retention-filter code should preserve/extend that commenting convention so a future reader doesn't "fix" these queries to also filter by retention.

### Integration Points
- `internal/events/service.go`'s `Service.List` is the single call site the retention filter threads through — it already computes/clamps `PageSize` before calling `s.q.ListEvents`, so computing a `cutoff` `time.Time` (using the new `EVENT_RETENTION_DAYS` config value) alongside that clamp is a natural, minimal-diff extension.
- `internal/httpserver/events.go`'s `handleListEvents` is where D-06's new "are there older events" response field would be wired, if research/planning decides it belongs on the existing `/events` response envelope.
- `web/app/routes/history.tsx`'s existing `isFiltered ? ... : ...` conditional is where D-05's third branch gets added.

</code_context>

<specifics>
## Specific Ideas

No particular UI/visual references beyond D-05's requirement that the new empty-state message read as "your history isn't empty, it's just outside the window" — exact wording is left open. The recurring theme across this discussion was minimizing surprise: prefer fail-fast over silent reinterpretation of config edge cases (D-03), prefer an honest third UI message over reusing an inaccurate existing one (D-05), and prefer a reliable timestamp column over a display-oriented one for the actual filtering logic (D-01) — all in service of the roadmap's already-locked "nothing is ever deleted, detection state never regresses" design decision.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.

</deferred>

---

*Phase: 10-Event Retention Window*
*Context gathered: 2026-08-13*
