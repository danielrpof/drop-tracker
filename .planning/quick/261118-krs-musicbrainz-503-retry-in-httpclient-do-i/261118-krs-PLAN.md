---
phase: quick-261118-krs
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
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
autonomous: true
requirements: [GH-17]

estimate:
  tokens: 110000
  raw_tokens: 110000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "A MusicBrainz or Deezer GET answered 503, 503, then 200 returns the 200 result to its caller after exactly 3 upstream hits (R1, R2, R8)"
    - "429s and transport errors are retried; 400, 404, 500 and any request that is not GET/HEAD make exactly one attempt (R1)"
    - "When all 3 attempts return 503 the caller reports its existing component-prefixed error, e.g. 'musicbrainz: search artists: unexpected status 503' (R8, DSC-1)"
    - "Every attempt waits on the shared rate.Limiter and runs under its own httpClient.Timeout; the returned response body stays readable until the caller closes it (R5, R6)"
    - "A cancelled caller ctx never triggers another attempt, and a ctx deadline too short for the next delay plus MinAttemptBudget returns the last response or error immediately (R1, R7)"
    - "A Retry-After header (delta-seconds or HTTP-date) delays the retry by at least that long, capped at 5s by DefaultPolicy (R3)"
    - "GET /search runs every source under a 12s deadline, inside the 15s server WriteTimeout (R7)"
    - "No unit test waits out real backoff: the httpclient, musicbrainz and deezer suites each pass under a 20s go test timeout (R9)"
  artifacts:
    - path: internal/httpclient/httpclient.go
      provides: "Policy type, DefaultPolicy constructor, retrying Do"
      contains: "func DefaultPolicy"
    - path: internal/deezer/client.go
      provides: "Deezer client installs the default retry policy"
      contains: "httpclient.DefaultPolicy()"
    - path: internal/musicbrainz/client.go
      provides: "MusicBrainz client installs the default retry policy"
      contains: "httpclient.DefaultPolicy()"
    - path: internal/httpserver/search.go
      provides: "12s request deadline for GET /search"
      contains: "searchDeadline"
  key_links:
    - from: internal/deezer/client.go
      to: internal/httpclient/httpclient.go
      via: "doRequest passes c.retry as Do's Policy argument"
      pattern: "httpclient.Do\\(ctx, req, c.limiter, c.httpClient, c.retry"
    - from: internal/musicbrainz/client.go
      to: internal/httpclient/httpclient.go
      via: "doRequest passes c.retry as Do's Policy argument"
      pattern: "httpclient.Do\\(ctx, req, c.limiter, c.httpClient, c.retry"
    - from: internal/httpserver/search.go
      to: SearchSource.SearchArtists
      via: "fan-out goroutines receive the ctx derived from r.Context() with searchDeadline"
      pattern: "context.WithTimeout\\(r.Context\\(\\), searchDeadline\\)"
---

<objective>
GitHub issue #17, Layer 1: make internal/httpclient.Do retry transient upstream failures (503, 429, transport errors) for GET/HEAD. Each retry goes back through the shared rate limiter and gets its own timeout. Retries honour Retry-After, never fire after the caller gives up, and skip when the caller's deadline can't fit another attempt. Wire it into both callers (Deezer, MusicBrainz) and give GET /search a 12s deadline so retries always finish inside the 15s server write timeout.

Purpose: spike 001 measured 38% HTTP 503 from MusicBrainz at 1 req/s with the production User-Agent (.planning/spikes/001-paste-confidence-tiers/README.md). Today one 503 fails the search source or loses a poller fetch.
Output: retrying httpclient.Do with an injectable Policy, both clients on DefaultPolicy, /search deadline, and httptest coverage for every locked behaviour.

Out of scope: Layer 2 (adaptive rate) of #17. Do not plan or build it.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.claude/CLAUDE.md
@internal/httpclient/httpclient.go
@internal/httpclient/httpclient_test.go
@internal/httpserver/search.go
@internal/musicbrainz/client.go
@internal/deezer/client.go

## Locked requirements (user, issue #17 Layer 1) — cited as R1..R11

- R1: Retry 503, 429 and transport errors, GET/HEAD only. Never retry once the caller's ctx is cancelled or past its deadline.
- R2: At most 3 attempts in total. Exponential backoff of about 1s, then about 2s, with jitter.
- R3: Honour Retry-After (delta-seconds or HTTP-date) up to a cap.
- R4: Drain and close the failed response body before retrying.
- R5: Every attempt waits on the shared rate.Limiter again.
- R6: Each attempt gets its own httpClient.Timeout. The final returned response keeps today's semantics: the timeout bounds the body read through cancelReadCloser and is released on Close.
- R7: Retry only when the caller ctx has time for the backoff plus another attempt; otherwise return the last error or response immediately. handleSearch gets a deadline of about 12s via context.WithTimeout on r.Context() (server WriteTimeout is 15s, cmd/server/main.go).
- R8: Both callers (internal/musicbrainz/client.go doRequest, internal/deezer/client.go doRequest). Error messages stay prefixed "musicbrainz: ..." / "deezer: ...".
- R9: Backoff and sleep are injectable. No real 1–2s sleeps in unit tests.
- R10: httptest tests: 503,503,200 succeeds; 3x503 outcome (choice justified, DSC-1); Retry-After honoured and capped; no retry on 404/400; no retry after ctx cancel; deadline-aware skip; limiter consulted per attempt.
- R11: Comments 1–3 lines, why not what, one design reference (CLAUDE.md "Comment discipline").

## Planner decisions (Claude's discretion) — cited as DSC-1..DSC-9

- DSC-1 (3x503 outcome): Do returns the final 503 *response* with a nil error, not a new error. Why: Do's contract is already "any HTTP status is a response". Every caller's existing `resp.StatusCode != http.StatusOK` check then produces "musicbrainz: search artists: unexpected status 503". That message carries the component and the operation name, which an error built inside Do can't know. The existing "names 503" caller tests keep passing, and the deadline-skip path (R7) returns the last response the same way.
- DSC-2 (injection seam): Do takes a `Policy` value. Exported fields are MaxAttempts and BaseBackoff (Task 1) plus MaxRetryAfter and MinAttemptBudget (Task 3). Unexported `sleep` and `jitter` hooks are set only by whitebox tests in internal/httpclient; nil means the real timer and ±20% jitter. The zero Policy makes a single attempt. `DefaultPolicy()` is a constructor, not an exported var, so no package can mutate the shared default. Each client stores a `retry httpclient.Policy` field that NewClient fills, and the whitebox `newTestClient` helpers shrink only BaseBackoff to 1ms. Tests therefore keep the production attempt count and prove NewClient installed it.
- DSC-3 (delay): retry n (1-based) waits jitter(BaseBackoff << (n-1)). With DefaultPolicy that's about 1s, then about 2s. When the retried response is 503/429 with a parseable Retry-After, the delay is max(that backoff, min(Retry-After, MaxRetryAfter)). That never retries sooner than the server asked (up to the cap) or sooner than our own backoff. DefaultPolicy caps Retry-After at 5s. A zero MaxRetryAfter ignores Retry-After.
- DSC-4 ("enough time", R7): retry only when the caller ctx has no deadline, or time.Until(deadline) >= delay + MinAttemptBudget. DefaultPolicy MinAttemptBudget is 2s. Requiring the full 10s client timeout instead would cap /search at 2 attempts inside 12s. A 503 normally returns in well under a second, so with 2s a typical 503 storm still gets all 3 attempts in about 5s. The attempt itself stays bounded by min(httpClient.Timeout, caller deadline).
- DSC-5 (per-attempt timeout counts as a transport error): if httpClient.Timeout fires but the caller ctx is still alive, the attempt is retried (R1 "transport errors"). Consequence: the poller passes no deadline, so a fully hung upstream now costs up to 3 x 10s plus about 3s of backoff per call instead of 10s. That's accepted (T-261118krs-06) because MaxAttempts bounds it and shutdown cancellation still aborts the sleep and the attempt.
- DSC-6: rate-limiter wait failures are returned at once and never retried. They mean the time budget failed, not the upstream.
- DSC-7 (task order): Deezer is the tracer path because only one of its tests pins the old no-retry contract. MusicBrainz, the #17 target, follows in Task 2 with its five such tests. Between the two commits MusicBrainz passes its still-zero Policy (one attempt, today's behaviour), so every commit stays green.
- DSC-8: the existing tests that pin "exactly N requests (no retry)" with a 503 switch their injected status to 500, which is never retried, and keep their exact counts. They still prove that caller pagination stops on a non-OK page. 503 retry behaviour gets its own new tests in each package.
- DSC-9: no new dependencies. Jitter uses stdlib math/rand/v2 with a one-line `#nosec G404`, following internal/authgate/login.go. The drain is capped at 64 KiB (named const) so a large error body can't be read in full.

## Interfaces (target shape after Task 3)

- `type Policy struct { MaxAttempts int; BaseBackoff time.Duration; MaxRetryAfter time.Duration; MinAttemptBudget time.Duration; sleep func(context.Context, time.Duration) error; jitter func(time.Duration) time.Duration }`
- `func DefaultPolicy() Policy` returns MaxAttempts 3, BaseBackoff 1s, MaxRetryAfter 5s, MinAttemptBudget 2s.
- `func Do(ctx context.Context, req *http.Request, limiter *rate.Limiter, httpClient *http.Client, policy Policy, component string) (*http.Response, error)`
- `func parseRetryAfter(v string, now time.Time) (time.Duration, bool)` (unexported, Task 3)
- Call sites: `httpclient.Do(ctx, req, c.limiter, c.httpClient, c.retry, "deezer")` and `httpclient.Do(ctx, req, c.limiter, c.httpClient, c.retry, "musicbrainz")`. NewClient signatures stay unchanged, so cmd/server/main.go needs no edit.

## Grounded tooling facts (this checkout, 2026-11-18)

- `golangci-lint` v2.13.2 is at `$(go env GOPATH)/bin/golangci-lint` and is not on the Git Bash PATH. Use that path form or bare `golangci-lint` where it is on PATH. Baseline: 0 issues.
- `make` is not installed in Git Bash. `-race` needs cgo, which is unavailable here (CI runs -race, so every test counter or recorder must be mutex- or atomic-guarded). The Docker daemon was not answering when this plan was written.
- Baseline: `go test -short -count=1 ./...` passes; httpclient about 2s, musicbrainz about 2.3s, deezer about 2s.
- Grep-gate baselines at HEAD are all 0: `func DefaultPolicy` (httpclient.go), `httpclient.DefaultPolicy()` (both client.go), `c.retry` (both client.go), `searchDeadline` (search.go).

## Multi-source coverage audit

| Source | Item | Covered by |
|--------|------|-----------|
| GOAL | #17 Layer 1: retry transient MB/Deezer failures in httpclient.Do | Tasks 1–3 |
| REQ | R1 retry 503/429/transport, GET/HEAD, not after cancel | Task 1 (Task 3 adds the deadline half) |
| REQ | R2 3 attempts, ~1s/~2s jittered backoff | Task 1 |
| REQ | R3 Retry-After seconds/HTTP-date, capped | Task 3 |
| REQ | R4 drain + close before retry | Task 1 |
| REQ | R5 limiter per attempt | Task 1 |
| REQ | R6 per-attempt timeout, cancelReadCloser on final response | Task 1 |
| REQ | R7 deadline-aware skip + /search 12s deadline | Task 3 |
| REQ | R8 both callers, component prefix | Task 1 (Deezer), Task 2 (MusicBrainz) |
| REQ | R9 injectable sleep/backoff, fast tests | Tasks 1–3 (hooks + 20s test timeout gate) |
| REQ | R10 test matrix | Task 1 (status, transport, cancel, limiter, timeout), Task 3 (Retry-After, deadline skip) |
| REQ | R11 short comments | every task |
| RESEARCH | Spike: MB may not send Retry-After | DSC-3: backoff is the floor; Retry-After only lengthens it |
| CONTEXT | Layer 2 adaptive rate | Excluded (out of scope by user) |
</context>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: Retrying httpclient.Do, end to end through the Deezer client (tracer)</name>
  <files>internal/httpclient/httpclient.go, internal/httpclient/httpclient_test.go, internal/deezer/client.go, internal/deezer/search_test.go, internal/deezer/albums_test.go, internal/musicbrainz/client.go</files>
  <read_first>internal/httpclient/httpclient.go, internal/httpclient/httpclient_test.go, internal/deezer/client.go, internal/deezer/search_test.go (newTestClient at about line 83), internal/deezer/albums_test.go (TestArtistAlbums_MidFetchErrorStopsWithNoRetry at about line 452), internal/musicbrainz/client.go, internal/authgate/login.go lines 183-195 (#nosec G404 precedent)</read_first>
  <behavior>
    httpclient_test.go. Use a test Policy with MaxAttempts 3, BaseBackoff 1s, identity jitter, and a sleep hook that records each delay under a mutex and returns immediately:
    - TestDo_RetriesTransientStatus (table): GET 503,503,200 returns 200 with body "ok" readable after Do returns, 3 hits, delays [1s 2s]. GET 429,200 returns 200, 2 hits, delays [1s]. HEAD 503,200 returns 200, 2 hits.
    - TestDo_ExhaustedRetriesReturnFinalResponse: 503 x3 returns resp.StatusCode 503 with a nil error, 3 hits, delays [1s 2s] (DSC-1).
    - TestDo_NoRetryOnPermanentStatus (table 400, 404, 500): 1 hit, no delays, response returned.
    - TestDo_NoRetryForNonIdempotentMethod: POST answered 503 gets 1 hit.
    - TestDo_RetriesTransportErrors: a stub RoundTripper fails the first 2 calls with a sentinel error, then 200: success, 3 calls. Always failing: errors.Is(err, sentinel), message starts "testcomp: do request:", 3 calls, 2 delays.
    - TestDo_DrainsAndClosesRetriedBody: a stub RoundTripper returns a 503 whose tracking body records read-to-EOF and Close, then a 200. The 503 body was fully read and closed before the second RoundTrip ran.
    - TestDo_WaitsOnLimiterEveryAttempt: rate.NewLimiter(rate.Every(time.Hour), 3) with 503,503,200 leaves limiter.Tokens() below 0.5. Control: a fresh limiter of the same shape and a single 200 leaves Tokens() between 1.5 and 2.5.
    - TestDo_EachAttemptGetsItsOwnTimeout: httpClient.Timeout 150ms, first hit blocks (block-channel pattern of the existing timeout test), second answers 200. Do succeeds, which proves the timeout is per attempt, not shared.
    - TestDo_NoRetryAfterCallerCancel: subtest "during request": a stub RoundTripper cancels the caller ctx and returns a synthetic 503. Do returns that response, 1 call, sleep never called. Subtest "during backoff": the sleep hook cancels and returns context.Canceled. Do returns nil resp, err wraps context.Canceled and starts "testcomp: ", 1 hit.
    - TestDefaultJitter_WithinTwentyPercent: 1000 draws of jitter(1s) all land in [800ms, 1200ms]; jitter(0) == 0.
    - TestDefaultSleep: with d = 1h and an already-cancelled ctx it returns context.Canceled promptly; with d = 1ms and a live ctx it returns nil.
    - The four existing tests still pass on the new signature (see action).
    deezer/search_test.go, through the real Client and httptest:
    - TestSearchArtists_RetriesTransient503: 503,503,200 with the search fixture decodes artists, 3 hits.
    - TestSearchArtists_Persistent503NamesComponent: always 503 gives an error with prefix "deezer: " that contains "503", 3 hits.
  </behavior>
  <action>
    Implement R1, R2, R4, R5, R6, R8 (Deezer), R9 and R10 (status/transport/cancel/limiter/timeout rows) using DSC-1, DSC-2, DSC-5, DSC-6, DSC-7, DSC-8 and DSC-9. Write the behavior tests first, see them fail, then implement.

    internal/httpclient/httpclient.go:
    (a) Add the exported Policy type with MaxAttempts and BaseBackoff, plus the unexported sleep and jitter hooks (nil means the defaults). Add DefaultPolicy() returning MaxAttempts 3 and BaseBackoff 1s. The zero Policy makes one attempt (DSC-2).
    (b) Change Do to the Interfaces signature (policy before component). Move today's single-attempt body into an unexported helper with unchanged semantics: a per-attempt context.WithTimeout(ctx, httpClient.Timeout) only when Timeout > 0, limiter.Wait on that attempt ctx, httpClient.Do(req.WithContext(attemptCtx)), cancel on error, cancelReadCloser wrapping on success (R5, R6). Keep the `#nosec G704` annotation directly above the httpClient.Do call, with its justification cut to at most 2 lines: callers build req from package-const base URLs (T-03-07) and Do never alters req.URL. The helper must report a limiter-wait failure distinctly so Do returns it immediately as "<component>: rate limiter wait: ..." without retrying (DSC-6).
    (c) Do runs up to max(policy.MaxAttempts, 1) attempts when req.Method is GET, HEAD or empty (net/http treats empty as GET). Every other method gets one attempt. After an attempt, retry only if attempts remain, the ctx Do received (the caller's, not the attempt ctx) has a nil Err(), and the outcome was either an httpClient.Do error (including a per-attempt timeout, DSC-5) or status 503 or 429. Otherwise return that attempt's resp and err untouched. An exhausted 503 goes back as a response with a nil error (DSC-1).
    (d) Before retrying a response, copy at most a 64 KiB named-const limit of its body to io.Discard, then Close it. Draining lets the transport reuse the keep-alive connection, and Close releases that attempt's timeout (R4).
    (e) Retry n waits policy jitter(BaseBackoff << (n-1)) through the policy sleep. Default sleep: a time.NewTimer raced against ctx.Done(), returning ctx.Err() on cancel, with the timer stopped. Default jitter: uniform within ±20% via math/rand/v2 Int64N; d <= 0 is returned unchanged, which avoids Int64N(0); one-line `#nosec G404` (pacing, not security). If sleep returns an error, return nil and fmt.Errorf("%s: retry backoff: %w", component, err).
    (f) Replace the 7-line package comment with at most 3 lines naming this as the shared rate-limited and retrying request seam (#17). Shorten Do's doc to at most 4 lines: which outcomes are retried, and that exhausted retries return the final response so callers' status checks keep their messages. Every comment is 1–3 lines, why not what (R11).

    internal/httpclient/httpclient_test.go: move the four existing tests to the new signature. TestDo_Success, TestDo_LimiterWaitErrorOnCancelledContext and TestDo_CloseCancelsTheDerivedContext pass Policy{}. TestDo_TimeoutReturnsWrappedDeadlineExceeded passes the recording test policy and keeps every assertion: 3 x 200ms stays under its 2s bound, and the error still wraps context.DeadlineExceeded. Add the behavior tests. Hit counters and recorders use sync/atomic or a mutex, because CI runs -race.

    internal/deezer/client.go: add a `retry httpclient.Policy` field. NewClient sets it to httpclient.DefaultPolicy() and doRequest passes c.retry. Trim doRequest's comment to at most 3 lines.
    internal/deezer/search_test.go: in newTestClient, after NewClient, set only c.retry.BaseBackoff to time.Millisecond so tests keep the production attempt count. Add the two Deezer behavior tests.
    internal/deezer/albums_test.go: in TestArtistAlbums_MidFetchErrorStopsWithNoRetry, change the n == 2 status to http.StatusInternalServerError and the expected substring to "500". Keep the exact count of 2 (DSC-8).
    internal/musicbrainz/client.go: add the same `retry httpclient.Policy` field and pass c.retry in doRequest. NewClient doesn't set it in this task: the zero Policy makes one attempt, which is MusicBrainz's current behaviour, so its suite stays green at this commit. Task 2 switches it on (DSC-7).

    Commit as `fix(httpclient): retry transient upstream failures, wired into deezer (#17)`. No AI trailers, never --no-verify (CLAUDE.md).
  </action>
  <verify>
    <automated>go test -count=1 -v -run 'TestSearchArtists_RetriesTransient503|TestSearchArtists_Persistent503NamesComponent' ./internal/deezer/ | grep -c -- '--- PASS: TestSearchArtists_' | grep -qx 2 && go build ./... && go vet ./... && "$(go env GOPATH)/bin/golangci-lint" run && go test -count=1 -timeout 20s ./internal/httpclient/... ./internal/deezer/... ./internal/musicbrainz/...</automated>
  </verify>
  <done>
    - Both Deezer end-to-end retry tests report PASS (the grep proves they exist and ran), and every listed httpclient test exists and passes.
    - `grep -v '^\s*//' internal/httpclient/httpclient.go | grep -c 'func DefaultPolicy'` is 1 (baseline 0). `grep -v '^\s*//' internal/deezer/client.go | grep -cF 'httpclient.DefaultPolicy()'` is 1 (baseline 0). `grep -v '^\s*//' internal/musicbrainz/client.go | grep -cF 'c.retry'` is at least 1 (baseline 0).
    - go build, go vet and golangci-lint are clean. The httpclient, deezer and musicbrainz suites pass under a 20s timeout.
    - One commit, conventional message, no AI attribution.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: MusicBrainz retries 503s through DefaultPolicy</name>
  <files>internal/musicbrainz/client.go, internal/musicbrainz/search_test.go, internal/musicbrainz/artist_lookup_test.go, internal/musicbrainz/recording_lookup_test.go, internal/musicbrainz/releasegroups_test.go, internal/musicbrainz/releases_test.go</files>
  <read_first>internal/musicbrainz/client.go, internal/musicbrainz/search_test.go (newTestClient at about line 58, permanent-503 test at about line 240, timeout test at about line 322), internal/musicbrainz/artist_lookup_test.go (TestLookupArtist_NonOKStatus), internal/musicbrainz/recording_lookup_test.go (TestReleasesForRecording_NonOKStatus), internal/musicbrainz/releasegroups_test.go (TestReleaseGroupsByArtist_UpstreamErrorStatusNoRetry, TestReleaseGroupsByArtist_MidFetchErrorStopsWithNoRetry), internal/musicbrainz/releases_test.go (TestReleasesByReleaseGroup_NonOKStatus)</read_first>
  <behavior>
    - TestSearchArtists_RetriesTransient503 (search_test.go): 503,503,200 with the search fixture decodes artists, 3 hits, and the User-Agent header is present on every hit.
    - TestSearchArtists_Persistent503NamesComponent: always 503 gives an error with prefix "musicbrainz: " that contains "503", 3 hits.
    - The five rewritten contract tests inject 500 and keep their exact request counts (1, or 2 for mid-fetch), with the error naming "500".
    - The existing permanent-503 tests that only check the "503" substring (search, recordings, plus deezer artist) keep passing unchanged. The existing timeout test still finishes under its 2s bound (3 x 200ms).
  </behavior>
  <action>
    Implement R8 (MusicBrainz half), R9 and R10 for the #17 target, using DSC-2 and DSC-8.
    internal/musicbrainz/client.go: NewClient sets retry to httpclient.DefaultPolicy(). Trim doRequest's comment to at most 3 lines (R11).
    internal/musicbrainz/search_test.go: in newTestClient, after NewClient, set only c.retry.BaseBackoff to time.Millisecond, the same as the Deezer helper. Add the two behavior tests, using a mutex-guarded hit counter.
    Contract tests (DSC-8): in TestLookupArtist_NonOKStatus, TestReleasesForRecording_NonOKStatus, TestReleaseGroupsByArtist_UpstreamErrorStatusNoRetry, TestReleaseGroupsByArtist_MidFetchErrorStopsWithNoRetry and TestReleasesByReleaseGroup_NonOKStatus, change the injected http.StatusServiceUnavailable to http.StatusInternalServerError and the expected substring to "500". Keep every exact-count assertion. Make no other edits to those tests.
    Commit as `fix(musicbrainz): retry transient 503s via the shared httpclient policy (#17)`. No AI trailers, never --no-verify.
  </action>
  <verify>
    <automated>go test -count=1 -v -run 'TestSearchArtists_RetriesTransient503|TestSearchArtists_Persistent503NamesComponent' ./internal/musicbrainz/ | grep -c -- '--- PASS: TestSearchArtists_' | grep -qx 2 && go vet ./... && "$(go env GOPATH)/bin/golangci-lint" run && go test -count=1 -timeout 20s ./internal/musicbrainz/... ./internal/deezer/... ./internal/httpclient/...</automated>
  </verify>
  <done>
    - `grep -v '^\s*//' internal/musicbrainz/client.go | grep -cF 'httpclient.DefaultPolicy()'` is 1 (baseline 0).
    - Both MusicBrainz end-to-end retry tests report PASS. The five contract tests pass with 500 and unchanged counts.
    - go vet and golangci-lint are clean. All three client-side suites pass under a 20s timeout.
    - One commit, conventional message, no AI attribution.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Retry-After, deadline-aware skip, and a 12s /search deadline, then the full Definition of Done</name>
  <files>internal/httpclient/httpclient.go, internal/httpclient/httpclient_test.go, internal/httpserver/search.go, internal/httpserver/search_test.go</files>
  <read_first>internal/httpclient/httpclient.go (as left by Task 1), internal/httpserver/search.go, internal/httpserver/search_test.go (stubSearchSource at about line 62, TestSearch_Success harness at about line 89), cmd/server/main.go lines 60-70 (writeTimeout 15s), Makefile lines 9, 38, 75-112 (test-integration and coverage-gate recipes)</read_first>
  <behavior>
    httpclient_test.go (recording test policy plus MaxRetryAfter 5s and MinAttemptBudget 2s unless a case says otherwise):
    - TestParseRetryAfter (table, fixed now): "3" gives 3s, ok. "0" gives 0, ok. "-1" is not ok. "soon" is not ok. "" is not ok. An HTTP-date 4s after now gives 4s, ok. An HTTP-date in the past gives 0, ok.
    - TestDo_HonoursRetryAfter (table, 503 with header, then 200, success, 2 hits): "Retry-After: 3" records 3s. "Retry-After: 120" records 5s (the cap). An HTTP-date about 4s ahead records between 2s and 4s. "soon" records 1s (backoff fallback). A 0 MaxRetryAfter with "Retry-After: 3" records 1s (header ignored).
    - TestDo_SkipsRetryWhenDeadlineTooShort: caller ctx with a 500ms deadline and BaseBackoff 1s. Subtest status: always 503 with body "busy" returns the 503 with its body still readable as "busy" (not drained), 1 hit, no recorded delay. Subtest transport: a stub RoundTripper error gives errors.Is on the sentinel, 1 call, no delay.
    - TestDo_RetriesWhenDeadlineAllows: caller ctx with a 10s deadline, 503,200 succeeds with 2 hits (the skip isn't over-eager).
    search_test.go:
    - TestSearch_SourcesRunUnderSearchDeadline: a stubSearchSource sends time.Until(deadline) and ok from ctx.Deadline() over a buffered channel. The deadline is present, and remaining time is above 11s and at most 12s.
    - All existing TestSearch_* tests still pass, including TestSearch_CancelledInboundRequestWritesNoPartialBody.
  </behavior>
  <action>
    Implement R3, R7 and R10 (Retry-After and deadline rows) using DSC-3 and DSC-4, then run the full Definition of Done.

    internal/httpclient/httpclient.go:
    (a) Add the MaxRetryAfter and MinAttemptBudget fields to Policy. DefaultPolicy sets 5s and 2s. Zero values are safe: MaxRetryAfter 0 ignores Retry-After, and MinAttemptBudget 0 skips only when the delay alone exceeds the remaining time.
    (b) Add parseRetryAfter(v, now): a non-negative integer is delta-seconds. Otherwise try http.ParseTime, where a past date gives 0. Anything else is not ok.
    (c) Compute the delay for retry n as in DSC-3: the jittered backoff, raised to the capped Retry-After when the retried response is 503 or 429 and carries a parseable header. Use time.Now() for the HTTP-date case.
    (d) Deadline-aware skip (R7, DSC-4): after deciding an outcome is retryable and computing its delay, but before draining or sleeping, check the caller ctx. If it has a deadline and time.Until(deadline) is less than delay + MinAttemptBudget, return the last resp (undrained, body intact) or last error untouched. The order is: classify, compute delay, deadline check, drain and close, sleep, next attempt.
    (e) Keep added comments to 1–3 lines (R11).

    internal/httpserver/search.go: add `const searchDeadline = 12 * time.Second` with a one-line why comment (keeps upstream retries inside the 15s server WriteTimeout, #17). In handleSearch, after q validation, derive ctx and cancel with context.WithTimeout(r.Context(), searchDeadline), defer cancel(), and pass that ctx to every src.SearchArtists call. Leave httplog.SetAttrs on r.Context(), because httplog keys its state off the request ctx. Change only the phrase in handleSearch's doc comment that says it fans out "using r.Context()", so it describes the derived, deadline-bound ctx.
    internal/httpclient/httpclient_test.go and internal/httpserver/search_test.go: add the behavior tests.

    Then run the Definition of Done for Go-only changes (CLAUDE.md). This change touches no SQL or web files, so skip sqlc-check and prettier.
    1. go vet ./...
    2. golangci-lint run (path form below)
    3. go test of the four touched packages, then go test -short -count=1 ./...
    4. Coverage gate. If `make` is on PATH and Docker is running: make db-up, make test, make coverage-gate. If `make` is missing but `docker info` succeeds: docker compose up -d --wait postgres. Then run the test-integration recipe from Makefile lines 75-77 with TEST_DATABASE_URL from Makefile line 9 and -coverpkg built from the COVER_PKGS expression on line 38; drop -race if cgo is unavailable, since coverage doesn't change. Then go run ./cmd/coverage-report --mode=total --profile=coverage.out and confirm the result is at least 80. If the Docker daemon is unavailable, write "coverage gate not run locally: Docker unavailable" in the SUMMARY so the user can run it before pushing.
    Commit as `fix(httpclient): honour Retry-After and bound /search retries with a 12s deadline (#17)`. No AI trailers, never --no-verify.
  </action>
  <verify>
    <automated>go test -count=1 -v -run 'TestSearch_SourcesRunUnderSearchDeadline' ./internal/httpserver/ | grep -q -- '--- PASS: TestSearch_SourcesRunUnderSearchDeadline' && go test -count=1 -v -run 'TestParseRetryAfter|TestDo_HonoursRetryAfter|TestDo_SkipsRetryWhenDeadlineTooShort|TestDo_RetriesWhenDeadlineAllows' ./internal/httpclient/ | grep -c -- '^--- PASS: Test' | grep -qx 4 && go vet ./... && "$(go env GOPATH)/bin/golangci-lint" run && go test -count=1 ./internal/httpclient/... ./internal/httpserver/... ./internal/musicbrainz/... ./internal/deezer/... && go test -count=1 -timeout 20s ./internal/httpclient/... ./internal/musicbrainz/... ./internal/deezer/... && go test -short -count=1 ./...</automated>
  </verify>
  <done>
    - `grep -v '^\s*//' internal/httpserver/search.go | grep -c 'searchDeadline'` is at least 2 (declaration and use; baseline 0). `grep -v '^\s*//' internal/httpserver/search.go | grep -c 'context.WithTimeout(r.Context(), searchDeadline)'` is 1.
    - All four new httpclient tests and the search deadline test report PASS. Every existing test passes.
    - go vet, golangci-lint and go test -short ./... are clean. The coverage gate shows at least 80%, or the SUMMARY records why it couldn't run locally.
    - One commit, conventional message, no AI attribution.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| drop-tracker → MusicBrainz/Deezer | Outbound GETs. Upstream status codes, headers (Retry-After) and bodies are untrusted input that now drives retry timing |
| browser → GET /search | Untrusted q. Request lifetime is bounded by the server write timeout |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-261118krs-01 | Denial of Service | httpclient.Do retry loop to upstream | high | mitigate | Every attempt waits on the shared limiter (R5, TestDo_WaitsOnLimiterEveryAttempt), so retries can't raise the request rate above the configured budget. Hard cap of 3 attempts with jittered exponential backoff avoids synchronized hammering during an MB 503 storm |
| T-261118krs-02 | Denial of Service | Retry-After parsing | medium | mitigate | A hostile or buggy Retry-After (e.g. 86400 or a far-future date) is clamped to MaxRetryAfter = 5s (TestDo_HonoursRetryAfter cap row). The deadline-aware skip returns at once when even the capped delay doesn't fit |
| T-261118krs-03 | Denial of Service | GET /search | medium | mitigate | 12s searchDeadline on a ctx derived from r.Context() bounds limiter waits, attempts and backoff below the 15s WriteTimeout (TestSearch_SourcesRunUnderSearchDeadline). The inbound cancel still propagates |
| T-261118krs-04 | Tampering | Non-idempotent requests | medium | mitigate | Only GET/HEAD (or empty method) are retried; any other method makes one attempt (TestDo_NoRetryForNonIdempotentMethod), so a retry can never duplicate a side effect |
| T-261118krs-05 | Denial of Service | Retried response bodies / connections | low | mitigate | Failed bodies are drained through a 64 KiB LimitReader and closed before the next attempt (TestDo_DrainsAndClosesRetriedBody). No unbounded read, and the keep-alive connection plus the per-attempt cancel are released |
| T-261118krs-06 | Denial of Service | Poller against a fully hung upstream | low | accept | DSC-5: per-attempt timeouts are retried, so a hung MB costs up to about 33s per call instead of 10s in the deadline-free poll loop. Bounded by MaxAttempts, and shutdown cancellation aborts both sleep and attempt (TestDefaultSleep, TestDo_NoRetryAfterCallerCancel) |
| T-261118krs-07 | Information Disclosure | /search error body | low | mitigate | Unchanged T-03-01 invariant: the response body still carries only "source unavailable", and retry-related error text goes only to httplog attrs |
| T-261118krs-08 | Spoofing/SSRF | httpclient.Do request target | low | accept | Retries reuse the caller-built req and never change req.URL. Base URLs stay package consts (T-03-07), and the #nosec G704 justification is kept |
| T-261118krs-SC | Tampering | dependency installs | low | accept | No package installs. Jitter uses stdlib math/rand/v2, and golang.org/x/time v0.15.0 is already in go.mod |
</threat_model>

<verification>
- `go vet ./...` clean
- `"$(go env GOPATH)/bin/golangci-lint" run` reports 0 issues (bare `golangci-lint run` where it is on PATH)
- `go test -count=1 ./internal/httpclient/... ./internal/httpserver/... ./internal/musicbrainz/... ./internal/deezer/...` passes
- `go test -count=1 -timeout 20s ./internal/httpclient/... ./internal/musicbrainz/... ./internal/deezer/...` passes (no real backoff sleeps, R9)
- `go test -short -count=1 ./...` passes
- `make db-up && make test && make coverage-gate` (or the Task 3 fallback) shows backend coverage of at least 80%, or the SUMMARY records that Docker was unavailable
- No sqlc or web changes, so sqlc-check and prettier don't apply
</verification>

<success_criteria>
- A MusicBrainz or Deezer GET survives two consecutive 503s (or 429s, or transport errors) and returns the eventual 200 to its caller.
- After 3 failed attempts, callers keep their existing component-prefixed "unexpected status 503" errors.
- Retries consume the shared limiter, get a fresh per-attempt timeout, honour Retry-After up to 5s, stop on caller cancel, and skip when the deadline can't fit another attempt.
- GET /search runs under a 12s deadline.
- Every new comment is 1–3 lines. Three conventional commits, no AI attribution, no hook bypass.
</success_criteria>

<output>
Create `.planning/quick/261118-krs-musicbrainz-503-retry-in-httpclient-do-i/261118-krs-SUMMARY.md` when done
</output>
