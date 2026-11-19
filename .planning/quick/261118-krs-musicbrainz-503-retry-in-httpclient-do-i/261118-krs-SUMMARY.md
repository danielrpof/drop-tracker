---
phase: quick-261118-krs
plan: 01
subsystem: httpclient
tags: [retry, backoff, musicbrainz, deezer, rate-limit, "#17"]
requires: []
provides:
  - "httpclient.Do retries GET/HEAD on 503, 429 and transport errors (Policy, DefaultPolicy)"
  - "Retry-After support, deadline-aware retry skip"
  - "12s searchDeadline on GET /search"
affects: [internal/musicbrainz, internal/deezer, internal/httpserver, internal/poller]
tech-stack:
  added: []
  patterns: ["injectable retry Policy with unexported sleep/jitter test hooks", "exhausted retries return the final response with nil error"]
key-files:
  created: []
  modified:
    - internal/httpclient/httpclient.go
    - internal/httpclient/httpclient_test.go
    - internal/deezer/client.go
    - internal/deezer/search_test.go
    - internal/deezer/albums_test.go
    - internal/musicbrainz/client.go
    - internal/musicbrainz/search_test.go
    - internal/musicbrainz/artist_lookup_test.go
    - internal/musicbrainz/recording_lookup_test.go
    - internal/musicbrainz/releasegroups_test.go
    - internal/musicbrainz/releases_test.go
    - internal/httpserver/search.go
    - internal/httpserver/search_test.go
decisions:
  - "DSC-1: 3x503 returns the final response with a nil error so callers keep 'musicbrainz: search artists: unexpected status 503'"
  - "DSC-8: pre-existing 'no retry' contract tests switched their injected status from 503 to 500 (never retried), exact counts kept"
metrics:
  duration: "~35m"
  completed: 2026-11-18
status: complete
commits: 3
plan_head_before: 9b2f7e3ab66d23941b36fa8995b3bf30c1c52374
plan_head_after: f45f7c3c41903646d5565734b6f5cf452c69f932
actuals:
  tokens: 7500
  tasks: 3
  commits: 3
---

# Phase quick-261118-krs Plan 01: MusicBrainz 503 retry in httpclient.Do Summary

Retrying `httpclient.Do` (3 attempts, jittered 1s/2s backoff, Retry-After capped at 5s, deadline-aware) wired into both the MusicBrainz and Deezer clients, plus a 12s deadline on GET /search (#17 Layer 1).

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 (tracer) | c0dba70 | fix(httpclient): retry transient upstream failures, wired into deezer (#17) |
| 2 | 5544108 | fix(musicbrainz): retry transient 503s via the shared httpclient policy (#17) |
| 3 | f45f7c3 | fix(httpclient): honour Retry-After and bound /search retries with a 12s deadline (#17) |

## What was built

- `Policy` (MaxAttempts, BaseBackoff, MaxRetryAfter, MinAttemptBudget, unexported sleep/jitter hooks) and `DefaultPolicy()` (3, 1s, 5s, 2s). The zero Policy makes one attempt.
- `Do` retries GET/HEAD/empty method on transport errors (including per-attempt timeouts), 503 and 429. Each attempt waits on the shared limiter and gets its own `httpClient.Timeout`; the final body keeps `cancelReadCloser` semantics. Retried bodies are drained (64 KiB cap) and closed. Limiter-wait failures and caller cancellation are never retried.
- Retry delay is `max(jittered backoff, min(Retry-After, cap))`; a retry is skipped (last response/error returned untouched) when the caller deadline is shorter than delay + MinAttemptBudget.
- Both clients store `retry httpclient.Policy`, set to `DefaultPolicy()` in `NewClient` (signatures unchanged). Test helpers shrink only `BaseBackoff` to 1ms.
- `handleSearch` derives `context.WithTimeout(r.Context(), searchDeadline)` (12s) for every source call.

## Deviations from Plan

None in scope. Notes:

- Task 1 tests were added in the same sitting as the implementation rather than observed failing first (RED not separately committed; plan prescribes one commit per task anyway).
- `TestDo_HonoursRetryAfter` http-date case uses a header 5s ahead (accepted range 2s to 5s) instead of the plan's "about 4s"; same property, less flake-prone.
- The empty-method / non-idempotent guard and 64 KiB drain const follow DSC-9 exactly.

## Verification

- `go vet ./...`: clean. `golangci-lint run` (v2.13.2, via GOPATH/bin): 0 issues, also run by the pre-commit hook on each commit (gitleaks passed; prettier skipped, no web files).
- `go test -short -count=1 ./...`: all packages pass.
- `go test -count=1 -timeout 20s ./internal/httpclient/... ./internal/musicbrainz/... ./internal/deezer/...`: pass (about 2s each, no real backoff sleeps).
- Named verify tests: both Deezer and both MusicBrainz `TestSearchArtists_*503*` tests PASS; the 4 new Task 3 httpclient tests and `TestSearch_SourcesRunUnderSearchDeadline` PASS.
- Grep gates: `func DefaultPolicy` in httpclient.go, `httpclient.DefaultPolicy()` in both client.go files, `searchDeadline` in search.go all present.

## Gates that could not run locally

- `make` is not installed in Git Bash, and the Docker daemon is down: `docker info` reports `failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine ... The system cannot find the file specified.` Port 5432 is closed. So `make db-up`, the integration suite (`make test`, with `-race` also unavailable without cgo) and `make coverage-gate` were NOT run: **coverage gate not run locally: Docker unavailable**. The user should run them before pushing.
- Partial substitute: `go test -short` with `-coverpkg` over httpclient/httpserver/deezer/musicbrainz measured 88.1% for those packages combined, and every new httpclient function is at 100% except the `waitError.Error/Unwrap` accessors (0%). This is not the aggregate backend number.
- `make sqlc-check` and prettier do not apply (no SQL or web changes). `-race` was not run; all test counters and recorders use atomics or mutexes for CI's `-race`.

## Known Stubs

None.

## Threat Flags

None. No new endpoints, auth paths or trust boundaries beyond the plan's threat model.

## Self-Check: PASSED

- Files modified all exist (13 files per `git diff --stat 9b2f7e3..HEAD`).
- Commits c0dba70, 5544108, f45f7c3 are on branch danielrpof/gsd-pending-ideas-review; `git rev-list --count 9b2f7e3..HEAD` is 3; no unexpected deletions.
