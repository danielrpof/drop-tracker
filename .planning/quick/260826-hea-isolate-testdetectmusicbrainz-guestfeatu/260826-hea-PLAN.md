---
phase: quick-260826-hea
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/detection/detector_test.go
autonomous: true
requirements:
  - QUICK-260826-hea
user_setup: []

estimate:
  tokens: 24000
  raw_tokens: 12000
  tasks: 1
  confidence: low

must_haves:
  truths:
    - "Running internal/detection's test suite against the shared fixture Postgres leaves every pre-existing pending events row in the default (public) schema with notified_at still NULL — the live app's real Discord notifications survive a local `go test ./...` run."
    - "TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier still passes unchanged in substance: exactly 1 request reaches the httptest webhook, no request body contains the muted recording's title, and the new_release row's notified_at is non-NULL afterward."
    - "The isolated pool's schema name does not collide with internal/notifier's `notifier_test` schema, nor with any other schema name in use in this repo."
    - "Every other DB-backed test in internal/detection keeps using the shared pool constructor — exactly one call site changes."
  artifacts:
    - "internal/detection/detector_test.go (modified — one pool constructor call site, plus its top-of-file convention comment)"
  key_links:
    - "TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier -> testutil.NewIsolatedTestPool(t, \"detection_notify_test\")"
    - "the isolated pool -> sqlc.New(pool) inside BOTH detection.New(...) and notifier.New(...), plus the three raw pool.QueryRow calls, all resolving unqualified table names through the DSN's search_path parameter"
    - "notifier.NotifyPending's global unfiltered ListUnnotified scan -> bounded to the dedicated schema, never the public schema the live docker-compose app writes into"
---

<objective>
Isolate `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` (internal/detection/detector_test.go) onto a dedicated Postgres schema via the already-established `testutil.NewIsolatedTestPool` fixture, so its real `notifier.NotifyPending` call can no longer sweep up and mark the live dev app's real pending Discord notifications as sent.

Purpose: this is the last test in the repo that makes a real `NotifyPending` call against the shared fixture's default (`public`) schema. `NotifyPending`'s underlying `ListUnnotified` query is a deliberately global, unfiltered scan across every artist (documented as D-06 in `internal/testutil/postgres.go`) — correct production behavior for draining the whole outbox, but destructive when the same schema also holds the live docker-compose app's real rows. A developer running `go test ./...` or `make test-integration` while `docker compose up` is running silently loses real Discord alerts: the test's fake `httptest` webhook "delivers" them and stamps `notified_at`, so the real notifier skips them forever. Root-caused during quick task 260826-gj8 and explicitly left unfixed there as out of scope.

Output: one changed source file, one changed pool constructor call site, plus an updated top-of-file convention comment. Zero production code changes.

Scope discipline — do NOT touch:
- `notifier.NotifyPending` / `ListUnnotified`. The unfiltered global scan is intentional, correct production behavior. This is a test-fixture isolation fix, not a production behavior change.
- Any other test in `internal/detection/detector_test.go` (43 other `NewTestPool` call sites), nor `deezer_test.go` / `baseline_test.go` / `filter_test.go`. They only assert on their own `artist_id`-scoped rows via `WHERE artist_id = $1`, never a global scan — they do not have this problem.
- `internal/notifier/notifier_test.go` — already correct; it is the reference pattern being copied, nothing more.
- `docker-compose.yml` / `Makefile`. The shared-database-by-design dev setup is intentional and documented; the right layer to fix this at is the test fixture, per the existing `NewIsolatedTestPool` precedent.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.claude/CLAUDE.md

@internal/detection/detector_test.go
@internal/testutil/postgres.go
@internal/notifier/notifier_test.go
</context>

<interface_context>
Established facts, already verified during planning — do not re-derive:

- `testutil.NewIsolatedTestPool(t *testing.T, schema string) *pgxpool.Pool` — `internal/testutil/postgres.go:119`. Drops and recreates `schema` (CASCADE, if-exists guarded), runs the embedded migrations against only that schema, and returns a pool built from a DSN carrying `search_path=<schema>`. pgx applies unrecognised URL query parameters as Postgres runtime parameters on every connection the pool opens, so ALL unqualified table references resolve inside the schema — both `sqlc.New(pool)`-generated statements and raw `pool.QueryRow(...)` SQL. No call-site change beyond the constructor is required.
- Cleanup ordering is safe. `NewIsolatedTestPool` registers three `t.Cleanup`s in order (close sql.DB, drop schema, close pool). `insertTestArtist` registers its `DELETE FROM artists` cleanup afterward, and `t.Cleanup` runs LIFO — so the DELETE runs first, while the pool is still open and the schema still exists. Nothing to reorder.
- The target test's helper usage is already pool-parameterised: `insertTestArtist(t, pool, ...)` (`detector_test.go:162`) takes the pool as an argument and issues unqualified raw SQL. `testMBID` and `testLogger` touch no database.
- Schema-name collision check (run during planning, current as of this plan): `NewIsolatedTestPool` is called in exactly one package repo-wide, `internal/notifier`, with the single schema name `"notifier_test"`. `internal/detection` has zero existing calls. `"detection_notify_test"` is therefore free and satisfies the constructor's own doc-comment rule that different packages must pass different schema names.
- Current occurrence counts in `internal/detection/detector_test.go`: 44 lines matching `testutil.NewTestPool(t)`, 0 matching `NewIsolatedTestPool`. After this change: 43 and 1.
- The target call site is `detector_test.go:1168`, the first line of `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` (function opens at line 1167). The test does not call `t.Parallel()`.
- Dev fixture coordinates (`Makefile:9`, `docker-compose.yml`): user `drop_tracker`, database `drop_tracker`, host port `5432`, compose service name `postgres`.
- `events` columns relevant to the sentinel probe (`internal/db/migrations/000003_events.up.sql`): `artist_id`, `source`, `event_type`, `external_id`, `title` NOT NULL, `artist_name` NOT NULL, `notified_at` nullable; unique constraint `events_dedup_key (event_type, source, external_id)`. `artists.mbid` is UNIQUE and `events.artist_id` is `ON DELETE CASCADE`.
- `go test -race` is unusable on this Windows dev machine (ThreadSanitizer allocation failure — pre-existing, documented in STATE.md under Phase 11.1-04). Run plain `go test` for verification here; CI still runs `-race`.
</interface_context>

<tasks>

<task type="auto">
  <name>Task 1: Route the muted-guest-feature notifier test onto a dedicated Postgres schema</name>

  <precondition>The docker-compose Postgres fixture is up and reachable on localhost:5432 with the public schema already migrated (`docker compose up -d --wait postgres`, or `make db-up`). The sentinel probe in `<verify>` writes to and reads from that live public schema; without it the probe cannot run and the fix cannot be proven.</precondition>

  <files>internal/detection/detector_test.go</files>

  <behavior>
    Observable contract this change must establish — prove it with the RED/GREEN sentinel probe, not by inspection:
    - RED (current, broken): with a sentinel `events` row sitting `notified_at IS NULL` in the public schema, running only `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` flips that sentinel's `notified_at` to non-NULL. The probe query returns `f`.
    - GREEN (after the change): the identical run leaves the sentinel's `notified_at` NULL. The probe query returns `t`.
    - Invariant across both: the test itself passes. Its three assertions — exactly 1 request to the httptest server, no request body containing `Suppressed Guest Track`, and the new_release row's own `notified_at` non-NULL — are unchanged and must still hold, now satisfied entirely from rows inside the dedicated schema.
  </behavior>

  <action>
    Work in `internal/detection/detector_test.go`. Two edits, nothing else.

    First, run the RED half of the sentinel probe in `<verify>` against the UNMODIFIED file and record that it returns `f`. This is the whole justification for the change — skipping it means shipping a fix with no evidence the bug was ever reachable. If it unexpectedly returns `t`, stop and report: the diagnosis does not reproduce on this machine and the plan needs revisiting, not a blind edit.

    Edit 1 — the fix. In `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` (function opens at line 1167), replace the shared-fixture pool constructor on its first body line (line 1168) with `testutil.NewIsolatedTestPool(t, "detection_notify_test")`. Assign to the same `pool` variable. Change nothing else inside the function: `insertTestArtist(t, pool, ...)`, `detection.New(sqlc.New(pool), ...)`, `notifier.New(sqlc.New(pool), ...)`, and all three raw `pool.QueryRow(ctx, ...)` calls (the two count queries, the event-id select, and the notified_at select) already resolve their unqualified table names through the returned pool's `search_path`-scoped DSN — see `<interface_context>`. Do not qualify any table name with a schema prefix; do not add a `SET search_path` statement.

    The schema literal must be exactly `detection_notify_test` — distinct from `internal/notifier`'s `notifier_test`, per `NewIsolatedTestPool`'s own doc-comment requirement that different packages use different schema names.

    Edit 2 — the convention comment. Two comments now state a convention this change makes an exception to; bring both current so the next reader is not misled:
    (a) The file's top-of-file package comment (lines 3-8) asserts that every test in the file shares one database. Amend it to record the single exception: this one test needs a dedicated schema because it makes a real `NotifyPending` call, whose `ListUnnotified` is a global unfiltered scan (D-06) that would otherwise reach both other packages' rows AND the live dev app's real pending notifications sitting in the same default schema. Model the wording on `internal/notifier/notifier_test.go`'s own top-of-file explanation (lines 9-17), which documents this exact reasoning for that package.
    (b) The target test's own doc comment (lines 1159-1166) explains what the test proves; append a sentence naming why its pool is the isolated one.

    Constraint on comment wording, so the count gates in `<verify>` stay meaningful: when either comment refers to the shared-fixture constructor, name it without its call parentheses and argument (the existing line 6 already does this correctly). Do not write the constructor's full call form inside a comment.

    Then run the GREEN half of the probe and confirm `t`. Clean up the sentinel rows in the same pass — leaving a fake artist in the dev database is exactly the class of pollution this task exists to eliminate.

    No production file is touched. No new dependency, no new helper, no new test function.
  </action>

  <verify>
    <automated>
# --- Sentinel probe: seed a pending row in the SHARED public schema, simulating a live-app notification ---
# (run this whole block once BEFORE the edit expecting 'f', once AFTER expecting 't')
docker compose exec -T postgres psql -U drop_tracker -d drop_tracker -v ON_ERROR_STOP=1 \
  -c "INSERT INTO artists (mbid, name) VALUES ('hea-sentinel-mbid', 'HEA Sentinel Artist') ON CONFLICT (mbid) DO NOTHING;" \
  -c "INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name, notified_at) SELECT id, 'musicbrainz', 'new_release', 'hea-sentinel-ext', 'HEA Sentinel Release', 'HEA Sentinel Artist', NULL FROM artists WHERE mbid = 'hea-sentinel-mbid' ON CONFLICT ON CONSTRAINT events_dedup_key DO UPDATE SET notified_at = NULL;"

# Run ONLY the target test (no -race: unusable on this Windows box, see interface_context)
TEST_DATABASE_URL="postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable" \
  go test ./internal/detection/ -run '^TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier$' -count=1 -v

# The proof. Must print 'f' BEFORE the edit (bug reproduces) and 't' AFTER (bug fixed).
docker compose exec -T postgres psql -U drop_tracker -d drop_tracker -tAc \
  "SELECT notified_at IS NULL FROM events WHERE external_id = 'hea-sentinel-ext';"

# Tidy up the sentinel (CASCADE removes the events row with the artist)
docker compose exec -T postgres psql -U drop_tracker -d drop_tracker -v ON_ERROR_STOP=1 \
  -c "DELETE FROM artists WHERE mbid = 'hea-sentinel-mbid';"

# --- Call-site gates (comment lines stripped first; bare counts on unfiltered files are not trustworthy) ---
# Exactly 1 isolated-pool call site in the package:
grep -rhv '^\s*//' internal/detection/*_test.go | grep -c 'NewIsolatedTestPool(t, "detection_notify_test")'   # must be 1

# Exactly 43 shared-pool call sites remain in detector_test.go (was 44 — one and only one converted):
grep -v '^\s*//' internal/detection/detector_test.go | grep -c 'testutil\.NewTestPool(t)'                     # must be 43

# Schema name is unique per package across the repo (must list exactly two distinct names):
grep -rho 'NewIsolatedTestPool(t, "[a-z_]*")' internal/ | sort -u                                             # detection_notify_test + notifier_test only

# Blast radius: detector_test.go is the only changed source file
git diff --name-only -- ':!.planning'                                                                          # must be exactly internal/detection/detector_test.go

# --- Full suite still green ---
TEST_DATABASE_URL="postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable" \
  go test ./internal/detection/ ./internal/notifier/ -count=1
go build ./... && go vet ./...
    </automated>
  </verify>

  <done>
    - The sentinel probe returned `f` before the edit and `t` after — the loss of a real pending notification is demonstrated, then demonstrably gone.
    - `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` passes, still asserting exactly 1 webhook request, no `Suppressed Guest Track` in any body, and its own new_release row acked.
    - `internal/detection` and `internal/notifier` suites both pass; `go build ./...` and `go vet ./...` are clean.
    - Counts are exactly 1 isolated call site and 43 shared call sites in `detector_test.go`; the repo's isolated-schema names are exactly `detection_notify_test` and `notifier_test`.
    - `git diff --name-only` (excluding `.planning/`) lists only `internal/detection/detector_test.go`.
    - No sentinel rows remain in the dev database.
    - Both amended comments explain the exception and its D-06 reason; neither writes the shared constructor in full call form.
  </done>

  <reversibility rating="reversible">Single test-file call-site swap with no production surface; reverting is a one-line git revert and the dedicated schema is dropped by the fixture's own t.Cleanup.</reversibility>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| test process -> shared dev Postgres | The `go test` binary and the live docker-compose `app` service write to the same database and, today, the same `public` schema. A test's writes cross into production-shaped data. |
| test process -> fake httptest webhook | Real notifier code paths "deliver" real pending events to a throwaway in-process server, then durably ack them in the database. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-HEA-01 | Tampering | `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` -> `events.notified_at` in the public schema | high | mitigate | Route the test's real `NotifyPending` call onto the dedicated `detection_notify_test` schema via `testutil.NewIsolatedTestPool`, so `ListUnnotified`'s global unfiltered scan (D-06) is structurally unable to observe or mutate live app rows. Proven by the RED/GREEN sentinel probe. |
| T-HEA-02 | Denial of Service | live Discord notification pipeline | high | mitigate | Same control. Pre-fix, a local test run silently consumed real pending alerts — they were acked as sent but delivered only to an httptest server, so the real notifier skipped them permanently and no error surfaced anywhere. |
| T-HEA-03 | Tampering | cross-package test schema collision | low | mitigate | Schema literal `detection_notify_test` verified distinct from the only other in-use name (`notifier_test`), satisfying `NewIsolatedTestPool`'s documented per-package uniqueness rule. Gated by the `sort -u` check in `<verify>`. |
| T-HEA-04 | Tampering | sentinel probe rows left in the dev database | low | mitigate | Probe cleanup (`DELETE FROM artists WHERE mbid = 'hea-sentinel-mbid'`, cascading to the event row) is part of `<verify>` and an explicit acceptance criterion. |
| T-HEA-SC | Tampering | npm/pip/cargo installs | low | accept | No package-manager installs in this task — zero new dependencies, zero `go get`, zero `npm install`. The package-legitimacy gate has no applicable surface. |
</threat_model>

<verification>
1. RED/GREEN sentinel probe against the live public schema: `f` before, `t` after.
2. Target test passes in isolation and within its package suite.
3. `internal/notifier` suite passes — confirms no schema collision between `detection_notify_test` and `notifier_test`.
4. `go build ./...` and `go vet ./...` clean.
5. Call-site counts: 1 isolated, 43 shared in `detector_test.go`.
6. `git diff --name-only -- ':!.planning'` lists only `internal/detection/detector_test.go` — no production file, no other test file, no Makefile/compose change.
7. Sentinel rows removed from the dev database.
</verification>

<success_criteria>
A developer can run `go test ./...` or `make test-integration` on a machine where `docker compose up` is simultaneously running the live app, and every real pending Discord notification in the database is still pending afterward — while `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` continues to prove exactly what it proved before (a muted guest_feature never reaches the notifier). One test file changed, one pool constructor call site converted, no production behavior altered.
</success_criteria>

<output>
Create `.planning/quick/260826-hea-isolate-testdetectmusicbrainz-guestfeatu/260826-hea-SUMMARY.md` when done.
</output>
