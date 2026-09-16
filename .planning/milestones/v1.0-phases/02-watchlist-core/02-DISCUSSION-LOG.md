# Phase 2: Watchlist Core - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-05
**Phase:** 02-Watchlist Core
**Areas discussed:** Artist identity model, Preferences model, Duplicate-add behavior, API resource shape

---

## Artist identity model

| Option | Description | Selected |
|--------|-------------|----------|
| MusicBrainz ID (MBID) required | MBID becomes the stable external key for the whole pipeline | ✓ |
| Either MBID or Deezer ID accepted | Dual identity, more flexible but pushes canonical-ID resolution downstream | |
| Name-only, free text | Simplest, but no stable external key for later diffing | |

**User's choice:** MusicBrainz ID (MBID) required

| Option | Description | Selected |
|--------|-------------|----------|
| Nullable column now | Add `deezer_id` nullable now; Phase 3's Deezer client populates it | ✓ |
| Add it in Phase 3 | Keep schema MBID-only until Phase 3 needs it | |

**User's choice:** Nullable column now

| Option | Description | Selected |
|--------|-------------|----------|
| Separate artists + watchlist tables | Master `artists` data + `watchlist` entry with FK | ✓ |
| Single flat watchlist table | Artist fields inlined into the watchlist row | |

**User's choice:** Separate artists + watchlist tables

| Option | Description | Selected |
|--------|-------------|----------|
| MBID + name both required | Matches a MusicBrainz search result shape | ✓ |
| MBID required, name optional/fetched later | Looser, needs a follow-up lookup Phase 2 doesn't do | |

**User's choice:** MBID + name both required
**Notes:** None — user selected recommended option for all four questions in this area.

---

## Preferences model

| Option | Description | Selected |
|--------|-------------|----------|
| Two distinct axes (recommended) | Release-type filter = catalog scope; mute preference = alert-noise control (event category) | ✓ |
| One unified preferences object | Single structure covering both — simpler surface, blurs "don't track" vs. "track but don't alert" | |

**User's choice:** Two distinct axes

| Option | Description | Selected |
|--------|-------------|----------|
| Postgres text[]/enum array column | One column holding enabled release types; new types added by value not migration | ✓ |
| Boolean columns per type | More explicit/queryable, needs a migration for a 5th type | |

**User's choice:** Postgres text[]/enum array column (release-type filter)

| Option | Description | Selected |
|--------|-------------|----------|
| Postgres text[]/enum array of muted categories | Consistent with release-type filter representation | ✓ |
| Boolean columns per category | Explicit per-category booleans | |

**User's choice:** Postgres text[]/enum array (mute preference)

| Option | Description | Selected |
|--------|-------------|----------|
| Everything on by default (recommended) | New entries start fully watched/unmuted | ✓ |
| Opt-in required | New entries silent until configured | |

**User's choice:** Everything on by default

---

## Duplicate-add behavior

| Option | Description | Selected |
|--------|-------------|----------|
| 409 Conflict (recommended) | Reject re-add with a clear error; standard REST semantics | ✓ |
| Idempotent no-op (200 with existing entry) | Returns existing entry unchanged, silently ignores new preference values | |
| Treat as update to preferences | Re-add overwrites preferences — blurs with dedicated update endpoint | |

**User's choice:** 409 Conflict

| Option | Description | Selected |
|--------|-------------|----------|
| Hard delete (recommended) | Row removed entirely; history is about events, not the watchlist row | ✓ |
| Soft delete (deleted_at column) | Row flagged inactive, preserves preferences if re-added | |

**User's choice:** Hard delete

---

## API resource shape

| Option | Description | Selected |
|--------|-------------|----------|
| Single resource, preferences embedded (recommended) | One `/watchlist` resource covers add/list/update/remove | ✓ |
| Split resources | Separate `/watchlist/{id}/preferences` sub-resource | |

**User's choice:** Single resource, preferences embedded

| Option | Description | Selected |
|--------|-------------|----------|
| Plain JSON objects, no envelope (recommended) | Body is the resource itself, no `{"data": ...}` wrapper | ✓ |
| Enveloped (`{"data": ...}` / `{"error": ...}`) | More structure for future pagination/metadata | |

**User's choice:** Plain JSON objects, no envelope

| Option | Description | Selected |
|--------|-------------|----------|
| JSON `{"error": "message"}` (recommended) | Simple error body + HTTP status code | ✓ |
| RFC 7807 Problem Details | `application/problem+json`, more standards-compliant | |

**User's choice:** JSON `{"error": "message"}`

| Option | Description | Selected |
|--------|-------------|----------|
| No prefix: `/watchlist` (recommended) | Matches Phase 1's `/health` convention | ✓ |
| `/api` prefix: `/api/watchlist` | Namespaces API routes from future embedded SPA assets | |

**User's choice:** No prefix — `/watchlist`

---

## Claude's Discretion

- Exact column names/types beyond what's specified (timestamps, extra artist metadata like image URL/disambiguation comment).
- Enum vs. plain `text[]` with app-level validation for release-type/event-category values.
- Exact Go error-handling/validation-message conventions, following Phase 1's established patterns.

## Deferred Ideas

None — discussion stayed entirely within Phase 2's scope. No scope-creep suggestions arose.
