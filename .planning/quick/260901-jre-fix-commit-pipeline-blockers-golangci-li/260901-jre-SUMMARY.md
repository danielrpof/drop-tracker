---
task: quick-260901-jre
title: Fix commit-pipeline blockers — golangci-lint findings + prettier drift
status: complete
completed: 2026-09-01
subsystem: ci
tags: [ci, golangci-lint, prettier, gosec, staticcheck, phase-14]
key-files:
  modified:
    - internal/authgate/session.go
    - internal/httpserver/server.go
    - web/app/components/auth/PassphraseScreen.test.tsx
    - web/app/components/auth/PassphraseScreen.tsx
    - web/app/lib/api.test.ts
    - web/app/lib/authStore.ts
    - web/app/root.test.tsx
    - web/app/routes/watchlist.test.tsx
decisions:
  - "G115 pair silenced with two line-scoped //nolint:gosec directives on the flagged conversion lines — no .golangci.yml exclusion added."
  - "SA1019 silenced with one line-scoped //nolint:staticcheck on the guarded middleware.RealIP registration — RealIP stays registered, stays behind gate != nil && cfg.trustProxyHeaders (D-14 fail-safe intact)."
  - "Only the six prettier-drifted Phase 14 frontend files reformatted, named explicitly — no whole-tree glob, to avoid a CRLF mass-rewrite."
metrics:
  duration: 20m
  tasks: 3
  commits: 2
actuals:
  tokens: 9000
  tasks: 3
  commits: 2
---

# Quick Task quick-260901-jre: Fix commit-pipeline blockers Summary

Turned the two red CI jobs on `origin/main` green — `lint` (3 golangci-lint findings) and `frontend-test` (prettier `--check`) — with three line-scoped suppression directives carrying rationale and six mechanically reformatted frontend files. No behavior, dependency, or CI-config change.

## What changed

### Task 1 — golangci-lint suppressions (commit `1b31ae4`)

- `internal/authgate/session.go:123-124` — the two `int64(binary.BigEndian.Uint64(...))` conversions in `unmarshalPayload` each gained a trailing `//nolint:gosec` directive naming G115. Rationale: exact two's-complement inverse of `marshalPayload`'s paired `uint64(...UnixNano())` write; both sides 64 bits so the round-trip is total, not narrowing; a forged payload only reaches these lines after the constant-time MAC check and is then rejected by `Verify`'s absolute-cap / expiry checks.
- `internal/httpserver/server.go:133` — the single `r.Use(middleware.RealIP)` line gained a trailing `//nolint:staticcheck` directive naming SA1019. Rationale points at the existing D-14 comment block: registration is gated behind `TRUST_PROXY_HEADERS` (default `false`) and Phase 17 enables it only with the unpublished-container-port topology; removing `RealIP` is not the fix (it is a Phase 17 prerequisite).
- The `if gate != nil && cfg.trustProxyHeaders {` guard is unchanged.
- `gofmt` run over both files; no `.golangci.yml`, `go.mod`, or `go.sum` change.

### Task 2 — prettier reformat (commit `30839fb`)

Six Phase 14 frontend files reformatted with `web/node_modules/.bin/prettier --write` (cwd `web/`), named explicitly:

- `app/components/auth/PassphraseScreen.tsx`
- `app/components/auth/PassphraseScreen.test.tsx`
- `app/lib/api.test.ts`
- `app/lib/authStore.ts`
- `app/root.test.tsx`
- `app/routes/watchlist.test.tsx`

All hunks are 80-column reflow — no identifier, string-literal, assertion, import, or control-flow change. `app/lib/api.ts` was NOT touched (verified clean). Staged diff is content-only; no line-ending-only file (`git diff --cached --numstat` matches `--ignore-cr-at-eol`).

### Task 3 — combined verification (no commit)

Both gates re-run back to back against the combined change set.

## Verification results

| Gate | Before | After |
|------|--------|-------|
| golangci-lint v2.12.2 `run ./...` | 3 issues (2× G115, 1× SA1019) | **0 issues**, exit 0 |
| `go build ./...` / `go vet ./...` | — | pass |
| `internal/authgate` + `internal/httpserver` short tests | — | pass |
| LF-normalized `prettier --check` over all tracked `web/**/*.{ts,tsx}` | 6 unformatted | **0 unformatted** |
| `vitest run` | 125/125 | **125/125**, 12 files |

- Change set is exactly the 8 files in `files_modified` (`git diff --name-only HEAD~2 HEAD`).
- `go.mod`, `go.sum`, `.golangci.yml`, `.github/workflows/full-pipeline.yml`, `.env.example`, `docker-compose.yml`, `web/package.json`, `web/pnpm-lock.yaml` all absent from `git status --porcelain`.
- No staged file's diff is line-ending-only.

## Deviations from Plan

None — plan executed exactly as written. `gofmt` normalization of the two Go files after adding the trailing comments is expected formatter behavior, not a deviation.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/authgate/session.go (modified, committed 1b31ae4)
- FOUND: internal/httpserver/server.go (modified, committed 1b31ae4)
- FOUND: 6 web/ files (modified, committed 30839fb)
- FOUND: commit 1b31ae4
- FOUND: commit 30839fb
- FOUND: .planning/quick/260901-jre-fix-commit-pipeline-blockers-golangci-li/260901-jre-SUMMARY.md
