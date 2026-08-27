---
status: resolved
trigger: "Users are getting Discord notifications for old songs, not just new ones. The backlog of an artist shouldn't trigger the Discord notification unless the release date is the same date as today."
created: 2026-08-26
updated: 2026-08-26
---

## Current Focus

bug_class: Bohrbug (deterministic — reproduces on every poll cycle, visible live in production logs right now)

reasoning_checkpoint:
  hypothesis: "Backlog releases reach Discord because notification suppression is decided by WHEN a row was inserted (seed mode = the artist's first-ever cycle for a source), not by WHAT the row is (how old the release actually is). Seed mode is a one-shot latch that flips off the instant the first event row for (artist_id, source) lands. The guest-feature pass deliberately spreads one artist's backlog across many cycles (maxNewGuestFeatureLookupsPerCycle = 20, plus per-recording lookup errors), so every backlog row inserted after cycle 1 gets notified_at = NULL. ListUnnotified has no release-date predicate whatsoever, so the notifier delivers the entire back catalogue to Discord, 20 at a time, every 15 minutes, forever."
  confirming_evidence:
    - "Live app logs, right now: repeated `detection result` lines with event_type=guest_feature, seed_mode=false, guest_feature_lookup_cap_reached_at=20, inserted_count=9..17 — for three artists, on every single cycle (musicbrainz-2, -6, -9, -16, -20)."
    - "Production DB: 249 guest_feature rows created today; 242 have release_date < 2026-08-19 (clear backlog), 0 have release_date = today. Oldest sent: 2015-05-21."
    - "Each of those rows carries a DISTINCT notified_at (57 rows -> 57 distinct timestamps for Playboi Carti), which is MarkNotified stamping now() per row after a confirmed Discord 204 — i.e. these were real Discord messages, one per row."
    - "Differential control: De La Rose (added 2026-08-23) has 22 guest_feature rows sharing ONE notified_at, and Gunna (added 2026-08-24) has 413 sharing ONE notified_at — the seedNotifiedAt single-timestamp signature. Neither produced any Discord message. Both were seeded BEFORE the Phase 13 lookup cap shipped."
    - "queries/events.sql ListUnnotified is `SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at, id` — no release-date filter exists anywhere in the notify path."
    - "git: the per-recording ReleasesForRecording lookup and maxNewGuestFeatureLookupsPerCycle=20 cap were introduced 2026-08-24 (2b19467, b9f0c62, phase 13). Before that commit detectGuestFeatures inserted every unseen recording in one uncapped pass, so an artist's whole guest-feature backlog fitted inside the single seed cycle. This matches the user's 'started recently' timeline exactly."
  falsification_test: "If the hypothesis were wrong, backlog rows would carry the shared seed timestamp (suppressed) and pending rows would be spread across event types. Observed: the pending queue is 100% guest_feature, every backlog row has its own distinct notified_at, and seed_mode=false is logged on every inserting cycle. Hypothesis survives."
  fix_rationale: "Make the suppression decision depend on a stable per-row property (is this release actually recent?) instead of a fragile temporal latch (was this the artist's first cycle?). A release-date freshness gate is evaluated per inserted row, so correctness no longer depends on whether the backlog happened to fit inside one cycle — it is immune to the lookup cap, per-recording lookup errors, rate-limiter stalls, cycle boundaries and process restarts, all of which can and do push backlog rows past the seed window. Seed mode is retained unchanged as a first-cycle belt-and-braces suppressor."
  blind_spots:
    - "Undated rows: 64% of all guest_feature rows carry no release_date at all. RESOLVED BY USER DECISION -- undated SUPPRESSES, on both the insert side (detection.onOrAfterCutoff) and the delivery side (notifier.staleReleaseDate). This deliberately INVERTS the codebase's usual 'err toward an extra alert' doctrine (isGuestFeature, withinDeluxeRecheckWindow), because an undated row is absence of evidence, not evidence of freshness, and the existing pending backlog is 64% undated -- treating undated as 'notify' would have re-flooded Discord on the first post-fix pass, which is the exact failure being fixed. Bounded cost: only 5 of the 249 back-catalogue alerts sent that day were undated. (An earlier draft of this line leaned the other way, 'undated = notify'; that leaning is SUPERSEDED and the shipped behaviour is suppress.)"
    - "Deluxe-change narrowing (NEW, found during fix review; accepted and documented in code at internal/detection/musicbrainz.go's deluxe insert): detectDeluxeChanges is bounded by withinDeluxeRecheckWindow (90 days) on the GROUP's first-release date, but the notify gate applies the tighter 7-day window to the WINNING RELEASE's date. A deluxe edition of an album released 8-90 days ago is therefore delivered only if MusicBrainz dates the deluxe release itself recently. Zero deluxe_change rows have ever been created in production, so this narrows nothing observable today. Arguably a track-count INCREASE is itself the freshness signal for this event type, making a release-date gate a category error there -- but exempting deluxe_change is a product decision, not part of this fix. Flagged for the user; revisit if deluxe_change starts firing and alerts go missing."
    - "Delivery-side staleness re-evaluation: notifier.suppresses re-checks freshness at SEND time, not insert time. A row inserted as deliverable that sits in the outbox longer than the window (e.g. a multi-day Discord outage) is silently acked without being sent. Accepted: the notify pass runs every cycle immediately after detection, so the window is only reachable under a sustained outage, and the new 'notify pass suppressed stale events' summary log makes it visible if it ever happens."
    - "Why the SEED cycle itself inserted zero guest_feature rows for the four 2026-08-26 artists is not fully pinned down (candidates: app restart mid-cycle, a RecordingsByArtist error, or rate-limiter stall pushing the pass past a cycle boundary). This is deliberately NOT load-bearing for the fix: even a perfectly-executed seed cycle leaks every guest feature past the 20th, which the live logs show happening continuously."
  candidate_causes:
    - "code: seed-mode latch is per-cycle while the guest-feature pass is deliberately multi-cycle (CONFIRMED)"
    - "code: ListUnnotified has no release-date predicate, so any NULL row is delivered regardless of age (CONFIRMED — the second half of the AND-gate)"
    - "data: MusicBrainz recording browse returns the artist's whole historical catalogue, not a recent window (CONFIRMED — recording_count 353/388/1000 per artist)"
    - "config: POLL_INTERVAL / worker counts — ELIMINATED, changes drain rate only, not the notify decision"
    - "environment: deluxe recheck window (quick/260826-gj8) — ELIMINATED, zero deluxe_change rows exist"
  and_gate: "YES — two conditions are simultaneously required and neither is sufficient alone. (1) A backlog row is inserted on a non-seed cycle so it gets notified_at = NULL; AND (2) the notify path applies no release-date filter, so NULL unconditionally means 'send to Discord'. Fixing only (1) would still leak on any future multi-cycle insert path; fixing only (2) is the durable gate. Mirrors the guest-feature-label-missing KB entry's own AND-gate shape: a per-row state that never heals, combined with a write path that cannot correct it."

next_action: "NONE -- session resolved. Fix confirmed working end-to-end in the live container: multiple poll cycles ran, 116 back-catalogue guest_feature rows were inserted pre-acknowledged (notified_at non-null), zero rows pending, zero Discord messages sent."

## Symptoms

- **Expected behavior:** A Discord notification should only fire for a release that is genuinely new — per the user, specifically: only when the release's release date is today's date. An artist's back catalogue (releases from the past) should never trigger a notification.
- **Actual behavior:** Discord notifications are firing for old/backlog songs — releases that are not new.
- **Error messages:** None reported; this is a behavioral/logic bug, not a crash or visible error.
- **Timeline:** Started recently — the user indicates this used to work correctly and only recently started misbehaving (not "every new artist has always done this since day one").
- **Reproduction:** Happens in BOTH scenarios reported by the user:
  1. Adding a brand-new artist with an existing discography — old releases get announced as if new.
  2. An existing, already-watched artist — suddenly gets a notification for an old release that was never flagged before.

## Eliminated

- hypothesis: "quick/260826-gj8's deluxe recheck window (deluxeRecheckWindowDays/withinDeluxeRecheckWindow) makes an old release look new instead of skipping it"
  evidence: "The events table contains ZERO deluxe_change rows (`SELECT count(*) FILTER (WHERE event_type='deluxe_change') FROM events` = 0). No deluxe_change event has ever been created, so this code path has never produced a notification of any kind. Additionally the gate is a pure omission — it can only suppress fetches, never create events."
  timestamp: 2026-08-26

- hypothesis: "quick/260825-g6i's History-tab release-chronology reordering causes the notifications"
  evidence: "That change touches ListEvents only — a read-side query serving GET /events. The notify path is ListUnnotified + MarkNotified, which g6i did not modify. Ordering of a UI feed cannot cause an outbound Discord POST."
  timestamp: 2026-08-26

- hypothesis: "quick/260825-09r's artistart.Matcher Deezer-link/alias retry changes release-group identity, defeating the already-seen check"
  evidence: "The artists table holds 6 rows with 6 distinct MBIDs — no duplicate/re-matched artist rows exist, so no artist_id has shifted underneath its event history. The seen-set (ListExternalIDs) is keyed on (artist_id, source, event_type) and every affected artist's events share one stable artist_id."
  timestamp: 2026-08-26

- hypothesis: "The new_release pass is leaking backlog rows"
  evidence: "Every new_release row in the DB carries a shared per-seed-cycle notified_at (2026-08-26 16:00 hour: 345 rows / 6 distinct timestamps; 18:00 hour: 378 rows / 4 distinct). Zero new_release rows are pending. The pending queue is 100% guest_feature. new_release completes inside one cycle because it needs no per-item lookup, so it never outlives the seed window."
  timestamp: 2026-08-26

## Evidence

- timestamp: 2026-08-26
  checked: "queries/events.sql — the entire notify path"
  found: "ListUnnotified is `SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC`. No release-date predicate. MarkNotified sets notified_at = now() per row after a confirmed send."
  implication: "notified_at IS NULL is the ONLY thing standing between a row and a Discord message. Release age is never consulted anywhere in the delivery path."

- timestamp: 2026-08-26
  checked: "internal/detection/detector.go isSeedMode + seedNotifiedAt"
  found: "isSeedMode is `NOT EXISTS(SELECT 1 FROM events WHERE artist_id=$1 AND source=$2)`. It is a one-shot latch: it returns true only until the very first event row for that (artist, source) pair exists. seedNotifiedAt stamps now() so seeded rows are born pre-acknowledged."
  implication: "Backlog suppression is entirely a function of insert timing, not of release age. Any row inserted after cycle 1 is unconditionally treated as news."

- timestamp: 2026-08-26
  checked: "internal/detection/musicbrainz.go detectGuestFeatures, maxNewGuestFeatureLookupsPerCycle"
  found: "The cap is 20 per artist per cycle. On reaching it, remaining recordings are explicitly 'skipped this cycle (not inserted, not marked seen), exactly like a lookup error, so they are naturally retried next cycle.' Per-recording ReleasesForRecording errors do the same."
  implication: "The guest-feature pass is by design multi-cycle, while seed mode is by design single-cycle. The two contracts are in direct conflict — the deferred remainder can never be seeded."

- timestamp: 2026-08-26
  checked: "git log / git show b9f0c62~1:internal/detection/musicbrainz.go"
  found: "Before commit b9f0c62 + 2b19467 (both 2026-08-24, phase 13), detectGuestFeatures had no per-recording lookup and no cap — it inserted every previously-unseen recording in one uncapped pass. The cap and the per-recording lookup were added together on 2026-08-24."
  implication: "Confirms the user's 'started recently' timeline. Pre-13, the whole guest-feature backlog fit inside the seed cycle and was correctly suppressed; post-13 it cannot."

- timestamp: 2026-08-26
  checked: "Live production DB — events grouped by type/source and notified state"
  found: "guest_feature: 658 total, 4 pending. new_release/deezer: 354 total, 0 pending. new_release/musicbrainz: 487 total, 0 pending. deluxe_change: 0 rows."
  implication: "The pending (about-to-be-Discord'd) queue is exclusively guest_feature. Isolates the leak to one pass."

- timestamp: 2026-08-26
  checked: "Live production DB — distinct notified_at per creation hour"
  found: "2026-08-23 21:00 -> 22 gf + 17 nr rows sharing 1 notified_at. 2026-08-24 16:00 -> 413 gf + 101 nr sharing 1 notified_at. 2026-08-26 16:00 -> 38 gf rows / 38 distinct notified_at; 17:00 -> 119 rows / 119 distinct; 18:00 -> 31 / 31; 19:00 -> 36 / 34."
  implication: "A shared timestamp is seedNotifiedAt's signature (suppressed, no Discord). One distinct timestamp per row is MarkNotified's signature (a real Discord send each). The behaviour flips exactly at the 2026-08-24 phase-13 boundary."

- timestamp: 2026-08-26
  checked: "Live production DB — release dates of guest_feature rows created today"
  found: "249 rows sent today. 242 have release_date < 2026-08-19 (backlog). 2 fall within a 7-day/future window. 5 are undated. 0 are dated today. Oldest notified release: 2015-05-21 (Playboi Carti). Per-artist: Don Toliver 92 (oldest 2019-08-03), Vory 65 (oldest 2017-12-08), Playboi Carti 57 (oldest 2015-05-21), Lil Uzi Vert 10 (oldest 2017-07-21)."
  implication: "Direct confirmation of the user's report, quantified: 97% of today's Discord traffic was pure back catalogue. Also sizes the fix — a release-date gate suppresses 242/249 while an undated-permissive policy costs only 5 alerts."

- timestamp: 2026-08-26
  checked: "Live app container logs (docker logs drop-tracker-app-1)"
  found: "Recurring on every cycle: {\"msg\":\"detection result\",\"event_type\":\"guest_feature\",\"seed_mode\":false,\"guest_feature_lookup_cap_reached_at\":20,\"inserted_count\":9..17} for artist_mbids a0723a3c, 4b2d00a2, 8837b875 across cycles musicbrainz-2/-6/-9/-16/-20. recording_count 353/388/1000."
  implication: "The bug is live and ongoing, not historical. Three artists are each dripping 9-17 backlog rows into the Discord queue every poll cycle, and will continue until their entire multi-hundred-recording catalogue drains."

- timestamp: 2026-08-26
  checked: "Release-date precision distribution across all events"
  found: "guest_feature: 684 total, 440 undated, 5 year-only, 0 year-month, 239 full-date. new_release: 841 total, 5 undated, 21 year-only, 2 year-month, 813 full-date."
  implication: "The freshness gate must handle MusicBrainz partial dates without parsing them into time.Time — the same constraint withinDeluxeRecheckWindow/earlierDate already solve by prefix-truncated string comparison. That helper is directly reusable."

## Resolution

root_cause: "AND-gate, two conditions simultaneously required and neither sufficient alone. (1) Backlog suppression is decided by INSERT TIMING via a one-shot seed-mode latch (isSeedMode returns true only until the first event row exists for an (artist_id, source) pair), but since phase 13 (2026-08-24, commits b9f0c62/2b19467) the guest-feature pass is deliberately MULTI-CYCLE — maxNewGuestFeatureLookupsPerCycle=20 plus per-recording ReleasesForRecording error-skips defer the remainder of an artist's catalogue to later cycles, by which time the latch has flipped and every deferred backlog row is inserted with notified_at = NULL; AND (2) the delivery path applies no release-date filter at all — ListUnnotified is a bare `WHERE notified_at IS NULL`, so NULL unconditionally means 'send to Discord'. Together, an artist with more than 20 guest features drips 9-20 back-catalogue rows into Discord every poll cycle indefinitely. Confirmed live: 242 of 249 guest_feature rows delivered today had release dates older than a week, reaching back to 2015, with 0 dated today."

fix: |
  Replaced the insert-timing suppression latch with a per-row RELEASE-DATE FRESHNESS GATE, applied at
  both ends of the AND-gate so neither condition alone can re-open the flood.

  (1) INSERT SIDE -- internal/detection. Added notifyGate (detector.go), which replaces seedNotifiedAt.
      Where seedNotifiedAt asked only "is this the artist's first-ever cycle for this source?",
      notifyGate.notifiedAt(releaseDate) suppresses a row if EITHER seed mode is on OR the release date
      fails onOrAfterCutoff -- a stable per-row property, so correctness no longer depends on whether an
      artist's backlog happened to fit inside one cycle. It is therefore immune to
      maxNewGuestFeatureLookupsPerCycle, per-recording lookup errors, rate-limiter stalls, cycle
      boundaries and restarts. Seed mode is retained unchanged as a first-cycle belt-and-braces
      suppressor, not as the mechanism. The gate is constructed once per Detect* call, preserving D-13
      (all rows from one seed cycle share one timestamp) and preventing a long pass that straddles
      midnight from judging its first and last rows by two different cutoffs. Wired into all four insert
      sites: DetectDeezer, DetectMusicBrainz's new_release pass, detectGuestFeatures, detectDeluxeChanges
      (which now take a notifyGate instead of a pre-computed pgtype.Timestamptz).
      onOrAfterCutoff compares dates as zero-padded ISO-8601 STRINGS, never parsing to time.Time --
      the same technique withinDeluxeRecheckWindow already uses, because MusicBrainz dates are
      legitimately partial ("2015", "2015-06") and would fail time.Parse. A partial or absent date fails
      the length check and suppresses.

  (2) DELIVERY SIDE -- internal/notifier. NotifyPending now acks-without-sending any pending row that
      fails the same freshness test (suppresses/staleReleaseDate, the exact negation of
      onOrAfterCutoff). This does two jobs the insert-side gate cannot: it drains the pre-existing
      pending backlog (rows already in the outbox were inserted by the old ungated code and would
      otherwise ALL have been delivered on the very next pass -- the exact flood being fixed), and it is
      defence in depth for the second half of the AND-gate, since the bug required a delivery path that
      never consults release age. Suppressed rows skip the inter-send spacing (no Discord request is
      made, so pacing them would only slow the first post-fix drain) and are summarised in one
      "notify pass suppressed stale events" Info log per pass, so over-suppression -- the one real risk
      this gate carries -- is visible in production instead of silent.

  REVIEW CORRECTION APPLIED: suppresses() originally returned false (DELIVER) for a nil/undated
  release date, contradicting detection's own tested behaviour. Aligned to SUPPRESS per the user's
  decision. This is deliberately the opposite of the codebase's usual "err toward an extra alert"
  doctrine, because the existing pending backlog is 64% undated and delivering it would have re-created
  the flood on the first pass. staleReleaseDate was split out of suppresses as a clock-free pure
  predicate so both halves of the gate could be pinned to one shared table of cases.

  DEAD CODE REMOVED: seedNotifiedAt (zero call sites once notifyGate landed; `unused` is in this repo's
  golangci-lint standard set, so it would have failed CI). Its D-13 rationale was preserved by moving it
  onto newNotifyGate's doc comment, and the three stale doc references to it were repointed
  (internal/detection/musicbrainz.go, internal/events/service.go, and queries/events.sql plus the two
  sqlc-generated files, hand-edited identically so `sqlc generate` reproduces them byte-for-byte).

  CONFIGURABILITY: NOTIFY_MAX_RELEASE_AGE_DAYS (default 7, rejected at config-parse time if negative)
  threads through cfg -> detection.WithNotifyMaxReleaseAgeDays + notifier.WithMaxReleaseAgeDays, so both
  halves are the same value by construction in the running process. 7 rather than the literal "same date
  as today" because three normal lags sit between a release dropping and a row being inserted
  (community-edited MusicBrainz backfill, the 15m poll interval plus the per-cycle lookup cap, and
  release_date carrying no timezone) -- a zero-day window would convert this false-positive bug into a
  strictly worse silent-loss bug. Operators wanting the strict reading can set 0.

verification: |
  Environment: Docker Postgres (docker compose up -d --wait postgres),
  TEST_DATABASE_URL=postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable

  - go build ./...  -> clean
  - go vet ./...    -> clean
  - gofmt           -> clean on all 10 changed files (checked with line endings normalised; the repo-wide
                       `gofmt -l` noise is pre-existing CRLF on Windows and unrelated to this change)
  - go test ./... -count=1 -> ALL 16 packages pass
  - Stability: go test ./internal/detection/... ./internal/notifier/... -race -count=3 -> pass, no races,
    no flakes

  MUTATION TESTING (the guardrail that proves the new tests actually bite -- each mutation was applied,
  the suite run, and the file restored):
  - Mutation 1, revert the insert-side gate (onOrAfterCutoff always true, i.e. pre-fix behaviour):
    7 TestNotifyGate_NotifiedAt subtests fail, TestDetectMusicBrainz_NonSeedCycle_Backlog... fails, and
    TestDetectGuestFeatures_PastLookupCap_BacklogNeverGoesPending fails with
    "back-catalogue guest features queued for Discord delivery = 20, want 0" -- reproducing the exact
    production symptom (the 20-per-cycle drip) inside the test suite.
  - Mutation 2, restore the reviewed-out bug (staleReleaseDate nil -> false, "undated = deliver"):
    TestStaleReleaseDate/absent_date_(SQL_NULL)_is_suppressed fails. The review correction is pinned.
  - Mutation 3, disable the delivery-side gate (suppresses always false):
    TestNotifyPending_StaleRowsAckedWithoutSending fails with 4 Discord requests instead of 1, the
    captured bodies literally showing the 2015 "Ancient Album" backlog release being posted.

  NEW TESTS:
  - internal/detection/notifygate_test.go (from the prior session; one assertion bug fixed -- its
    new_release count used `external_id LIKE mbid-%`, which also matched insertPriorEvent's own row,
    so it counted 3 and expected 2. Now matches the two exact external ids.)
    Covers: the 13-case seed/freshness truth table, D-13's shared seed timestamp, and two real-Postgres
    regression tests that run on an artist explicitly OUT of seed mode -- the exact condition the old
    code had no defence for -- including one exceeding maxNewGuestFeatureLookupsPerCycle.
  - internal/notifier/suppress_test.go (NEW -- suppresses() previously had ZERO coverage):
    the same truth table as notifygate_test.go against a fixed cutoff (so the two halves of the gate are
    provably in agreement and neither can drift), plus maxAgeDays wiring cases and a default-value test.
  - internal/notifier/notifier_test.go: TestNotifyPending_StaleRowsAckedWithoutSending, an end-to-end
    test modelling the production state at the moment this ships -- an outbox holding backlog, undated
    and partial-date rows alongside one fresh row. Asserts exactly one Discord request (the fresh row),
    that all four rows get acked (an unacked stale row would be re-suppressed forever), and that the
    suppression summary is logged.

  FIXTURE UPDATES (pre-existing tests whose fixtures predated the gate -- all four failed for the right
  reason, and were fixed by making the fixture deliverable rather than by weakening the assertion):
  - internal/notifier/notifier_test.go's two pending-row helpers now stamp release_date = today; they
    test delivery MECHANICS (spacing, retry, CAS guard, mark-notified failure) and their rows must
    actually reach the Sender.
  - internal/detection/detector_test.go: TestDetector_SecondCycleLeavesNotifiedAtNull,
    TestDetector_ReAddDoesNotReSeed and TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier
    now date their non-seed group today, so they keep testing seed-vs-non-seed semantics instead of
    accidentally re-testing the freshness gate.

  LIVE END-TO-END VERIFICATION (2026-08-26, CONFIRMED FIXED):
  The user ran `docker compose up -d` and let multiple poll cycles run (musicbrainz-14, musicbrainz-18,
  deezer-16 through deezer-19), and reported ZERO Discord notifications. Independently corroborated by
  two pieces of direct evidence, so the confirmation does not rest on the user's report alone:

  - Binary identity check (rules out a stale pre-fix image serving the "no notifications" result):
    `docker exec drop-tracker-app-1 sh -c "strings /usr/local/bin/server | grep -c 'notify pass suppressed stale events'"`
    returned 1 -- the running container's binary genuinely contains this change.
  - Live DB state:
    `SELECT event_type, count(*), count(*) FILTER (WHERE notified_at IS NULL) AS pending,
     count(*) FILTER (WHERE created_at > now() - interval '15 minutes') FROM events GROUP BY event_type`
    -> guest_feature: 1087 total, 0 pending, 116 inserted in the last 15 minutes. A spot-check of those
    116 freshly-inserted rows (release dates 2017-2021 -- unambiguous back catalogue) showed EVERY one
    already carrying a non-null notified_at. Zero pending. The exact rows that were drip-feeding Discord
    9-20 at a time before the fix are now born pre-acknowledged.

  Why no "notify pass suppressed stale events" line appears in the logs, and why that is the CORRECT
  observation rather than a sign the gate is inert: that log fires only in the DELIVERY-side gate
  (internal/notifier), which by construction only ever sees rows that reached the pending queue. The
  INSERT-side gate (internal/detection's notifyGate) is catching all 116 backlog rows before they can
  become pending, so the delivery-side gate is handed nothing to suppress and has nothing to report.
  Both halves of the AND-gate fix are present; the first one is simply doing all the work now that the
  pre-existing pending backlog has drained. Zero Discord messages sent, zero pending rows.

files_changed:
  - "internal/detection/detector.go: added notifyGate/newNotifyGate/notifiedAt/onOrAfterCutoff, DefaultNotifyMaxReleaseAgeDays, Option + WithNotifyMaxReleaseAgeDays, variadic New; REMOVED dead seedNotifiedAt"
  - "internal/detection/musicbrainz.go: new_release/guest-feature/deluxe passes take a notifyGate and gate per row; documented the known deluxe narrowing; repointed a stale seedNotifiedAt doc reference"
  - "internal/detection/deezer.go: DetectDeezer gates per row on a.ReleaseDate"
  - "internal/notifier/notifier.go: maxAgeDays field, Option + WithMaxReleaseAgeDays, defaultMaxReleaseAgeDays, suppresses + staleReleaseDate, ack-without-send branch and suppression summary log in NotifyPending"
  - "internal/config/config.go: NotifyMaxReleaseAgeDays env var (default 7) + non-negative validation"
  - "cmd/server/main.go: threads cfg.NotifyMaxReleaseAgeDays into detection.New and notifier.Select"
  - ".env.example: documents NOTIFY_MAX_RELEASE_AGE_DAYS=7"
  - "internal/events/service.go: repointed a stale seedNotifiedAt doc reference (comment only)"
  - "queries/events.sql + internal/db/sqlc/events.sql.go + internal/db/sqlc/querier.go: repointed the same stale doc reference, hand-edited identically so sqlc generate is a no-op (comment only)"
  - "internal/detection/notifygate_test.go: NEW regression tests (fixed one assertion bug in the prior session's draft)"
  - "internal/notifier/suppress_test.go: NEW -- first-ever coverage of suppresses()"
  - "internal/notifier/notifier_test.go: NEW TestNotifyPending_StaleRowsAckedWithoutSending; pending-row fixtures now dated"
  - "internal/detection/detector_test.go: todayReleaseDate helper; three pre-existing tests re-dated to keep testing seed semantics"
