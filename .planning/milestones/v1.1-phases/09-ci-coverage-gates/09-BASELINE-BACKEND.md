# Phase 9 Plan 01: Backend Coverage Starting Baseline

**Measured:** 2026-08-13
**Command used to produce `coverage.out`:**

```
go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg=$(go list ./... | grep -v '/internal/db/sqlc' | paste -sd, -)
```

This is the exact `-coverprofile`/`-coverpkg` invocation `make test-integration` now runs (Task 1),
minus `-race`. See "Known environment limitation: `-race`" below for why `-race` was omitted for
this local measurement only.

## Aggregate Total

Verbatim `total:` line from `go tool cover -func=coverage.out`:

```
total:										(statements)			83.5%
```

Bare percentage extracted by the gate (`go tool cover -func` field 3, trailing `%` stripped): **83.5**

## Gate Verdict at the Required Floor (80%)

```
$ make coverage-gate
Backend coverage: 83.5% (required: 80%)
PASS
```

**Verdict: PASS, +3.5 points above the 80% floor.**

This is a real measured result, not designed toward. The plan anticipated a possible sub-80% first
measurement (with `09-03` closing the gap) — the actual starting point already clears the floor by
3.5 points. That headroom does not reduce the value of the gap-closing work below: `cmd/server`'s
boot/shutdown orchestration and the MusicBrainz search-source wrapper are both at 0% today despite
the aggregate passing, and per D-09 the aggregate percentage is not the goal — meaningful coverage of
real behavior is.

## Zero-Coverage Function Map (D-09 gap-closing input)

Counted, descending list of files holding at least one `0.0%`-coverage function. Produced via:

```
go tool cover -func=coverage.out | awk '$3 == "0.0%"' | cut -d: -f1 | sort | uniq -c | sort -rn
```

```
      3 github.com/danielrpof/drop-tracker/internal/httpserver/search.go
      2 github.com/danielrpof/drop-tracker/cmd/server/main.go
      1 github.com/danielrpof/drop-tracker/internal/db/pool.go
```

`cmd/server` appears in this list (2 zero-coverage functions), confirming the `-coverpkg` scope
correctly widened measurement across package boundaries per D-05 — it did not silently vanish from
the profile.

## Ten Worst Individual Functions (file, function, coverage %)

```
0.0%   cmd/server/main.go:56               main
0.0%   cmd/server/main.go:68               run
0.0%   internal/db/pool.go:112             redactedTarget
0.0%   internal/httpserver/search.go:76    NewMusicBrainzSource
0.0%   internal/httpserver/search.go:80    Name (musicBrainzSource)
0.0%   internal/httpserver/search.go:82    SearchArtists (musicBrainzSource)
50.0%  internal/detection/musicbrainz.go:420  displayArtistName
60.0%  internal/deezer/client.go:188       clampLimit
60.0%  internal/events/service.go:132      toEvent
60.0%  internal/logging/logging.go:43      parseLevel
```

Only six functions sit at exactly `0.0%`; the remaining four in the worst-ten list are partially
covered (50-60%), a materially different (and lower-priority) gap than a function with zero coverage.

## Prioritization Note (D-09 order: meaningful behavior first, not fastest-to-close)

1. **`cmd/server/main.go` `run`/`main`** — the single largest, most meaningful gap. This is real
   hand-written orchestration (config load, migration run, pool connect, HTTP server start,
   `signal.NotifyContext`-driven graceful shutdown on SIGTERM vs. serve-error paths) that Phase 1's
   UAT already had to verify by hand because nothing automated covered it. RESEARCH.md Pitfall 2
   flagged this as the expected highest-value gap-closing target, and it is. Recommended seam:
   `cmd/server/main_test.go`, `package main`, calling `run()` directly with
   `testutil.RequirePostgresDSN(t)`, exercising (a) successful boot + graceful shutdown via the
   `stop()` cancel func, and (b) the config-load-failure early-return path.

2. **`internal/httpserver/search.go` `NewMusicBrainzSource`/`Name`/`SearchArtists`** — three of the
   six zero-coverage functions live here. The sibling `deezerSource` implementation of the same
   `SearchSource` interface (same file) is NOT in the zero-coverage list, meaning the search-proxy's
   MusicBrainz path specifically has no test exercising it — likely connected to the pre-existing,
   documented, environment-specific MusicBrainz TLS issue on this dev box (PROJECT.md Context,
   Broken Windows Ledger entry #3) rather than a genuine design gap, but it is still real,
   user-facing, untested behavior and belongs ahead of the smaller partial gaps below.

3. **`internal/db/pool.go` `redactedTarget`** — a small function, but it is this codebase's own
   DSN-redaction helper, the exact class of function Phase 1's security review (T-01-01, T-01-09)
   required be tested precisely because it is the only thing standing between a DB connection
   failure and a leaked password reaching logs or stderr. An untested redaction helper is a
   security-relevant gap, not a cosmetic one — D-09 explicitly ranks this kind of untested
   error/security path above an easy-to-cover getter, even though it would close the numeric gap
   fastest of anything on this list.

4. **Lower priority (partially covered, 50-60%):** `displayArtistName`, `clampLimit`, `toEvent`,
   `parseLevel` — real functions with some existing exercise via other packages' tests (per
   `-coverpkg`'s cross-package measurement), not zero-coverage gaps. These should only be picked up
   after 1-3 above if plan 09-03 still needs points to stay comfortably clear of the 80% floor.

## Wall-Clock Duration

One instrumented `go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg=...` run
(equivalent to `make test-integration` minus `-race`, see limitation note below) measured via
`date +%s` before/after: **61 seconds**.

`full-pipeline.yml`'s `test` job currently has `timeout-minutes: 15` (900 seconds). 61 seconds is
comfortably inside that budget (under 7% of it) — plan 09-05 does not need to raise
`timeout-minutes` on the strength of this measurement, though CI's real `-race`-enabled run should
still be watched on its first execution since `-race` instrumentation was not measurable locally
(see below).

## Folded Todo Re-Check: Flaky Tests Under Parallel `go test` (resolves_phase: 9)

Per `.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`, this phase's
`-coverprofile`/`-coverpkg` instrumentation was re-checked against the two named flakiness classes:
the four `internal/notifier` timing tests
(`TestNotifyPending_SpacingAppliedEvenAfterFailedSend`,
`TestNotifyPending_CrossCycleRecoveryAfterOutage`,
`TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows`,
`TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass`) and
`internal/poller`'s `TestPoller_RunMusicBrainzCycle_RecordsNewRelease` schema-visibility race.

Ran the coverage-instrumented suite **three additional consecutive times** (run 2, run 3, run 4;
run 1 was Task 1's own verification run) at `-p 1`. Observed result: **all four runs passed clean
for all packages, including all four named `internal/notifier` tests and the named
`internal/poller` test.** No occurrence of the `relation "artists" does not exist` error and no
notifier timing flake in any of the four runs. Coverage instrumentation did not surface either
flakiness class under `-p 1` — consistent with the todo's own note that both classes reproduce only
without `-p 1`, and `make test-integration` (with or without coverage flags) has always run at
`-p 1`. The todo can remain closed for this phase's purposes; it was not silently dropped, it was
re-checked as scoped and found not reproducible under this phase's instrumentation.

## Known Environment Limitation: `-race` Could Not Be Measured Locally

This baseline (and the local `make test-integration` proof-of-mechanism in Task 1) was measured
**without** `-race`, for the same pre-existing, code-independent reason documented in
`01-02-SUMMARY.md` and `01-03-SUMMARY.md`: this dev machine's Go 1.26.5 + MSYS2 GCC 15.2.0 cgo
toolchain cannot run `-race`-instrumented binaries. Confirmed this session: a plain
`go test ./internal/config/... -race -count=1` fails with a ThreadSanitizer allocation error
(`ThreadSanitizer failed to allocate ... (error code: 87)`), reproducible in isolation with no
other Go processes running, on the same gcc toolchain already flagged as broken in Phase 1. The
`Makefile`'s `test-integration` target retains `-race` unchanged (per the plan's explicit
instruction and D-02) — this is a real requirement for CI (GitHub Actions, Linux, standard
toolchain, `-race` expected to succeed) and for any contributor on a working toolchain. Locally on
this machine, the coverage mechanism (measurement scope, gate pass/fail/missing-profile paths) was
proven correct using the identical flags minus `-race`; the aggregate percentage and zero-coverage
function map above are not expected to shift meaningfully once `-race` runs successfully in CI,
since `-race` changes execution timing/scheduling, not which statements execute.

---

## Closing Measurement (Plan 09-03)

**Measured:** 2026-08-13
**Command used** (identical scope/flags to the starting baseline above, `-race` omitted for the
same pre-existing local-toolchain reason):

```
go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg=$(go list ./... | grep -v '/internal/db/sqlc' | paste -sd, -)
```

### Closing Aggregate

Verbatim `total:` line from `go tool cover -func=coverage.out`:

```
total:										(statements)			87.1%
```

**Delta from starting baseline: 83.5% -> 87.1%, +3.6 points.**

### Gate Verdict

```
$ make coverage-gate
Backend coverage: 87.1% (required: 80%)
PASS
```

The starting baseline already cleared the 80% floor (+3.5 points); Tasks 1 and 2 of this plan
closed the gap-closing work anyway, per D-09's own framing: the aggregate passing does not make an
untested boot path or an untested handler branch acceptable when they are the codebase's most
meaningful uncovered behavior. No threshold or measured-set change was made (`Makefile` is
unmodified by this plan) -- every point of the +3.6 delta came from the three test files below.

### Test Files Added By This Plan

- `cmd/server/main_test.go` -- boot-path tests for `run(ctx)`'s config-load early return and its
  boot-then-graceful-shutdown branch, against a real Postgres instance via
  `testutil.RequirePostgresDSN`. Required a behavior-preserving seam refactor first (`run` now
  takes its cancellation context from its caller; `main` owns `signal.NotifyContext`).
- `internal/logging/logging_test.go` -- `parseLevel`'s four named branches plus its
  unrecognized-value fallback to Info, handler selection by format (text vs JSON), the service
  attribute, and end-to-end level filtering.
- `internal/webassets/embed_test.go` -- the embedded SPA handler's static-file branch (requesting a
  real hashed asset and asserting its JavaScript content type), plus the root path, an unregistered
  path, and a missing-but-asset-shaped path, all falling back correctly to `index.html`.

### Refreshed Zero-Coverage Function Map

Produced via the same `awk '$3 == "0.0%"'` extraction as the starting baseline, against the
post-gap-closing `coverage.out`:

```
      2 github.com/danielrpof/drop-tracker/internal/httpserver/search.go
      1 github.com/danielrpof/drop-tracker/internal/db/pool.go
      1 github.com/danielrpof/drop-tracker/cmd/server/main.go
```

`cmd/server/main.go` dropped from 2 zero-coverage functions (`main`, `run`) to 1 (`main` only) --
`run` is now exercised at 81.8% by the two new boot-path tests. `main` itself stays at 0.0%: it is
now just the signal-registration-then-delegate-to-`run` shim (WR-03), which this plan's seam
refactor deliberately kept thin rather than adding a test-only wrapper or build tag to reach it
(the plan's own action text forbids both). `internal/httpserver/search.go` incidentally dropped
from 3 zero-coverage functions to 2: the boot-path test's real call into `httpserver.New(...)`
exercises `NewMusicBrainzSource` as a side effect of constructing the real search-source slice,
though `Name`/`SearchArtists` on that source remain untested (pre-existing, out of this plan's
three named targets, and already ranked #2 in the starting baseline's own prioritization note for
a future plan). `internal/db/pool.go`'s `redactedTarget` also remains untested, ranked #3 in the
same prioritization note -- neither was in scope for this plan's three named packages, and the
gate already passes comfortably above the 80% floor without them.
