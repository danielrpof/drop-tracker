# `GET /status` — frozen response contract

This is the **Phase 18 freeze point** for the operator status surface. Phase 19 types the
SPA API client (`web/app/lib/api.ts`) directly against the JSON body described here, so
this document and the `json:"..."` tags on the response structs in
`internal/httpserver/status.go` must stay character-identical. Renaming or retyping a key
here is a **contract change**: it requires a paired update to the SPA API client in the
same or a following phase, not a silent refactor.

The handler is `handleStatus` in `internal/httpserver/status.go`; the shape is produced
from `internal/pollruns.Snapshot()` plus the watchlist count, the schema-version seam,
the injected build version, and the configured poll interval.

## Authentication and status codes

`/status` is registered inside `registerDataRoutes`, so on a passphrase-gated instance it
sits behind `gate.Authenticate` and carries the `X-Instance-Gated` response header exactly
as `/events` does. It is a `GET`, so the CSRF-header requirement does not apply.

| Condition | Status | Body |
|-----------|--------|------|
| Gated instance, no valid session cookie | `401` | the gate's own error body — never a partial `/status` payload |
| Server built without the status option (unreachable from a real `cmd/server` binary) | `503` | `{"error":"status not available"}` |
| Watchlist-count read fails | `500` | `{"error":"internal error"}` — the raw cause goes to `httplog.SetAttrs` under `status_watchlist_error`, never the body |
| Schema-version read fails | `200` | full body with `instance.schema_applied` `null`; raw cause logged under `status_schema_error` |
| Otherwise (including a fresh instance with no recorded cycles) | `200` | the full envelope below |

The schema read and the watchlist count are deliberately **not** symmetric. `/status` is an
operator panel, not a probe: a momentary database blip degrades `instance.schema_applied`
to `null` and still returns `200`, because the run history and the counts are the payload's
point and `/ready` is the endpoint that turns red on a database blip. A watchlist-count
failure is a `500` because there is no meaningful partial answer without it.

No field on any response path — at any depth — carries a DSN, a webhook URL, a filesystem
path, or a raw driver error string. The envelope declares no `error`, `detail`, `message`,
or `last_error` key anywhere in the `200` shape.

## Envelope

| JSON path | Type | Notes |
|-----------|------|-------|
| `poll_interval_seconds` | int | The configured poll interval truncated to whole seconds. `900` for the 15-minute default; `90` for a 90-second interval. |
| `watchlist_size` | int | From the `CountWatchlist` query — a `count(*)`, not the length of the list query's result. |
| `instance.app_version` | string | Short commit SHA, at most 12 characters, on a CI-built image; `dev` on a flagless local build. |
| `instance.schema_applied` | int or null | Live read of the applied migration version. `null` only when the database is unreachable at request time. |
| `instance.schema_expected` | int | The boot-time expected migration version. Same key names as `/ready`'s body — one schema-version concept, not two. |
| `sources` | object | Keyed by source name. Always carries exactly `musicbrainz` and `deezer`, including on a fresh instance. |
| `sources.<name>.last_run` | run object or null | `null` until that source records its first cycle. Byte-identical to `history[0]` when present. |
| `sources.<name>.history` | array of run objects | Newest-first. An empty array — never `null` — when nothing has been recorded. At most 50 entries. |
| `sources.<name>.last_skipped_at` | RFC3339 string or null | `null` until that source is first overlap-skipped. |
| `sources.<name>.consecutive_skips` | int | Reset to `0` by the next recorded run. |

## Run object

Used by both `sources.<name>.last_run` and every `sources.<name>.history` element.

| JSON key | Type | Notes |
|----------|------|-------|
| `cycle_id` | string | The existing per-cycle correlation id (`<source>-<n>`). |
| `started_at` | RFC3339 string | |
| `finished_at` | RFC3339 string | |
| `duration_ms` | int | |
| `artists_checked` | int | Entries dispatched to a worker. |
| `artists_skipped` | int | Entries skipped before dispatch. |
| `artists_errored` | int | |
| `events_recorded` | int | Zero until Phase 18.1 populates it. |
| `outcome` | string | Closed set: `ok`, `error`, `cancelled`. |
| `summary` | string | Composed by the store from the counts and the outcome — never derived from an error. |

The run object has **no** `source` key — the source is the map key in `sources`.

## Example — fresh instance (no cycles recorded)

```json
{
  "poll_interval_seconds": 900,
  "watchlist_size": 12,
  "instance": {
    "app_version": "dev",
    "schema_applied": 7,
    "schema_expected": 7
  },
  "sources": {
    "deezer": {
      "last_run": null,
      "history": [],
      "last_skipped_at": null,
      "consecutive_skips": 0
    },
    "musicbrainz": {
      "last_run": null,
      "history": [],
      "last_skipped_at": null,
      "consecutive_skips": 0
    }
  }
}
```

## Example — instance with history

```json
{
  "poll_interval_seconds": 900,
  "watchlist_size": 12,
  "instance": {
    "app_version": "a1b2c3d4e5f6",
    "schema_applied": 7,
    "schema_expected": 7
  },
  "sources": {
    "deezer": {
      "last_run": null,
      "history": [],
      "last_skipped_at": "2026-09-09T11:45:00Z",
      "consecutive_skips": 2
    },
    "musicbrainz": {
      "last_run": {
        "cycle_id": "musicbrainz-42",
        "started_at": "2026-09-09T12:00:00Z",
        "finished_at": "2026-09-09T12:00:03Z",
        "duration_ms": 3120,
        "artists_checked": 12,
        "artists_skipped": 0,
        "artists_errored": 1,
        "events_recorded": 3,
        "outcome": "ok",
        "summary": "ok — 12 checked, 1 errored, 3 events"
      },
      "history": [
        {
          "cycle_id": "musicbrainz-42",
          "started_at": "2026-09-09T12:00:00Z",
          "finished_at": "2026-09-09T12:00:03Z",
          "duration_ms": 3120,
          "artists_checked": 12,
          "artists_skipped": 0,
          "artists_errored": 1,
          "events_recorded": 3,
          "outcome": "ok",
          "summary": "ok — 12 checked, 1 errored, 3 events"
        }
      ],
      "last_skipped_at": null,
      "consecutive_skips": 0
    }
  }
}
```

## Changing this contract

Any change to a key name or type in the tables above is a published-contract change. It
must land together with the matching change to `internal/httpserver/status.go`'s `json`
tags and to the SPA API client that Phase 19 builds against this document. The
contract-drift check in `18-04-PLAN.md`'s verification (`every handler json tag present
here and vice versa`) is what catches a one-sided edit.
