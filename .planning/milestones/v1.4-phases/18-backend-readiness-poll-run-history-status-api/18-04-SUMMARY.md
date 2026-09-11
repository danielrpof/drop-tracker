---
phase: 18-backend-readiness-poll-run-history-status-api
plan: 04
subsystem: api
tags: [status-endpoint, sqlc, consumer-seam, functional-options, ring-buffer, contract-freeze, chi]

requires:
  - phase: 18-backend-readiness-poll-run-history-status-api
    provides: "plan 18-01 SchemaVersioner seam + Server.schema/expectedSchema fields + WithReadiness; plan 18-02 internal/pollruns (Store, SourceSnapshot, RunResult, SourceMusicBrainz/SourceDeezer) + poller.RunRecorder/WithRunRecorder; plan 18-03 internal/buildinfo.Short()"
  - phase: 14-instance-passphrase-gate
    provides: "registerDataRoutes gated group (gate.Authenticate + gate.RequireCSRFHeader + X-Instance-Gated)"
provides:
  - "GET /status: gated JSON operator surface — poll_interval_seconds, watchlist_size, an instance block (app_version, schema_applied, schema_expected), and a per-source object (last_run, history[<=50] newest-first, last_skipped_at, consecutive_skips) for musicbrainz and deezer"
  - "CountWatchlist sqlc query (SELECT count(*) FROM watchlist) + regenerated method on Queries/Querier"
  - "httpserver.WatchlistCounter seam (method CountWatchlist, satisfied directly by *sqlc.Queries — no adapter) and httpserver.StatusStore seam (method Snapshot, satisfied by *pollruns.Store)"
  - "httpserver.StatusDeps + httpserver.WithStatus(StatusDeps) Option"
  - "docs/api/status-contract.md — the frozen /status shape Phase 19 types web/app/lib/api.ts against"
  - "cmd/server: one pollruns.Store wired as both the poller's RunRecorder and the HTTP server's StatusStore"
affects: [19, "18.1"]

actuals:
  tokens: 11300
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Consumer-declared seam named to match the generated method (WatchlistCounter.CountWatchlist) so *sqlc.Queries satisfies it at the composition root with no adapter type"
    - "Functional option taking a single named-field deps struct (WithStatus(StatusDeps)) rather than a positional argument list, for a 4-dependency option"
    - "Asymmetric database-failure handling on one handler: the count read is load-bearing (500), the schema read is best-effort (log + 200 + null), documented in a test comment so a later reader does not 'fix' it"
    - "A wire contract frozen in a committed docs/api/*.md file, gated against drift from the handler's json tags by a plan verify loop over every key name"

key-files:
  created:
    - internal/httpserver/status.go
    - internal/httpserver/status_test.go
    - internal/db/watchlist_count_test.go
    - docs/api/status-contract.md
  modified:
    - queries/watchlist.sql
    - internal/db/sqlc/watchlist.sql.go
    - internal/db/sqlc/querier.go
    - internal/httpserver/server.go
    - internal/httpserver/boot_e2e_test.go
    - cmd/server/main.go

key-decisions:
  - "Checkpoint resolved before execution: option-a — per-source grouping under a single sources object, each key carrying its own last_run/history/last_skipped_at/consecutive_skips (no flat parallel top-level maps). No commit for the checkpoint task."
  - "WatchlistCounter's method is CountWatchlist (not Count) so the generated *sqlc.Queries satisfies the seam directly; cmd/server passes sqlc.New(pool) straight into WithStatus with no adapter."
  - "poll_interval_seconds = int(pollInterval / time.Second) — integer truncation, no float; 15m renders 900, 90s renders 90."
  - "started_at/finished_at encode via time.Time's own RFC3339 marshaller; last_skipped_at is *time.Time so it is null (not a zero timestamp) until a source is first skipped."
  - "A server built without WithStatus answers 503 {\"error\":\"status not available\"} via the shared writeError helper, mirroring /ready's not_configured, so the route table is identical with and without the option."
  - "Left server_test.go's gatedRoutes table untouched — it names the exact v1.2 route set and /status is not one; TestStatus_Gated401 / TestStatus_Ungated200 cover the route directly instead."

patterns-established:
  - "docs/api/ as the home for frozen HTTP contracts, written in the migrations/README.md house voice, with a closing 'changing this contract' section"
  - "Contract-drift gate: a plan verify loop asserts every json key in the handler appears in the contract doc and vice versa"

requirements-completed: [STAT-01, STAT-02, RUN-02, RUN-04]

coverage:
  - id: D1
    description: "GET /status behind the gate returns the frozen envelope — last run per source, last 50 runs per source newest-first, per-source last_skipped_at + consecutive_skips, watchlist_size, poll_interval_seconds, and an instance block with app_version and both schema versions"
    requirement: "STAT-01"
    verification:
      - kind: integration
        ref: "internal/httpserver/boot_e2e_test.go#TestBootToStatus_EndToEnd (real migrated Postgres, recorded run surfaces as last_run, schema applied==expected==7)"
        status: pass
      - kind: unit
        ref: "internal/httpserver/status_test.go#TestStatus_EmptyHistory,TestStatus_PollInterval,TestStatus_HistoryNewestFirst,TestStatus_HistoryBounded,TestStatus_WatchlistSize,TestStatus_Ungated200"
        status: pass
    human_judgment: false
  - id: D2
    description: "The same request with no session cookie on a gated instance returns 401 from the gate — never a partial /status body"
    requirement: "STAT-01"
    verification:
      - kind: unit
        ref: "internal/httpserver/status_test.go#TestStatus_Gated401 (asserts 401 and that the body carries none of poll_interval_seconds/watchlist_size/sources)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A fresh instance with no cycles returns 200 with sources.musicbrainz and sources.deezer both present, last_run null, history an empty JSON array (not null)"
    requirement: "STAT-01"
    verification:
      - kind: unit
        ref: "internal/httpserver/status_test.go#TestStatus_EmptyHistory (explicit raw-body assertion that \"history\":[] is present and \"history\":null is absent)"
        status: pass
    human_judgment: false
  - id: D4
    description: "No /status response field at any depth contains a DSN, webhook URL, filesystem path, or raw driver error string; a count/schema failure sends its raw cause to httplog.SetAttrs and returns a fixed body; an empty-message error still produces the fixed body"
    requirement: "STAT-02"
    verification:
      - kind: unit
        ref: "internal/httpserver/status_test.go#TestStatus_NoLeak,TestStatus_EmptyErrorMessage,TestStatus_SchemaErrorStillTwoHundred,TestStatus_NotConfigured"
        status: pass
      - kind: other
        ref: "grep gate: status.go has exactly two httplog.SetAttrs calls (status_watchlist_error, status_schema_error) and no Error/Detail/Message/LastError struct field"
        status: pass
    human_judgment: false
  - id: D5
    description: "The per-source skip signal reaches the response — a source with recorded skips shows last_skipped_at and consecutive_skips, the other stays null/0"
    requirement: "RUN-02"
    verification:
      - kind: unit
        ref: "internal/httpserver/status_test.go#TestStatus_SkipSignal"
        status: pass
    human_judgment: false
  - id: D6
    description: "history is bounded to the last 50 per source, newest-first, with last_run byte-identical to history[0]"
    requirement: "RUN-04"
    verification:
      - kind: unit
        ref: "internal/httpserver/status_test.go#TestStatus_HistoryBounded,TestStatus_HistoryNewestFirst (bytes.Equal on the raw last_run vs history[0])"
        status: pass
    human_judgment: false
  - id: D7
    description: "watchlist_size comes from a count query (CountWatchlist), proven against a real database to track the watchlist table row for row including after an insert"
    requirement: "STAT-01"
    verification:
      - kind: integration
        ref: "internal/db/watchlist_count_test.go#TestCountWatchlist_Integration"
        status: pass
      - kind: other
        ref: "make sqlc-check clean with the regenerated watchlist.sql.go + querier.go committed"
        status: pass
    human_judgment: false
  - id: D8
    description: "cmd/server/main.go builds exactly one pollruns.Store and hands the same instance to the poller as its RunRecorder and to the HTTP server as its StatusStore (RUN-04, ROADMAP success criterion 5); runCycle gains no recorder call"
    requirement: "RUN-04"
    verification:
      - kind: other
        ref: "grep -c 'pollruns.NewStore()' cmd/server/main.go == 1; grep -c 'poller.WithRunRecorder(runs)' cmd/server/main.go == 1; ! grep -q 'p\\.runs\\.' internal/poller/poller.go"
        status: pass
      - kind: integration
        ref: "internal/httpserver/boot_e2e_test.go#TestBootToStatus_EndToEnd (a run recorded on the store shows through /status)"
        status: pass
    human_judgment: false
  - id: D9
    description: "The frozen /status shape is written in docs/api/status-contract.md and every json key in the handler appears there and vice versa — the Phase 19 freeze point"
    requirement: "STAT-01"
    verification:
      - kind: other
        ref: "plan task-3 verify loop over all 21 key names: KEYS_OK (no MISSING_IN_HANDLER / MISSING_IN_DOC)"
        status: pass
    human_judgment: false
  - id: D10
    description: "End-to-end operator observation: docker compose up, log in via the SPA, GET /status in the same session matches the contract field for field; GET /status in a private window returns 401"
    verification:
      - kind: manual_procedural
        ref: "18-04-PLAN.md <human-check>"
        status: unknown
    human_judgment: true
    rationale: "The gate + contract + composition-root wiring proven together against a real running instance in a browser session is an observation automation cannot make; boot_e2e covers the wiring against a real DB but not the SPA login round-trip."

duration: ~30 min
completed: 2026-09-09
status: complete
---

# Phase 18 Plan 04: Backend — /status Surface & Contract Freeze Summary

**`GET /status` ships as a gated JSON operator panel — poll interval, watchlist size, an instance block, and a per-source object carrying the last run, the last 50 runs newest-first, and the skip signal — with its wire shape frozen in `docs/api/status-contract.md` and one `pollruns.Store` wired at the composition root as both the poller's recorder and the server's reader.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-09-10T01:41:51Z
- **Completed:** 2026-09-10T01:51:17Z (execution); metadata commit follows
- **Tasks:** 3 executed (the `checkpoint:decision` task was pre-resolved — option-a — with no commit)
- **Files modified:** 10 (4 created, 6 modified)

## Accomplishments

- `GET /status` behind `registerDataRoutes`: inherits `gate.Authenticate` + `gate.RequireCSRFHeader` + `X-Instance-Gated` on a gated instance, 401 without a session, 200 (never a partial body) otherwise. `registerDataRoutes` now registers seven routes.
- The frozen envelope: `poll_interval_seconds` (int, 900 for the 15m default), `watchlist_size` (from `CountWatchlist`), `instance.{app_version, schema_applied, schema_expected}` reusing `/ready`'s schema key names, and `sources` keyed by `musicbrainz`/`deezer` — each `{last_run, history (<=50, newest-first, `[]` never null), last_skipped_at, consecutive_skips}`. No `error`/`detail`/`message`/`last_error` key at any depth.
- Two consumer seams: `WatchlistCounter` (method `CountWatchlist`, satisfied directly by the generated `*sqlc.Queries`) and `StatusStore` (method `Snapshot`, satisfied by `*pollruns.Store`). `WithStatus(StatusDeps)` is the new functional option; the schema seam stays `WithReadiness`'s to own.
- `CountWatchlist` sqlc query added and regenerated with the pinned v1.31.1 toolchain; `make sqlc-check` clean with the generated files committed alongside the query.
- Asymmetric failure handling: a `CountWatchlist` error is a 500 with the shared fixed body; a schema-read error is logged and the response is still 200 with `schema_applied: null` — `/ready` is the endpoint that turns red on a DB blip. A test comment records why so it is not "fixed" later.
- `cmd/server/main.go` builds one `pollruns.NewStore()` and passes it to both `httpserver.WithStatus(...)` and `poller.WithRunRecorder(...)`. `runCycle` is untouched — Phase 18.1 is left a pure edit there.
- `docs/api/status-contract.md`: the durable freeze point, with the auth/status-code table, the field-by-field envelope and run-object tables, fresh-instance and with-history example bodies, and a closing "changing this contract" section. Gated against drift from the handler's json tags by the plan's verify loop over all 21 key names.

## Task Commits

1. **Task 1 (tracer): wire GET /status end-to-end with the frozen envelope** — `7c040c0` (feat)
2. **Task 2 (tdd): pin the first-run state, skip signal, and ordering guarantees** — `0654997` (test)
3. **Task 3: close the gate and leak paths, write the contract down** — `3118af7` (test)

**Plan metadata:** _this commit_ (docs: complete plan)

_Task 2 is a single `test` commit: the tracer built `handleStatus` to spec, so the TDD cycle pinned already-correct behaviour rather than driving new implementation — the same pattern plan 18-02 recorded._

## Files Created/Modified

- `internal/httpserver/status.go` — `WatchlistCounter`/`StatusStore` seams, `StatusDeps`, `statusResponse`/`statusInstance`/`statusSource`/`statusRun`, `handleStatus`, `toStatusSource`/`toStatusRun`.
- `internal/httpserver/status_test.go` — 13 tests + file-local `fakeStatusStore`/`fakeWatchlistCounter` doubles (reuses `fakeSchemaVersioner` from `ready_test.go`).
- `internal/db/watchlist_count_test.go` — `TestCountWatchlist_Integration` (relative-movement assertion, artist+watchlist insert, cleanup by mbid).
- `docs/api/status-contract.md` — new; the frozen contract.
- `queries/watchlist.sql` — `+ CountWatchlist :one`.
- `internal/db/sqlc/watchlist.sql.go`, `internal/db/sqlc/querier.go` — regenerated (`CountWatchlist` method + `Querier` entry).
- `internal/httpserver/server.go` — `Server` + `serverConfig` status fields, `WithStatus`, `r.Get("/status", s.handleStatus)` in `registerDataRoutes`, `time` import.
- `internal/httpserver/boot_e2e_test.go` — `+ TestBootToStatus_EndToEnd` against real Postgres.
- `cmd/server/main.go` — `runs := pollruns.NewStore()`, `httpserver.WithStatus(...)`, `poller.WithRunRecorder(runs)`, `pollruns` import.

## Decisions Made

- Checkpoint pre-resolved as **option-a** (per-source grouping); recorded here, no commit for the checkpoint task.
- Seam method named `CountWatchlist` so `*sqlc.Queries` satisfies `WatchlistCounter` with no adapter at the composition root.
- `poll_interval_seconds` via integer division `int(pollInterval / time.Second)` — no float.
- `last_skipped_at` is `*time.Time` (null until first skip); run timestamps use `time.Time`'s own RFC3339 marshaller.
- Not-configured `/status` → 503 `{"error":"status not available"}` via the shared `writeError`, so the route table is option-independent.
- `gatedRoutes` in `server_test.go` left alone — it names the v1.2 route set; `/status` is covered by dedicated tests instead (noted in the task-3 commit body).

## Deviations from Plan

None — plan executed as written. The plan's Task 2 anticipated the handler might need correction where a test proved it wrong; the tracer's implementation was already correct, so Task 2 is a pin-only `test` commit (an explicitly-allowed outcome, mirrored from 18-02).

## Issues Encountered

- **`make test` / `make test-short` carry `-race`, unusable on this dev box** (ThreadSanitizer allocation failure — a standing STATE.md blocker since Phase 11.1; `-race` is also absent from CI). Substituted the DoD integration gate with a plain `go test ./... -count=1 -coverprofile=coverage.out -coverpkg=<COVER_PKGS>` run, then `make coverage-gate` unchanged. Same accepted precedent as Phases 11.1 / 15 / 16 / 18-01 / 18-02 / 18-03. This plan adds no new concurrency surface (`pollruns.Store` and its mutex are plan 18-02's, already covered by its looped invariant test). Full suite green; backend coverage **90.60%** (floor 80%).
- **Pre-commit `golangci-lint` hook lints the whole repo** and can exceed a 2-minute command timeout; commits ran under an extended timeout. No hook bypassed (`--no-verify` never used).
- **`TEST_DATABASE_URL` not exported in the shell** — DB-backed tests skipped silently at first. Set `TEST_DATABASE_URL=postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable` (the value in `docker-compose.yml` / Makefile comment) for the integration runs.

## Definition of Done

- `go build ./...` — clean
- `go vet ./...` — clean
- `golangci-lint run` (whole repo) — 0 issues
- Full `go test ./...` (race substituted) — all packages ok, no FAIL
- `make coverage-gate` — 90.60% (required 80%) PASS
- `make sqlc-check` — clean, regenerated files committed
- Contract-drift gate — all 21 json keys present in both `status.go` and `docs/api/status-contract.md` (`KEYS_OK`)
- Forbidden-field gate — no `Error`/`Detail`/`Message`/`LastError` struct field in `status.go`
- Scope-fence gate — `! grep -q 'p\.runs\.' internal/poller/poller.go` holds; `internal/detection` untouched

## Known Stubs

None. `events_recorded` is always 0 until Phase 18.1 populates it — that is documented in both the contract doc and the plan, and the field is part of the deliberately-frozen shape, not a placeholder this plan should have wired.

## Threat Flags

None. `/status` is an additive read endpoint inside the existing gated group; no new auth path, file access, outbound call, or schema change (the one new query is a `count(*)`).

## Next Phase Readiness

- **Phase 19 (SPA System view)** can now type `web/app/lib/api.ts` against `docs/api/status-contract.md` — a real, frozen Go response body, not a guess. Its stated fetch-on-mount + manual-refresh design (no polling) is the mitigation for T-18-21 and is recorded in its ROADMAP notes.
- **Phase 18.1** has a pure `runCycle` edit ahead: `runs` is wired as `poller.WithRunRecorder` at the composition root, so 18.1 adds only the `RecordRun`/`RecordSkip` call sites and the per-cycle counter aggregation, and must invert `TestRunRecorderInertThisPhase`.
- **Deferred / manual:** the `<human-check>` — `docker compose up --build` with `INSTANCE_PASSPHRASE` set, log in via the SPA, confirm `/status` matches the contract field-for-field and a no-session private window gets 401. This is the observation that proves gate + contract + wiring together against a real instance.

## Self-Check: PASSED

- Created files present: `internal/httpserver/status.go`, `internal/httpserver/status_test.go`, `internal/db/watchlist_count_test.go`, `docs/api/status-contract.md` — all found
- Commits present: `7c040c0`, `0654997`, `3118af7` — all in `git log`
- `internal/poller/poller.go` `runCycle` unchanged; no `p.runs.` reference
- `make sqlc-check` clean; `make coverage-gate` 90.60% PASS

---
*Phase: 18-backend-readiness-poll-run-history-status-api*
*Completed: 2026-09-09*
