# Phase 3: External Clients & Search - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-06
**Phase:** 3-External Clients & Search
**Areas discussed:** Search-proxy contract, Scheduler scope in Phase 3 vs 4, Rate-limiting & concurrency shape, MusicBrainz entity scope

---

## Search-proxy contract

| Option | Description | Selected |
|--------|-------------|----------|
| Combined endpoint | GET /search?q=... hits both sources server-side, returns a single response with results tagged by source | ✓ |
| Separate endpoints | /search/musicbrainz and /search/deezer as distinct resources | |

**User's choice:** Combined endpoint

| Option | Description | Selected |
|--------|-------------|----------|
| No merge — show both, tagged | Return MusicBrainz and Deezer results as separate lists/tagged entries; no fuzzy-matching | ✓ |
| Fuzzy-merge by name | Attempt to match same-name results across sources into one combined entry | |

**User's choice:** No merge — show both, tagged

| Option | Description | Selected |
|--------|-------------|----------|
| Partial results + flag | Return whatever succeeded plus a per-source status/error flag | ✓ |
| Fail the whole request | Any source failure returns an overall error | |

**User's choice:** Partial results + flag
**Notes:** All three questions answered with the recommended option; no additional follow-up.

---

## Scheduler scope in Phase 3 vs 4

| Option | Description | Selected |
|--------|-------------|----------|
| Wire cron now, log-only fetch | robfig/cron wired in Phase 3, poll handler logs a structured line per artist/source; Phase 4 replaces the log with real diff logic | ✓ |
| Defer scheduler entirely to Phase 4 | Phase 3 only builds clients + rate limiters; cron wiring happens in Phase 4 | |

**User's choice:** Wire cron now, log-only fetch

| Option | Description | Selected |
|--------|-------------|----------|
| Poller reads watchlist.Store directly | Reuses Phase 2's existing list query and DB seam | ✓ |
| New dedicated poll-list query | A separate sqlc query optimized for poll iteration | |

**User's choice:** Poller reads watchlist.Store directly

| Option | Description | Selected |
|--------|-------------|----------|
| Skip Deezer poll for that artist | If deezer_id is null, only poll MusicBrainz for that artist | ✓ |
| Resolve deezer_id via search first | Poll cycle does a Deezer name-search to backfill deezer_id | |

**User's choice:** Skip Deezer poll for that artist
**Notes:** No follow-up questions requested — moved directly to next area.

---

## Rate-limiting & concurrency shape

| Option | Description | Selected |
|--------|-------------|----------|
| Sequential loop + shared limiter.Wait() | One rate.Limiter per client, poll cycle loops artists one at a time | ✓ |
| Bounded-concurrent workers + shared limiter | A small worker pool pulls artists off a channel, still gated by the limiter | |

**User's choice:** Sequential loop + shared limiter.Wait()

| Option | Description | Selected |
|--------|-------------|----------|
| Independent per-source cycles | Two separate cron jobs/goroutines, each with its own rate.Limiter | ✓ |
| Single combined cycle | One loop per artist does MusicBrainz then Deezer sequentially | |

**User's choice:** Independent per-source cycles

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — simple in-process guard now | Atomic/mutex "cycle in progress" flag per source, skips overlapping cron ticks | ✓ |
| No — defer to Phase 4 | Phase 4 adds the overlap guard alongside DTCT-05 | |

**User's choice:** Yes — simple in-process guard now
**Notes:** All recommended options selected; no additional follow-up requested.

---

## MusicBrainz entity scope

| Option | Description | Selected |
|--------|-------------|----------|
| Release-groups only | Phase 3 poller fetches release-groups; recordings/artist-credit fetch deferred to Phase 4 | ✓ |
| Release-groups + recordings | Both fetch paths built now so Phase 4 only adds diff logic | |

**User's choice:** Release-groups only

| Option | Description | Selected |
|--------|-------------|----------|
| Artist search endpoint | MusicBrainz's /ws/2/artist?query= search endpoint | ✓ |
| Artist + release-group search combined | Search both artists and release-groups in one call | |

**User's choice:** Artist search endpoint

| Option | Description | Selected |
|--------|-------------|----------|
| Deezer artist albums/releases only | Deezer's artist albums endpoint by deezer_id, no track-level data | ✓ |
| Deezer albums + track-level data | Also fetch track listings per album now | |

**User's choice:** Deezer artist albums/releases only
**Notes:** After this area, user confirmed ready to write context — no further gray areas explored.

---

## Claude's Discretion

- Exact `GET /search` query-param shape (result limit, pagination if any) and per-request timeout values for the MusicBrainz/Deezer HTTP clients.
- Whether the overlap guard is a `sync.Mutex`, `atomic.Bool`, or similar.
- Exact structured-log field names for the log-only poll handler.
- Go project layout for the two new client packages (already implied by CLAUDE.md's package naming).

## Deferred Ideas

- Distributed/multi-instance scheduler locking (Postgres advisory lock) — noted in tech stack doc as a later consideration, not this phase.
- Deezer name-search fallback to backfill `deezer_id` — rejected for the poll cycle, could resurface as a small enhancement later.
- Combined artist+release-group search for the search-proxy — rejected, no current requirement drives it.
