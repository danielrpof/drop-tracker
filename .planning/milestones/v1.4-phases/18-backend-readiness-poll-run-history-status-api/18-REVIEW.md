---
phase: 18-backend-readiness-poll-run-history-status-api
reviewed: 2026-09-09T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - .github/workflows/full-pipeline.yml
  - Dockerfile
  - cmd/server/main.go
  - docs/api/status-contract.md
  - internal/buildinfo/buildinfo.go
  - internal/buildinfo/buildinfo_test.go
  - internal/db/migrate.go
  - internal/db/schema_version.go
  - internal/db/schema_version_test.go
  - internal/db/sqlc/querier.go
  - internal/db/sqlc/watchlist.sql.go
  - internal/db/watchlist_count_test.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/ready.go
  - internal/httpserver/ready_test.go
  - internal/httpserver/server.go
  - internal/httpserver/status.go
  - internal/httpserver/status_test.go
  - internal/poller/poller.go
  - internal/poller/poller_test.go
  - internal/pollruns/pollruns.go
  - internal/pollruns/pollruns_test.go
  - queries/watchlist.sql
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 18: Code Review Report

**Reviewed:** 2026-09-09
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

Reviewed the readiness / poll-run-history / status-API phase against its six declared focus
areas. The core mechanisms hold up:

- **`internal/pollruns` concurrency** — single `sync.Mutex` covers the ring, `lastSkippedAt`,
  and `consecutiveSkips`; every path (`RecordRun`, `RecordSkip`, `stateLocked`, `Snapshot`)
  takes it. No lock-free reads. `RunResult` is a pure-value struct (scalars + `time.Time` +
  strings), and `Snapshot` copies each ring into a freshly allocated slice under the lock
  and returns `*RunResult` / `*time.Time` pointing at fresh locals, never at ring storage.
  A later `append`/trim cannot mutate an in-flight response. The mutex model is sound.
- **Information leak on `/ready` and `/status`** — every reason/message on every failure path
  is a compile-time constant (`db_unreachable`, `schema_dirty`, `schema_behind`,
  `not_configured`, `"internal error"`, `"status not available"`). Raw `err.Error()` strings
  go only to `httplog.SetAttrs`. `RunResult.Summary` is recomposed by the store from counts +
  normalized outcome, discarding any caller-supplied text. No DSN / webhook / path / driver
  string reaches a body at any depth.
- **Gating** — `/ready` is registered on the root router outside the gate `Group`, so one
  registration serves both configs and it is never 401. `/status` sits in
  `registerDataRoutes`, so it is behind `gate.Authenticate` when gated and open when inert.
- **Single-build guarantee** — `build-scan` bakes `VERSION=${{ github.sha }}`, `docker save`s
  the scanned image, and `release` `docker load`s and pushes that exact tar. No second
  `docker build`. The `VERSION` build-arg addition does not create a divergent release image.
- **Scope fence** — `poller.runCycle` and both cycle methods never reference `p.runs`; the
  `RunRecorder` seam is wired (`noopRunRecorder` default, `WithRunRecorder`) but inert, as
  `TestRunRecorderInertThisPhase` pins.
- **Schema-version correctness** — `!dirty && applied >= expected` ⇒ ready; `dirty` is checked
  before the version comparison; `ErrNoRows` → `(0,false,nil)`; a negative stored version is
  rejected rather than wrapped to a huge `uint`. `ExpectedSchemaVersion` reuses the same
  `maxSourceVersion` walk the ahead-of-source guard uses, so it cannot drift from what
  `RunMigrations` would apply.

Two robustness gaps are worth fixing before this ships, plus two informational notes.

## Warnings

### WR-01: `/status` database reads are not bounded by a timeout

**File:** `internal/httpserver/status.go:92-109`
**Issue:** `handleStatus` runs `s.watchlistCounter.CountWatchlist(ctx)` and
`s.schema.SchemaVersion(ctx)` against `ctx := r.Context()` with **no** `context.WithTimeout`.
Both `/health` (`healthPingTimeout`, `health.go:17,29`) and `/ready` (`readyCheckTimeout`,
`ready.go:24,41`) deliberately bound their DB calls, and `health.go`'s own comment states
the reason: "a hung network path to Postgres would hang the health request itself and
accumulate blocked goroutines under a monitor polling every few seconds." `/status` is the
endpoint the Phase 19 SPA polls on an interval, so it has the same exposure. `http.Server`'s
`WriteTimeout` (15s, `cmd/server/main.go:63`) does not rescue this: it fails the eventual
response write but never cancels the handler context or unblocks a stalled pgx call, so the
goroutine stays parked on the driver until the driver itself returns.

This also undercuts the documented design intent in `docs/api/status-contract.md:28-32`
("a momentary database blip degrades `instance.schema_applied` to `null` and still returns
`200`"): that graceful degradation only happens when the schema read *errors* quickly. If the
blip is a hang rather than a refused connection, the request hangs instead of degrading.

**Fix:** Bound both DB interactions the same way `/ready` does, e.g.:
```go
ctx, cancel := context.WithTimeout(r.Context(), statusCheckTimeout) // separate const, mirror readyCheckTimeout
defer cancel()
```
Keep it a distinct constant from `readyCheckTimeout` / `healthPingTimeout` so retuning one
surface does not silently retune the others (the same rationale `ready.go:22-23` already
gives).

### WR-02: `pollruns.Store` never validates `Source`, so the frozen `/status` `sources` map can be polluted

**File:** `internal/pollruns/pollruns.go:86,98-113,141-162`
**Issue:** `stateLocked(source)` lazily creates and permanently retains a `sourceState` for
*any* string passed to `RecordRun`/`RecordSkip`, and `Snapshot` emits every key present in
`s.sources`. `normalizeOutcome` guards `Outcome`, but nothing guards `Source` against
`KnownSources`. `docs/api/status-contract.md:47` freezes the contract as "Always carries
exactly `musicbrainz` and `deezer`", and the Phase 19 SPA is typed to map over exactly those
keys. A single mis-cased or typo'd source name from the Phase 18.1 wiring (or any future
caller) would add a third, permanent key to the published contract with no error anywhere.

This is currently latent (the seam is inert this phase), but the risk is baked in now:
`internal/poller/poller.go:46-47` defines its **own** `sourceMusicBrainz` / `sourceDeezer`
string constants rather than importing `pollruns.SourceMusicBrainz` / `SourceDeezer`, so the
two packages already carry duplicate literals that can drift when 18.1 calls `RecordRun`.

**Fix:** Make `RecordRun` / `RecordSkip` reject (or normalize-and-drop) an unknown source —
mirror `normalizeOutcome`:
```go
func (s *Store) stateLocked(source string) (*sourceState, bool) {
    st, ok := s.sources[source]
    return st, ok // caller ignores writes for an unknown source
}
```
or at minimum have `internal/poller` reference `pollruns.SourceMusicBrainz` /
`pollruns.SourceDeezer` directly instead of re-declaring the literals, so 18.1 physically
cannot pass a drifted value.

## Info

### IN-01: `/status` silently reports `schema_expected: 0` when `WithStatus` is supplied without `WithReadiness`

**File:** `internal/httpserver/status.go:101-123`
**Issue:** `handleStatus` guards `s.schema != nil` for the applied read but unconditionally
emits `SchemaExpected: s.expectedSchema`. If a binary wires `WithStatus` but not
`WithReadiness`, the response is a plausible-looking `{"schema_applied":null,"schema_expected":0}`
rather than a failure. `cmd/server/main.go` always wires both, so impact is low, but a
partial wiring produces a wrong body instead of a loud error.
**Fix:** Either assert both seams are present at `New` time when `WithStatus` is used, or omit
the `instance` schema fields when `s.schema == nil`.

### IN-02: `cycleID` test helper produces non-ASCII ids past index 25

**File:** `internal/httpserver/status_test.go:333`
**Issue:** `cycleID(i) = "musicbrainz-" + string(rune('a'+i))`; `TestStatus_HistoryBounded`
calls it with `i` up to 59, yielding non-alphabetic / non-ASCII runes. The test only asserts
`len(history) == 50`, so it passes, but the ids are meaningless and a future assertion on
`cycle_id` content here would be confusing.
**Fix:** Use `fmt.Sprintf("musicbrainz-%d", i)`.

---

_Reviewed: 2026-09-09_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
