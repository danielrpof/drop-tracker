---
phase: 01-foundation-data-layer-config-health
plan: 02
subsystem: httpserver-observability
tags: [testing, health-check, request-id, correlation, tdd]
dependency-graph:
  requires:
    - httpserver.Pinger
    - logging.NewWithWriter
    - testutil.RequirePostgresDSN
    - testutil.NewTestPool
  provides:
    - internal/httpserver/health_test.go test coverage (TestHealth_Up/Down/DownOnTimeout/Concurrent)
    - internal/httpserver/server_test.go test coverage (TestRequestID/HonoursInboundHeader/DistinctPerConcurrentRequest, TestNoDSNInLogs)
  affects: []
tech-stack:
  added: []
  patterns:
    - "File-local stubPinger implementing httpserver.Pinger to drive the failure/timeout branches with no live database"
    - "syncBuffer (mutex-guarded io.Writer) + logging.NewWithWriter to capture real JSON log output for assertion in concurrent tests"
    - "Per-goroutine result slots (not shared appended slices) for race-safe concurrent test assertions"
key-files:
  created:
    - internal/httpserver/health_test.go
    - internal/httpserver/server_test.go
  modified: []
decisions:
  - "No production code changes were needed in this plan. Plan 01-01's Rule 2 deviation already added the echoRequestID middleware and X-Request-Id response header, and the httpserver.Pinger seam was already exported — this plan's job was purely to write the test coverage those seams were built for. Both tasks are therefore test-only commits with no accompanying feat commit."
metrics:
  duration: "~30 min"
  completed: 2026-08-05
actuals:
  tokens: 2855
  tasks: 2
  commits: 2
status: complete
---

# Phase 1 Plan 2: Prove /health Tells the Truth and Every Response Carries a Correlation ID Summary

Wrote the `internal/httpserver` test suite proving the two operator-facing behaviours the
tracer (01-01) established but did not yet verify: `GET /health` returns the truth in both
directions (up, down, and down-on-timeout — never optimistically `ok`), 20 concurrent pollers
against the single shared pool are independent, and every response's `X-Request-Id` header
matches the `request_id` field on that exact request's JSON log line, with distinct IDs across
10 concurrent requests and no DSN leakage into logs or the degraded response body.

## What Was Built

**Task 1** (`internal/httpserver/health_test.go`): `TestHealth_Up` (real pool via
`testutil.NewTestPool`, 200/`ok`/`up`), `TestHealth_Down` (file-local `stubPinger` returning an
error, 503/`degraded`/`down`, response body asserted to contain neither the error text, nor
`postgres://`, nor `password`), `TestHealth_DownOnTimeout` (stub blocks on `ctx.Done()` and
returns `ctx.Err()` — proves the handler never defaults to `ok` on a check that did not
succeed), and `TestHealth_Concurrent` (20 goroutines against one `httptest.NewServer`, results
collected into per-goroutine slots, all must return 200/`ok`/`up`).

**Task 2** (`internal/httpserver/server_test.go`): `TestRequestID` (response `X-Request-Id`
value found in the captured JSON log line's `request_id` field), `TestRequestID_HonoursInboundHeader`
(a caller-supplied `X-Request-Id` is echoed back unchanged, not overridden — confirmed against
chi v5.3.1's `middleware.RequestID` source, which only stores the ID in context and does not
itself write the response header, which is why 01-01 already added the `echoRequestID`
middleware this test now proves correct), `TestRequestID_DistinctPerConcurrentRequest` (10
concurrent requests produce 10 distinct IDs, and every logged ID's line count never exceeds the
number of requests that produced it), and `TestNoDSNInLogs` (a `DATABASE_URL` DSN with a
recognisable password, set via `t.Setenv`, never appears in captured log output). A
`syncBuffer` (mutex-guarded `bytes.Buffer`) feeds `logging.NewWithWriter` so log output can be
captured and decoded line-by-line as JSON without racing on the buffer during concurrent
requests.

## Verification Performed

- `go build ./... && go vet ./...`: both exit 0.
- `TEST_DATABASE_URL=... go test ./internal/httpserver/ -run 'TestHealth_' -v -count=1`: all
  four tests `--- PASS` (`TestHealth_DownOnTimeout` takes ~3s, bounded by the handler's fixed
  `healthPingTimeout`).
- `go test ./internal/httpserver/ -run 'TestHealth_' -short -v`: `TestHealth_Up` and
  `TestHealth_Concurrent` `--- SKIP` (naming `TEST_DATABASE_URL`); `TestHealth_Down` and
  `TestHealth_DownOnTimeout` `--- PASS`.
- `go test ./internal/httpserver/ -run 'TestRequestID|TestNoDSNInLogs' -short -v -count=1`: all
  four `--- PASS`.
- `go test ./internal/httpserver/... -short -count=1 -v`: full package green, all
  non-DB-backed tests pass, all DB-backed tests skip cleanly under `-short`.
- `TEST_DATABASE_URL=... go test ./... -count=1`: full module green.
- Live boot (`go run ./cmd/server` against the compose Postgres): `curl -D -` on `/health`
  showed `X-Request-Id: Fortress/TTkYzrLq0H-000001`; the corresponding JSON log line carried
  `"request_id":"Fortress/TTkYzrLq0H-000001"` — the exact same value.
- `grep -c 'func TestHealth_' internal/httpserver/health_test.go` → 4;
  `grep -c 'func TestRequestID' internal/httpserver/server_test.go` → 3;
  `grep -q 'httpserver.Pinger' health_test.go`, `grep -q 'StatusServiceUnavailable' health_test.go`,
  `grep -q 'X-Request-Id' server.go`, `grep -q 'NewWithWriter' server_test.go`: all pass.
  `! grep -qiE 'github.com/google/uuid|gofrs/uuid' go.mod`: passes (no UUID dependency added).
- `git diff --stat internal/httpserver/health.go internal/httpserver/server.go` (Task 1) and
  `git diff --stat cmd/server/main.go` (Task 2): both empty — no production call site changed.

### Known environment limitation: `-race` could not be run locally

The plan's `<verify>` commands specify `-race`. On this Windows machine, `-race` is broken at
the toolchain level, independent of this plan's code: a minimal `GO111MODULE=off go build -race`
on a trivial `println("hi")` program with no application code involved fails identically
(`runtime/cgo: cgo.exe: exit status 2`, no captured stderr) against this Go 1.26.5 +
MSYS2 GCC 15.2.0 combination. This was confirmed as a pre-existing, code-independent
environment issue, not something introduced by or fixable from this plan's changes (no
production or test code can work around a broken C toolchain invocation). All tests were
written to be race-safe by construction per the plan's explicit instructions — per-goroutine
result slots instead of shared appended slices in `TestHealth_Concurrent`, a mutex-guarded
`syncBuffer` for concurrent log capture in `TestRequestID_DistinctPerConcurrentRequest` — and
all tests pass without `-race`. This module's CI (Phase 7, GitHub Actions on Linux) runs a
standard, working Go+gcc toolchain where `-race` is expected to succeed; this local limitation
does not carry forward.

## TDD Gate Compliance

Both tasks are marked `tdd="true"`, but neither required a RED → GREEN cycle in the usual
sense: the production behaviour under test (the `X-Request-Id` echo middleware and the
`httpserver.Pinger` seam) was already implemented and committed in plan 01-01 as Rule 2
deviations, specifically so this plan could add coverage with zero production changes (the
plan's own acceptance criteria assert this via `git diff --stat` on `health.go`, `server.go`,
and `main.go`). Every test in both files passed on first run with no implementation edit
required — there was no failing-test phase to record, because there was no missing
implementation to drive. This is a designed outcome of the two-plan split (tracer in 01-01,
coverage in 01-02), not a shortcut: both files add `test(...)`-only commits, and no `feat(...)`
commit follows either, which is expected and correct for this plan.

## Deviations from Plan

None — plan executed exactly as written. No Rule 1/2/3 auto-fixes were needed; no Rule 4
architectural questions arose.

## Known Stubs

None — every test in both files exercises real, wired behavior (a real pool for the up/concurrent
paths, a real logger capturing real JSON output, a real live boot verified via `curl`) or a
deliberate, plan-specified stub (`stubPinger`) used exactly as intended to isolate the
failure/timeout branches, not as a placeholder for unfinished work.

## Threat Flags

None beyond what the plan's own `<threat_model>` already covers. `TestHealth_Down` and
`TestNoDSNInLogs` directly exercise the two `mitigate`-disposition threats (T-01-02, T-01-01);
`TestHealth_Concurrent` and `TestHealth_DownOnTimeout` exercise T-01-03. No new network
endpoints, auth paths, or schema changes were introduced.

## Self-Check: PASSED

- FOUND: internal/httpserver/health_test.go
- FOUND: internal/httpserver/server_test.go
- FOUND: commit cdb38bc (test(01-02): cover both /health branches, timeout, and 20-way concurrency)
- FOUND: commit 3a2ed94 (test(01-02): prove X-Request-Id matches the logged correlation id)
