---
phase: "15"
slug: "pr-coverage-diff-comment"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: false
wave_0_complete: true
created: "2026-09-02"
validated: "2026-09-03"
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

| Task ID | Plan | Requirement | Test Type | Automated Command | Verifying Test(s) | Status |
|---------|------|-------------|-----------|-------------------|-------------------|--------|
| 15-01-01 | 01 | CICD-13 | unit (scaffold) | `go test ./cmd/coverage-report/... -count=1` | `main_test.go` — 21 test funcs | ✅ green |
| 15-01-02 | 01 | CICD-13 | unit | `go test ./cmd/coverage-report/... -run 'TestBackendTotalPct'` | `TestBackendTotalPct`, `TestBackendTotalPct_MergesDuplicateBlocks`, `TestParseBlockLine_LastColonSplit` | ✅ green |
| 15-01-03 | 01 | CICD-13 | unit | `go test ./cmd/coverage-report/... -run TestFrontendLinesPct` | `TestFrontendLinesPct` | ✅ green |
| 15-01-04 | 01 | CICD-13 | unit | `go test ./cmd/coverage-report/... -run 'TestDelta|TestRenderComment_Unchanged'` | `TestDelta`, `TestRenderComment_Unchanged` (`±0.00pp` distinct from `—`) | ✅ green |
| 15-01-05 | 01 | CICD-13 | unit (golden) | `go test ./cmd/coverage-report/... -run 'TestRenderComment_Golden|TestRenderComment_NoUntrustedInterpolation'` | `TestRenderComment_Golden`, `_GoldenHasFixedShape`, `_NoUntrustedInterpolation`, `TestSHAValidation` | ✅ green |
| 15-01-06 | 01 | CICD-13 | unit | `go test ./cmd/coverage-report/... -run 'TestRenderComment_MissingProfile|TestRenderComment_UnparseableProfile|TestBackendTotalPct_MalformedHeader'` | `TestRenderComment_MissingProfile` (missing), `TestRenderComment_UnparseableProfile` + `TestBackendTotalPct_MalformedHeader` (present-but-unparseable — **added by this audit**, WR-04) | ✅ green |
| 15-01-07 | 01 | CICD-14 | unit | `go test ./cmd/coverage-report/... -run 'TestModeTotal|TestModeSidecar'` | `TestModeTotal_PrintsOnlyNumber`, `_MissingProfile`, `TestModeSidecar_Roundtrip`, `TestSidecar_RejectsBadSHA` | ✅ green |
| 15-01-08 | 01/03 | CICD-13 | unit | `go test ./cmd/coverage-report/... -run TestRenderComment_UpstreamRed` | `TestRenderComment_UpstreamRed` — `--upstream-red=true` footer present / absent (**added by this audit**, WR-03) | ✅ green |
| 15-02-01 | 02 | CICD-14 | integration (CI-only) | `make coverage-gate` (after `make test-integration`) | Real integration run recorded in 15-02-SUMMARY: gate PASS at 90.03%, margin 10.03pp above the 80 floor (D-17 cutover). Not runnable on the dev box (A1 cgo/-race). | ✅ green (CI) |
| 15-02-02 | 02 | CICD-14 | procedural | `make -n test-integration \| grep -c 'cmd/coverage-report'` == 0; `'cmd/server'` >= 1 | Procedural assertion (15-02-SUMMARY D1), mirrors Phase 09's `COVER_PKGS` guard posture — no committed test by precedent | ✅ green (procedural) |
| 15-03-01 | 03 | CICD-13 | static (local) | `actionlint .github/workflows/full-pipeline.yml` | Clean on every workflow-touching commit; local-only by design (15-03 key-decision — not wired into CI/pre-commit) | ✅ green (local) |
| 15-03-02 | 03 | CICD-13, CICD-14 | manual / live | scratch-branch PR against a throwaway branch — **never `main`** | Not automatable. Completed via live-CI UAT — see `15-UAT.md`: SC #1–#5 all pass on PR #2 / PR #3 (run IDs recorded). | ✅ done (UAT) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `cmd/coverage-report/main.go` — parse (`coverage.out` raw counts + Vitest `coverage-summary.json`) + delta vs sidecar + render markdown + `--mode` flag
- [x] `cmd/coverage-report/main_test.go` — table tests per the map above (backend total, frontend lines, delta, render golden, missing-profile, `--mode=total`); 21 test funcs
- [x] `cmd/coverage-report/testdata/` — `backend-profile*.txt`, `coverage-summary*.json`, `baseline-metrics-{backend,frontend}.json`, four `comment-*.golden.md`
- [x] `COVER_PKGS`-excludes-`cmd/coverage-report` assertion — procedural (`make -n`), mirrors Phase 09
- [x] `actionlint` invocation on `full-pipeline.yml` — local verification tool (not CI/pre-commit, per 15-03 decision)

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

- [x] All tasks have automated verify, a recorded real-CI run, or a documented manual-only entry
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s (Go tool loop — `go test ./cmd/coverage-report/...` ~0.5s)
- [ ] `nyquist_compliant: true` — **PARTIAL**: one legitimately-manual item (15-03-02, live-PR runtime behavior) remains, completed via UAT but not automatable

**Approval:** validated (PARTIAL) — 2026-09-03

---

## Validation Audit 2026-09-03

| Metric | Count |
|--------|-------|
| Gaps found | 2 |
| Resolved (tests added) | 2 |
| Escalated | 0 |

**Input state:** A (existing VALIDATION.md, map stale — all rows `⬜ pending`, frontmatter `draft`).

**What the audit found:** All three plans executed and closed; `15-VERIFICATION.md` (2/5 truths static-verified, 3 routed to human) and `15-UAT.md` (SC #1–#5 all pass, live, PR #2/#3) already complete. 9 of 11 requirement-behaviors had green automated coverage; the map had simply never been updated post-execution. Two genuine committed-test gaps remained, both flagged in `15-REVIEW.md`:

- **WR-03** — the `--upstream-red=true` comment footer line had no test (every comment-mode test passed the flag false).
- **WR-04** — `testdata/backend-profile-malformed.txt` was committed but referenced by no test; the `backendTotalPct` header-error branch and comment-mode degradation on a *present-but-unparseable* profile were both uncovered.

**Resolution:** `gsd-nyquist-auditor` added 3 test functions to `cmd/coverage-report/main_test.go` (`TestRenderComment_UpstreamRed`, `TestBackendTotalPct_MalformedHeader`, `TestRenderComment_UnparseableProfile`) — test-only, no production change, existing fixture reused. Package now 21 test funcs, all green; `go vet` + `golangci-lint` clean.

**Not converted to tests (deliberate):** 15-02-01 (integration gate — CI-only, verified on a real run), 15-02-02 (`COVER_PKGS` exclusion — procedural per Phase 09 precedent), 15-03-01 (`actionlint` — local tool by 15-03 decision), 15-03-02 (live-PR behavior — not automatable, done via UAT). Review finding WR-05 (mixed-baseline em-dash) is an implementation-behavior question, out of scope for validation — left for a follow-up.
