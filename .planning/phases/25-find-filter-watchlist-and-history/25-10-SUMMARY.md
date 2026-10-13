---
phase: 25-find-filter-watchlist-and-history
plan: 10
subsystem: embedded-spa-and-phase-gate
tags: [go-embed, spa-bundle, definition-of-done, phase-gate]
requires: [25-02, 25-08, 25-09]
provides:
  - "Rebuilt internal/webassets/build/client carrying the Watchlist toolbar, latest-release lines, and History Tag control"
  - "Green Definition of Done evidence for Phase 25"
affects: [internal/webassets/build/client]
tech-stack:
  added: []
  patterns: ["make web recipe run directly (frozen install, build, replace) when make/pnpm are not on PATH"]
key-files:
  created: []
  modified:
    - internal/webassets/build/client
key-decisions:
  - "No gate fix was needed, so Task 2 produced no commit"
status: complete
commits: 1
plan_head_before: 4ec4cc761c7055c0065ac702a4d692040d0a749e
actuals:
  tokens: 0
  tasks: 2
  commits: 1
metrics:
  completed: 2026-10-10
---

# Phase 25 Plan 10: Rebuild Embedded SPA and Phase Gate Summary

Rebuilt the go:embed bundle from Phase 25's `web/` so a Go-only build serves the new UI, then ran the full Definition of Done and the blast-radius check; everything is green.

## Tasks

| Task | Name | Commit |
| ---- | ---- | ------ |
| 1 | Rebuild embedded SPA (tracer) | c38d832 |
| 2 | Definition of Done and blast radius | no commit (no gate fix needed) |

### Task 1: rebuild

- Ran the `make web` recipe directly: `corepack pnpm --dir web install --frozen-lockfile`, `run build`, replace `internal/webassets/build/client`.
- The frozen install left `web/package.json` and `web/pnpm-lock.yaml` untouched.
- Bundle contains `Filter watchlist` and `Latest release:` (watchlist chunk) and `Show all tags` (history chunk).
- `go build ./...` passes. The commit swapped hashed assets (8 old files deleted, new ones added, `index.html` updated); the deletions are the superseded hashed bundle files, intended.

### Task 2: gates

| Gate | Result |
| ---- | ------ |
| `go vet ./...` | pass |
| `golangci-lint run` | 0 issues |
| Integration suite with coverage profile | all packages pass (see Deviations for the `covdata` note) |
| Backend coverage (`cmd/coverage-report --mode=total`) | 90.19% vs 80% floor |
| `sqlc generate` + `git diff --exit-code internal/db/sqlc/` | clean |
| `pnpm typecheck` | pass |
| `prettier --check "**/*.{ts,tsx}"` | pass |
| `pnpm test` | 30 files, 566 tests pass; statements 95.18%, branches 90.57%, functions 95.75%, lines 97.18% (all above 70%) |
| Blast radius vs `9030bfa` | see Deviations (one baseline artifact) |

No migration, `go.mod`/`go.sum`, `web/package.json`, `web/app/root.tsx`, or notifier/detection/discord/musicbrainz/deezer/poller change.

## Deviations from Plan

**1. [Baseline artifact, not a Phase 25 change] `web/pnpm-lock.yaml` differs from `9030bfa`.**
The blast-radius diff against `9030bfa` lists `web/pnpm-lock.yaml`. Its only commit in that range is `7eaa989 feat(24): artist tags and notes (#9)`, a Phase 24 merge that landed after the planning-time head (pnpm overrides plus patch bumps for `@modelcontextprotocol/sdk`, `brace-expansion`, `proxy-addr`, `source-map-js`, `undici`). No Phase 25 commit touched the lockfile, and the frozen install in Task 1 left it unchanged, so T-25-SC holds. The `9030bfa` base for that file was stale; the true baseline is `7eaa989`.

**2. [Environment] `go: no such tool "covdata"`.**
`go test ./... -coverpkg=...` prints this for `internal/testutil` (a package with no tests) on this Windows toolchain and returns non-zero for that package alone. Every package with tests passed, `coverage.out` was produced, and the coverage total (90.19%) was computed from it. CI's Linux job is the authoritative gate. `-race` was not run (cgo disabled); CI is the race gate.

## Out of scope (pre-existing)

`gofmt -l` still lists the five files already on main: `internal/artistart/match_test.go`, `internal/httpserver/server.go`, `internal/httpserver/settings_test.go`, `internal/notifier/timeout_test.go`, `internal/sqlscan/lex.go`. golangci-lint passes; left untouched.

## Known Stubs

None.

## Threat Flags

None.

## Manual UAT carried to /gsd-verify-work (human-check, end-of-phase)

Run the built binary against the dev DB with 50+ watched artists and walk the UI-SPEC UAT list: one request on open; `beyonce` finds Beyoncé; sort-direction stability with `No releases yet` last; reload/Back reproduce the view; toolbar chip x and pressed-toggle Check at 3:1 or better against `#27272a`; unchecking the last mute under `Has muted events` keeps the card until the next filter change; no scroll jump on chip clicks/toggles; adding an artist under `Has muted events` shows it; a selected tag survives the post-add refresh. On History, pick a tag and page with Load more across the retention edge. Plus UAT items deferred by plans 25-01..25-09.

## Self-Check: PASSED

- Commit c38d832 exists; `internal/webassets/build/client` contains the phase strings.
- Measured `commits: 1` from the persisted ledger (`4ec4cc7..HEAD`).
