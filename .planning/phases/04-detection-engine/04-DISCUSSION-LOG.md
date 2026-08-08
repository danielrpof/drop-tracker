# Phase 4: Detection Engine - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-08
**Phase:** 04-Detection Engine
**Areas discussed:** Deluxe/tracklist-change signal, Guest-feature detection scope, Seen-store & event schema, First-run / seed behavior, Preference filtering scope, Idempotency under partial failure

---

## Deluxe/tracklist-change signal

| Option | Description | Selected |
|--------|-------------|----------|
| Track-count fetch (recommended) | Browse releases within the group and compare track counts | ✓ |
| Title heuristic | Flag releases with "Deluxe"/"Expanded" in the title | |
| You decide | | |

**User's choice:** Track-count fetch (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Any track-count increase | Compare against the highest previously-seen count for the group | ✓ |
| Meaningful increase only | Require a minimum jump or title-keyword match | |
| You decide | | |

**User's choice:** Any track-count increase

| Option | Description | Selected |
|--------|-------------|----------|
| MusicBrainz only (recommended) | Deezer's Tracklist field is a URL, not real data | ✓ |
| Both sources | Fetch Deezer per-album track counts too | |
| You decide | | |

**User's choice:** MusicBrainz only (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Only for already-seen groups (recommended) | A brand-new group is a new-release event, not a tracklist check | ✓ |
| Every group, every cycle | Simpler logic, worse request-volume scaling | |
| You decide | | |

**User's choice:** Only for already-seen groups (recommended)

---

## Guest-feature detection scope

| Option | Description | Selected |
|--------|-------------|----------|
| Same bounded pagination (recommended) | Reuse maxPages=10 x pageSize=100 | ✓ |
| Tighter cap or lookback filter | Stricter limit given higher volume | |
| You decide | | |

**User's choice:** Same bounded pagination (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Position-based (recommended) | Not first in artist-credit list = guest | ✓ |
| Position + title confirmation | Also require "feat."/"ft." in title | |
| You decide | | |

**User's choice:** Position-based (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Same cadence (recommended) | One MusicBrainz cycle fetches both release-groups and recordings | ✓ |
| Separate, slower cadence | A third independent cron entry | |
| You decide | | |

**User's choice:** Same cadence (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| MusicBrainz only (recommended) | Deezer has no track/credit-level fetch capability | ✓ |
| You decide | | |

**User's choice:** MusicBrainz only (recommended)

---

## Seen-store & event schema

| Option | Description | Selected |
|--------|-------------|----------|
| One combined table (recommended) | Event row IS the seen record; unique constraint enforces idempotency | ✓ |
| Two separate tables | Lean seen-store plus richer events table | |
| You decide | | |

**User's choice:** One combined table (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Per-type external ID (recommended) | Release-group/release/recording MBID depending on event type | ✓ |
| Synthetic content hash | Hash artist+type+fields into a fingerprint | |
| You decide | | |

**User's choice:** Per-type external ID (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Add notified_at now (recommended) | Phase 5 reads WHERE notified_at IS NULL | ✓ |
| Separate table in Phase 5 | Phase 5 owns its own notifications table with event_id FK | |
| You decide | | |

**User's choice:** Add notified_at now (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Inline snapshot (recommended) | Title/cover-art/release-date copied in at detection time | ✓ |
| IDs only, re-fetch later | Smaller rows, requires re-query at render/notify time | |
| You decide | | |

**User's choice:** Inline snapshot (recommended)

---

## First-run / seed behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Full event rows, pre-notified (recommended) | notified_at pre-set at seed time | ✓ |
| Dedup keys only, no event row | Thinner seed path, needs a second table | |
| You decide | | |

**User's choice:** Full event rows, pre-notified (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Implicit: zero event rows (recommended) | No new schema column | ✓ |
| Explicit seeded_at column | Clearer signal, handles partial-seed retries | |
| You decide | | |

**User's choice:** Implicit: zero event rows (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Per-source (recommended) | Prevents a flood when a second source is backfilled later | ✓ |
| Global per-artist | Simpler, but risks the flood scenario | |
| You decide | | |

**User's choice:** Per-source (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve history, no reseed (recommended) | Event rows keyed by artist_id, not watchlist id | ✓ |
| Clear history on remove | Adds a deletion side-effect | |
| You decide | | |

**User's choice:** Preserve history, no reseed (recommended)

---

## Preference filtering scope

| Option | Description | Selected |
|--------|-------------|----------|
| Skip at detection time (recommended) | Filtered-out release-types never become event rows | ✓ |
| Always record, filter at notify time | Phase 5 checks live preferences before posting | |
| You decide | | |

**User's choice:** Skip at detection time (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Same as release_types: skip at detection | Consistent behavior across both preference axes | ✓ |
| Different: always record, mute only suppresses the post | Matches NTFY-04's literal wording | |
| You decide | | |

**User's choice:** Same as release_types: skip at detection

---

## Idempotency under partial failure

| Option | Description | Selected |
|--------|-------------|----------|
| Rely on the unique constraint (recommended) | Re-derivable from the next full fetch+diff | ✓ |
| Explicit resume/checkpoint tracking | Tracks last-processed artist, adds new state | |
| You decide | | |

**User's choice:** Rely on the unique constraint (recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| ON CONFLICT DO NOTHING (recommended) | Original snapshot is never overwritten | ✓ |
| ON CONFLICT DO UPDATE (refresh snapshot) | Keeps display data fresh but alters historical record | |
| You decide | | |

**User's choice:** ON CONFLICT DO NOTHING (recommended)

---

## Claude's Discretion

- Exact table/column names and migration numbering beyond what's specified in CONTEXT.md.
- Go project layout for new detection logic (package placement).
- Structured-log field naming for detection log lines (follow existing conventions).
- Whether new MusicBrainz fetch methods live on the existing `Client` type or a new one.

## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.
