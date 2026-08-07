---
phase: 03-external-clients-search
plan: 01
subsystem: api
tags: [musicbrainz, net-http, rate-limiting, chi, golang.org/x/time]

# Dependency graph
requires:
  - phase: 02-watchlist-core
    provides: httpserver.Pinger / watchlist.Store narrow-interface seam pattern and the writeError/httplog.SetAttrs no-leak convention this plan's search handler reuses
provides:
  - "internal/musicbrainz: rate-limited, User-Agent-identified MusicBrainz ws/2 client with SearchArtists and the ArtistSearcher seam"
  - "GET /search combined-source proxy endpoint on the chi router, with a source-keyed, D-03 partial-failure-tolerant response envelope"
  - "httpserver.New widened to a fourth sources []SearchSource parameter, ready for plan 03-02's Deezer source"
  - "exactly one process-wide *musicbrainz.Client and rate.Limiter constructed in cmd/server/main.go, ready for plan 03-04's poller to reuse"
affects: [03-02-deezer-search, 03-03-musicbrainz-poll-client, 03-04-scheduler]

# Actuals (#2632)
actuals:
  tokens: 14394
  tasks: 2
  commits: 6

# Tech tracking
tech-stack:
  added: [golang.org/x/time v0.15.0]
  patterns:
    - "doRequest single-request-path helper: every outbound call in a client package goes through one method that waits on the rate limiter and sets identifying headers, so no call site can bypass either"
    - "source-keyed response envelope with per-source status/error/artists, so a second source is a map key addition, never a response reshape"
    - "cancelReadCloser: a context.CancelFunc from a per-request context.WithTimeout is attached to the response body's Close(), not deferred at return, so the timeout keeps bounding a caller's body read without truncating a healthy response"

key-files:
  created:
    - internal/musicbrainz/client.go
    - internal/musicbrainz/search.go
    - internal/musicbrainz/search_test.go
    - internal/httpserver/search.go
    - internal/httpserver/search_test.go
  modified:
    - internal/httpserver/server.go
    - internal/httpserver/health_test.go
    - internal/httpserver/server_test.go
    - internal/httpserver/watchlist_test.go
    - internal/httpserver/boot_e2e_test.go
    - cmd/server/main.go
    - go.mod
    - go.sum

key-decisions:
  - "doRequest wraps ctx in context.WithTimeout(ctx, httpClient.Timeout) only when Timeout is positive -- net/http treats a zero Timeout as 'unbounded', and feeding that straight into context.WithTimeout would create an already-expired deadline and fail every request immediately"
  - "The WithTimeout cancel func is attached to the response body via a cancelReadCloser wrapper instead of being deferred inside doRequest, so the deadline keeps bounding the caller's body read after doRequest returns rather than cancelling a healthy in-flight body read the instant doRequest returns"
  - "WLST-01 and CLNT-03 are NOT marked complete in REQUIREMENTS.md by this plan despite appearing in its frontmatter requirements list -- both requirement descriptions explicitly name 'MusicBrainz and Deezer', and this plan delivers MusicBrainz only; plan 03-02 (which also lists both IDs) is left to mark them complete once Deezer lands"

patterns-established:
  - "Pattern: hand-rolled external API client package (internal/musicbrainz) with baseURL/userAgent/httpClient/limiter fields, an unexported doRequest choke point, and a narrow *Searcher interface consumers depend on -- internal/deezer (plan 03-02) mirrors this shape exactly"
  - "Pattern: httpserver.SearchSource consumer-side seam plus a NewXSource(...) adapter defined in internal/httpserver, so a new external-client package never has to import internal/httpserver"

requirements-completed: []

coverage:
  - id: D1
    description: "GET /search?q=<query> returns HTTP 200 with a source-tagged MusicBrainz artist list (MBID, name, disambiguation, type) under sources.musicbrainz"
    requirement: "WLST-01"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_Success"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/search_test.go#TestSearchArtists_DecodesFixture"
        status: pass
    human_judgment: true
    rationale: "The plan's human-check step (curl against the running binary and real musicbrainz.org, confirming real Drake results) could not be completed in this sandbox -- outbound TLS to musicbrainz.org fails at the TLS handshake from this environment (schannel handshake failure), independent of the application code. Unit tests against a live-verified fixture and an end-to-end run against real Postgres both pass; only the live-network leg needs a human/CI environment with outbound HTTPS egress."
  - id: D2
    description: "A source that fails returns HTTP 200 with a per-source status:error and empty artists list, never a 5xx, and never leaks raw upstream error text or musicbrainz.org into the response body"
    requirement: "CLNT-03"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_SourceErrorReturns200WithErrorStatus"
        status: pass
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_PartialFailure_OneSourceDownAnotherHealthy"
        status: pass
      - kind: integration
        ref: "manual run: local binary against real Postgres, GET /search?q=drake with the real musicbrainz.Client -- observed 200 with status:error after this sandbox's outbound TLS to musicbrainz.org failed, no leaked error text in the body (see musicbrainz_search_error log field instead)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every outbound MusicBrainz request is rate-limited and carries the configured User-Agent; a hung upstream fails within the client timeout instead of blocking indefinitely; a cancelled inbound /search request never emits a partial JSON body"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/search_test.go#TestSearchArtists_RateLimiterPacesRequests"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/search_test.go#TestSearchArtists_TimeoutReturnsWrappedDeadlineExceeded"
        status: pass
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_CancelledInboundRequestWritesNoPartialBody"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-08-07
status: complete
---

# Phase 3 Plan 1: MusicBrainz Client and GET /search Proxy Summary

**Rate-limited MusicBrainz ws/2 client (`internal/musicbrainz`) wired through a new source-keyed `GET /search` proxy on the chi router, with partial-failure tolerance and a per-request timeout that also bounds the shared rate limiter.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-08-07T16:38:00-05:00 (approx, file discovery)
- **Completed:** 2026-08-07T16:57:32-05:00
- **Tasks:** 2 completed
- **Files modified:** 13

## Accomplishments
- `internal/musicbrainz`: a hand-rolled, rate-limited `net/http` client for MusicBrainz's `/ws/2/artist` search endpoint, with a single `doRequest` choke point that unconditionally waits on the rate limiter and sets `User-Agent`/`Accept` -- no call site can bypass either.
- `GET /search?q=...` combined-source proxy endpoint: fans out to every configured `SearchSource` concurrently, joins before writing, and always responds 200 with a per-source `status`/`error`/`artists` entry -- a failing source never fails the whole request and never leaks raw upstream text into the body.
- `httpserver.New` widened to a fourth `sources []SearchSource` parameter (all 36 existing call sites updated); `cmd/server/main.go` now constructs exactly one `*musicbrainz.Client` and one `rate.Limiter` for the whole process, ready for plan 03-04's poller to reuse the same instance.
- Safety envelope proven: a hung MusicBrainz upstream fails within the client's configured timeout (wrapped as `context.DeadlineExceeded`) instead of blocking indefinitely; the same timeout also bounds the rate limiter's wait via a `cancelReadCloser` that releases the deadline only once the caller closes the response body, so a healthy response is never truncated.
- Manually ran the built binary end-to-end against a real Postgres instance (`docker compose up -d postgres`) and confirmed `GET /search?q=drake` responds 200 with the D-03 degraded-source shape when this sandbox's outbound TLS to `musicbrainz.org` fails -- see Issues Encountered.

## Task Commits

Each task was committed atomically, with a RED test commit ahead of its GREEN implementation commit per `tdd="true"`:

1. **Task 1: End-to-end "search MusicBrainz for an artist"**
   - `4a3cd91` test(03-01): add failing tests for musicbrainz artist search client
   - `5388b45` feat(03-01): add rate-limited musicbrainz artist search client
   - `367f850` test(03-01): add failing tests for GET /search proxy handler
   - `2f7b010` feat(03-01): add GET /search proxy and wire the musicbrainz client
2. **Task 2: Prove the search path's safety envelope**
   - `e882cc9` test(03-01): add safety-envelope tests for timeout, cancellation, and partial failure
   - `086cb60` feat(03-01): bound doRequest's rate-limiter wait by the client timeout

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/musicbrainz/client.go` - `Client`, `NewClient`, `ArtistSearcher` seam, `doRequest` (rate limit + User-Agent + timeout bounding), `cancelReadCloser`, `clampLimit`
- `internal/musicbrainz/search.go` - `Artist`, `SearchArtists` against `/ws/2/artist`, `ErrEmptyQuery`
- `internal/musicbrainz/search_test.go` - httptest.Server-backed fixtures covering decode shape, headers, query params, empty/ordering, upstream error, rate pacing, cancellation, timeout
- `internal/httpserver/search.go` - `SearchArtist` DTO, `SearchSource` seam, `NewMusicBrainzSource` adapter, `handleSearch` (concurrent fan-out, D-03 partial results)
- `internal/httpserver/search_test.go` - stub-source-backed tests covering success, per-source error, q validation, empty results, concurrent fan-out, partial failure, cancellation
- `internal/httpserver/server.go` - `Server.sources` field, `New`'s fourth parameter, `/search` route registration
- `internal/httpserver/health_test.go`, `server_test.go`, `watchlist_test.go`, `boot_e2e_test.go` - updated all `httpserver.New(...)` call sites for the new signature (`nil` sources where `/search` isn't exercised)
- `cmd/server/main.go` - constructs the single process-wide `musicbrainz.Client`/`rate.Limiter` and wires `NewMusicBrainzSource` into `httpserver.New`
- `go.mod`, `go.sum` - `golang.org/x/time v0.15.0` as a direct dependency

## Decisions Made
- `doRequest` only wraps `ctx` in `context.WithTimeout` when `httpClient.Timeout > 0` -- a zero Timeout means "unbounded" in net/http's own convention, and blindly feeding it to `context.WithTimeout` would create an already-expired deadline, discovered when the full test suite failed with "context deadline exceeded" against `httptest.Server`'s default zero-Timeout client.
- The `context.CancelFunc` from that `WithTimeout` is attached to the response body via a small `cancelReadCloser` wrapper rather than deferred inside `doRequest`, so the timeout keeps bounding the caller's body read (matching http.Client.Timeout's own semantics) without cancelling a perfectly healthy in-flight body read the instant `doRequest` returns.
- WLST-01 and CLNT-03 are left unmarked in REQUIREMENTS.md by this plan (see `key-decisions` in frontmatter) since their descriptions require both MusicBrainz and Deezer; plan 03-02 also lists both IDs and is the natural place to close them once Deezer search lands.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Zero-Timeout http.Client caused every request to fail with "context deadline exceeded"**
- **Found during:** Task 2, running `go test -short ./...` after adding the `context.WithTimeout` wrap to `doRequest`
- **Issue:** `context.WithTimeout(ctx, 0)` creates an already-expired context. Every existing musicbrainz test builds its client with `ts.Client()`, whose `Timeout` field defaults to `0` ("no timeout" in net/http's convention) -- wrapping that literally broke every previously-passing test in the package.
- **Fix:** `doRequest` now only applies `context.WithTimeout` when `c.httpClient.Timeout > 0`, leaving `ctx` unwrapped (using the caller's own deadline, if any) when the client has no configured timeout.
- **Files modified:** `internal/musicbrainz/client.go`
- **Verification:** `go test -short ./... -count=1` and `TEST_DATABASE_URL=... go test ./... -count=1` both green after the fix.
- **Committed in:** `086cb60` (task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary correctness fix for the exact "cancellation/timeout" behavior task 2 set out to prove; no scope creep -- caught by the plan's own required full-suite verification step before committing.

## Issues Encountered
- **Sandbox has no outbound TLS egress to musicbrainz.org.** Attempting the plan's human-check (`curl` against the running binary hitting real MusicBrainz) surfaced a `schannel: failed to receive handshake` TLS failure when this environment's `curl` tried to reach `musicbrainz.org:443` directly -- confirmed independent of the application (a raw `curl -v https://musicbrainz.org/...` from this shell fails identically). The application-level behavior this exercised is still valuable: `GET /search?q=drake` against the real, running binary correctly returned HTTP 200 with `sources.musicbrainz.status: "error"` and no leaked upstream text, with the real error (`Get "https://musicbrainz.org/...": EOF`) landing only in the structured log line via `musicbrainz_search_error` -- exactly the D-03/T-03-01 contract this plan targets. The "happy path with real Drake results" leg of the human-check could not be completed here and is deferred to a network-unrestricted environment (CI or local dev machine) -- see `coverage: D1`'s `human_judgment: true` rationale.

## User Setup Required

None - no external service configuration required. (MusicBrainz's `/ws/2/artist` search endpoint needs no API key.)

## Next Phase Readiness
- `internal/httpserver.SearchSource` and the `sources` map envelope are ready for plan 03-02 to add `internal/deezer` and `NewDeezerSource` as a second key with zero response-shape change.
- The single `*musicbrainz.Client`/`rate.Limiter` pair constructed in `cmd/server/main.go` is ready for plan 03-04's poller to reuse directly, per D-07.
- Full test suite (`go test -short ./...` and `TEST_DATABASE_URL=... go test ./... -count=1`) is green; `go build ./...` and `go vet ./...` are clean.
- Outstanding: confirm the real-network happy path (`curl 'http://localhost:PORT/search?q=drake'` returning real MusicBrainz artists) in an environment with outbound HTTPS access, since this sandbox cannot complete a TLS handshake to musicbrainz.org.

---
*Phase: 03-external-clients-search*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 9 claimed files (`internal/musicbrainz/client.go`, `internal/musicbrainz/search.go`, `internal/musicbrainz/search_test.go`, `internal/httpserver/search.go`, `internal/httpserver/search_test.go`, `internal/httpserver/server.go`, `cmd/server/main.go`, `go.mod`, `go.sum`) exist on disk. All 6 claimed commit hashes (`4a3cd91`, `5388b45`, `367f850`, `2f7b010`, `e882cc9`, `086cb60`) resolve in `git log --oneline --all`.
