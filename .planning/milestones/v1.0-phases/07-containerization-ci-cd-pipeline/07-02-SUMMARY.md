---
phase: 07-containerization-ci-cd-pipeline
plan: 02
subsystem: ci-cd-tooling
tags: [golangci-lint, pre-commit, linting, ci-config]
dependency-graph:
  requires: []
  provides:
    - .golangci.yml (v2-schema golangci-lint config, CI's lint step in plan 07-03 will read this)
    - .pre-commit-config.yaml golangci-lint hook (local lint gate)
  affects:
    - .github/workflows/full-pipeline.yml (plan 07-03, consumes .golangci.yml via golangci-lint-action)
tech-stack:
  added:
    - golangci-lint v2.12.2 (CLI, v2 config schema)
  patterns:
    - "defer func() { _ = x.Close() }() for best-effort deferred cleanup where the Close/Rollback error is not actionable"
key-files:
  created:
    - .golangci.yml
  modified:
    - .pre-commit-config.yaml
    - internal/config/config_test.go
    - internal/db/migrate.go
    - internal/db/migrate_test.go
    - internal/deezer/albums.go
    - internal/deezer/search.go
    - internal/discord/client.go
    - internal/httpserver/boot_e2e_test.go
    - internal/httpserver/events_test.go
    - internal/httpserver/health_test.go
    - internal/httpserver/search_test.go
    - internal/httpserver/server_test.go
    - internal/httpserver/spa_test.go
    - internal/httpserver/watchlist_test.go
    - internal/musicbrainz/recordings.go
    - internal/musicbrainz/releasegroups.go
    - internal/musicbrainz/releases.go
    - internal/musicbrainz/search.go
    - internal/watchlist/service_test.go
decisions:
  - "golangci-lint's default max-same-issues cap (3) hides most errcheck findings sharing the same message text behind a truncated first run -- used `--max-same-issues=0 --max-issues-per-linter=0` to surface the true count (69) before fixing, rather than iterating blind against the capped view."
  - "All 69 errcheck findings were the identical mechanical pattern (unchecked resp.Body.Close()/Close()/Rollback()) -- fixed via a single scripted regex substitution across the 8 affected files rather than 69 individual edits, since it is one transformation applied repeatedly, not 69 distinct judgment calls."
  - "version: \"2\" must be .golangci.yml's literal first line per the plan's acceptance criteria -- moved the explanatory header comment below it (comments can't precede the acceptance-gated head -1 check)."
metrics:
  duration: 45min
  completed: 2026-08-12
actuals:
  tokens: 8400
  tasks: 2
  commits: 2
status: complete
---

# Phase 07 Plan 02: golangci-lint Config + Pre-commit Hook Summary

Added `.golangci.yml` (v2 schema, full standard linter set) and closed the golangci-lint half of the local pre-commit gate, resolving all 69 pre-existing errcheck findings the repo had never been linted against before.

## What Was Built

**Task 1 — `.golangci.yml` + lint-clean repo (CICD-01):** Created a v2-schema golangci-lint config (`version: "2"` on line 1, `linters: default: standard`, `run: timeout: 5m`). Installed golangci-lint v2.12.2 and ran it against the whole repo for the first time ever. It surfaced 69 `errcheck` findings — every one an unchecked deferred or inline `Close()`/`Rollback()` call (`resp.Body.Close()`, `sqlDB.Close()`, `f.Close()`, `tx.Rollback(...)`) where the returned error genuinely isn't actionable at a cleanup site. Fixed all 69 by explicitly discarding the error (`defer func() { _ = x.Close() }()` / `_ = x.Close()`), the idiomatic Go pattern for "I know this can fail, I'm choosing not to act on it" — never a bare unwrapped `defer x.Close()` that silently swallows the error without saying so. No `//nolint` directives were needed; every finding was a genuine, fixable issue, not a false positive. `linters.default: standard` was never narrowed to reach green.

**Task 2 — pre-commit golangci-lint hook (CICD-10):** Added a second `repos:` entry to `.pre-commit-config.yaml` pointing at `golangci-lint`'s own pre-commit hook repo, pinned `rev: v2.12.2` (matching the CLI version Task 1 installed and the version plan 07-03's CI job will pin), using hook id `golangci-lint` (changed-files-only, `--new-from-rev HEAD --fix`) to match the existing gitleaks hook's staged-diff-only scope. Rewrote the file's stale header comment, which previously forward-referenced this now-completed phase ("intentionally deferred to Phase 07").

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `.golangci.yml`'s `version: "2"` key was not on line 1**
- **Found during:** Final acceptance-criteria verification of Task 1 (`head -1 .golangci.yml` check).
- **Issue:** The file's explanatory header comment (naming its two consumers) was written above `version: "2"`, so `head -1` returned the comment line, not `version: "2"` — violating the plan's own acceptance criterion.
- **Fix:** Moved `version: "2"` to line 1; the explanatory comment now sits below it, still ahead of the `run:`/`linters:` blocks it describes.
- **Files modified:** `.golangci.yml`
- **Commit:** `0fe6405`

**2. [Rule 3 - Blocking] golangci-lint's default issue-display caps hid the true finding count**
- **Found during:** Task 1, second `golangci-lint run ./...` pass.
- **Issue:** golangci-lint's default `--max-same-issues=3` collapsed 69 distinct errcheck findings (same message text, different locations) down to 3-at-a-time, so each fix-and-rerun cycle revealed only 3 new findings instead of the true remaining count — a naive iterate-until-clean loop would have taken ~23 round trips.
- **Fix:** Re-ran with `--max-same-issues=0 --max-issues-per-linter=0` to get the authoritative full list (69 issues, all the same `resp.Body.Close`-shaped pattern) up front, then fixed all 69 in one scripted pass.
- **Files modified:** (see key-files list above)
- **Commit:** `3b4f209`

### Auth Gates

None.

## Known Stubs

None — no stub/placeholder patterns introduced by this plan.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries were introduced. Both threats registered in this plan's `<threat_model>` (T-07-07 `--fix` transparency, T-07-08 lint-suppression repudiation, T-07-09 supply-chain pinning, T-07-10 `--no-verify` bypass) are addressed as designed: no `//nolint` suppressions exist, both hook repos pin exact tags, and `--fix`'s working-tree-visible-rewrite behavior is documented in a comment above the new hook entry.

## Verification Evidence

```
golangci-lint config verify          -> exit 0
golangci-lint run ./...              -> 0 issues
go vet ./...                          -> exit 0
go test ./... -short -count=1        -> ok (all 12 tested packages)
python -m pre_commit run --all-files -> gitleaks Passed, golangci-lint Passed
python -m pre_commit run golangci-lint --all-files -> Passed (standalone)
python -m pre_commit run gitleaks --all-files      -> Passed (standalone, unregressed)
git diff --exit-code (post pre-commit run)          -> clean (no unexpected --fix rewrite)
grep -c 'https://github.com/golangci/golangci-lint' .pre-commit-config.yaml -> 1
grep -c 'rev: v2.12.2' .pre-commit-config.yaml       -> 1
grep -c 'rev: v8.30.1' .pre-commit-config.yaml       -> 1 (gitleaks pin untouched)
grep -c 'deferred to Phase 07' .pre-commit-config.yaml -> 0 (stale forward-reference removed)
head -1 .golangci.yml                                -> version: "2"
grep -qE '^\s*default:\s*standard' .golangci.yml     -> match (full standard set enabled)
grep -rn 'nolint' --include='*.go' .                  -> no matches (no suppressions used)
```

**Note on `make test-short`:** the plan's `<verify>` block runs `make test-short`, which invokes `go test ./... -short -race -count=1`. On this Windows dev machine, `-race` fails with a ThreadSanitizer allocation error across every package (`==NNNNN==ERROR: ThreadSanitizer failed to allocate ... bytes ... error code: 87`) — this is the same pre-existing mingw64/cgo toolchain limitation already documented in STATE.md ("pre-existing cgo toolchain break already documented for -race in 01-02/01-03"), not a regression introduced by this plan's changes (which only wrap already-deferred `Close`/`Rollback` calls in `_ = ...`, a change with no plausible mechanism to affect TSan's memory allocator). Verified equivalence by running `go test ./... -short -count=1` (identical to `make test-short` minus `-race`): all 12 tested packages pass. This machine-specific `-race` limitation is inherited, unrelated to this plan's scope (Rule 3's scope boundary — pre-existing environment issues in unrelated tooling are out of scope for a lint-config plan), and not re-litigated here.

## Requirements Closed

- **CICD-01** (configuration half): `.golangci.yml` exists in v2 schema, full standard linter set enabled, repo lints clean. The CI lint step (plan 07-03) now gates on a deliberate ruleset, not silent defaults.
- **CICD-10**: pre-commit runs both gitleaks (existing) and golangci-lint (new) locally before commit.

## Self-Check: PASSED

- `.golangci.yml` exists: FOUND
- `.pre-commit-config.yaml` contains golangci-lint entry: FOUND
- Commit `3b4f209` exists: FOUND
- Commit `0fe6405` exists: FOUND
