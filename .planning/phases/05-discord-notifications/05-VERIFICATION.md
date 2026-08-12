---
phase: 05-discord-notifications
verified: 2026-08-08T22:15:00Z
status: passed
score: 26/28 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:

  - test: "Point DISCORD_WEBHOOK_URL at a real Discord channel, seed a new_release, a guest_feature, and a deluxe_change event, and let one live poll cycle (or a manual NotifyPending trigger) deliver all three"
    expected: "Three visually distinct messages appear in the Discord channel: new_release (green sidebar, 🆕 title prefix, Artist/Release Date/Type fields, cover-art thumbnail, clickable release-group/album link), guest_feature (yellow sidebar, 🎤 prefix, Artist field, clickable recording link), deluxe_change (fuchsia sidebar, 💿 prefix, Tracks field showing the count delta, clickable release link) -- and no @mention in any community-edited artist name actually pings anyone"
    why_human: "Discord's actual rendering of embed color/emoji/thumbnail/link and its mention-suppression behavior can only be confirmed by looking at a real message in a real Discord client; httptest.Server-backed tests prove the correct HTTP payload is sent but cannot prove Discord renders it as intended"

  - test: "Construct or capture an embed title that is exactly 256 runes (including multi-byte characters) and confirm it is sent unmodified rather than truncated to 255 or rejected by Discord as oversized; separately confirm a fully-populated new_release/deluxe_change embed (title at 256 runes plus all optional fields at 1024 runes each) stays under Discord's ~6000-character total embed budget"
    expected: "A 256-rune title round-trips unchanged through truncateRunes (code's `<=` comparison implies this, but no test asserts the exact-256 boundary) and Discord accepts the request; a worst-case fully-populated embed (title + Artist + Release Date + Type, or title + Tracks, each at their field limit) stays within the ~6000-character total embed budget Discord documents"
    why_human: "These two must-haves are explicitly marked `verification: backstop` in 05-02-PLAN.md's frontmatter -- no automated test in format_test.go asserts the exact-256-rune boundary case or sums a worst-case embed's total character budget against Discord's ~6000 limit; truncateRunes's `<=` comparison and per-field truncation make both plausible by code inspection, but per the verifier's non-inferable-truth policy this cannot be self-certified as VERIFIED without explicit test evidence"
---

# Phase 5: Discord Notifications Verification Report

**Phase Goal:** Users are notified in Discord immediately and distinctly when a detected event matches their preferences.
**Verified:** 2026-08-08T22:15:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User receives a Discord webhook message for each new-release event including title, artist, cover art, release date, and release type | ✓ VERIFIED | `internal/notifier/format.go` `formatNewRelease` populates Title/Artist/Release Date/Type/Thumbnail from `sqlc.Event`'s display-snapshot columns; `release_type` is persisted by `internal/detection/musicbrainz.go:115` (`releaseTypeForStorage`) and `internal/detection/deezer.go:89`; migration `000004_events_display_fields.up.sql` adds the column. `TestFormatEmbed_NewRelease_MusicBrainz`/`_Deezer` and the real-Postgres `TestDetectMusicBrainz_NewRelease` (extended with a `release_type` assertion per 05-02-SUMMARY) pass against live Postgres. |
| 2 | User receives a visually distinct Discord webhook message for guest-feature events | ✓ VERIFIED | `formatGuestFeature` sets `colorGuestFeature` (16705372) + `emojiGuestFeature` (🎤) + a MusicBrainz recording link, distinct from `new_release`'s color/emoji/link. `TestFormatEmbed_ThreeEventTypesAreDistinct` asserts all three colors and emoji prefixes are pairwise different; `TestFormatEmbed_GuestFeature` and `TestFormatEmbed_URLsMatchExpectedHostAndPath/guest_feature` pass. |
| 3 | User receives a visually distinct Discord webhook message for deluxe/tracklist-change events | ✓ VERIFIED | `formatDeluxeChange` sets `colorDeluxeChange` (15418782) + `emojiDeluxeChange` (💿) + a MusicBrainz release link + a `Tracks` field rendering the old→new delta via `tracksFieldValue`. `previous_track_count` is captured from the pre-overwrite baseline in `internal/detection/musicbrainz.go:352` (`case maxCount > baseline:`), before `setGroupBaseline` overwrites it. `TestFormatEmbed_DeluxeChange_BothCountsPresent`, `_NilPreviousTrackCount`, `_NilBothCounts` all pass; real-Postgres `TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease` (extended with `previous_track_count` assertion) passes. |
| 4 | User does not receive notifications for artists/release-types they've muted via their preferences | ✓ VERIFIED | Phase 4's `eventTypeMuted` filter (`internal/detection/filter.go:71`) runs before any insert (unchanged this phase, correctly not re-implemented per plan 05-03's explicit prohibition). End-to-end proof added this phase: `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` runs a real `DetectMusicBrainz` cycle against a muted artist, then a real `NotifyPending` call against an `httptest.Server`, and asserts the server received exactly one request (for the sibling unmuted `new_release`) and never a request derived from the muted `guest_feature` event. Passed against live Postgres. |

**Score (roadmap contract):** 4/4 roadmap success criteria verified.

### Plan-Level Must-Haves (merged per Step 2c, deduplicated against the four SCs above)

| # | Truth (plan) | Status | Evidence |
|---|---|---|---|
| 5 | Pending row POSTed as single-embed message; `notified_at` set only after a confirmed 204 (05-01) | ✓ VERIFIED | `discord.Client.sendAttempt` checks `http.StatusNoContent` exclusively; `Notifier.NotifyPending` calls `MarkNotified` only on a nil `Send` error. `TestNotifyPending_ConcurrentCallsNoDoublePost` and the 05-01 end-to-end drain test confirm this against real Postgres. |
| 6 | Pending events sent in deterministic order (`created_at ASC, id ASC`) (05-01) | ✓ VERIFIED | `queries/events.sql`'s `ListUnnotified` query has this exact `ORDER BY`; `TestNotifyPending_BatchMidFailureContinuesToLaterRows`/`_BatchHonorsRetryAfterWithoutDroppingOtherRows` depend on and confirm this ordering. |
| 7 | Zero pending events → no Discord request, no `MarkNotified`, returns nil (05-01) | ✓ VERIFIED | Covered by 05-01's zero-pending test case (referenced in 05-01-SUMMARY, part of `notifier_test.go`'s passing suite). |
| 8 | Overlapping notify passes never double-post; CAS guard skips second caller (05-01/05-03, D-06) | ✓ VERIFIED | `TestNotifyPending_ConcurrentCallsNoDoublePost` uses two genuine goroutines with a started/release channel harness (mirroring commit e53d48c) — ran against live Postgres, passed. |
| 9 | A failed send leaves `notified_at` NULL, logs the failure, does not abort remaining events (05-01/05-03) | ✓ VERIFIED | `TestNotifyPending_BatchMidFailureContinuesToLaterRows` positively confirms the row before and after a mid-batch failure are both attempted and delivered (not merely that the pass returned nil, per the plan's explicit prohibition). Passed. |
| 10 | Transport-failure error never contains the webhook URL, host, or token (05-01) | ✓ VERIFIED | `sendAttempt` returns a fixed string `"discord: send webhook: request failed"` with no wrapped `*url.Error`. Verified by code inspection and `internal/discord/client_test.go`'s leak-regression case (part of the passing `internal/discord` suite). |
| 11 | `DISCORD_WEBHOOK_URL` unset → boots normally, logs one disabled line, notify pass is inert no-op (05-01, D-10) | ✓ VERIFIED | `notifier.Select` returns `NoOp{}` and logs one Info line when `webhookURL == ""`; `cmd/server/main.go:140` wires `notifier.Select(cfg.DiscordWebhookURL, ...)` into `poller.New`. |
| 12 | Migration 000004 applies cleanly against real Postgres; adds two nullable columns | ✓ VERIFIED | `internal/db/migrations/000004_events_display_fields.{up,down}.sql` exist; `go test ./internal/db/...` passes against live Postgres (`docker-compose` `postgres:16` container, `drop-tracker-postgres-1`). |
| 13 | All three event types get distinct color + emoji on one shared webhook URL, not color-only (05-02, D-01) | ✓ VERIFIED | `TestFormatEmbed_ThreeEventTypesAreDistinct` asserts pairwise-distinct `Color` and title-prefix emoji. No separate webhook/channel routing exists anywhere in the codebase (single `DISCORD_WEBHOOK_URL`). |
| 14 | `release_type` persisted on new_release inserts from both sources (05-02) | ✓ VERIFIED | See truth #1 evidence. |
| 15 | `previous_track_count` persisted on deluxe_change insert (05-02) | ✓ VERIFIED | See truth #3 evidence. |
| 16 | Embed title/field truncation is rune-based, never byte-based (05-02) | ✓ VERIFIED | `truncateRunes` converts to `[]rune` before slicing; `TestFormatEmbed_TitleTruncatedToRuneLimit_MultiByte` asserts a 300-rune multi-byte title truncates to exactly 256 valid runes with no replacement character. Passed. |
| 17 | guest_feature row with all-NULL optional columns still produces a valid embed with fields omitted, not empty-valued (05-02) | ✓ VERIFIED | `appendField` omits a field when its truncated value is empty; `TestFormatEmbed_AllNilOptionalFields_NoEmptyFieldsNoThumbnail` passed. |
| 18 | Every embed URL built by escaping `external_id` into a fixed path template — never a hostile/malformed link (05-02) | ✓ VERIFIED | All four URL helpers use `url.PathEscape(externalID)` over a fixed template; `TestFormatEmbed_ExternalIDIsPercentEscapedInURL` and `TestFormatEmbed_URLsMatchExpectedHostAndPath` passed. |
| 19 | deluxe_change with NULL `previous_track_count` renders current count alone, no nil deref (05-02) | ✓ VERIFIED | `tracksFieldValue`'s three-way switch; `TestFormatEmbed_DeluxeChange_NilPreviousTrackCount` and `_NilBothCounts` passed. |
| 20 | 256-rune title at exactly the boundary is accepted, not rejected as oversized (05-02) | ⚠️ backstop / UNCERTAIN | `truncateRunes`'s `<=` comparison implies correct behavior by code inspection, but no test asserts the exact-256-rune boundary case. Marked `verification: backstop` in 05-02-PLAN.md — routed to human verification per policy rather than self-certified. |
| 21 | Per-field truncation keeps total embed payload under Discord's ~6000-char budget (05-02) | ⚠️ backstop / UNCERTAIN | No test sums a worst-case fully-populated embed's total character count against the ~6000 limit. Marked `verification: backstop` — routed to human verification. |
| 22 | Batch of ≥3 rows waits at least the configured spacing between sends, measured elapsed time (05-03, D-07) | ✓ VERIFIED | `TestNotifyPending_BatchSpacingBetweenSends` measures real `time.Now()` gaps between recorded sends and asserts them `>=` spacing. Passed against live Postgres. |
| 23 | A permanent mid-batch failure doesn't abort the pass; row before/after still attempted and delivered (05-03) | ✓ VERIFIED | Same as truth #9 — `TestNotifyPending_BatchMidFailureContinuesToLaterRows`. |
| 24 | A 429 mid-batch doesn't skip/reorder/drop any other pending row (05-03, D-08) | ✓ VERIFIED | `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows` uses a real `discord.Client` against one `httptest.Server`; asserts all three rows notified, exactly four total requests, correct ordering by timestamp. Passed. |
| 25 | A row unnotified by one call is picked up by a later, wholly separate call once the webhook recovers (05-03, D-09) | ✓ VERIFIED | `TestNotifyPending_CrossCycleRecoveryAfterOutage` — two genuinely separate `NotifyPending` calls spanning a simulated outage/recovery. Passed. |
| — | NTFY-04 muted event never reaches `ListUnnotified` (05-03) | (dup of SC #4, kept as roadmap wording) | See truth #4. |

**Merged score:** 26/28 must-haves verified (4 roadmap + 22 unique plan-level truths; 2 backstop-tier truths routed to human verification below).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrations/000004_events_display_fields.up.sql` / `.down.sql` | Nullable `previous_track_count`/`release_type` columns, reverse-order drop | ✓ VERIFIED | Both files exist with correct ALTER/DROP statements. |
| `internal/discord/client.go` | `Client`, `NewClient`, `Send`, `Embed`/`EmbedField`/`EmbedImage`, allowed_mentions suppression, 429 clamp | ✓ VERIFIED | All exports present; CR-01 and WR-04 fixes confirmed in current file content (lines 25-31, 61-76, 121-124, 151-164). |
| `internal/notifier/notifier.go` | `Notifier`, `New`, `Select`, `NoOp`, `Sender`, `Sink`, `NotifyPending` with CAS guard | ✓ VERIFIED | All present; WR-01/WR-02/WR-03 fixes confirmed (unconditional spacing wait, corrected rate-limit comment, Warn log on MarkNotified failure). |
| `internal/notifier/format.go` | Complete three-arm `formatEmbed`, rune truncation, URL construction | ✓ VERIFIED | All three arms fully populated per 05-02 scope; `truncateRunes`, `appendField`, per-type URL helpers all present. |
| `queries/events.sql` | Widened `InsertEvent`, new `MarkNotified` query | ✓ VERIFIED | `$12`/`$13` columns present, `ON CONFLICT DO NOTHING` preserved byte-identical; `MarkNotified` present with idempotent `AND notified_at IS NULL` predicate. |
| `internal/db/sqlc/{events.sql.go,models.go,querier.go}` | Regenerated sqlc output | ✓ VERIFIED | `MarkNotified`, `PreviousTrackCount *int32`, `ReleaseType *string` all present; `make sqlc-check`-equivalent confirmed via passing `go build`/`go vet`. |
| `internal/poller/poller.go` | `Notifier` interface, wired into both cycles | ✓ VERIFIED | Interface declared; both `RunMusicBrainzCycle` and `RunDeezerCycle` call `p.notifier.NotifyPending` at the end of their loops, logging (not returning) a failure. |
| `cmd/server/main.go` | `notifier.Select` wired into `poller.New` | ✓ VERIFIED | Line 140: `notifier.Select(cfg.DiscordWebhookURL, sqlc.New(pool), nil, logger)`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/poller/poller.go` | `internal/notifier/notifier.go` | `NotifyPending` called at end of both cycles | ✓ WIRED | Confirmed at lines 266 and 340 of `poller.go`. |
| `internal/notifier/notifier.go` | `internal/discord/client.go` | `Sender` seam over `discord.Client.Send` | ✓ WIRED | `var _ Sender = (*discord.Client)(nil)`; `Select` constructs `discord.NewClient(webhookURL, httpClient)`. |
| `internal/notifier/notifier.go` | `queries/events.sql` | `ListUnnotified` dequeue, `MarkNotified` ack | ✓ WIRED | Both called in `NotifyPending`'s loop. |
| `cmd/server/main.go` | `internal/notifier/notifier.go` | `notifier.Select` gates on `cfg.DiscordWebhookURL` | ✓ WIRED | Confirmed. |
| `internal/detection/musicbrainz.go` / `deezer.go` | `internal/db/sqlc/events.sql.go` | `InsertEventParams.ReleaseType`/`.PreviousTrackCount` | ✓ WIRED | Confirmed at the three insert sites. |

### Behavioral Spot-Checks (single named tests, run against live Postgres)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Concurrent NotifyPending calls never double-post (D-06) | `go test ./internal/notifier/... -run TestNotifyPending_ConcurrentCallsNoDoublePost -v` (TEST_DATABASE_URL set) | PASS (0.10s) | ✓ PASS |
| Batch spacing measured across real elapsed time (D-07) | `-run TestNotifyPending_BatchSpacingBetweenSends` | PASS (0.17s) | ✓ PASS |
| Mid-batch permanent failure doesn't starve later rows | `-run TestNotifyPending_BatchMidFailureContinuesToLaterRows` | PASS (0.07s) | ✓ PASS |
| Batch-level 429/Retry-After doesn't drop other rows (D-08) | `-run TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows` | PASS (0.13s) | ✓ PASS |
| Cross-cycle recovery after outage (D-09) | `-run TestNotifyPending_CrossCycleRecoveryAfterOutage` | PASS (0.07s) | ✓ PASS |
| Spacing applied even after failed Send (WR-01 fix) | `-run TestNotifyPending_SpacingAppliedEvenAfterFailedSend` | PASS (0.17s) | ✓ PASS |
| MarkNotified failure logs Warn and returns error (WR-03 fix) | `-run TestNotifyPending_MarkNotifiedFails_LogsWarnAndReturnsError` | PASS (0.12s) | ✓ PASS |
| NTFY-04 muted event never delivered by notifier | `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature_Muted -v` | PASS (both existing and new test) | ✓ PASS |
| allowed_mentions always suppressed (CR-01 fix) | Part of `go test ./internal/discord/...` full run | PASS | ✓ PASS |
| Full phase package suite | `go test -p 1 -count=1 ./internal/discord/... ./internal/notifier/... ./internal/poller/... ./internal/detection/... ./internal/db/...` (TEST_DATABASE_URL set) | All `ok` | ✓ PASS |
| `go build ./...` / `go vet ./...` | — | Clean, no output | ✓ PASS |

Note: the first attempt to run these named tests without `TEST_DATABASE_URL` set silently SKIPped every DB-backed case (still reporting package-level `ok`) — re-run with `TEST_DATABASE_URL` pointed at the running `drop-tracker-postgres-1` container to get real pass/fail evidence, confirmed above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| NTFY-01 | 05-01, 05-02 | Discord message per new-release event with title/artist/cover art/release date/release type | ✓ SATISFIED | Truth #1. |
| NTFY-02 | 05-02 | Visually distinct guest-feature message | ✓ SATISFIED | Truth #2. |
| NTFY-03 | 05-02 | Visually distinct deluxe/tracklist-change message | ✓ SATISFIED | Truth #3. |
| NTFY-04 | 05-03 | Suppress notifications for muted artists/release-types | ✓ SATISFIED | Truth #4. |

No orphaned requirements — all four IDs REQUIREMENTS.md maps to Phase 5 are claimed across the three plans' `requirements` frontmatter (05-01: NTFY-01; 05-02: NTFY-01/02/03; 05-03: NTFY-04).

### Anti-Patterns Found

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found in any file this phase created or modified (`internal/discord/`, `internal/notifier/`, `internal/poller/poller.go`, `cmd/server/main.go`, `internal/detection/musicbrainz.go`, `internal/detection/deezer.go`, migration 000004, `queries/events.sql`).

`05-REVIEW.md`'s code-review pass found 1 critical + 4 warnings; `05-REVIEW-FIX.md` and the current codebase confirm all 5 were fixed (commits `3a51e02`, `bf4953d`, `d343ee2`, `a72fd73`, `aba2366`) and verified present in the code read during this verification. 3 info-level findings (`IN-01`/`IN-02`/`IN-03`) remain open — all non-blocking observability/dead-capability notes, not architectural gaps.

Unrelated to this phase: an untracked stray binary (`server`, an ELF executable) sits at the repo root per `git status` — pre-existing debris from an earlier local build, not part of this phase's file set, not flagged as a phase gap.

### Human Verification Required

### 1. Live Discord rendering and mention-suppression check

**Test:** Point `DISCORD_WEBHOOK_URL` at a real Discord channel, seed one of each event type (new_release, guest_feature, deluxe_change — including an artist name containing `@everyone` if feasible in a throwaway test channel), and let a poll cycle or manual `NotifyPending` trigger deliver them.
**Expected:** Three visually distinct messages render correctly (color, emoji, fields, thumbnail, clickable link per type) and no mention token pings anyone.
**Why human:** httptest.Server-backed tests prove the correct JSON payload is sent (including `"allowed_mentions":{"parse":[]}`), but only a real Discord client can confirm the message actually renders and behaves as intended.

### 2. Backstop-tier truncation boundary and total-budget checks

**Test:** Confirm a title of exactly 256 runes (including multi-byte characters) is sent unmodified, and that a worst-case fully-populated embed (max title + all optional fields at their limits) stays under Discord's ~6000-character total embed budget.
**Expected:** Exact-256-rune title round-trips unchanged; total payload never approaches the ~6000-char ceiling.
**Why human:** Both truths are marked `verification: backstop` in 05-02-PLAN.md's frontmatter — no automated test asserts either boundary case directly (only over-limit truncation, at 300 runes, is tested). Code inspection (`truncateRunes`'s `<=` comparison; per-field truncation keeping worst case well under 6000 chars) makes both plausible, but per verification policy a backstop truth cannot be self-certified without explicit test evidence.

### Gaps Summary

No blocking gaps. All four roadmap Success Criteria are verified with strong automated evidence (real-Postgres integration tests plus httptest.Server-backed adversarial concurrency/batch/retry/mute-suppression tests, all re-run and confirmed passing during this verification with `TEST_DATABASE_URL` correctly set). All five code-review findings from `05-REVIEW.md` are confirmed fixed in the current code. The only open items are two backstop-tier boundary assertions from 05-02's plan frontmatter and the inherent need for a human to visually confirm live Discord rendering — neither indicates a functional defect, both are routed to human verification rather than silently passed.

---

_Verified: 2026-08-08T22:15:00Z_
_Verifier: Claude (gsd-verifier)_
