---
phase: 22-scheduled-digest-send
verified: 2026-09-17T03:12:51Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
deferred:

  - truth: "An oversized digest (Description over Discord's 4096-character limit) degrades safely instead of looping forever on every due-check."
    addressed_in: "Phase 23"
    evidence: "ROADMAP.md Phase 23 goal: 'A digest states the window it covers and stays complete — with Phase 22's grouping intact — even when it is large enough to exceed what one Discord message can hold.' ROADMAP.md 'Deploy sequencing' and 22-CONTEXT.md D-19 explicitly record that Phases 21/22/23 ship in one release precisely because Phase 22 ships without chunking; this is a release-sequencing decision, not a Phase 22 implementation gap, and none of Phase 22's own ROADMAP success criteria require chunking."
human_verification:

  - test: "Push this branch (or merge per the documented 21+22+23 release-sequencing rule) and observe the `build-scan` job's new 'Boot the built image and assert it resolves the digest zone' step in a real GitHub Actions run."
  - expected: "The step's `docker logs` poll finds both `\"msg\":\"digest zone resolved\"` and `\"zone\":\"America/New_York\"` in the booted `drop-tracker:scan` container's output within 60s, and the step exits 0. If the embedded `time/tzdata` were missing or broken, the step would exit 1 with the `::error::` annotation and dump the container logs/exit code."
  - why_human: "This is the one truth in success criterion 4 that only a live CI boot can prove — Go consults the host's own zoneinfo before the binary's embedded `time/tzdata` package, so a local `go test` or `go build` run (including everything this verification exercised) passes identically whether or not tzdata is actually embedded correctly, on any machine that already has system zoneinfo (this Windows dev box does). `git status`/`git ls-remote` at verification time show this branch has never been pushed, so `full-pipeline.yml`'s `build-scan` job has not yet run against this code — the step exists, is correctly placed and wired (verified below), but its actual green/red outcome is unobserved."

---

# Phase 22: Scheduled Digest Send Verification Report

**Phase Goal:** With digest mode on, everything accumulated since the last digest arrives as one Discord message at a predictable time — and the schedule survives restarts, DST transitions, and the container's minimal timezone database.
**Verified:** 2026-09-17T03:12:51Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | One Discord message per due slot carries all three event types grouped under headings; every covered event is marked delivered | ✓ VERIFIED | `internal/notifier/digest.go` `SendDigestIfDue` + `digest_format.go` `buildDigestEmbed`/`digestHeadings`; `TestSendDigestIfDue_DueSlotAllThreeTypes_OneSendThreeAcksBothColumns` and `TestSendDigestIfDue_DueSlotOnlyNewRelease_OtherHeadingsOmitted` re-run live against Postgres this session — both PASS |
| 2 | A zero-event window sends nothing, no empty message, and the skip is visible in structured logs | ✓ VERIFIED | `digest.go` lines 86-98: `len(sendable)==0` short-circuits before `buildDigestEmbed`, no `Sender.Send` call, logs `"digest slot had nothing to send"` at Info; `TestSendDigestIfDue_EmptyOutbox_ZeroSendSlotAdvancesSentAtStaysNull` and `TestSendDigestIfDue_AllSuppressed_...` re-run live — both PASS |
| 3 | A missed fire is caught within its grace window (12h daily / 48h weekly); past the window it is skipped with a Warn and pending events go out at the next fire | ✓ VERIFIED | `digest.go` lines 54-65 (grace comparison) + `scheduler.go` `Start`'s immediate first check (restart catch-up); `TestDigestScheduler_Catchup_RestartInsideGrace`, `TestDigestScheduler_Grace_Boundary_SlotPlus12h00m_StillSends`, `TestDigestScheduler_Grace_Boundary_SlotPlus12h01m_DoesNotSend`, `TestDigestScheduler_Grace_WeeklyBoundary_*`, `TestDigestScheduler_Grace_NothingLost_CarriesForwardToNextSlot` all re-run live — all PASS, including the exact-boundary-inclusive assertion |
| 4a | Exactly one digest per calendar day/week across both spring-forward and fall-back, both 2026 and 2027, both cadences | ✓ VERIFIED | `internal/settings/slot.go` `MostRecentSlot` (calendar-based, never duration-based) + `TestDigestScheduler_DST_Daily`, `TestDigestScheduler_DST_Daily_FallBack_SlotRecordNotEmptyOutboxPreventsSecondSend`, `TestDigestScheduler_DST_Weekly` re-run live this session, sweeping 2026-03-08/2026-11-01/2027-03-14/2027-11-07 in 5-minute steps through a real `*DigestScheduler` — all PASS; the fall-back subtest specifically proves the slot record, not an empty outbox, prevents the second fire |
| 4b | The shipped Alpine image resolves the zone it schedules against rather than falling back to UTC | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `cmd/server/main.go` fail-fast `time.LoadLocation(settings.ZoneName)` + blank `_ "time/tzdata"` import (code proven, `grep -n 'digest zone resolved'` = 1 match) and `.github/workflows/full-pipeline.yml`'s new `build-scan` boot step (structurally verified: correct job placement, `needs:` unchanged, no new `uses:`, correct literals) are both present and correctly wired — but no live CI run of this workflow against this branch has ever executed (branch unpushed at verification time), so the actual Alpine-tzdata-resolves-correctly outcome is unobserved. See Human Verification. |
| 5 | Slot record decides due, outbox decides content; late/duplicate/skipped/zero-then-populated ticks converge on one digest per fire with none dropped or duplicated; enabling digest mode or changing cadence never itself triggers an immediate send | ✓ VERIFIED | `internal/settings/settings.go` `Update`'s D-14 re-anchor (`slot := MostRecentSlot(s.now(), cadence, s.loc)` written into `digest_last_slot_at` on enable/cadence-change, via `queries/notification_settings.sql`'s CASE reading pre-UPDATE values) means the just-enabled slot is immediately marked handled, so the next due-check does not fire for it; `TestService_UpdateReanchorsSlot_EnablingFromOff`/`_CadenceChangeWhileOn`/`TestService_UpdateLeavesSlotUntouched` (22-01) plus the full DST/grace matrix above jointly prove convergence |

**Score:** 5/5 truths verified (1 present, behavior-unverified — routed to human verification, not counted toward score per the verifier's own scoring rule)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Oversized digest (Description > 4096 chars) has no interim safety valve; a persistently oversized batch would loop forever on every due-check (code-review CR-01) | Phase 23 | ROADMAP.md's "Deploy sequencing" note and 22-CONTEXT.md D-19: Phases 21/22/23 ship in one release specifically because Phase 23 supplies the chunking that closes this gap; nothing in Phase 22's own ROADMAP success criteria requires it. Confirmed in code: `buildDigestEmbed` (digest_format.go) caps only per-line title length (100 runes), never the whole `Embed.Description`; `SendDigestIfDue`'s send-failure branch (digest.go:114-120) logs and returns without acking, so a failed send retries the same (or larger) batch every 5 minutes until grace expires — exactly as the review describes. This is a real, currently-live risk if Phase 22 is deployed alone, but the project's own documented release-sequencing rule (not yet violated — this branch has not been merged to `main`) is what prevents that. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrations/000009_digest_last_slot_at.{up,down}.sql` | Additive nullable slot-record column | ✓ VERIFIED | Confirmed exact `ADD COLUMN digest_last_slot_at timestamptz` / `DROP COLUMN digest_last_slot_at`; `TestRunMigrations_AppliesFromScratch` re-run live — lands on version 9 |
| `internal/settings/slot.go` | `MostRecentSlot`, `GraceFor`, `ZoneName`, grace constants | ✓ VERIFIED | All exports present; `TestMostRecentSlot_*`/`TestGraceFor` re-run live — all PASS |
| `queries/notification_settings.sql` | D-14 re-anchor CASE + D-16 `AckDigestBatch` CTE | ✓ VERIFIED | Read in full; CASE reads pre-UPDATE row values as designed; `AckDigestBatch`'s unreferenced data-modifying CTE atomically acks events + writes both settings columns |
| `internal/notifier/digest.go` | `SendDigestIfDue` implementing D-17's sequence | ✓ VERIFIED | Read in full; sequence matches CONTEXT.md D-17 exactly (CAS lock → zone guard → settings read → due/grace decision → outbox partition → empty-skip → embed → re-check → send → ack) |
| `internal/notifier/digest_format.go` | Single-embed builder: escaping, collation, headings, host credit, track suffix | ✓ VERIFIED | Read in full; `escapeMarkdown`, `sortDigestGroup` (collator), `artistKey`/`lineLabel`, deluxe suffix all present and match CONTEXT.md decisions |
| `internal/notifier/scheduler.go` | `DigestScheduler` immediate-check + 5-min loop + bounded drain | ✓ VERIFIED | Read in full; dual stop-signal design (`stopCh` vs `runCancel`) correctly separates graceful drain from forced cancellation; all 8 lifecycle tests re-run live — PASS |
| `.github/workflows/full-pipeline.yml` | `build-scan` boot step asserting Alpine zone resolution | ✓ VERIFIED (structurally) | Steps sit between `build-scan:` and `release:`, `needs:` array unchanged (8 entries), no new `uses:`, correct literals (`drop-tracker-zone-pg`, `drop-tracker:scan`, `America/New_York`); live execution unobserved (see human verification) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/settings/settings.go` | `queries/notification_settings.sql` | `ReanchorSlot` param | ✓ WIRED | `Update` computes `slot` and passes `ReanchorSlot: pgtype.Timestamptz{...}` on every call; SQL CASE decides application |
| `cmd/server/main.go` | `internal/settings/slot.go` | `time.LoadLocation(settings.ZoneName)` fail-fast | ✓ WIRED | Confirmed in source; no fallback branch exists |
| `internal/notifier/digest.go` | `internal/settings/slot.go` | `settings.MostRecentSlot`/`GraceFor` | ✓ WIRED | Called directly in `SendDigestIfDue` |
| `internal/notifier/digest.go` | `internal/db/sqlc/querier.go` | `AckDigestBatch` | ✓ WIRED | `ackDigestBatch` helper calls `q.AckDigestBatch` under `context.WithoutCancel` |
| `internal/notifier/scheduler.go` | `internal/notifier/notifier.go` | `Sink.SendDigestIfDue` | ✓ WIRED | `check()` calls `s.sink.SendDigestIfDue(...)` against the `Sink` interface, never a concrete type |
| `cmd/server/main.go` | `internal/notifier/scheduler.go` | `NewDigestScheduler`/`Start`/drain defer | ✓ WIRED | Constructed from `notif` (the `Sink`), started with the run `ctx`, drained before `pool.Close()` per LIFO defer ordering — confirmed by line-number check (`NewDigestScheduler` call at line >`defer pool.Close()`) |
| `.github/workflows/full-pipeline.yml` | `cmd/server/main.go` | boot log grep for `"digest zone resolved"` | ✓ WIRED (structurally) | Exact literal match confirmed on both sides; live behavior unexercised |

### Behavioral Spot-Checks (re-run live this session, not trusted from SUMMARY)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| SendDigestIfDue full D-17 branch matrix | `go test ./internal/notifier/ -run TestSendDigestIfDue -v` (real Postgres) | 12/12 PASS | ✓ PASS |
| DigestScheduler lifecycle (immediate check, N+1 ticks, graceful/forced Stop, error continuation) | `go test ./internal/notifier/ -run 'TestDigestScheduler_(Start|TickSource|ChecksReceive|Stop|CheckError|ContextCancelled)' -v` | 8/8 PASS | ✓ PASS |
| DST matrix (4 daily + 4 weekly transition dates, 2026/2027) | `go test ./internal/notifier/ -run 'TestDigestScheduler_DST' -v` (real Postgres, 5-min-step sweeps) | 9/9 PASS (13.7s) | ✓ PASS |
| Grace-window matrix (restart catch-up, inclusive boundaries, nothing-lost, not-due-quiet) | `go test ./internal/notifier/ -run 'TestDigestScheduler_(Grace|Catchup|NotDue)' -v` | 8/8 PASS | ✓ PASS |
| Slot math (DST transitions, both cadences) | `go test ./internal/settings/ -run 'TestMostRecentSlot|TestGraceFor' -v` | 12/12 PASS | ✓ PASS |
| Migration 000009 lands schema at version 9 from scratch | `go test ./internal/db/ -run TestRunMigrations -v` | 10/10 PASS | ✓ PASS |
| Full workspace build/vet/lint/test/coverage | `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./... -count=1` (24 packages), `make coverage-gate` | clean / clean / 0 issues / all pass / 91.34% (≥80% floor) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DGST-05 | 22-01, 22-02, 22-04 | Stable predictable fire time, resilient to downtime within grace window | ✓ SATISFIED | Catch-up + grace-boundary tests, re-anchor-on-enable logic |
| DGST-06 | 22-01, 22-04 | DST-safe, no skip/double-fire | ✓ SATISFIED | Calendar-based `MostRecentSlot`, full DST sweep matrix passing live |
| DGST-07 | 22-01, 22-02, 22-04 | Works in Alpine's minimal tzdata | ? NEEDS HUMAN | Code path proven (`time/tzdata` blank import, fail-fast load); CI boot proof structurally correct but never executed against this branch |
| DGST-08 | 22-02, 22-03 | All three event types batched into one message | ✓ SATISFIED | `buildDigestEmbed`, live end-to-end test |
| DGST-09 | 22-02 | Zero-event digest is a silent skip | ✓ SATISFIED | Empty-outbox / all-suppressed tests passing live |
| DGST-10 | 22-03 | Grouped by type/artist for readability | ✓ SATISFIED | Collated sort, fixed heading order, tests passing live |
| DGST-15 | 22-01, 22-02, 22-04 | Slot record decides due, outbox decides content | ✓ SATISFIED | Three-role separation confirmed in SQL and Go; nothing-lost carry-forward test passing live |

No orphaned requirement IDs: REQUIREMENTS.md maps exactly DGST-05, 06, 07, 08, 09, 10, 15 to Phase 22, and all seven are claimed across the four plans' `requirements:` frontmatter.

### Anti-Patterns Found

None of the blocker classes (TBD/FIXME/XXX, empty stub implementations, hardcoded-empty data flowing to output) were found in the phase's changed files. Three findings from the phase's own code review (`22-REVIEW.md`) were independently re-confirmed against current source in this session:

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/server/main.go` | 338 vs 346-350 | `digestSched.Start(ctx)` runs before the fallible `poller.New(...)` call; that call's error return path (line 348) precedes the digest drain defer's registration | ⚠️ Warning | On a `poller.New` construction failure at boot, the digest scheduler's background goroutine is left running without an explicit bounded drain — it is eventually stopped only when the parent `ctx` is cancelled during process exit. Narrow window, boot-failure-only, does not affect steady-state digest delivery. Confirmed still present; not fixed since the code review. |
| `internal/notifier/digest.go` | 54-65 | Grace-expired Warn re-logs every 5-minute tick for the rest of the missed period (no dedup) | ⚠️ Warning | Noisy logs during an extended outage, not a functional defect — success criterion 2's "visible in structured logs" requirement is still met (it IS visible, just repeated). Confirmed still present. |
| `internal/notifier/digest_format.go` | 93-124 | An `EventType` outside the three known headings would be acked as delivered but never rendered in the Description | ⚠️ Warning | Currently unreachable — `events_event_type_valid` CHECK constraint permits only the three existing types — so no live gap today; latent trap for a future fourth event type. Confirmed still present. |

## Human Verification Required

### 1. Live CI boot proof of Alpine zone resolution

**Test:** Push this branch (or land it per the project's documented 21+22+23 combined-release rule) and observe the `build-scan` job's three new steps in `.github/workflows/full-pipeline.yml`.
**Expected:** The boot step's log poll finds `"msg":"digest zone resolved"` and `"zone":"America/New_York"` in `drop-tracker:scan`'s output within 60 seconds and exits 0; the containers are cleaned up regardless of outcome.
**Why human:** Go resolves `time.LoadLocation` against the host's system zoneinfo before ever falling through to a binary's embedded `time/tzdata`. This dev machine (and this GitHub Actions runner class generally, outside the Alpine container under test) has system zoneinfo present, so every local `go build`/`go test`/`go vet` run in this verification session would pass identically whether or not the embedded tzdata inside the actual Alpine-based `drop-tracker:scan` image is correct. Only booting that exact image and reading its own log output proves the claim — and `git status`/`git ls-remote` confirm this branch has not yet been pushed, so that CI job has never run against this code.

## Gaps Summary

No blocking gaps. All five ROADMAP success criteria have direct, live-re-run behavioral evidence except the Alpine-tzdata sub-claim of criterion 4, which is code-complete and structurally verified but requires an actual CI run to close out — a one-time observation, not rework. One item (CR-01, oversized-digest safety valve) is a real, currently-unmitigated risk if Phase 22 were deployed in isolation, but it is explicitly and correctly deferred to Phase 23 under the project's own documented release-sequencing decision (D-19) and is not a Phase 22 scope failure. Three narrower code-review warnings (scheduler drain ordering on a rare boot failure, repeated grace-expired logging, a latent silent-drop trap for a hypothetical future event type) remain open in source as the review found them — none block any of the five success criteria.

---

_Verified: 2026-09-17T03:12:51Z_
_Verifier: Claude (gsd-verifier)_
