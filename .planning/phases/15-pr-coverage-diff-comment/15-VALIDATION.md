---
phase: "15"
slug: "pr-coverage-diff-comment"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-02"
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (Go 1.26 per `go.mod`); frontend config change verified by existing Vitest 4.1.10 suite |
| **Config file** | none — standard `go test`; `web/vitest.config.ts` for the frontend reporter change |
| **Quick run command** | `go test ./cmd/coverage-report/... -count=1` |
| **Full suite command** | `make test-integration` then `make coverage-gate` (now also exercises the `coverage-report` path) |
| **Estimated runtime** | ~5 s quick (Go tool unit tests); ~2–4 min full integration suite |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/coverage-report/... -count=1`
- **After every plan wave:** Run `go vet ./... && golangci-lint run && make test && make coverage-gate` (Definition of Done, `.claude/CLAUDE.md`); `actionlint .github/workflows/full-pipeline.yml` for workflow-touching waves
- **Before `/gsd-verify-work`:** Full suite green + `actionlint` clean + the live scratch-branch PR walkthrough (SC #1–#5)
- **Max feedback latency:** ~10 s for the Go tool loop; ~4 min for a full integration pass

---

## Per-Task Verification Map

> Task IDs are provisional — the planner assigns final IDs/waves. This map binds each phase requirement to an automated check and names the Wave 0 test scaffold it depends on.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 0 | CICD-13 | — | N/A | unit (scaffold) | `go test ./cmd/coverage-report/... -count=1` | ❌ W0 `cmd/coverage-report/main_test.go` + `testdata/` | ⬜ pending |
| 15-01-02 | 01 | 1 | CICD-13 | T-15 V5 | Parser fails closed — renders `unavailable`, never panics | unit | `go test ./cmd/coverage-report/... -run TestBackendTotal` | ❌ W0 `testdata/sample.out` | ⬜ pending |
| 15-01-03 | 01 | 1 | CICD-13 | — | N/A | unit | `go test ./cmd/coverage-report/... -run TestFrontendLines` | ❌ W0 `testdata/coverage-summary.json` | ⬜ pending |
| 15-01-04 | 01 | 1 | CICD-13 | — | `±0.00pp` distinct from `—`; no nonsense delta when sidecar absent (D-11/D-12) | unit | `go test ./cmd/coverage-report/... -run TestDelta` | ❌ W0 `testdata/baseline-metrics-*.json` | ⬜ pending |
| 15-01-05 | 01 | 1 | CICD-13 | T-15 V5 | Only numbers + fixed strings emitted into the comment body; no file text interpolated | unit (golden) | `go test ./cmd/coverage-report/... -run TestRender` | ❌ W0 `testdata/*.golden.md` | ⬜ pending |
| 15-01-06 | 01 | 1 | CICD-13 | — | Per-row `unavailable` when a profile is missing/unparseable (D-18) | unit | `go test ./cmd/coverage-report/... -run TestRender_MissingProfile` | ❌ W0 | ⬜ pending |
| 15-01-07 | 01 | 1 | CICD-14 | — | `--mode=total` prints ONLY the number to stdout (D-17 / Pitfall 9) | unit | `go test ./cmd/coverage-report/... -run TestModeTotal` | ❌ W0 | ⬜ pending |
| 15-02-01 | 02 | 2 | CICD-14 | — | Gate still passes consuming the tool's number; margin above 80 confirmed on a real run before cutover (D-17 / A1) | integration | `make coverage-gate` (after `make test-integration`) | existing target, refactored | ⬜ pending |
| 15-02-02 | 02 | 2 | CICD-14 | — | `COVER_PKGS` excludes `cmd/coverage-report` (D-07), anchored regex | grep/unit | mirror Phase 09 `COVER_PKGS` guard assertion | ❌ W0 | ⬜ pending |
| 15-03-01 | 03 | 2 | CICD-13 | T-15 V1/V4/V14 | Report-only job, no `needs:` consumer, job-scoped `pull-requests: write`, SHA-pinned actions | static | `actionlint .github/workflows/full-pipeline.yml` | existing workflow | ⬜ pending |
| 15-03-02 | 03 | 3 | CICD-13, CICD-14 | T-15 V1 | One comment; edits in place; degrades to "delta unavailable" on no baseline; never blocks merge; baseline published on merge to main | manual / live | scratch-branch PR against a throwaway branch — **never `main`** | not automatable | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/coverage-report/main.go` — parse (`coverage.out` raw counts + Vitest `coverage-summary.json`) + delta vs sidecar + render markdown + `--mode` flag
- [ ] `cmd/coverage-report/main_test.go` — table tests per the map above (backend total, frontend lines, delta, render golden, missing-profile, `--mode=total`)
- [ ] `cmd/coverage-report/testdata/` — `sample.out` (a real `mode: atomic` profile slice), `coverage-summary.json` (a real Vitest v8 summary), `baseline-metrics-backend.json` / `baseline-metrics-frontend.json`, `*.golden.md`
- [ ] `COVER_PKGS`-excludes-`cmd/coverage-report` assertion (mirror Phase 09's anchored-regex guard pattern)
- [ ] `actionlint` invocation on `full-pipeline.yml` (pre-commit hook or a CI step — planner's discretion; not strictly required by CONTEXT)

Framework install: none — Go stdlib `testing` already in use.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SC #1 — one comment with backend + frontend totals and each delta vs the main baseline | CICD-13 | Needs a real PR + a populated Actions cache; `act`/`actionlint` cannot emulate cache/artifact/comment behavior | Open a PR from a scratch branch; confirm exactly one comment with both rows and signed deltas |
| SC #2 — further pushes edit the same comment | CICD-13 | Requires the sticky-comment hidden-marker upsert against the live PR API | Push two more commits; confirm still one comment, not three |
| SC #3 — no-baseline state | CICD-13 | Requires an evicted/empty cache entry on a live run | Delete the cache entry (or run before any main baseline exists); confirm the comment shows absolute numbers + "delta unavailable", no error, no nonsense delta |
| SC #4 — coverage drop still mergeable | CICD-13, CICD-14 | Requires a real PR whose coverage falls below a gate | Push a change that drops backend below 80% (or a Vitest axis below 70%); confirm the comment posts with the real number + ⚠️, the `test`/`frontend-test` gate goes red, and the PR itself stays mergeable (no new blocking check) |
| SC #5 — merge publishes baseline; no PR recompute | CICD-14 | Requires a merge to main then a subsequent PR | Merge the scratch PR; open a new PR; confirm its delta is measured against the merged run's coverage and no PR job re-runs main's suite |

Delete the scratch branch and any throwaway target branch after the walkthrough. Mirrors Phase 09's live-CI verification approach (scratch branch, never `main`).

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s (Go tool loop)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
