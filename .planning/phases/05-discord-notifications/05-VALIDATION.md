---
phase: 05
slug: discord-notifications
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-08
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
| 05-TBD | TBD | TBD | NTFY-01 | V5/V7 | new_release embed carries title/artist/cover/date/release-type, POSTs successfully on 204 | unit (httptest.Server) | `go test ./internal/discord/... -run TestClient_Send -short` | ❌ Wave 0 | ⬜ pending |
| 05-TBD | TBD | TBD | NTFY-01 | — | `previous_track_count`/`release_type` columns round-trip through `InsertEvent`/`ListUnnotified` | integration (real Postgres) | `go test ./internal/detection/... -run TestDetectMusicBrainz -race` | ❌ Wave 0 (extends existing `detector_test.go`) | ⬜ pending |
| 05-TBD | TBD | TBD | NTFY-02 | — | guest_feature embed is visually distinct and links to the recording | unit | `go test ./internal/notifier/... -run TestFormatEmbed_GuestFeature -short` | ❌ Wave 0 | ⬜ pending |
| 05-TBD | TBD | TBD | NTFY-03 | — | deluxe_change embed shows old→new track delta | unit | `go test ./internal/notifier/... -run TestFormatEmbed_DeluxeChange -short` | ❌ Wave 0 | ⬜ pending |
| 05-TBD | TBD | TBD | NTFY-04 | — | A muted event never reaches `ListUnnotified` | already covered | Existing Phase 4 `detector_test.go` mute-axis tests — confirm still green | ✓ (Phase 4) | ⬜ pending |
| 05-TBD | TBD | TBD | D-06 | DoS (accepted) | Concurrent MB/Deezer cycles never double-post the same pending event | integration (real Postgres, genuine concurrency) | `go test ./internal/notifier/... -run TestNotifyPending_ConcurrentCyclesNoDoublePost -race` | ❌ Wave 0 | ⬜ pending |
| 05-TBD | TBD | TBD | D-08/D-09 | — | A 429 honors `Retry-After` once; a persistent failure leaves `notified_at` NULL for next-cycle pickup | unit (httptest.Server) | `go test ./internal/discord/... -run TestClient_Send_HonorsRetryAfter -short` | ❌ Wave 0 | ⬜ pending |
| 05-TBD | TBD | TBD | D-10 | — | Empty `DISCORD_WEBHOOK_URL` boots normally, notify no-ops, one startup log line | unit | `go test ./cmd/server/... -run TestMain_DiscordDisabled -short` | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — Task IDs/Plan/Wave columns are filled in by the planner once PLAN.md files exist.*

---

## Wave 0 Requirements

- [ ] `internal/discord/client_test.go` — new file, covers `Client.Send`'s 204/429/other-status paths, mirroring `internal/musicbrainz/search_test.go`'s `httptest.Server` pattern
- [ ] `internal/notifier/notifier_test.go` — new file, covers the fetch/format/send/mark loop and the D-06 concurrency guard, mirroring `internal/detection/detector_test.go`'s real-Postgres integration style
- [ ] `internal/notifier/format_test.go` — new file, covers per-event-type embed formatting (color/emoji/link construction/truncation)
- [ ] Existing `internal/detection/musicbrainz_test.go`/`deezer_test.go` — extend with assertions that `PreviousTrackCount`/`ReleaseType` populate correctly on insert (new test cases, not a new file)

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification. (A live Discord webhook end-to-end smoke test is optional/manual UAT, not a blocking verification — CI never hits a real webhook per CLAUDE.md's no-live-external-calls constraint.)*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
