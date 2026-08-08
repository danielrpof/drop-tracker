---
phase: 05
slug: discord-notifications
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-08
validated: 2026-08-08
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`), matching every prior phase |
| **Config file** | none — see `Makefile` targets |
| **Quick run command** | `go test ./internal/discord/... ./internal/notifier/... -short -race -count=1` |
| **Full suite command** | `make test` (`test-integration` requires `make db-up` first — real Postgres) |
| **Estimated runtime** | ~30 seconds (quick), ~2-3 minutes (full, incl. Postgres integration) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/discord/... ./internal/notifier/... -short -race -count=1`
- **After every plan wave:** Run `make test` (full integration suite against real Postgres)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-T1 | 05-01 | 1 | NTFY-01 (schema) | — | Migration 000004 adds nullable `previous_track_count`/`release_type`; `MarkNotified` ack is idempotent (`AND notified_at IS NULL`) | integration (real Postgres) | `go test ./internal/db/... -count=1` | ✅ `internal/db/migrations/000004_*.up.sql`/`.down.sql` | ✅ green |
| 05-01-T2 | 05-01 | 1 | NTFY-01 (tracer, happy+failure path) | V5/V7 | `discord.Client.Send` POSTs a single embed, succeeds only on 204 | unit (httptest.Server) | `go test ./internal/discord/... -run TestSend_Success204 -short` | ✅ `internal/discord/client_test.go` | ✅ green |
| 05-01-T2 | 05-01 | 1 | NTFY-01 (tracer) | — | `NotifyPending` drains one pending row end-to-end and marks it notified after 204 | integration (real Postgres) | `go test ./internal/notifier/... -run TestNotifyPending_OnePendingRow_204MarksNotified -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-01-T2 | 05-01 | 1 | D-09 | — | A failed send leaves `notified_at` NULL; the next pass re-picks up the row | integration (real Postgres) | `go test ./internal/notifier/... -run TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-01-T2 | 05-01 | 1 | D-06 (single-goroutine reentry) | DoS (accepted) | A re-entrant `NotifyPending` call is skipped while a pass is in flight | integration (real Postgres) | `go test ./internal/notifier/... -run TestNotifyPending_ReentrantCallSkippedWhileInFlight -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-01-T2 | 05-01 | 1 | D-08 | — | A 429 with `Retry-After` is honored once, then succeeds; a second consecutive 429 surfaces as an error with no third request | unit (httptest.Server) | `go test ./internal/discord/... -run 'TestSend_429' -short` | ✅ `internal/discord/client_test.go` | ✅ green |
| 05-01-T2 | 05-01 | 1 | T-05-01 | Information Disclosure (mitigate) | A transport-failure error string never contains the webhook host or token path segment | unit (httptest.Server) | `go test ./internal/discord/... -run TestSend_TransportFailure_ErrorNeverLeaksHostOrToken -short` | ✅ `internal/discord/client_test.go` | ✅ green |
| 05-01-T2 | 05-01 | 1 | D-10 | — | Empty `DISCORD_WEBHOOK_URL` returns `NoOp`, logs one disabled line, and `NotifyPending` is an inert no-op | unit | `go test ./internal/notifier/... -run TestSelect_EmptyWebhookURL_ReturnsNoOpAndLogsDisabledLine -short` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-02-T1 | 05-02 | 2 | NTFY-01 (persistence) | — | MusicBrainz new_release insert stores normalized `release_type` (lowercase/trim), NULL when `PrimaryType` is absent | unit + integration | `go test ./internal/detection/... -run 'TestReleaseTypeForStorage|TestDetectMusicBrainz_NewRelease' -count=1` | ✅ `internal/detection/musicbrainz_test.go`, `detector_test.go` | ✅ green |
| 05-02-T1 | 05-02 | 2 | NTFY-01 (persistence) | — | Deezer new_release insert stores `release_type` "album" from `RecordType` | integration (real Postgres) | `go test ./internal/detection/... -run TestDetectDeezer_NewRelease -count=1` | ✅ `internal/detection/deezer_test.go` | ✅ green |
| 05-02-T1 | 05-02 | 2 | NTFY-03 (D-04) | — | Deluxe-change insert captures `previous_track_count` from the baseline before it is overwritten | integration (real Postgres) | `go test ./internal/detection/... -run TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease -count=1` | ✅ `internal/detection/detector_test.go` | ✅ green |
| 05-02-T2 | 05-02 | 2 | NTFY-01 | — | new_release embed carries title/artist/cover-thumbnail/release-date/release-type and links to the correct source (MusicBrainz release-group or Deezer album) | unit | `go test ./internal/notifier/... -run 'TestFormatEmbed_NewRelease' -short` | ✅ `internal/notifier/format_test.go` | ✅ green |
| 05-02-T2 | 05-02 | 2 | NTFY-02 | — | guest_feature embed has a distinct color/emoji and links to the MusicBrainz recording | unit | `go test ./internal/notifier/... -run 'TestFormatEmbed_GuestFeature|TestFormatEmbed_ThreeEventTypesAreDistinct' -short` | ✅ `internal/notifier/format_test.go` | ✅ green |
| 05-02-T2 | 05-02 | 2 | NTFY-03 | — | deluxe_change embed shows old→new track delta ("12 → 18 tracks"), degrades to current-count-only when `previous_track_count` is NULL, omits the field when both are NULL | unit | `go test ./internal/notifier/... -run TestFormatEmbed_DeluxeChange -short` | ✅ `internal/notifier/format_test.go` | ✅ green |
| 05-02-T2 | 05-02 | 2 | T-05-02 | Tampering (mitigate) | Title/field truncation is rune-safe (not byte-safe) — no split multi-byte character | unit | `go test ./internal/notifier/... -run TestFormatEmbed_TitleTruncatedToRuneLimit -short` | ✅ `internal/notifier/format_test.go` | ✅ green |
| 05-02-T2 | 05-02 | 2 | T-05-06 | Tampering (mitigate) | Embed URLs are built via `url.PathEscape` over `external_id`, never by interpolating free-text `title`/`artist_name` | unit | `go test ./internal/notifier/... -run 'TestFormatEmbed_URLsMatchExpectedHostAndPath|TestFormatEmbed_ExternalIDIsPercentEscapedInURL' -short` | ✅ `internal/notifier/format_test.go` | ✅ green |
| 05-02-T2 | 05-02 | 2 | — | — | All-NULL optional snapshot columns produce no empty-valued `EmbedField` and a nil `Thumbnail` | unit | `go test ./internal/notifier/... -run TestFormatEmbed_AllNilOptionalFields_NoEmptyFieldsNoThumbnail -short` | ✅ `internal/notifier/format_test.go` | ✅ green |
| 05-03-T1 | 05-03 | 3 | D-06 (genuine concurrency) | DoS (accepted) | Two real goroutines calling `NotifyPending` against the same pending row never both send | integration (real Postgres, genuine concurrency) | `go test ./internal/notifier/... -run TestNotifyPending_ConcurrentCallsNoDoublePost -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-03-T1 | 05-03 | 3 | D-07 | — | Inter-send spacing is real elapsed time across a multi-row batch, including after a failed send | integration (real Postgres) | `go test ./internal/notifier/... -run 'TestNotifyPending_BatchSpacingBetweenSends|TestNotifyPending_SpacingAppliedEvenAfterFailedSend' -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-03-T1 | 05-03 | 3 | D-09 (mid-batch) | — | A permanent failure on one row mid-batch does not abort the pass; later rows are still attempted and delivered | integration (real Postgres) | `go test ./internal/notifier/... -run TestNotifyPending_BatchMidFailureContinuesToLaterRows -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-03-T1 | 05-03 | 3 | D-08 (batch-level) | — | A 429/Retry-After on one row mid-batch does not skip, reorder, or drop any other row | integration (real Postgres + httptest.Server) | `go test ./internal/notifier/... -run TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-03-T1 | 05-03 | 3 | D-09 (cross-cycle) | — | A row unnotified after one `NotifyPending` call (webhook outage) is delivered by a later, separate call once the webhook recovers | integration (real Postgres + httptest.Server) | `go test ./internal/notifier/... -run TestNotifyPending_CrossCycleRecoveryAfterOutage -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-03-T1 | 05-03 | 3 | — | Repudiation | A `MarkNotified` DB failure after a successful send is logged and returned as a hard error | integration (real Postgres) | `go test ./internal/notifier/... -run TestNotifyPending_MarkNotifiedFails_LogsWarnAndReturnsError -count=1` | ✅ `internal/notifier/notifier_test.go` | ✅ green |
| 05-03-T2 | 05-03 | 3 | NTFY-04 | Information Disclosure (mitigate) | A muted artist's guest_feature candidate never reaches `ListUnnotified`/Discord, while a sibling unmuted new_release from the same cycle is delivered normally — proven end-to-end through the real notifier, not only at the detection layer | integration (real Postgres + httptest.Server) | `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier -count=1` | ✅ `internal/detection/detector_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky. Reconstructed 2026-08-08 against the actual executed plans (05-01/05-02/05-03) — the prior table above was the pre-execution draft seeded by plan-phase and was never updated after execution; task IDs, commands, and file paths below reflect what was actually built and tested, not the Wave-0 speculation.*

---

## Wave 0 Requirements (as originally planned — superseded)

All four items below were satisfied by the actual plans, under the file names the planner/executor chose (which differ slightly from this speculative list — see the Per-Task Verification Map above for the real mapping):

- [x] `internal/discord/client_test.go` — covers `Client.Send`'s 204/429/other-status paths (7 test functions, including a token-leak regression), matching `internal/musicbrainz/search_test.go`'s `httptest.Server` pattern
- [x] `internal/notifier/notifier_test.go` — covers the fetch/format/send/mark loop, the D-06 concurrency guard (both single-goroutine reentry and genuine two-goroutine proof), D-07 spacing, D-08 batch-level retry, and D-09 cross-cycle recovery (12 test functions)
- [x] `internal/notifier/format_test.go` — covers per-event-type embed formatting: color/emoji distinctness, link construction, rune-safe truncation, nil-field handling (13 test functions)
- [x] `internal/detection/musicbrainz_test.go`/`deezer_test.go`/`detector_test.go` — extended with assertions that `PreviousTrackCount`/`ReleaseType` populate correctly on insert, plus the NTFY-04 through-notifier regression test

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification. (A live Discord webhook end-to-end smoke test is optional/manual UAT, not a blocking verification — CI never hits a real webhook per CLAUDE.md's no-live-external-calls constraint.)*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-08 — 0 gaps found during audit; all 4 requirements (NTFY-01–04) and all named threat mitigations (T-05-01, T-05-02, T-05-06, T-05-09–12) had real, passing automated coverage already in place from execution. No new tests were generated by this audit.

## Validation Audit 2026-08-08

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — full coverage already existed) |
| Escalated | 0 |

Verification run for this audit: `go build ./...` clean, `go vet ./...` clean, `go test ./... -count=1` green (all 13 packages, real Postgres via `TEST_DATABASE_URL`, including `internal/discord`, `internal/notifier`, `internal/detection`, `internal/poller`, `internal/db`). The pre-execution VALIDATION.md draft (frontmatter `status: draft`, `05-TBD` placeholder rows) was never updated after 05-01/05-02/05-03 executed; this audit reconstructed the Per-Task Verification Map against the actual PLAN/SUMMARY artifacts and confirmed every planned behavior, plus several tests added beyond the original Wave 0 scope (D-07 spacing-after-failure, MarkNotified-failure handling, T-05-02/T-05-06 truncation and URL-safety regressions).
