# Phase 18: Backend — Readiness, Poll-Run History & Status API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-09
**Phase:** 18-backend-readiness-poll-run-history-status-api
**Areas discussed:** /ready ready-condition & 503 body, skipped_overlap / cancelled run rows, /status contract completeness, History depth & what the operator reads

---

## /ready ready-condition & 503 body

### Ready condition

| Option | Description | Selected |
|--------|-------------|----------|
| `applied >= expected && !dirty` | Ready as long as the schema is at or ahead of the binary's expected max and not dirty. Matches Phase 16's ahead-of-source guard; a rolled-back binary serving a newer additive schema reports ready instead of flapping the Phase 17 deploy gate. | ✓ |
| strict `applied == expected` | Ready only on an exact match. Simpler, but a healthy rolled-back instance reports not-ready permanently and flaps the deploy gate. | |

### 503 body detail

| Option | Description | Selected |
|--------|-------------|----------|
| Machine reason enum | Fixed vocabulary `db_unreachable` / `schema_behind` / `schema_dirty`; no DSN/driver/paths. Lets a monitor or the Phase 17 gate distinguish causes without logs. | ✓ |
| Generic "not ready" | One opaque string; real cause to logs only. Leakproof by construction but forces a log dive. | |

### 200 body content

| Option | Description | Selected |
|--------|-------------|----------|
| `status + applied + expected` | `{status, schema_applied, schema_expected}`; both numbers so a reader (or Phase 19's bare-fetch readiness badge) sees ahead/behind at a glance. 503 mirrors the shape + reason. | ✓ |
| `status + applied` only | Just the applied version on 200; expected is implicit. Leaner but Phase 19 can't show "schema ahead" without also hitting /status. | |
| Planner picks exact keys | Lock semantics, leave JSON key names to implementation. | |

**User's choice:** `>=` && `!dirty`; machine reason enum; 200 body carries status + applied + expected.
**Notes:** Key names are locked because Phase 19's readiness badge parses this body directly.

---

## skipped_overlap / cancelled run rows

### Reconciling RUN-02 with the burst-eviction risk

| Option | Description | Selected |
|--------|-------------|----------|
| Coalesce to one skip row per gap | At most one `skipped_overlap` row between two real runs; a repeat skip bumps a counter on it. Operator still sees skips happened; a 10-min overrun can't flush real runs. | ✓ |
| Separate budget for skip rows | Prune keeps last N non-skip runs unconditionally + a small separate cap for skip rows. One row per skipped tick, but they can't evict real runs. | |
| Amend RUN-02 — no row for skips | Skipped ticks write nothing; the WARN log is the only record. Requires editing REQUIREMENTS.md. | |

### Cancelled-cycle row

| Option | Description | Selected |
|--------|-------------|----------|
| `context.WithoutCancel` + short timeout | Recorder call detaches from the cancelled context so the row lands during graceful shutdown, bounded so a hung DB can't extend shutdown. | ✓ |
| Best-effort, drop if shutdown is tight | Try to record but skip silently if the shutdown window is closing. | |

**User's choice:** Coalesce to one skip row per gap; record cancelled rows via `context.WithoutCancel` + short timeout.
**Notes:** Coalescing moves the skip instrumentation point to where `ErrCycleInProgress` is observable and makes the recorder's skip write an upsert. `skipped_overlap` must be in the inline CHECK constraint.

---

## /status contract completeness

### Where app version / schema version / db-reachable come from

| Option | Description | Selected |
|--------|-------------|----------|
| Add an `instance` object to /status | `/status` grows `{instance:{app_version, schema_applied, schema_expected}}`; db-reachable implicit. System view is one fetch. | ✓ |
| /status stays minimal; Phase 19 composes | `/status` carries only STAT-01's four things; Phase 19 gets schema + db-reachable from a bare /ready and app version elsewhere. Two fetches. | |

### How the app learns its own version

| Option | Description | Selected |
|--------|-------------|----------|
| Inject via `-ldflags -X` at build | Dockerfile/CI passes the svu tag into a main var. Deterministic, matches the published image tag. Small Dockerfile + full-pipeline.yml change. | ✓ |
| `debug.ReadBuildInfo()` VCS stamp | Runtime read of vcs.revision/time; no build change but needs `.git` in the Docker context (absent today) or reports `(devel)`. Reports a SHA, not the tag. | |
| Defer — show schema version only | Drop "app version" from scope this milestone. No build-pipeline change. | |

**User's choice:** Add an `instance` object to `/status`; inject app version via `-ldflags -X` at build time.
**Notes:** Adds a Dockerfile + `full-pipeline.yml` change to Phase 18 (shared-file hazard). Open research item flagged for the planner: the svu tag is computed in the `release` job, which runs after `build-scan` builds the image — settle how the version reaches the image build without ballooning the phase.

---

## History depth & what the operator reads

### Retention N per source

| Option | Description | Selected |
|--------|-------------|----------|
| 50 per source | ~100 rows total; ~1–2 days at typical intervals. Conservative; smaller table, faster prune. | ✓ |
| 100 per source | ~200 rows total; several days of history. Generous headroom. | |

### The per-run "outcome summary" field

| Option | Description | Selected |
|--------|-------------|----------|
| Stored string, composed from counts at insert | Deterministic line built purely from count/enum values, never from error text. Stored so it's stable and greppable; the "why" of an error stays in logs. | ✓ |
| No summary column — compose at render time | Store only columns; /status and the UI build the sentence. One less column; may need RUN-01 wording tweaked. | |

**User's choice:** N = 50 per source; stored `summary` string composed deterministically from counts + outcome enum at insert.
**Notes:** RUN-01's "short outcome summary" is satisfied by the stored column — no requirement amendment needed.

---

## Claude's Discretion

- **`events_recorded` source** — widen the `EventRecorder` seam to `(int, error)` vs a downstream `SELECT count(*)`. Research leans seam-widening; downstream is safe under the per-source overlap guard. Planner picks one explicitly; the rejected option is OBS-05.
- Exact JSON encoding of `poll_interval`, precise key names inside `runs` / `history`, and whether `instance` reuses `/ready` key names.
- Whether `ExpectedSchemaVersion` is an exported `internal/db` func or a boot-time value passed down.

## Deferred Ideas

- Auto-refresh / live System view (OBS-01), manual "poll now" (OBS-02), paginated history beyond N (OBS-03), poll-failure alerting (OBS-04) — all already deferred.
- OBS-05 — the non-chosen `events_recorded` approach.
- A dedicated `/version` endpoint — rejected in favour of `/status`'s `instance` block.
- Promoting `redactDSN` / `redactError` to a shared exported package — only if a free-text error must be stored; D-09 avoids that.
