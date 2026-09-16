---
phase: 21-real-time-digest-mutual-exclusion
verified: 2026-09-16T00:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 21: Real-Time / Digest Mutual Exclusion Verification Report

**Phase Goal:** "Turning digest mode on makes the real-time notifier stand down cleanly — events queue instead of firing — and turning it back off delivers everything that queued, nothing lost, nothing duplicated."
**Verified:** 2026-09-16
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | Digest on: a pass over pending rows of all three event types sends zero Discord messages and leaves every one `notified_at IS NULL` | ✓ VERIFIED | `internal/notifier/notifier.go:246-255` reads settings and returns before `listUnnotified` when `DigestEnabled`. `TestNotifyPending_DigestModeOn_SendsNothingAndLeavesRowsPending` inserts one row of each of `new_release`/`guest_feature`/`deluxe_change`, asserts `sender.calls == 0` and `isNotified == false` for all three. Independently re-run against real Postgres: PASS. |
| 2 | Toggling digest off delivers everything accumulated through the ordinary real-time path on the next pass — one message per event, existing order/spacing, none dropped, none duplicated | ✓ VERIFIED | `TestNotifyPending_DigestToggleRoundTrip_QueuedEventsFlushExactlyOnce` (same `*Notifier` instance, on-pass then off-pass: 0 then exactly 3 sends/acks), `TestNotifyPending_DigestOffFlush_OrderNoDuplication` (embed titles equal `ListUnnotified` order element-by-element), `TestNotifyPending_DigestOffFlush_IdempotentSecondPass` (third pass sends 0 more), `TestNotifyPending_DigestRealSettingsStore_TogglesWithoutRestart` (real `settings.Service`, `Update` flips the singleton row, same instance). All independently re-run against real Postgres: PASS. |
| 3 | Mode is re-read on every notify pass — flipping the toggle changes behavior on the next poll cycle of a running process, no restart, no cached boolean | ✓ VERIFIED | `readSettings` is called fresh at the top of every pass (`notifier.go:246`) and again before every individual send (`notifier.go:284`) — no field caches the gate decision (`lastDigestMode`/`lastDigestModeSet` are logging-only, never read in the send/ack decision — confirmed by reading `observeDigestMode` and its three call sites, none of which precede a gate check). `TestNotifyPending_DigestToggleRoundTrip_QueuedEventsFlushExactlyOnce` and `TestNotifyPending_DigestRealSettingsStore_TogglesWithoutRestart` both drive two passes on one instance with no reconstruction. PASS. |
| 4 | Digest off: notify path indistinguishable from v1.4 — same message per event, same 400ms spacing, same idempotent MarkNotified ack, same error handling | ✓ VERIFIED | The entire pre-existing v1.4 notifier suite (spacing, retry/429, mark-notified-fails Warn, cross-cycle recovery, CAS re-entrancy, stale-row suppression) passes unmodified apart from the new constructor argument. Independently re-run: all 33 tests in `internal/notifier` PASS, package coverage 95.2%. `defaultSpacing = 400 * time.Millisecond` unchanged (`notifier.go:23`). |
| 5 | Mode check is the pass's first decision after the sender lock, repeated before every send — never a post-hoc filter; a failed settings read skips the pass, leaves events pending, never delivers in real time | ✓ VERIFIED | `readSettings` sits between `defer n.notifying.Store(false)` and `listUnnotified` (`notifier.go:240-257`), with no row fetch between them. Per-send re-read sits between the suppression-ack branch and `n.sender.Send` (`notifier.go:271-296`). A settings-read error returns `nil` before any send/list (`logSettingsReadFailure`, `notifier.go:200-207`, called at both gate points) — never a non-nil error, never fail-open. `TestNotifyPending_SettingsReadFails_FailsClosedWithOneWarnAndNoSends`, `TestNotifyPending_SettingsReadCtxCancelled_NoWarnLogged`, `TestNotifyPending_MidPass_ReadErrorStopsPassIdenticallyToTopOfPass`, `TestNotifyPending_SettingsReadUnresponsive_ReturnsInsteadOfWedgingLock` all independently re-run: PASS. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/notifier/notifier.go` | `SettingsReader` seam, gate wiring, per-send re-read, transition logging, `created_at` anchor | ✓ VERIFIED | All present, wired, exercised by tests (read in full) |
| `internal/notifier/notifier_test.go` | Doubles + tracer/off-flush/fail-closed/mid-pass/transition-log/stale-anchor test suites | ✓ VERIFIED | 1640 lines read in full; every test named in both SUMMARYs exists and passes |
| `internal/notifier/suppress_test.go` | `created_at`-anchored `TestNotifierSuppresses_WiresMaxAgeDays`, unchanged `gateCases` | ✓ VERIFIED | Anchor-based table with 9 cases including 60-day and slack-boundary cases; `gateCases`/`TestStaleReleaseDate` untouched |
| `internal/notifier/timeout_test.go` | `wedgingSettingsReader`, bounded-read test | ✓ VERIFIED | Present; test passes. gofmt misalignment on `wedgingSettingsReader` struct confirmed present (WR-02, non-blocking per review) |
| `cmd/server/main.go` | `notifier.Select` fed `settingsStore`, same instance as `httpserver.WithSettings` | ✓ VERIFIED | Line 302: `notifier.Select(cfg.DiscordWebhookURL, sqlc.New(pool), settingsStore, nil, logger, ...)`; one `settings.NewService(` call in file (line 255), same identifier passed to `httpserver.WithSettings` (line 286) |
| `web/app/components/system/DigestSettings.tsx` | Always-visible helper text under Digest mode row | ✓ VERIFIED | Exact sentence present as a JSX expression container inside the `<dd>`, outside the `SaveStatus` conditional; switch still reachable by accessible name |
| `internal/webassets/build/client/` | Committed bundle carries new copy | ✓ VERIFIED | `grep -rl` finds the sentence in `assets/system-C6s0fhiM.js`; `go build`/`go vet` clean against the refreshed embed |

### Key Link Verification

| From | To | Via | Status |
|---|---|---|---|
| `cmd/server/main.go` `settingsStore` | `notifier.Select` | positional constructor arg | ✓ WIRED |
| `Notifier.notifying.CompareAndSwap` | `readSettings` (top-of-pass) | strictly before `listUnnotified` | ✓ WIRED |
| `readSettings` (per-send) | `n.sender.Send` | strictly after suppression branch, before Send | ✓ WIRED |
| `readSettings` error | `logSettingsReadFailure` → `return nil` | fail-closed, no `listUnnotified`/`Send` reached | ✓ WIRED |
| `listUnnotified` result length | `observeDigestMode`'s `pending_count` | no COUNT query | ✓ WIRED |
| `ev.CreatedAt` | `suppresses` cutoff | anchor + `-maxAgeDays-1` days | ✓ WIRED |

### Behavioral Spot-Checks / Test Re-Execution

Independently re-ran (not trusting SUMMARY claims):

| Check | Command | Result | Status |
|---|---|---|---|
| Build clean | `go build ./... && go vet ./...` | no output, exit 0 | ✓ PASS |
| Notifier + detection suites | `go test ./internal/notifier/ ./internal/detection/ -count=1 -v` | 33 notifier tests PASS, all detection tests PASS/SKIP as expected (SKIP only on non-DB-configured run) | ✓ PASS |
| Full notifier suite w/ real Postgres | `TEST_DATABASE_URL=... go test ./internal/notifier/... -count=1 -v` | all 33 tests PASS including all digest-gate, mid-pass, transition-log, and stale-anchor tests | ✓ PASS |
| Full backend suite | `go test ./... -count=1 -coverprofile=...` | all packages `ok`; `internal/notifier` 95.2% coverage | ✓ PASS |
| gofmt on notifier package | `gofmt -l internal/notifier/` | flags `timeout_test.go` | confirms WR-02 (known, non-blocking) |
| golangci-lint | `golangci-lint run ./internal/notifier/...` | `0 issues` | ✓ PASS |
| Frontend component tests | `pnpm exec vitest run DigestSettings.test.tsx` | 17/17 passed | ✓ PASS |
| Committed bundle carries copy | `grep -rl "switching back off..." internal/webassets/build/client` | found in `assets/system-C6s0fhiM.js` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|---|---|---|---|---|
| DGST-13 | 21-01, 21-02, 21-03 | Digest mode on stops real-time per-event delivery, no duplicate delivery | ✓ SATISFIED | Truths 1, 3, 5 above; REQUIREMENTS.md marks Complete |
| DGST-14 | 21-01, 21-02, 21-03 | Toggling digest off flushes accumulated events through the normal real-time path | ✓ SATISFIED | Truths 2, 3 above; REQUIREMENTS.md marks Complete |

No orphaned requirements: REQUIREMENTS.md maps only DGST-13/DGST-14 to Phase 21, and both are declared in all three plans' frontmatter.

### Anti-Patterns Found

None blocking. Two pre-documented WARNING-level findings from `21-REVIEW.md`, independently confirmed still present and non-blocking:

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `internal/notifier/notifier.go` | 267-338 | Mid-pass digest-mode early return can skip the "notify pass suppressed stale events" summary log if suppression occurred earlier in the same pass | WARNING | Observability gap only — confirmed no data loss (suppressed rows are acked before the early return; the gate/fail-closed correctness truths above are unaffected). Does not violate any of the 5 ROADMAP success criteria. |
| `internal/notifier/timeout_test.go` | 83-86 | `wedgingSettingsReader` struct field alignment is not gofmt-canonical | WARNING | Confirmed via `gofmt -l`; not caught by `golangci-lint run` (clean) or any current CI gate. Cosmetic only. |

No debt markers (`TBD`/`FIXME`/`XXX`) found in files modified by this phase. No stub patterns (empty returns, hardcoded arrays, console-log-only handlers) found in the reviewed Go or TSX source.

### Human Verification Required

None. All must-haves are verifiable programmatically and were independently re-executed against real Postgres and the frontend test runner, not inferred from SUMMARY.md claims.

### Gaps Summary

No gaps. All 5 ROADMAP success criteria are verified against the actual codebase:

1. Digest-on pass sends zero messages across all three event types and leaves them pending — confirmed by re-running the tracer test against real Postgres.
2. Digest-off flush delivers every queued event exactly once, in order, with no duplication, including via the real `settings.Service` — confirmed by re-running four dedicated tests.
3. The mode is read fresh every pass and every send, with no cached boolean feeding the gate decision — confirmed by code reading and by two-pass-on-one-instance tests.
4. Digest-off behavior is byte-for-byte the pre-existing v1.4 suite, unmodified apart from the threaded constructor argument — confirmed by re-running the full pre-existing test suite (33/33 pass, 95.2% coverage).
5. The gate is the pass's first decision and is repeated before every send, fails closed (nil return, one Warn, zero sends/acks) on a settings-read error, and never fails open — confirmed by code reading of the exact statement ordering and by re-running four fail-closed/mid-pass tests.

The two WARNING findings from the prior code review (observability gap on a rare mid-pass-suppression-then-toggle interaction; a gofmt formatting nit) were independently re-confirmed present but do not affect any of the 5 success criteria and do not constitute data loss, duplication, or a fail-open path. They are appropriately non-blocking.

---

_Verified: 2026-09-16_
_Verifier: Claude (gsd-verifier)_
