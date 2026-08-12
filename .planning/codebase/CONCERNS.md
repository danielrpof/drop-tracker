# Codebase Concerns

**Analysis Date:** 2026-08-12

## Tech Debt

**Single-Instance Scheduler Limitation:**
- Issue: The robfig/cron scheduler in `internal/poller/poller.go` runs in a single process with no distributed coordination
- Files: `internal/poller/poller.go`, `cmd/server/main.go:154`
- Impact: Running multiple instances of this service will cause duplicate polling cycles and notifications for the same artists. MusicBrainz and Deezer will receive N times the traffic, and Discord will receive N duplicate embeds per detected release
- Fix approach: Before scaling to multiple instances, implement distributed locking (Postgres advisory locks are the natural fit given the existing DB dependency) or migrate to a dedicated scheduler service (Temporal, Airflow) that handles leader election and deduplication
- Priority: **High** (blocks horizontal scaling, which may be needed if watchlist grows very large)

**Sequential Artist Polling:**
- Issue: `internal/poller/poller.go`'s `RunMusicBrainzCycle` and `RunDeezerCycle` poll artists one at a time (sequential loop), not concurrently
- Files: `internal/poller/poller.go:229-250`
- Impact: With a 15-minute poll interval and 100+ artists on a watchlist, a single poll cycle could take hours to complete. The next cron tick would skip (CAS guard at line 215) because the previous cycle is still running
- Rationale: Intentional — per-source rate limiting via `rate.Limiter` is applied per HTTP client, not per request. Concurrent requests would multiply the effective rate unless each goroutine got its own limiter or the shared limiter's token bucket was further subdivided, adding complexity
- Fix approach: Bounded concurrency (semaphore or buffered channel) would allow parallel fetches within the configured rate limit, but requires careful analysis of MusicBrainz and Deezer's per-IP vs per-token rate limits to avoid hitting undocumented throttling thresholds. Research first before implementing
- Priority: **Medium** (optimizes runtime only; v1 goal is correctness, not performance)

**Notifier Database Timeout Bounds Recently Added:**
- Issue: Prior to commit `479c781`, the notifier's database calls in `internal/notifier/notifier.go` had no time bounds
- Files: `internal/notifier/notifier.go:130-132` (listUnnotified helper), `internal/notifier/timeout_test.go:60-98` (regression test)
- Impact: A half-open TCP connection (e.g., from Docker Desktop's port proxy) or a wedged Postgres would hang `NotifyPending` forever, wedging the shared `notifying` CAS guard and silently stopping all subsequent notify passes (the guard would remain `true` indefinitely)
- Fix Status: **FIXED** in commit `479c781` — `internal/db/pool.go` now sets explicit connect/ping/idle timeouts, and `internal/notifier.go` wraps each sqlc call in a 10s deadline
- Regression Test: `internal/notifier/timeout_test.go` proves `NotifyPending` returns on an unresponsive database and releases its guard so the next cycle can run
- Follow-up: `internal/poller`'s own `store.List(ctx)` and `internal/detection`'s DB calls share the same unbounded-context exposure (noted in 479c781's investigation, lines 652-657 of the debug report). Partially covered by the pool-level bounds in (a); deliberately deferred to keep this diff focused on the verified notifier surface

**Configuration Drift Between docker-compose.yml and .env.example:**
- Issue: `docker-compose.yml` publishes Postgres on `5433:5432` (line 11), but `.env.example` documents the default as `localhost:5432`
- Files: `docker-compose.yml:11`, `.env.example` (cannot read per permission rules, but noted in 479c781 investigation)
- Impact: A developer who clones the repo and copies `.env.example` verbatim (without reading docker-compose.yml) will connect to the wrong Postgres instance if another workspace is squatting on port 5432, leading to silent data loss where seeded test data goes to the wrong database
- Fix approach: One-line update to `.env.example` to match docker-compose.yml's published port; add a startup log line naming the resolved host:port and database so wrong-instance connection is visible in the first second of a run
- Priority: **Medium** (blocks non-expert local setup; prevents data-loss mistakes)
- Status: Environment file permissions prevent automated update; marked as outstanding follow-up in 479c781's investigation log (line 649)

## Known Bugs

**MusicBrainz TLS Handshake Failure (Environmental):**
- Symptoms: `TLSV1 ALERT DECODE_ERROR` or `unexpected eof` when connecting to musicbrainz.org from this developer's WSL2 environment
- Files: None in-repo (environmental, not app code)
- Trigger: Any live HTTP request to `https://musicbrainz.org/ws/2/`
- Workaround: Deezer API works fine; tests mock MusicBrainz with `httptest.Server` so CI is unaffected; if future phases need live MusicBrainz testing on this machine, expect this failure
- Status: **Accepted, Documented** — See `PROJECT.md` Context section and `.planning/WINDOWS.md`

**-race Testing Broken on Windows Dev Machine:**
- Symptoms: `ThreadSanitizer failed to allocate 0x... bytes` on any `go test -race` invocation
- Files: None in-repo (environmental limitation of Windows/WSL2)
- Impact: Data races invisible locally; only caught by CI's `go test -race` on ubuntu-latest (see commit `669ea5d` which fixed two races surfaced by CI)
- Status: **Accepted, Documented** — Noted in `STATE.md` under Phase 01-04 decision log and throughout test-related commits

## Security Considerations

**Database Connection String Secret Redaction:**
- Risk: Connection strings (DSN) embedded in error text can leak Postgres password/credentials to logs or error responses
- Files: `internal/db/migrate.go:77-109` (redactDSN, redactError)
- Mitigation: `redactDSN` uses `pgconn.ParseConfig` (handles both URL-form and keyword/value-form DSN) for safe redaction; `redactError` now handles both forms via regex. Applied on every migration retry log line and final error return
- Regression Test: `internal/db/migrate_test.go` and `internal/db/migrate.go`'s own `dsnFixtures` table
- Status: **FIXED** in commit `6bab55a` — added keyword/value-form password pattern to `redactError`, closing gap CR-01

**Discord Webhook Token Leak:**
- Risk: Discord webhook URLs are secrets (the path IS the token); `net/http` error messages can embed the full request URL
- Files: `internal/discord/client.go:84-90`
- Mitigation: `client.Do()` never wraps the error from `http.Post()` — returns a fixed error string instead, so webhook paths never reach logs
- Status: **Fixed** — Documented in commit message (line references to T-05-01); no webhook URL ever echoes in an error

**Frontend XSS via Untrusted Data:**
- Risk: Event titles, artist names, search results (from MusicBrainz/Deezer) are user-visible and could contain injected scripts
- Files: `web/app/` (React components)
- Mitigation: All display via plain JSX text nodes only — no `dangerouslySetInnerHTML` anywhere (verified by grep in STATE.md decision log)
- Status: **Fixed** — Enforced throughout Phase 06 implementation

**Container Image Supply Chain:**
- Risk: Base images (node, golang, alpine) could be compromised or their tags mutated
- Files: `Dockerfile:22,41,64`
- Mitigation: All base image tags pinned by SHA256 digest (`node:26-alpine3.24@sha256:...`, `golang:1.26.5-alpine3.24@sha256:...`, `alpine:3.24@sha256:...`)
- Status: **Fixed** in commit `6bab55a` (CR-01, IN-01) — every base image digest verified against official sources at implementation time

**Secret Management:**
- Risk: Secrets baked into Docker image layers; accidentally committed to git
- Files: `Dockerfile` (ENV/ARG rules at lines 17-19), `.pre-commit-config.yaml`, `.github/workflows/full-pipeline.yml`
- Mitigation: Dockerfile explicitly documents no configuration may be baked in; `gitleaks` pre-commit hook and CI gate prevent commits; environment variables only (documented in `.env.example`, not committed)
- Status: **Fixed** — Enforced across all phases; pre-commit hook prevents accidental commits; CI backstop

**Non-Root Container User:**
- Risk: Container running as root could allow privilege escalation if image is compromised
- Files: `Dockerfile:77` (addgroup/adduser), line 81 (USER 10001:10001)
- Mitigation: Container runs as numeric UID 10001 (deterministic across rebuilds, documented fixed UID for future SecurityContext use)
- Status: **Fixed** — Implemented Phase 07

## Performance Bottlenecks

**Poll Cycle Hangs if Upstream Timeouts Not Configured:**
- Problem: Prior to commit `479c781`, any upstream (MusicBrainz, Deezer, Postgres) could hang indefinitely and wedge an entire poller cycle
- Files: `internal/db/pool.go:55-60` (ConnectTimeout, PingTimeout), `internal/notifier/notifier.go:130-132` (dbOpTimeout)
- Current Bounds:
  - Connect timeout: 5 seconds (Postgres TCP handshake + TLS + startup)
  - Ping timeout: 2 seconds (acquire-time health check)
  - Max idle connection time: 1 minute (shorter than common NAT/proxy idle-drop windows, prevents reuse of half-open sockets)
  - Notifier query timeout: 10 seconds (per-query deadline)
- Improvement path: If HTTP clients to MusicBrainz/Deezer become a bottleneck, their timeouts are already set at construction time (`internal/musicbrainz/client.go`, `internal/deezer/client.go`); increase their timeout or add backoff/retry logic at the poller level

**Database Connection Pool Exhaustion:**
- Problem: Unbounded pool size or long-lived transactions could exhaust the pool
- Current safeguard: pgxpool defaults (4 max conns for 4 CPUs, 30min idle lifetime, 1h total lifetime, 1min health-check period) plus explicit overrides in `internal/db/pool.go`
- Likelihood: Low — all production queries use `defer rows.Close()` and return quickly; no long-running transactions or streaming responses
- Scaling path: If watchlist grows to 1000+ artists, monitor pool utilization; consider tuning MaxConns or adding a dedicated Postgres replica for read-heavy poll cycles

**Disk Space for Event History:**
- Problem: The `events` table (created in Phase 04, extended Phase 05) has no TTL or archival policy
- Files: `internal/db/migrations/000003_events.up.sql`, `internal/db/migrations/000004_events_display_fields.up.sql`
- Impact: Over months/years, a high-churn watchlist could accumulate millions of event rows
- Improvement path: Add an optional TTL column and a background cleanup job (simplest: a scheduled `DELETE FROM events WHERE notified_at < now() - interval '6 months'`, or partition by date range for larger scale)
- Priority: **Low** (v1 scopes to fresh install; backfill/cleanup belongs to v2+)

## Fragile Areas

**Poller CAS Guard Release:**
- Files: `internal/poller/poller.go:215-219` (mbRunning), `internal/poller/poller_test.go:71-112` (panic recovery test)
- Why fragile: The `defer p.mbRunning.Store(false)` release is correct on every returning path *and* on a panic — but if a future change adds a blocking operation *before* the defer (e.g., logging after the cycle ends) that itself blocks or panics, the guard would remain wedged
- Safe modification: Defer guard release immediately after CAS swap (already done); never move it; document that the guard's correctness depends on defer execution
- Test coverage: `TestPoller_RunMusicBrainzCycle_PanicRecovery` explicitly verifies guard release after a panicking cycle

**JSON Decode Blocks in HTTP Handlers:**
- Files: `internal/httpserver/watchlist.go:90-96` (handleAddWatchlist), `internal/httpserver/watchlist.go:223-229` (handleUpdateWatchlist)
- Why fragile: Both now use a shared `decodeJSONBody` helper (Phase 02-07), but the pattern could be silently reverted if a future developer copies the old code; trailing JSON values now correctly rejected, but losing the EOF check would silently accept them again
- Safe modification: The helper itself (`watchlist.go:269-279`) is the source of truth; always use it; add a lint rule or comment blocking direct `json.Decoder` usage in handlers
- Test coverage: `TestHandleAddWatchlist_RejectsTrailingJSON` and `TestHandleUpdateWatchlist_RejectsTrailingJSON` catch regressions

**Migration Ordering and Idempotency:**
- Files: `internal/db/migrations/000001_init.up.sql` through `000004_events_display_fields.up.sql`, `internal/db/migrate.go:175-195`
- Why fragile: New migrations must be appended in order (numbered 000005_*) and must be idempotent (safe to re-run if interrupted). A malformed migration could corrupt the schema or block boot
- Safe modification: Always add a .up and .down pair; test locally with `make test-integration` (runs migrations from scratch); verify `make down` cleans completely so the next test run starts fresh
- Test coverage: `TestRunMigrations_FromScratchIsIdempotent` recreates the schema twice and asserts they match

**Discord Embed Formatting:**
- Files: `internal/notifier/format.go:60-200` (formatEmbed), `internal/notifier/format_test.go` (comprehensive table-driven tests)
- Why fragile: Discord has strict constraints: 256 runes per title, ~65K per description, 25 fields max, color int must be 0-16777215. A future change that increases title length, adds a field, or uses an invalid color would silently truncate or produce an invalid API request
- Safe modification: Every `formatEmbed` change must be table-driven tested (`format_test.go` shows the pattern); run the suite before committing; never add fields or increase max lengths without verifying against Discord's API docs
- Test coverage: `TestFormatEmbed_*` cases cover title truncation, field count, color clamping, and nilability of optional fields

## Test Coverage Gaps

**Live Discord Rendering:**
- What's not tested: The actual Discord embed rendering (colors, field order, mention suppression) — can only be verified by eye against a real Discord webhook
- Files: `internal/notifier/format.go:*`, any Discord API change
- Risk: A future refactor could silently break rendering without failing any automated test
- Priority: **Low** (Phase 05 UAT closed this gap with manual verification; no automated test can replace it)

**MusicBrainz Edge Cases:**
- What's not tested: Live MusicBrainz API behavior on malformed queries, rate-limit recovery, or undocumented response shapes (tests use httptest.Server with fixtures)
- Files: `internal/musicbrainz/*_test.go` (all mock-based)
- Risk: An MusicBrainz API change could cause silent failures at runtime despite passing tests
- Mitigation: Fixtures are comprehensive and match observed real responses; rate limiting is proven by bounds-checking in tests; if live issues occur, fixtures can be updated to match the new reality
- Priority: **Low** (CI design explicitly rejects live external calls per CLAUDE.md; mocks are sufficient for v1)

**Deezer Pagination Edge Cases:**
- What's not tested: Pagination recovery if the page count changes mid-pagination, or if a later page returns fewer items than expected
- Files: `internal/deezer/albums.go:78-110` (pagination loop), `internal/deezer/albums_test.go` (mock-based)
- Risk: A Deezer API change could cause loops or skipped items
- Mitigation: Fixtures test both multi-page success (3 pages) and single-page success; loop termination condition is page-count based, not item-count based, so a shrinking later page is harmless
- Priority: **Low** (Deezer API is stable; observed behavior across multiple test runs validates the implementation)

**Event Deduplication Under Race Conditions:**
- What's not tested: Two poller cycles running concurrently (one for MusicBrainz, one for Deezer) detecting and inserting the same release simultaneously
- Files: `internal/detection/detector.go:*` (detection logic), `internal/detection/detector_test.go` (unit tests, single-threaded)
- Risk: A race could insert two identical event rows (violating the single-event-per-release invariant)
- Mitigation: Events table has no UNIQUE constraint (events can legitimately repeat if a release is detected multiple ways: new_release + guest_feature), but the `notified_at` field ensures each row notifies exactly once; two concurrent inserts of the identical row are harmless (idempotent); the CAS guards on mbRunning/dzRunning ensure the two sources never run simultaneously per artist
- Priority: **Low** (current architecture (per-artist, sequential) makes the race impossible; relevant only if concurrency is added)

---

*Concerns audit: 2026-08-12*
