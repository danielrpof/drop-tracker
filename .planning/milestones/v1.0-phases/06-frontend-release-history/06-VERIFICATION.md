---
phase: 06-frontend-release-history
verified: 2026-08-11T20:30:00Z
status: passed
score: 13/14 must-haves verified
behavior_unverified: 1
overrides_applied: 0
human_verification:

  - test: "Force a PATCH /watchlist/{id} failure while toggling a release-type or mute checkbox (e.g. devtools 'offline' or a 500 stub) and observe the checkbox in the browser"
    expected: "The checkbox visually reverts to the server's true prior value (not the clicked value) and the 'Couldn't update preferences — try again.' toast appears — the UI must never keep showing an unpersisted toggle state as saved"
    why_human: "PreferenceToggles.tsx's catch block calls onEntryChange(entry.id, { ...: previous }) and shows the fixed toast, which reads correctly on code inspection, and the phase's own 7-step UAT (06-04-PLAN.md Task 2) exercised only the success path (toggle-then-reload) — no step forced a PATCH failure to watch the rollback happen live. This is exactly the kind of state-transition truth presence/wiring checks can't confirm; it needs one live click with a forced failure."
---

# Phase 6: Frontend & Release History Verification Report

**Phase Goal:** Users can manage their watchlist and review detected release activity entirely through a web UI, without touching the API directly.
**Verified:** 2026-08-11
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Operator can search MusicBrainz/Deezer and see results in two separately-labelled columns, no modal (UI-01) | ✓ VERIFIED | `SearchBox.tsx` (debounced input) + `SearchResultsColumns.tsx` (`Object.keys(response.sources)` per-column render, no `.flat()`/`.concat()`); confirmed live in 06-04 UAT step 1 (approved) |
| 2 | One-click add appears in the watchlist list below with no reload, no pref form (UI-01, D-10) | ✓ VERIFIED | `watchlist.tsx#handleAddSearchResult` → `addWatchlist()` → `refresh()`; UAT step 1 |
| 3 | A Deezer-sourced add no longer corrupts `mbid` with the Deezer catalog id (CR-01, code-review Critical) | ✓ VERIFIED | Fixed in commit `8418587`. `SearchResultsColumns.tsx` disables "Add to Watchlist" for non-MusicBrainz results (`canAdd = sourceName === "musicbrainz"`), and `watchlist.tsx#handleAddSearchResult` defense-in-depth-rejects any non-MusicBrainz source before calling `addWatchlist`. Verified present on disk, not just claimed in SUMMARY |
| 4 | "Already watching" cross-reference dispatches on source (mbid for MusicBrainz, deezer_id for Deezer) (WR-01 fix, UI-01/D-11) | ✓ VERIFIED | `SearchResultsColumns.tsx:106-111`: `sourceName === "deezer" ? entry.deezer_id === artist.id : entry.mbid === artist.id`; UAT step 2 |
| 5 | A failing search source shows its own inline message while the healthy source stays populated (D-03) | ✓ VERIFIED | `SourceColumn` branches independently on `result.status === 'error'` per column |
| 6 | Operator can open a Watchlist tab and see every tracked artist via the web UI, without curl (UI-02) | ✓ VERIFIED | `web/app/routes/watchlist.tsx` fetches `listWatchlist()` on mount, renders `WatchlistRow` per entry, nav tab wired in `root.tsx`; UAT confirmed |
| 7 | Inline release-type/mute preference toggles persist immediately (single-axis PATCH), survive reload (UI-02, D-12) | ✓ VERIFIED | `PreferenceToggles.tsx#toggleReleaseType`/`toggleMutedEventType` call `updateWatchlistPreferences` with exactly one axis; UAT step 3 (toggle, reload, confirm persisted) approved |
| 8 | Concurrent independent-axis toggles (release-type + mute fired in quick succession) don't let a stale response clobber the other axis's UI state (WR-02 fix) | ✓ VERIFIED | Fixed in `8418587`. `onEntryChange` is now `(id, patch: Partial<WatchlistEntry>) => void` and `handleEntryChange` in `watchlist.tsx` merges `{ ...r, ...patch }` via a functional state update instead of replacing the whole row — code confirmed on disk matching the review's prescribed fix |
| 9 | A preference toggle whose PATCH fails visually reverts to the prior state and shows the fixed toast, never displaying an unpersisted value as saved (D-12 prohibition, judgment-tier) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Code path is present and correct on inspection (`catch` block restores `previous` via `onEntryChange` + fixed toast), but the 7-step UAT exercised only the success path — no step forced a PATCH failure to observe the live revert. See Human Verification below |
| 10 | Remove is one click, no blocking dialog, 5s Undo toast honestly labelled as defaults-only (UI-02) | ✓ VERIFIED | `watchlist.tsx#handleRemove`: optimistic filter, `toast.success(...)` with `description: "Undo re-adds this artist with default preferences..."`, no `AlertDialog`/`window.confirm`; UAT step 4 |
| 11 | `GET /events` filters (artist_id, event_type), paginates by cursor with no repeated rows, clamps an over-large limit, and rejects malformed params with 400 (HIST-01) | ✓ VERIFIED | `internal/httpserver/events.go` validated logic; ran the actual scoped tests against real Postgres — `TestHandleListEvents_Validation`, `TestListEvents_Filters`, `TestListEvents_OrderedNewestFirstAndKeysetPaginates` all `--- PASS` (this session, not just SUMMARY claims) |
| 12 | History feed renders type-specific cards (new_release/guest_feature/deluxe_change) with correct null-field fallbacks (UI-03, D-08, D-14) | ✓ VERIFIED | `EventCard.tsx` dispatches on `event_type`, `NewReleaseBody`'s "Release date unknown" fallback, `DeluxeChangeBody`'s null-`previous_track_count` no-arrow case; UAT step 5 |
| 13 | Filter/pagination composition and the filtered-vs-global-empty distinction hold (HIST-01, D-16) | ✓ VERIFIED | `history.tsx`: `isFiltered` branch selects "No matching events" vs "No release activity yet"; Load more hidden when `next_cursor` is null; UAT step 6 |
| 14 | A single Go binary serves both the JSON API and the embedded React SPA, no Node runtime (UI-03 architecture) | ✓ VERIFIED | `internal/webassets/embed.go` (`//go:embed all:build/client`, `fs.Sub`, chi `NotFound` fallback); `go build ./...`, `go vet ./...`, `go build -o ./bin/server ./cmd/server` all exit 0 in this session; committed `internal/webassets/build/client/` assets match the post-fix commit `8418587` (`git status` clean, no stale embed) |

**Score:** 13/14 truths verified (1 present + wired, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `queries/events.sql` | `ListEvents :many` keyset query | ✓ VERIFIED | present, `ORDER BY id DESC` confirmed by passing keyset test |
| `internal/events/service.go` | `Store`/`Service`/`NewService`, page-size clamp | ✓ VERIFIED | compiles, `go vet` clean, exercised by passing tests |
| `internal/httpserver/events.go` | validated `GET /events` handler | ✓ VERIFIED | 4-param validation present, `writeError`/`httplog.SetAttrs` pattern matches sibling handlers |
| `internal/webassets/embed.go` | go:embed + SPA fallback | ✓ VERIFIED | builds, `TestSPA_*` pass |
| `web/app/lib/api.ts` | typed wrappers for all 5 backend endpoints | ✓ VERIFIED | all 6 exported functions present, wire shapes match Go structs |
| `web/app/routes/history.tsx` | live History feed | ✓ VERIFIED | fetches `listEvents`, no hardcoded fixture, full state coverage |
| `web/app/routes/watchlist.tsx` | Watchlist management + search | ✓ VERIFIED | list/preferences/remove/undo/search all wired |
| `web/app/components/history/EventCard.tsx` | type-dispatching card | ✓ VERIFIED | 3 type bodies present |
| `web/app/components/history/HistoryFilters.tsx` | artist/event-type filters | ✓ VERIFIED | populated from `listWatchlist()` + fixed event-type set |
| `web/app/components/watchlist/WatchlistRow.tsx`, `PreferenceToggles.tsx` | row + toggles | ✓ VERIFIED | present, wired to `watchlist.tsx` state |
| `web/app/components/watchlist/SearchBox.tsx`, `SearchResultsColumns.tsx` | search + results | ✓ VERIFIED | present, wired |
| `web/app/components/common/CoverArt.tsx`, `EmptyState.tsx` | shared components | ✓ VERIFIED | null/error fallback present, reused by all 3 tabs' surfaces |
| `internal/webassets/build/client/` | committed SPA build output | ✓ VERIFIED | present, tracked, matches latest fix commit `8418587` (rebuilt per that commit's message), no stale-embed drift (`git status` clean) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/httpserver/server.go` | `internal/webassets/embed.go` | `r.NotFound(webassets.Handler().ServeHTTP)` after every explicit route | ✓ WIRED | confirmed by reading `server.go`; `TestSPA_APIRoutesStillReachTheirOwnHandlers` passes |
| `internal/httpserver/events.go` | `internal/events/service.go` | `s.events.List(r.Context(), events.ListParams{...})` | ✓ WIRED | confirmed in source, exercised by passing tests |
| `web/app/routes/history.tsx` | `web/app/lib/api.ts` | `listEvents(...)` in an effect | ✓ WIRED | confirmed in source |
| `web/app/routes/watchlist.tsx` | `web/app/lib/api.ts` | `listWatchlist`/`addWatchlist`/`removeWatchlist`/`updateWatchlistPreferences` | ✓ WIRED | confirmed in source |
| `web/app/components/watchlist/SearchBox.tsx` | `web/app/lib/api.ts` | `searchArtists(query)` after debounce | ✓ WIRED | confirmed; note `AbortSignal` is not actually threaded through to `fetch` (WR-04, deferred — doesn't break the wiring, only the cancellation optimization) |
| `web/app/components/watchlist/PreferenceToggles.tsx` | `web/app/routes/watchlist.tsx` | `onEntryChange(id, patch)` partial-merge callback | ✓ WIRED | confirmed post-WR-02-fix: `handleEntryChange` merges a partial patch via functional state update |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` | `go build ./...` | exit 0 | ✓ PASS |
| `go vet ./...` | `go vet ./...` | exit 0 | ✓ PASS |
| Real-Postgres `GET /events` filter/cursor/order tests | `go test ./internal/httpserver/ -run 'TestListEvents_OrderedNewestFirstAndKeysetPaginates\|TestListEvents_Filters\|TestHandleListEvents_Validation\|TestSPA' -count=1 -v` | all `--- PASS` | ✓ PASS |
| Full `internal/httpserver` package suite | `go test ./internal/httpserver/... -count=1` | `ok` (6.26s) | ✓ PASS |
| Frontend build (SPA Mode, no server bundle) | `cd web && pnpm run build` | `build/client` present, server bundle removed per `ssr:false` | ✓ PASS |
| Frontend typecheck | `cd web && pnpm exec tsc --noEmit -p tsconfig.json` | no output, exit 0 | ✓ PASS |
| Single-binary build | `go build -o ./bin/server ./cmd/server` | exit 0, binary produced | ✓ PASS |
| Working tree clean after builds (no stale/untracked embed drift) | `git status --short` | empty | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| UI-01 | 06-04 | Search for and add an artist via the web UI | ✓ SATISFIED | Truths 1-5; UAT step 1-2 approved; CR-01/WR-01 fixed and verified in code |
| UI-02 | 06-03 | View and manage (remove, set preferences) watchlist via web UI | ✓ SATISFIED (1 behavior-unverified sub-item) | Truths 6-10; UAT step 3-4 approved; WR-02 fixed and verified; PATCH-failure rollback not live-tested |
| UI-03 | 06-01, 06-02 | Browse a feed/history of detected release events via web UI | ✓ SATISFIED | Truths 12-14; UAT step 5, 7 approved |
| HIST-01 | 06-01, 06-02 | View history of detected events per artist, including what changed | ✓ SATISFIED | Truths 11, 13; UAT step 6 approved; automated real-Postgres tests pass |

All four requirement IDs declared in this phase's plan frontmatter (`HIST-01`, `UI-03` in 06-01/06-02; `UI-02` in 06-03; `UI-01` in 06-04) are accounted for and match REQUIREMENTS.md's traceability table, which already marks all four `Complete` for Phase 6. No orphaned requirements found — REQUIREMENTS.md's Frontend section maps exactly UI-01/UI-02/UI-03 to Phase 6, plus HIST-01 under History, all four claimed by this phase's plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `web/app/components/history/EventCard.tsx` | 39 | `EVENT_BADGE[event.event_type]` with no fallback for an unrecognized value | ⚠️ Warning (deferred, WR-03) | Would crash the whole History route via the top-level ErrorBoundary if the DB ever returns an event_type outside the known 3; low likelihood given the DB CHECK constraint, but no defensive fallback exists. **Deferred as a todo with explicit user approval** (`.planning/todos/pending/`), does not block this phase's goal |
| `web/app/lib/api.ts` (`apiFetch`), `web/app/components/watchlist/SearchBox.tsx` | 100-122, 44-68 | `AbortController.abort()` is called but the signal is never forwarded to `fetch()`, so the "cancelled" request still completes server-side | ⚠️ Warning (deferred, WR-04) | Superseded searches still consume MusicBrainz/Deezer rate-limit budget; doesn't break correctness (stale responses are discarded client-side via the `aborted` guard) or violate any must-have truth (debounce + spinner truth doesn't require true cancellation). **Deferred as a todo with explicit user approval** |
| `web/app/components/history/EventCard.tsx` | 108-116 | `guestFeatureHref` interpolates `external_id` into a URL without `encodeURIComponent` | ℹ️ Info (deferred, IN-01) | Low risk given MusicBrainz/Deezer ids are UUIDs/numeric strings; **deferred as a todo with explicit user approval** |

None of these three items are new findings — all three were already surfaced by `06-REVIEW.md` (code review, standard depth) and explicitly deferred by the human user with the orchestrator's capture into `.planning/todos/pending/` (commit `08e8fc3`). They do not threaten any of this phase's roadmap success criteria and are correctly excluded from this verification's gap list.

### Human Verification Required

1. **PATCH-failure preference rollback (D-12 prohibition, judgment-tier)**
   - **Test:** With the app running, open devtools, throttle/block the network (or otherwise force a `PATCH /watchlist/{id}` to fail), then click a release-type or mute checkbox in a watchlist row.
   - **Expected:** The checkbox visually reverts to its pre-click (server's true) state within a moment, and the toast "Couldn't update preferences — try again." appears. The UI must never keep showing the clicked (unsaved) value as if it persisted.
   - **Why human:** `PreferenceToggles.tsx`'s `catch` block is present and structurally correct on code inspection (`onEntryChange(entry.id, { release_types: previous })` restores the pre-toggle array, paired with the fixed toast), and this exact failure-path guarantee is what code review's WR-02 fix (verified above) depends on being correct — but the phase's own 7-step UAT walkthrough (06-04-PLAN.md Task 2, approved) only exercised the success path (toggle → reload → confirm persisted). No step forced a live PATCH failure to observe the revert happen in the browser. This is a state-transition truth that presence/wiring checks cannot confirm on their own.

## Gaps Summary

No gaps block phase goal achievement. All four roadmap/requirement success criteria (UI-01, UI-02, UI-03, HIST-01) are implemented, wired, and confirmed either by passing automated tests (backend `GET /events` filtering/pagination/validation, SPA embed fallback) or by the human user's already-completed 7-step UAT walkthrough (documented in `06-04-SUMMARY.md`, approved). The one Critical code-review finding (CR-01: Deezer id written into `mbid`) and both Warnings that would have affected this phase's must-haves (WR-01 wrong-field cross-reference, WR-02 concurrent-toggle UI clobber) were independently re-verified present-and-correct in the current source tree, not merely trusted from SUMMARY claims.

The sole open item is a single judgment-tier prohibition (`MUST NOT display an optimistically-updated preference toggle as saved when its PATCH failed`) whose code path looks correct on inspection but was not exercised live during UAT's success-path-only walkthrough — routed to human verification above rather than silently marked passed. Three lower-severity code-review findings (WR-03 badge-fallback crash risk, WR-04 non-functional AbortController, IN-01 missing URL-encoding) were correctly deferred as todos with the user's prior explicit approval and are reported here as informational, not gaps, since none of them threaten a roadmap success criterion.

---

_Verified: 2026-08-11_
_Verifier: Claude (gsd-verifier)_
