---
phase: 05-discord-notifications
plan: 03
subsystem: notifier
tags: [discord, notifier, concurrency, testing, mute, ntfy-04]
dependency-graph:
  requires:
    - "internal/notifier.Notifier / NotifyPending -- 05-01 (CAS guard, inter-send spacing, error-continue loop)"
    - "internal/discord.Client -- 05-01 (204 success, 429/Retry-After single retry)"
    - "internal/notifier/format.go's formatEmbed -- 05-02 (complete per-event-type rendering)"
    - "internal/detection.Detector's mute filter (D-17/D-18) -- Phase 4"
  provides:
    - "Genuine two-goroutine proof of D-06's CAS guard (TestNotifyPending_ConcurrentCallsNoDoublePost)"
    - "Measured-elapsed-time proof of D-07's inter-send spacing across a real batch (TestNotifyPending_BatchSpacingBetweenSends)"
    - "Proof a mid-batch permanent failure does not starve later rows (TestNotifyPending_BatchMidFailureContinuesToLaterRows)"
    - "Proof D-08's batch-level 429/Retry-After handling never drops or reorders other rows (TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows)"
    - "Proof D-09's cross-cycle re-pickup via two real separate NotifyPending calls spanning a webhook outage and recovery (TestNotifyPending_CrossCycleRecoveryAfterOutage)"
    - "NTFY-04 proven end to end through the real notifier dequeue, not only at the detection layer (TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier)"
  affects: []
tech-stack:
  added: []
  patterns:
    - "Genuine-concurrency test harness: buffered started channel + release channel + go func(){done<-...}(), mirroring commit e53d48c's TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning idiom, reused verbatim for the notifier's own D-06 guard"
    - "Title-keyed fake Sender / httptest.Server handler to distinguish which row in a multi-row batch a request corresponds to, since NotifyPending has no per-row correlation id to assert on directly"
    - "Seed-then-detect two-cycle fixture (mirrors TestDetector_SecondCycleLeavesNotifiedAtNull) to force a genuinely unnotified row into existence for a notifier-path test, since a bare first-cycle new_release row is always pre-notified by D-14's seed mode"
key-files:
  created: []
  modified:
    - internal/notifier/notifier_test.go
    - internal/detection/detector_test.go
decisions:
  - "The mute-through-notifier test (Task 2) required an unplanned seed cycle ahead of the actual detect cycle: a bare first DetectMusicBrainz call for a never-before-seen artist always pre-notifies its new_release row (D-14 seed mode), which would leave NotifyPending with nothing pending to drain and prove nothing about the notifier path. Added a seed cycle (one throwaway release group, no recordings) before the real two-event cycle, mirroring TestDetector_SecondCycleLeavesNotifiedAtNull's existing pattern -- adjusted the row-count assertion from 1 to 2 new_release rows accordingly (the seed row plus the genuinely new one) and scoped the notifier-facing event lookup by external_id rather than by event_type alone."
metrics:
  duration: 40min
  completed: 2026-08-08
actuals:
  tokens: 9200
  tasks: 2
  commits: 2
status: complete
---

# Phase 5 Plan 3: Adversarial Notifier Concurrency and NTFY-04 Regression Coverage Summary

Proves, with genuinely adversarial tests rather than any production-code change, that plan 05-01's CAS guard, inter-send spacing, and error-continue loop actually hold under the conditions D-06 through D-09 were designed for -- real concurrent goroutines, a real multi-row batch with a failure or a 429 buried in the middle, and two truly separate calls spanning a webhook outage and recovery -- and closes NTFY-04's previously-unclaimed requirements gap with a real detect-then-notify regression test.

## What Was Built

**Task 1 -- NotifyPending adversarial coverage (`internal/notifier/notifier_test.go`):** Five new tests, all against a real Postgres-backed `Notifier`.
- `TestNotifyPending_ConcurrentCallsNoDoublePost` launches the first `NotifyPending` call in its own goroutine, blocks it inside a fake `Sender.Send` signaled via a buffered `started` channel, then issues a second call from the test's own goroutine while the first is provably still blocked -- mirroring commit e53d48c's `TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning` idiom exactly. Confirms the second call returns nil with zero additional sends immediately, then confirms exactly one send total and a non-NULL `notified_at` once the first call completes.
- `TestNotifyPending_BatchSpacingBetweenSends` drains three pending rows with a fake `Sender` recording `time.Now()` per call and asserts at least two of the three inter-send gaps are `>=` the configured spacing -- real measured elapsed time, not an inspection of the spacing constant.
- `TestNotifyPending_BatchMidFailureContinuesToLaterRows` fails the middle of three rows (keyed by the embed's title, via a new `insertPendingEventTitled` helper) and positively confirms all three rows were attempted, the first and third are notified, and the middle stays NULL -- not merely that the pass returned nil.
- `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows` uses a real `discord.Client` against one `httptest.Server`: the middle row's first request gets a 429 with a small `retry_after`, its retry and both other rows get 204. Asserts all three rows end up notified, the server recorded exactly four requests, and the third row's request timestamp is after the second row's retry completed.
- `TestNotifyPending_CrossCycleRecoveryAfterOutage` points a `Notifier` at an `httptest.Server` whose handler toggles between always-500 and always-204: the first call leaves the row NULL, and a second, wholly separate call after the toggle delivers it -- proving recovery comes from `ListUnnotified` re-selecting the row on a later call, not any retry state inside the `Notifier`.

**Task 2 -- NTFY-04 through the notifier (`internal/detection/detector_test.go`):** `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` extends `TestDetectMusicBrainz_GuestFeature_Muted`'s fixture (a muted artist, a `guest_feature` candidate that would insert if the mute filter were broken, and an allowed sibling `new_release`) with a real `NotifyPending` call against a real `discord.Client`/`httptest.Server`. After confirming the precondition (zero `guest_feature` rows, the `new_release` row exists), it asserts the server received exactly one request, that no request body ever contained the muted recording's distinguishing title, and that the `new_release` row's `notified_at` became non-NULL.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added a seed cycle to the NTFY-04 notifier test to avoid D-14's pre-notify pre-empting the test**
- **Found during:** Task 2, first test run
- **Issue:** A bare first `DetectMusicBrainz` call for a never-before-seen artist always runs in seed mode (D-14), which pre-sets `notified_at` on every row it inserts at insert time. The test as originally drafted (mirroring `TestDetectMusicBrainz_GuestFeature_Muted`'s single-cycle fixture) left the `new_release` row already notified before `NotifyPending` ever ran, so the notifier had nothing pending to drain and the test's server received zero requests -- proving nothing about the notifier path it exists to test.
- **Fix:** Added a throwaway seed cycle (one release group, no recordings) ahead of the real two-event cycle, mirroring the existing `TestDetector_SecondCycleLeavesNotifiedAtNull` pattern, so the second cycle's `new_release` row is genuinely unnotified. Adjusted the row-count precondition assertion from 1 to 2 `new_release` rows (the seed row plus the real one) and scoped the notifier-facing event lookup to the specific `external_id` rather than any `new_release` row for the artist.
- **Files modified:** `internal/detection/detector_test.go`
- **Commit:** 529f4f1

No other deviations -- both tasks executed within the plan's stated scope; no production file under `internal/notifier`, `internal/discord`, `internal/poller`, or `internal/detection` (other than `detector_test.go`) was modified.

## Known Environment Limitations (not deviations, not fixed)

- **`-race` still fails on this Windows dev box** with the same `ThreadSanitizer failed to allocate` error documented in 05-01-SUMMARY and 05-02-SUMMARY -- re-confirmed this session by running `TestNotifyPending_ConcurrentCallsNoDoublePost` under `-race` and observing the identical allocation failure. All verification in this plan ran without `-race`; every specified test command otherwise passed cleanly, including the full `internal/notifier`, `internal/detection`, and repo-wide `-short` suites.

## Self-Check: PASSED

- `internal/notifier/notifier_test.go` -- FOUND, contains all five new `TestNotifyPending_*` functions (grep-confirmed, 1 each)
- `internal/detection/detector_test.go` -- FOUND, contains `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` (grep-confirmed, 1)
- Commit ef3e526 -- FOUND (`git log --oneline --all`)
- Commit 529f4f1 -- FOUND (`git log --oneline --all`)
- `go build ./...` -- clean
- `go vet ./...` -- clean
- `go test ./internal/notifier/... -count=1` -- PASS (full package, 05-01/05-02's existing tests plus this plan's five new tests)
- `go test ./internal/detection/... -count=1` -- PASS (full package, including this plan's new NTFY-04 test)
- `go test ./... -short -count=1` -- PASS (all packages)
