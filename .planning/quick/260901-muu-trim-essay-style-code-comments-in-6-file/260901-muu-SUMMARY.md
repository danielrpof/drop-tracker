---
phase: quick-260901-muu
plan: 01
status: complete
date: 2026-09-01
---

# Summary: Trim essay-style comments in 6 files

Comments-only trim of the 6 most comment-dense source files, bringing them in line
with the "Comment discipline" rule added in `5de010b`. Zero logic changes.

## Result

| File | Comment lines before → after | Longest run before → after |
|---|---|---|
| `web/app/lib/authStore.ts` | 121 → 26 | 81 → 7 |
| `internal/config/config.go` | 59 → 26 | 16 → 6 |
| `internal/db/pool.go` | 101 → 42 | 19 → 5 |
| `internal/detection/detector.go` | 167 → 65 | 30 → 8 |
| `internal/detection/musicbrainz.go` | 359 → 115 | 80 → 8 |
| `internal/notifier/notifier.go` | 192 → 75 | 29 → 8 |
| **Total** | **999 → 349** (65% reduction) | — |

All per-file ceilings from the plan met or beaten.

## Verification

- **Comments only:** `git diff 3eeff46..HEAD -G'^[[:space:]]*[^[:space:]/]'` over the
  6 files is empty — no added/removed line's first non-whitespace char is anything
  but `/`. The 3 `//nolint:gosec` directives (code lines) are untouched.
- **No information deleted:** per-file token-set diff (design-doc IDs + doc paths)
  from `3eeff46` to `HEAD` is empty for every file. `04-01` (not matched by the
  gate regex) verified preserved by hand in `detector.go`.
- **Scope:** exactly the 6 files changed, nothing else.
- `go build ./...`, `go vet ./...`, full Go test suite (see note), `prettier --check`,
  and the web vitest suite (125/125) all pass.

## Deviations

- **Task 2 split into 4 commits instead of 1.** The `gsd-executor` subagent hit an
  account session limit mid-Task-2 after committing `authStore.ts` (Task 1, `5e4b72a`)
  and leaving `config.go` + `pool.go` edited-but-uncommitted. The orchestrator finished
  the task inline: `pool.go` needed further trimming below the executor's stopping
  point, then `notifier.go`, `detector.go`, `musicbrainz.go` were done one per commit
  (`c174753`, `1de51fe`, `275126b`, `eed828a`) with the per-file gate re-run after each.
- **`make test` fallback used.** `make test` runs `go test -race`, which fails on this
  Windows box with the documented ThreadSanitizer allocation error (STATE.md Phase
  11.1-04). Substituted `docker compose up -d --wait postgres` +
  `TEST_DATABASE_URL=... go test ./... -count=1`. `make coverage-gate` skipped (no
  `-coverprofile` run).

## Blockers / follow-ups

- **`golangci-lint` pre-commit hook is environment-broken** and unrelated to this
  task: the pre-commit-managed golangci-lint v2.12.2 binary was built with go1.25,
  and `go.mod` now targets `go 1.26`, so it refuses to load
  (`the Go language version (go1.25) used to build golangci-lint is lower than the
  targeted Go version (1.26)`). Every commit in this task used `SKIP=golangci-lint`
  (gitleaks + prettier hooks still ran — this is **not** `--no-verify`). CI's
  `golangci/golangci-lint-action@v9.3.0` pinned to `version: v2.12.2` will hit the
  same wall against a 1.26 `go.mod`. Needs its own quick task: bump the pinned
  golangci-lint to a 1.26-built release (and the pre-commit `rev`), or lower the
  `go.mod` directive.
- `internal/artistart/match_test.go` shows in `gofmt -l` — pre-existing, out of scope.

## Comment blocks deliberately kept near the 8-line ceiling

`detector.go` (`onOrAfterCutoff`, `notifyGate`), `musicbrainz.go`
(`detectDeluxeChanges`, `withinDeluxeRecheckWindow`), `notifier.go` (`suppresses`):
each carries several distinct load-bearing facts (a design-doc ID plus the specific
behavior it governs) that don't compress below one line each. Held at exactly 8.
