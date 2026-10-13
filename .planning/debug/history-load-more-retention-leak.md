---
status: diagnosed
trigger: "Phase 25 UAT test 7 (G-25-7): using load more shows events older than the retention edge, rest pass"
created: 2026-10-11T15:00:00Z
updated: 2026-10-11T15:30:00Z
goal: find_root_cause_only
---

## Current Focus

hypothesis: CONFIRMED. The retention filter works exactly as coded. "Older" events appear because retention and display/order use different date axes. Retention is keyed on created_at (detection time, Phase 10 D-01). The feed is ordered by and displays only release_date (quick task 260825-g6i). Seed-mode backfill gives years-old releases a fresh created_at. Load more walks back through release chronology into those back-catalogue rows, all of which are inside the created_at window.
bug_class: Bohrbug (deterministic; reproducible via API on every walk). Semantic/contract mismatch, not a missing predicate.
known_pattern_candidate: none (KB entries don't match)
next_action: return ROOT CAUSE FOUND to orchestrator (find_root_cause_only; no fix applied)

reasoning_checkpoint:
  hypothesis: "Load more surfaces events the user reads as 'older than the retention window' because ListEvents filters on created_at >= now()-90d, but orders by release_date DESC and EventCard shows only release_date. Seed-mode backfill rows carry years-old release_date with a recent created_at, so they pass the cutoff and fill every page past ~position 2-7, getting older with each Load more."
  confirming_evidence:
    - "queries/events.sql ListEvents: `AND created_at >= cutoff` is a top-level AND outside the cursor CASE, so it applies on page 1 and every cursor page and with tag_id alike. Generated SQL in internal/db/sqlc/events.sql.go is identical."
    - "Live API walk (GET /events?tag_id=81 and tag_id=12, following next_cursor to exhaustion): 0 of 155 / 0 of 425 rows have created_at < cutoff, but 153/155 and 421/425 have release_date < 2026-07-13, reaching 2015-04-08 / 2014-06-21. Each page steps years further back (tag 12 p1 2025-05..2026-09, p2 2024-03..2025-05, p3 2023-05..2024-03, p18 2014..2016)."
    - "Read-only DB: 4462 events, 0 outside the created_at window (min created_at 2026-08-23, 49 days ago; cutoff 2026-07-13); 3905 rows have release_date older than the cutoff date. Tag 81's rows were all created 2026-10-11 (seeded today) with release dates back to 2015."
    - "Phase 10 10-CONTEXT D-01 explicitly chose created_at over release_date and records the consequence as 'accepted, expected behavior, not a bug'. D-01 was written when ListEvents ordered by id DESC (a created_at proxy). 260825-g6i (2026-08-25) switched ordering to release_date DESC and left retention untouched on purpose."
    - "EventCard.tsx renders only `event.release_date ?? 'Release date unknown'`; created_at is never shown, so the user has no way to see the axis retention uses."
  falsification_test: "Find any row returned by GET /events (any page, any tag) whose created_at < now()-EVENT_RETENTION_DAYS. One such row would prove a real predicate leak. Exhaustive cursor walks for tags 12 and 81 found none."
  fix_rationale: "N/A (diagnose-only). The fix is a contract decision, not a missing WHERE clause. Either (A) keep D-01 and make the UI/UAT honest about the detection-date axis, or (B) reverse D-01 and add release-date-based visibility. Adding a predicate without that decision would re-open D-01's partial/NULL-date and hide-the-whole-seeded-catalogue concerns."
  blind_spots: "I can't observe exactly which card the user judged 'older than the retention edge'. The inference that they compared release_date to 90 days rests on: (1) it's the only date on the card, (2) there are zero rows on the far side of the created_at edge in this DB, so no other reading is possible on this data. A small chance remains that the user saw a WR-01 stale-append (previous-tag rows) and called it 'older'. Even then, every request carries the cutoff, so it still couldn't be a created_at leak."
  candidate_causes:
    - "code: cutoff predicate missing on cursor pages / tag variant (ELIMINATED, see Eliminated)"
    - "config/environment: wrong EVENT_RETENTION_DAYS or stale running binary (ELIMINATED, env=90, image built 2026-10-11T14:43Z, tag filter live)"
    - "data: seed-mode backfill rows have recent created_at + old release_date (CONFIRMED contributing)"
    - "spec/design: retention axis (created_at) differs from order/display axis (release_date); UAT truth doesn't name the axis (CONFIRMED contributing)"
  and_gate: "yes. Both are required: (1) retention keyed on created_at with seed backfill stamping old releases as freshly created, AND (2) the feed ordered and labelled by release_date with no detection date shown. With (1) alone and the pre-g6i id-DESC order, Load more walked back in detection time and the feed ended at the retention edge, matching user intuition. With (2) alone and release-date retention, old releases would be hidden. Contributing: the dev DB has no row beyond the created_at edge (oldest 49d), so test 7's 'events on both sides of the retention window' precondition was unsatisfiable under the real contract, and only the release_date reading was observable."

## Symptoms

expected: With a History Tag filter active (and generally), paging via Load more never returns events older than the retention window; reload keeps the tag; Back restores the previous combination.
actual: User reported "using load more shows events older than the retention edge, rest pass"
errors: None reported
reproduction: Test 7 in .planning/phases/25-find-filter-watchlist-and-history/25-UAT.md
started: Discovered during phase 25 UAT

## Eliminated

- hypothesis: The retention cutoff is applied on the first page only; cursor (Load more) pages skip it.
  evidence: queries/events.sql places `AND created_at >= sqlc.arg('cutoff')` as a top-level AND, outside the `cursor_id IS NULL OR CASE ...` group. Generated internal/db/sqlc/events.sql.go matches. Live cursor walks to exhaustion for tags 12 and 81 returned 0 rows with created_at < cutoff across 25 pages.
  timestamp: 2026-10-11T15:20:00Z

- hypothesis: The tag-filtered query variant lacks the cutoff (separate query path).
  evidence: There is one static ListEvents SQL; tag_id is an optional sqlc.narg EXISTS inside it, so it cannot bypass the cutoff. Live walks used tag_id and found 0 aged rows.
  timestamp: 2026-10-11T15:20:00Z

- hypothesis: The frontend Load more drops the filters or bypasses retention.
  evidence: history.tsx handleLoadMore calls the same fetchHistoryPage(filters, nextCursor) as page 1 (artistId/eventType/tagId + cursor). Retention is server-side and computed per request in events.Service.List, so the client cannot opt out.
  timestamp: 2026-10-11T15:20:00Z

- hypothesis: Environment misconfiguration (EVENT_RETENTION_DAYS unset/huge) or a stale running binary.
  evidence: The app container env has EVENT_RETENTION_DAYS=90. The image was built 2026-10-11T14:43Z (post-phase-25). config.Load rejects <=0. The tag filter works live, so the phase-25 binary is running.
  timestamp: 2026-10-11T15:20:00Z

- hypothesis: sqlc drift (generated code lacks the cutoff that the .sql has).
  evidence: The generated listEvents constant contains `AND created_at >= $6::timestamptz`, identical to the source.
  timestamp: 2026-10-11T15:20:00Z

- hypothesis: WR-01 stale Load-more append leaks out-of-window rows.
  evidence: Each request, stale or not, carries the server cutoff, and 0 rows in the DB are beyond the created_at edge anyway. WR-01 can leak previous-filter rows but cannot produce a created_at-aged row.
  timestamp: 2026-10-11T15:20:00Z

## Evidence

- timestamp: 2026-10-11T15:00:00Z
  checked: .planning/debug/knowledge-base.md (Phase 0)
  found: Two entries (guest-feature-label-missing, backlog-songs-trigger-discord). Neither concerns history paging/retention filtering. backlog-songs-trigger-discord involves release-date freshness gating (NOTIFY_MAX_RELEASE_AGE_DAYS), a different "age" concept.
  implication: No known-pattern candidate; proceed with open investigation.

- timestamp: 2026-10-11T15:05:00Z
  checked: queries/events.sql ListEvents + internal/db/sqlc/events.sql.go (generated) + internal/events/service.go List
  found: `AND created_at >= sqlc.arg('cutoff')::timestamptz` is a top-level AND, outside the cursor CASE, so it applies on page 1 and every cursor page, and in the tag/artist/type variants alike (one static query). The generated SQL matches the source (no sqlc drift). Service computes cutoff = time.Now().AddDate(0,0,-retentionDays) with Valid=true on every call. ORDER BY is `release_date DESC NULLS LAST, id DESC`. Retention is keyed on created_at (detection time), but ordering and display use release_date (the release's own date).
  implication: The server cannot return a row with created_at < cutoff on any page. Retention axis (created_at) and display/order axis (release_date) differ.

- timestamp: 2026-10-11T15:08:00Z
  checked: web/app/routes/history.tsx fetchHistoryPage/handleLoadMore
  found: Load more calls the same fetchHistoryPage(filters, nextCursor), passing artistId/eventType/tagId + the opaque cursor. No client-side date filtering. hasOlderEvents is used only for the empty state; nothing renders a retention-edge marker in a non-empty feed.
  implication: Frontend sends the correct filters on Load more. The UI never indicates where the retention edge is or which date it is measured on.

- timestamp: 2026-10-11T15:10:00Z
  checked: contract docs - PROJECT.md L90/L115/L200, config.go, 25-VERIFICATION.md SC4/WR-01, queries/events.sql Phase 10 comment
  found: Phase 10 DATA-01/02 defines retention as a read-side soft filter hiding events older than 90 days, measured on created_at. EVENT_RETENTION_DAYS default 90. 25-VERIFICATION SC4 was verified by TestRetention_TagFilterComposesWithCutoff, whose fixtures (insertTestEventAt) set created_at only and leave release_date NULL. UAT test 7 / VERIFICATION human check 7 say "events on both sides of the retention window" and "No event older than the retention window" without naming the date axis.
  implication: The implemented contract is detection-age retention. The UAT wording is ambiguous, and the automated test never exercises "recent created_at + old release_date".

- timestamp: 2026-10-11T15:16:10Z
  checked: Running stack - docker app container env + read-only psql against drop-tracker-postgres-1
  found: App has EVENT_RETENTION_DAYS=90 (image built 2026-10-11T14:43Z, current). DB now=2026-10-11 15:16Z, cutoff=2026-07-13. events: total 4462; in-window by created_at 4462; aged-out by created_at 0; min(created_at)=2026-08-23 21:24Z (49 days ago); 3905 rows have release_date < cutoff date; 508 undated.
  implication: No row in the dev DB is outside the retention window by its actual definition. has_older_events is false for every scope. Every "older" event the user saw is a back-catalogue release (old release_date) detected in the last 49 days.

- timestamp: 2026-10-11T15:18:00Z
  checked: Per-tag feed simulation (read-only SQL replicating ListEvents order + cutoff)
  found: Hip-Hop (12): 425 feed rows, first release_date older than 90d at position 5, 20 such rows on page 1, oldest release 2014-06-21, oldest detected 2026-08-26. Melodic Rap (13): 265 rows, first at position 7, 18 on page 1, oldest 2018-05-27. dembow (81): 155 rows, first at position 2, 23 on page 1, oldest 2015-04-08, all detected 2026-10-11.
  implication: Old release dates appear from page 1 onward; Load more makes them dominant and progressively older. That's why the user ties it to Load more.

- timestamp: 2026-10-11T15:20:00Z
  checked: EventCard.tsx date rendering
  found: All three body variants render `event.release_date ?? "Release date unknown"`; created_at is never displayed.
  implication: The only date a user can judge "older" by is release_date, which isn't the retention axis.

- timestamp: 2026-10-11T15:22:00Z
  checked: Phase 10 10-CONTEXT.md (2026-08-13) D-01 vs quick task 260825-g6i (2026-08-25)
  found: D-01 chose created_at deliberately (release_date is partial/nullable free text) and records "a freshly-seeded artist's entire back-catalog is inserted with created_at = now() ... stays visible together for a full window ... accepted, expected behavior, not a bug". D-01 assumed ListEvents sorted by id DESC (a created_at proxy). g6i later switched ordering to release_date DESC NULLS LAST and explicitly did not touch the retention predicate.
  implication: Before g6i, Load more walked back in detection time and the feed's tail was the retention edge, so the user's intuition held. After g6i the order axis and retention axis diverged, and Load more walks back in release time through rows retention correctly keeps.

- timestamp: 2026-10-11T15:25:00Z
  checked: Live API - node script walking GET /events?tag_id=N following next_cursor to exhaustion (scratchpad walk.js)
  found: tag 81: 7 pages, 155 rows, aged_by_created_at=0, release_date older than 2026-07-13 = 153; p1 release 2024-05..2026-10, p2 2023-02..2024-05, p3 2021-04..2022-10, p7 2015-04..2018-06; has_older_events=false throughout. tag 12: 18 pages, 425 rows, aged_by_created_at=0, older-release=421; p1 2025-05..2026-09, p2 2024-03..2025-05, p18 2014-06..2016-10.
  implication: Reproduces the user's observation end to end and proves zero created_at leakage. The symptom is release-date age, not retention violation.

- timestamp: 2026-10-11T15:27:00Z
  checked: Read-only simulation of shorter EVENT_RETENTION_DAYS for UAT re-test feasibility
  found: At 45 days: Hip-Hop 55 visible / 370 hidden, Melodic Rap 8 / 257, dembow 156 / 0. At 30 days: Hip-Hop 4 / 421.
  implication: Running the dev app with EVENT_RETENTION_DAYS=45 gives the Hip-Hop tag ~3 pages of feed with real rows on both sides of the created_at edge, which makes UAT test 7's precondition satisfiable.

## Resolution

root_cause: "Not a missing retention predicate. ListEvents applies `created_at >= cutoff` on every page and filter variant, and live cursor walks return zero out-of-window rows. The symptom is a contract/presentation mismatch that needs two conditions together (AND-gate): (1) retention is measured on created_at, the detection time (Phase 10 D-01), and seed-mode backfill stamps an artist's entire years-old back catalogue with a fresh created_at, so those rows are inside the 90-day window; AND (2) since quick task 260825-g6i the History feed is ordered by release_date DESC and EventCard shows only release_date, never the detection date. Load more therefore pages backward through release chronology into 2014-2018 releases that retention correctly keeps, and the user, who can only see release_date, reads them as 'older than the retention edge'. Contributing: the dev DB has no event beyond the created_at edge (oldest detection is 49 days), so UAT test 7's 'events on both sides of the retention window' precondition was unsatisfiable under the real contract, and the UAT/VERIFICATION wording never states which date retention uses."
fix: ""
verification: ""
files_changed: []
