# Architecture Research

**Domain:** Feature-integration research for v1.6 Watchlist Organization (tags, notes, search/sort/filter, tag-filtered History, tagged notifications, bulk edit, paste-a-list bulk add)
**Researched:** 2026-09-22
**Confidence:** HIGH — every claim below is grounded in the actual current code (file paths cited), not general framework advice. The only genuinely open calls are the specific rune caps for notes/tags and the exact route names, both flagged explicitly.

## Standard Architecture

### System Overview — where v1.6 lands on the existing layered monolith

```
┌──────────────────────────────────────────────────────────────────────────┐
│  HTTP API Layer (internal/httpserver)                                    │
│  existing: /health /search /watchlist /events /settings/notifications    │
│  NEW: GET/PATCH/DELETE /tags, POST/DELETE /watchlist/{id}/tags,          │
│       POST /watchlist/bulk/{tags,preferences,remove}, POST /watchlist/   │
│       bulk/add (confirm step), PATCH /watchlist/{id} gains `notes`       │
│       MODIFIED: GET /events gains `tag_id` filter                        │
└───────┬───────────────┬───────────────┬───────────────┬──────────────────┘
        │               │               │               │
┌───────▼─────┐ ┌───────▼──────┐ ┌──────▼───────┐ ┌─────▼────────────┐
│ watchlist    │ │ NEW internal │ │ events       │ │ search (existing,│
│ .Service     │ │ /tags.Service│ │ .Service     │ │ unchanged)       │
│ (+notes,     │ │ Store/Rename/│ │ (+tag_id     │ │ GET /search fans │
│  +enrichment)│ │ Delete/      │ │  filter)     │ │ out to MB/Deezer,│
│              │ │ GetOrCreate/ │ │              │ │ shared rate      │
│              │ │ Attach/Detach│ │              │ │ limiters         │
└───────┬──────┘ └──────┬───────┘ └──────┬───────┘ └───────────────────┘
        │               │                │
        └───────┬───────┴────────┬───────┘
                 │                │
        ┌────────▼────────────────▼─────────────────────────────────┐
        │  Postgres (internal/db/sqlc, queries/*.sql)                │
        │  existing: artists, watchlist, events                      │
        │  NEW: tags, artist_tags (join, keyed on artists.id)        │
        │  MODIFIED: watchlist.notes (nullable TEXT, additive)       │
        └────────┬─────────────────────────────────────────────────┘
                 │
        ┌────────▼─────────────────────────────────────────────────┐
        │  Notification Layer (internal/notifier) — LAST integration │
        │  NotifyPending / SendDigestIfDue batch-fetch a              │
        │  map[artistID][]string via a new TagsReader seam BEFORE     │
        │  formatEmbed/digestLine render, so tag text is baked into   │
        │  rendered strings before digest_chunk.go ever measures      │
        │  rune length — the chunker's size math never changes.       │
        └───────────────────────────────────────────────────────────┘
```

No new background polling, no new external API traffic, no new process — this is entirely CRUD + read-side joins + a formatting-time decoration on data the app already has. That matches PROJECT.md's stated constraint for this milestone.

### Component Responsibilities

| Component | Responsibility | Where it lives |
|-----------|----------------|-----------------|
| `internal/tags` (NEW) | Tag CRUD, normalization (trim/case-fold/length-cap), rename/delete (global, cascades via FK), get-or-create-on-attach, attach/detach to an artist, list-for-autocomplete | new package, mirrors `internal/watchlist`'s `Store`/`Service`/`NewService` shape exactly |
| `internal/watchlist` (MODIFIED) | Gains `Notes *string` on `Entry`/`AddParams`/a third `PreferencesParams`-style axis; `List` gains tag-array and latest-release-date enrichment, batched (no N+1) | `internal/watchlist/service.go` |
| `internal/events` (MODIFIED) | `ListParams` gains an optional `TagID *int64` (or `TagName`, see Decision 3), threaded into the existing `sqlc.narg`-based `WHERE (... IS NULL OR ...)` idiom | `internal/events/service.go`, `queries/events.sql` |
| `internal/notifier` (MODIFIED) | Gains a `TagsReader` seam batch-fetched once per `NotifyPending` pass / once per `SendDigestIfDue` run, threaded into `formatEmbed`, `digestLine`, `buildDigestGroups`, `buildDigestChunks` | `internal/notifier/{notifier,digest,digest_format,digest_chunk}.go` |
| `internal/httpserver` (MODIFIED) | New handlers for tags CRUD, tag attach/detach, three bulk-action endpoints, one bulk-add-confirm endpoint; existing `handleListEvents`/`handleUpdateWatchlist` gain new optional fields | `internal/httpserver/{tags,watchlist,events}.go` |
| `web/app/lib/api.ts` (MODIFIED) | New typed wrappers for every new/changed endpoint above | one file, per existing convention |
| `web/app/routes/watchlist.tsx` (MODIFIED) | Client-side search/sort/filter over the already-fully-loaded `WatchlistEntry[]` (now carrying `tags`, `notes`, `latest_release_date`); multi-select bulk-edit UI; paste-a-list UI | existing route |
| `web/app/routes/history.tsx` (MODIFIED) | Tag filter wired the same way `event_type`/`artist_id` filters already are (server-side, via `listEvents({ tagId })`) | existing route |

## Decisions

### Decision 1 — Tag storage: normalized `tags` + `artist_tags` join table, keyed on `artists.id`, not `TEXT[]` on `watchlist`

**Settled: join table, keyed on the master `artists.id`, not `watchlist.id`.**

Three requirements force this, in order of how hard they rule out the alternative:

1. **Global rename/delete.** A `TEXT[]` column on `watchlist` would require an `array_replace`/`array_remove` `UPDATE` across every row that carries the tag, in Go-driven or SQL-driven bulk form, every time an operator renames or deletes a tag. A join table makes rename `UPDATE tags SET name = $1 WHERE id = $2` (one row) and delete `DELETE FROM tags WHERE id = $1` (one row, `ON DELETE CASCADE` on `artist_tags.tag_id` cleans up every attachment) — both O(1) regardless of how many artists carry the tag. This is the same reasoning the codebase already applies elsewhere: `ReleaseTypes`/`EventTypes` are a *fixed* small allow-list (fine as `TEXT[]` with a Go-side allow-list + DB `CHECK`), but tags are open-vocabulary, user-renameable, user-deletable data — a different shape entirely.
2. **Autocomplete.** A join table makes "distinct existing tag names" a trivial `SELECT name FROM tags ORDER BY name` with no cross-row `unnest`/`DISTINCT unnest(watchlist.tags)` scan.
3. **History-by-tag joins** (Decision 2 below) are a plain `JOIN`/`EXISTS` against `artist_tags`, not an `ANY(tags)` array-containment scan with no useful index shape once the array is renamed frequently.

**Why keyed on `artists.id`, not `watchlist.id`:** `internal/watchlist/service.go`'s `Remove` **hard-deletes** the watchlist row (`DELETE FROM watchlist WHERE id = $1`, no soft-delete, no cascade back onto `artists` — D-03/D-10, confirmed in `queries/watchlist.sql`'s `DeleteWatchlistEntry` and the comment block above it). The `artists` row survives a watchlist removal by design (master data, independent lifetime). Milestone v1.6 explicitly adds "confirmed bulk remove" as a feature — a user is now more likely than before to remove-then-re-add an artist (e.g. cleaning up, then reconsidering). Tags keyed on `watchlist.id` would silently evaporate on that round-trip; tags keyed on `artist_id` survive it, matching how `artists.image_url`/`deezer_id` already persist across watchlist churn. It also removes an indirection: `events.artist_id` (see Decision 2) already points at `artists.id` directly, so keying `artist_tags` the same way makes the History join a single hop, no detour through `watchlist`.

One consequence to flag explicitly: PROJECT.md's own wording is inconsistent ("multi-tags per artist" in the milestone goal line vs. "per watchlist entry" in the Active requirements bullet). This research resolves that ambiguity in favor of "per artist" for the reasons above — worth a one-line confirmation in `/gsd-discuss-phase` before locking it, but the technical case is strong.

**Recommended schema (additive migration, one release, no expand/contract split needed — nothing is being removed or renamed):**

```sql
CREATE TABLE tags (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Case-insensitive uniqueness backstop (Decision 2 layer), display casing preserved.
CREATE UNIQUE INDEX tags_name_lower_idx ON tags (lower(name));

CREATE TABLE artist_tags (
    artist_id  BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    tag_id     BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (artist_id, tag_id)
);
CREATE INDEX artist_tags_tag_idx ON artist_tags (tag_id);
```

`ALTER TABLE watchlist ADD COLUMN notes TEXT;` — nullable, no default, same style as `000004`/`000006` (see `internal/db/migrations/README.md`'s checklist: nullable-or-DEFAULT, no same-release drop/rename). Notes stay on `watchlist` (not `artists`): they're explicitly "Watchlist card only" per PROJECT.md, have no History requirement, and — unlike tags — there's no stated expectation they survive a remove/re-add (a note is closer to "why I'm watching this right now").

### Decision 2 — Tag normalization owned by the Go service layer, DB unique index as backstop

Mirrors the codebase's existing dual-layer pattern exactly (Go-side `watchlist.ReleaseTypes`/`EventTypes` allow-lists backed by DB `CHECK` constraints; digest cadence validated at handler → service → DB `CHECK`, three layers). For tags:

- **`internal/tags.Service`** (new, mirrors `internal/watchlist/service.go`'s `trimAndCap`/`normalizeSet` idiom) trims whitespace, rejects empty/over-length names before any DB call, and does a case-insensitive existence check via `lower(name)` before insert.
- **DB** carries `UNIQUE INDEX ON tags (lower(name))` as the non-bypassable backstop for the same reason `watchlist_artist_id_key` backstops duplicate-add races (`internal/watchlist/service.go`'s `ErrDuplicate` translation off a `pgerrcode.UniqueViolation` + constraint-name check) — a concurrent create-two-tags-with-the-same-name race is closed at the DB, translated to a friendly "tag already exists" (or silently resolved to the existing tag id, see get-or-create below) exactly the way `ErrDuplicate` is handled today.
- **Casing:** preserve the first-created casing for display, dedupe case-insensitively (`"R&B"` and `"r&b"` are the same tag). This is a one-line policy call, not locked elsewhere — flag it in `/gsd-discuss-phase` if a different casing rule is preferred (e.g. force-lowercase).
- **Get-or-create on attach:** the "type a new tag inline" autocomplete flow needs one atomic statement, not a two-step check-then-insert (which would repeat the exact race class `AdvanceGroupTrackCountBaseline`'s doc comment warns against): `INSERT INTO tags (name) VALUES ($1) ON CONFLICT (lower(name)) DO NOTHING RETURNING id`, falling back to a `SELECT id FROM tags WHERE lower(name) = lower($1)` when the insert affected zero rows (standard Postgres upsert-or-fetch idiom, one round trip in the common case).

### Decision 3 — History-by-tag joins `events.artist_id` → `artist_tags.artist_id` → `tags`

Confirmed from `internal/db/migrations/000003_events.up.sql`: `events.artist_id BIGINT NOT NULL REFERENCES artists(id)`. It references the **master artist**, the same `artists.id` both `watchlist.artist_id` and (per Decision 1) `artist_tags.artist_id` point at — never `watchlist.id`. This is exactly what makes keying tags on `artist_id` (not `watchlist.id`) the right call for Decision 1: it makes this join direct with no detour through `watchlist`, and it means History-by-tag keeps working for events belonging to an artist a user has since removed from the watchlist (consistent with History already showing events for artists independent of "seen store" lifetime — `events` rows persist under 90-day retention filtering regardless of watchlist membership).

Extend `queries/events.sql`'s `ListEvents` and `HasOlderEvents` with a third optional filter, following the exact `sqlc.narg` "IS NULL OR" idiom already used for `artist_id`/`event_type` (the file's own Anti-Patterns comment: "no dynamic SQL building in Go, one static string sqlc can type-check"):

```sql
AND (
  sqlc.narg('tag_id')::bigint IS NULL
  OR EXISTS (
       SELECT 1 FROM artist_tags at
       WHERE at.artist_id = events.artist_id AND at.tag_id = sqlc.narg('tag_id')::bigint
     )
)
```

`internal/events.ListParams` gains `TagID *int64`; `internal/httpserver/events.go`'s `handleListEvents` gains a `tag_id` query param parsed with the same `parseOptionalPositiveInt64` helper already used for `artist_id` — no new parsing idiom needed. `web/app/lib/api.ts`'s `listEvents` gains a `tagId?: number` param, same pattern as `artistId`/`eventType`.

### Decision 4 — Notifier tag integration: batch-fetch before render, never inside the chunker's size math

This is the integration point with the highest blast-radius risk in the milestone (Phase 23's chunker/ack invariants are extensively tested and comment-documented — see `internal/notifier/digest_chunk.go`'s `D-06`/`D-16`/`D-19` references), so the design has to be additive to the existing pure-function pipeline, not a rework of it.

**The critical property to preserve:** `chunkContentBudget` (`digest_chunk.go`) measures `utf8.RuneCountInString` on **already-rendered** text (`renderGroup`, `digestLine`'s output). Nothing about the chunker's splitting/capping/ack logic cares *what* text it's measuring — it only needs the text to be final before `chunkDigest`/`buildDigestChunks` ever see it. That means tags must be baked into the rendered line/embed **before** `buildDigestGroups`/`digestLine`/`formatEmbed` run, not injected afterward as a post-processing step on already-chunked output (which would silently blow the 4096-rune budget the chunker already proved safe).

**Concrete plumbing, following the existing seam-in-consumer pattern (`Sender`, `SettingsReader`):**

1. New narrow interface declared in `internal/notifier` (mirrors `SettingsReader`):
   ```go
   type TagsReader interface {
       ListTagsByArtistIDs(ctx context.Context, artistIDs []int64) (map[int64][]string, error)
   }
   ```
   Required constructor argument on `notifier.New`, same as `SettingsReader` (D-05's reasoning applies identically: a forgotten option would silently ship undecorated notifications, not a crash — worse than a compile error).
2. **Real-time path (`NotifyPending`, `internal/notifier/notifier.go`):** after `listUnnotified` returns `events`, collect the distinct `artist_id`s from the batch and call `ListTagsByArtistIDs` **once** (not per-event — avoiding the N+1 the existing code is careful to avoid everywhere else, e.g. `ListUnnotified`'s single query for the whole pass). Thread the resulting `map[int64][]string` into `formatEmbed(ev, tags)` — a new second parameter, since `formatEmbed` today is a pure `sqlc.Event -> discord.Embed` transform (`format.go`) with no DB access of its own; it stays pure, just gains an input.
3. **Digest path (`SendDigestIfDue`, `internal/notifier/digest.go`):** same batch fetch, once, right after `sendable` is computed (before `buildDigestChunks`). Thread the map through `buildDigestChunks(events, lastSentAt, tagsByArtist)` → `buildDigestGroups(events, tagsByArtist)` → `digestLine(eventType, ev, tags)`. Every one of these functions is already pure and already takes the full event list up front — adding one more input parameter is a signature change, not a structural change, and `renderGroup`'s rune-counting continues to measure the final string exactly as it does today.
4. **Where the tags render:** real-time embeds get an additional `discord.EmbedField{Name: "Tags", Value: strings.Join(tags, ", ")}` via the existing `appendField` helper (already truncates to `fieldValueLimit`, already omits the field when empty — tags reuse that exact mechanism for free). Digest lines get tags appended to `lineLabel`'s output, escaped through the existing `escapeMarkdown`/`truncateRunes` pipeline (`digest_format.go` already escapes/caps `artistKey`/title the same way) — recommend a new `digestTagsLimit` constant (e.g. 60 runes, matching `digestArtistLimit`'s existing sizing rationale) rather than leaving tags unbounded, for the same reason `digestArtistLimit` exists: an unbounded field breaks D-06's "no mid-line chunk boundary" legality rule.
5. **Ack/dedup are untouched.** `digestEntry{text, id}` still pairs one rendered string with one event id; tags only change what `text` contains, never how many entries exist or how ids map to chunks. `chunkOverheadReserve` (300 runes, already a "worst-case" reserve computed independently of any single event's content) does not need to grow for tags, because tags are part of *content* (subject to `digestArtistLimit`-style per-line capping), not part of the *overhead* the reserve protects (headers/position markers/continuation notes) — but flag this for the phase's own review since `chunkOverheadReserve`'s comment enumerates specific named budget lines and a reviewer should confirm tags don't need a line of their own there.

**Where the batch query lives:** `internal/tags.Service.ListTagsByArtistIDs(ctx, ids)` — one `SELECT artist_id, name FROM artist_tags JOIN tags ON ... WHERE artist_id = ANY($1)`, grouped into the map in Go. `notifier.TagsReader` is satisfied by `*tags.Service` directly (same shape as `SettingsReader` being satisfied by `*settings.Service`) — no adapter needed.

**Wiring:** `cmd/server/main.go` passes the same `*tags.Service` instance into both `notifier.New`/`notifier.Select` and `httpserver.New`, exactly as `settingsReader` is threaded to both `notifier.Select` and the `/settings/notifications` handlers today.

### Decision 5 — Bulk endpoints: one transactional, set-based endpoint per action, not N client calls

**Settled: one endpoint per bulk action, each backed by a single set-based SQL statement using `= ANY($ids)`.**

The codebase already has a direct precedent for exactly this shape: `queries/notification_settings.sql`'s `AckEventsOnly`/`AckDigestBatch` both take `sqlc.arg('ids')::bigint[]` and update every matching row in one statement (`WHERE id = ANY(sqlc.arg('ids')::bigint[])`). Bulk watchlist actions should follow the identical idiom:

- `POST /watchlist/bulk/tags` — body `{ ids: number[], add?: string[], remove?: string[] }`, one transaction: get-or-create every `add` tag name, then one `INSERT INTO artist_tags ... ON CONFLICT DO NOTHING` fed by a cross join of `ids × add-tag-ids`, and one `DELETE FROM artist_tags WHERE artist_id = ANY($ids) AND tag_id = ANY($remove_ids)`.
- `POST /watchlist/bulk/preferences` — body mirrors `updateWatchlistRequest`'s two axes plus `ids: number[]`; one `UPDATE watchlist SET ... WHERE id = ANY($1)` per axis supplied (same nil-means-untouched convention `UpdateWatchlistPreferences` already uses, just broadened from one id to `ANY(ids)`).
- `POST /watchlist/bulk/remove` — body `{ ids: number[] }`, confirmed client-side before the call lands (per the milestone's "confirmed bulk remove" wording — the confirmation is a UI gate, not a second server round trip); one `DELETE FROM watchlist WHERE id = ANY($1)`.

**Why not N client calls (a loop of existing `PATCH`/`DELETE /watchlist/{id}`):** three concrete costs the existing single-item endpoints don't have to pay today, that a 50-item bulk operation would:
1. **Atomicity.** N independent HTTP calls means partial failure is a visible, confusing UI state (23 of 50 tags added, no way to tell the user which 27 failed without per-call bookkeping the client would have to build itself). A single transactional endpoint either fully applies or reports one coherent error.
2. **Round trips.** 50 sequential `PATCH` calls against a single-instance server (no connection-pool contention concern, but real wall-clock latency) is materially slower than one query touching 50 rows — and multiplies chi's request-handling overhead 50x for no benefit.
3. **Consistency with the ack pattern already in this codebase.** `AckEventsOnly`/`AckDigestBatch` chose exactly this set-based shape for exactly this reason (a digest chunk's ids ack atomically, never one-by-one) — reusing the idiom keeps the codebase's two "operate on many rows from one action" code paths shaped the same way, which is good for anyone reading both later.

Each bulk endpoint still returns a structured per-id outcome if a caller needs to know which ids didn't exist (e.g. concurrently removed) — cheap to add via `:execrows` per statement or a `RETURNING id` list compared against the input, matching `DeleteWatchlistEntry`'s existing "0 rows affected is the 404 signal" idiom, just applied to a set instead of one row.

### Decision 6 — Paste-a-list bulk add: client drives per-name `GET /search`, server never batch-resolves in one request

**Settled: the client calls the existing `GET /search` once per pasted line, sequentially (or with small bounded concurrency capped low enough to respect the shared rate limiter), building the review screen incrementally as results arrive. No new server-side batch-resolve endpoint for the *search* half.**

This is forced by two existing, already-documented constraints, not a preference:

1. **The shared external rate limiter.** `internal/musicbrainz/client.go` and `internal/deezer/client.go` each bind to one process-wide `rate.Limiter` shared across *all* callers — search traffic and poll traffic draw from the same budget (`internal/httpserver/search.go`'s doc comment, `.planning/codebase/ARCHITECTURE.md`'s "Shared rate-limit budget per source" architectural constraint). MusicBrainz is throttled to roughly 1 req/sec. A pasted list of 30–50 artist names, resolved server-side inside one HTTP request, would take 30–50+ seconds purely waiting on that limiter — before any Deezer calls, network latency, or retries.
2. **The server's write timeout.** `internal/watchlist/service.go`'s `matchTimeout` comment states explicitly: *"cmd/server/main.go sets the HTTP server's write timeout to 15 seconds."* A single server-side batch-resolve endpoint handling more than ~10–15 pasted names would blow that timeout outright — this isn't a tuning question, it's a hard ceiling already set for an unrelated reason (bounding a single artist-art match call) that any new long-running synchronous endpoint inherits.

The milestone's own phrasing — *"per-name rate-limited search via the existing GET /search"* — already signals this design: reuse the endpoint as-is, call it once per line, let the existing limiter naturally pace the whole batch, and let the review screen render each line's result the moment it resolves rather than blocking on the whole batch. This also gives the UI free incremental progress feedback (a spinner-per-row that resolves independently) instead of one opaque "resolving 40 names…" wait state, and needs no new async-job/polling/streaming infrastructure this milestone explicitly doesn't need.

**The *confirm* half is different** — "nothing added until confirmed" describes a normal-latency, no-external-API, pure-DB write once the user reviews best-match/alternates and hits confirm. That step is a good fit for a genuine bulk endpoint (`POST /watchlist/bulk/add`, body `{ artists: [{mbid, name, deezerId?, ...}] }`), reusing `watchlist.Service.Add`'s existing per-artist validation and `ErrDuplicate` handling in a loop inside one transaction (or one loop with per-item results, since a duplicate here — the user pasted the same name twice, or an artist already on the watchlist — is an expected, non-fatal per-item outcome, not a reason to abort the whole batch). This keeps the same "one endpoint per bulk action, set-based/transactional" shape as Decision 5, applied to the one half of paste-a-list where it's actually safe to do synchronously.

### Decision 7 — Server-side enrichment, client-side search/sort/filter

**Settled: `GET /watchlist` gains server-computed enrichment fields (tags, notes, latest-release-date); all interactive search/sort/filter logic stays client-side over the already-fully-loaded list.**

`GET /watchlist` (`handleListWatchlist`, `internal/httpserver/watchlist.go`) already returns every watchlist entry as one bare JSON array with no pagination or filtering (WLST-04) — the whole 50-artist dataset is loaded into the browser on every page visit today. That existing shape is the right one to keep: at "keep a 50+ artist watchlist manageable" scale (PROJECT.md's own framing of this milestone's target size), client-side `Array.prototype.filter`/`sort` over an already-in-memory list is both simpler and faster than round-tripping query params to the server for every keystroke of a name search or every sort-order click.

What has to move server-side is only the **data the client doesn't have yet**:
- `tags: string[]` per entry — batch-joined the same way the notifier's `TagsReader` batch-fetches (Decision 4), or folded directly into `ListWatchlist`'s existing query via a correlated `array_agg` subquery (`(SELECT COALESCE(array_agg(t.name ORDER BY t.name), '{}') FROM artist_tags at JOIN tags t ON t.id = at.tag_id WHERE at.artist_id = a.id) AS tags`) — either avoids N+1; the correlated-subquery form keeps `ListWatchlist` a single round trip, consistent with how it already JOINs `artists` in one query.
- `notes: string | null` — already a plain column on `watchlist` once the migration lands, no extra join needed, already returned by `ListWatchlist`'s existing `SELECT w.*`-shaped columns once `notes` is added to the column list.
- `latest_release_date: string | null` — needed because "sort by latest release" is not derivable from any field the client already has. Recommend scoping this to `event_type = 'new_release'` only (an artist's guest features/deluxe changes are not, in the ordinary sense, "their latest release") via a correlated `MAX(release_date)` subquery against `events`, reusing the same lexicographic-text-sorts-chronologically property `queries/events.sql`'s `ListEvents` comment already documents and relies on. This is a judgment call worth a one-line confirmation in `/gsd-discuss-phase` (an alternative: scope to all three event types, or use `MAX(created_at)` — "latest activity" — instead of `MAX(release_date)` — "latest release" — which are subtly different sort orders).

Everything downstream of that enriched payload — name search (substring match), sort (name/date-added/latest-release), filter (tag/muted-event-types/non-default-release-types) — is pure client-side logic over `WatchlistEntry[]`, no new query params on `GET /watchlist`, no new server-side pagination. The existing hand-rolled accessible combobox (`web/app/components/history/HistoryFilters.tsx`, confirmed via its `aria-activedescendant` wiring — the component PROJECT.md's Key Decisions table calls out as replacing a native `<select>` for legibility) is the right pattern to reuse for the tag-filter dropdown on both Watchlist and History, rather than building a second combobox from scratch.

## Recommended Project Structure

```
internal/
├── tags/                        # NEW — mirrors internal/watchlist's shape exactly
│   ├── service.go                # Store interface, Service impl: List/Rename/Delete/
│   │                              # GetOrCreate/Attach/Detach/ListTagsByArtistIDs,
│   │                              # trimAndCap-style normalization before every write
│   └── service_test.go
├── watchlist/
│   └── service.go                # MODIFIED — Entry gains Notes, ReleaseTypes-style
│                                  # third axis for notes on UpdatePreferences OR a
│                                  # dedicated PatchNotes; List's query gains the
│                                  # tags/latest-release-date correlated subqueries
├── events/
│   └── service.go                # MODIFIED — ListParams gains TagID *int64
├── notifier/
│   ├── notifier.go                # MODIFIED — TagsReader seam, required ctor arg
│   ├── format.go                  # MODIFIED — formatEmbed(ev, tags) gains a param
│   ├── digest.go                  # MODIFIED — SendDigestIfDue batch-fetches tags once
│   ├── digest_format.go           # MODIFIED — digestLine(eventType, ev, tags)
│   └── digest_chunk.go            # MODIFIED — buildDigestGroups/buildDigestChunks
│                                  # thread the tags map through; chunking/ack logic
│                                  # itself is untouched
└── httpserver/
    ├── tags.go                    # NEW — GET /tags, PATCH/DELETE /tags/{id},
    │                              # POST/DELETE /watchlist/{id}/tags
    ├── watchlist.go                # MODIFIED — notes field on PATCH /watchlist/{id};
    │                              # NEW bulk handlers (or a sibling watchlist_bulk.go)
    └── events.go                   # MODIFIED — tag_id query param

queries/
├── tags.sql                       # NEW — tag CRUD, get-or-create, ListTagsByArtistIDs,
│                                   # bulk attach/detach set-based statements
├── watchlist.sql                  # MODIFIED — notes column, enrichment subqueries,
│                                   # bulk preference/remove ANY($ids) statements
└── events.sql                     # MODIFIED — tag_id EXISTS predicate

internal/db/migrations/
└── 0000XX_tags_and_notes.up.sql   # NEW — additive only: tags, artist_tags, watchlist.notes

web/app/
├── lib/api.ts                     # MODIFIED — new wrappers, WatchlistEntry gains
│                                  # tags/notes/latest_release_date, EventItem query
│                                  # gains tagId
├── components/
│   ├── watchlist/                 # tag chips, notes editor, bulk-select toolbar,
│   │                              # paste-a-list review screen — all NEW components
│   └── history/
│       └── HistoryFilters.tsx     # MODIFIED — reuse existing combobox for tag filter
└── routes/
    ├── watchlist.tsx               # MODIFIED — search/sort/filter state, bulk-select
    └── history.tsx                 # MODIFIED — tag filter wired like existing filters
```

### Structure Rationale

- **`internal/tags/` as its own package, not folded into `internal/watchlist`:** tags are consumed by three independent callers (`httpserver` for CRUD, `watchlist.Service` for `List` enrichment, `notifier` for embed/digest decoration) — exactly the shape that already justifies `internal/events` being its own package rather than living inside `watchlist` (both are read-mostly domains multiple other packages depend on). Keeping it separate also avoids `internal/notifier` importing `internal/watchlist` just to reach tags, preserving the existing one-way dependency discipline (`internal/musicbrainz`/`internal/deezer` never import `internal/httpserver`, `internal/artistart` never imports `internal/watchlist` — see `ArtistMatcher`'s comment).
- **`queries/tags.sql` as its own file:** matches the one-file-per-domain convention already established (`artists.sql`, `watchlist.sql`, `events.sql`, `notification_settings.sql`).

## Architectural Patterns

### Pattern 1: Narrow consumer-declared seam for a new cross-cutting read (TagsReader)

**What:** `internal/notifier` declares its own `TagsReader` interface rather than importing `*tags.Service` directly, exactly as it already declares `Sender` and `SettingsReader` rather than importing `internal/discord`/`internal/settings` types directly.
**When to use:** Any time a package needs read access to a domain it doesn't own, following this codebase's established rule (`.planning/codebase/ARCHITECTURE.md`'s "Seam-based design" characteristic).
**Trade-offs:** One extra interface declaration per consumer vs. a shared import — but it's what makes `notifier_test.go`-style stubbing possible with no DB, and it's the pattern every other cross-package dependency in this codebase already follows without exception.

**Example:**
```go
// internal/notifier/notifier.go
type TagsReader interface {
    ListTagsByArtistIDs(ctx context.Context, artistIDs []int64) (map[int64][]string, error)
}
var _ TagsReader = (*tags.Service)(nil)
```

### Pattern 2: Batch-fetch-then-decorate, never per-row queries inside a render loop

**What:** Collect all needed ids up front, issue one `= ANY($ids)` query, build an in-memory `map[int64]T`, then pass that map into pure formatting functions that do no I/O of their own.
**When to use:** Anywhere a loop over N rows would otherwise need per-row lookups — this is already how `ListUnnotified` (one query for the whole notify pass) and `AdvanceGroupTrackCountBaseline` (one atomic statement instead of check-then-act) are built.
**Trade-offs:** Slightly more up-front bookkeeping (collect distinct ids, build a map) vs. a naive per-row call — but the naive version reintroduces the exact N+1 shape this codebase has consistently avoided (PERF-04, Phase 11's `AdvanceGroupTrackCountBaseline` doc comment names the check-then-act race directly).

**Example:**
```go
ids := distinctArtistIDs(events)
tagsByArtist, err := n.tagsReader.ListTagsByArtistIDs(ctx, ids) // one query
for _, ev := range events {
    embed := formatEmbed(ev, tagsByArtist[ev.ArtistID]) // pure, no I/O
}
```

### Pattern 3: Set-based bulk mutation with `= ANY($ids)`, one statement per action

**What:** A bulk endpoint's SQL touches every affected row in one statement (`UPDATE ... WHERE id = ANY($1)`, `DELETE ... WHERE id = ANY($1)`), never a Go-side loop issuing N separate statements.
**When to use:** Any new "act on multiple ids at once" endpoint — already precedented by `AckEventsOnly`/`AckDigestBatch`.
**Trade-offs:** Slightly less granular per-row error reporting than a loop (mitigated by comparing input ids against `RETURNING id`/`:execrows` to report which ids didn't apply) — but atomicity and one-round-trip latency are worth it at this data scale, and it reuses an idiom already proven in this codebase.

## Data Flow

### New Flow: Tag attach with inline creation

```
User types a new tag name in the Watchlist card's tag combobox
    ↓
POST /watchlist/{id}/tags { name: "R&B" }
    ↓
httpserver.handleAttachTag → resolves watchlist id → artist_id
    ↓
tags.Service.AttachByName(ctx, artistID, "R&B")
    ↓ (Go-side trim/cap/normalize, then one atomic statement)
INSERT INTO tags (name) VALUES ($1) ON CONFLICT (lower(name)) DO NOTHING RETURNING id
  → 0 rows? → SELECT id FROM tags WHERE lower(name) = lower($1)
    ↓
INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
    ↓
Response: 201 with the resolved tag { id, name }
```

### New Flow: Digest send with tags (extends the existing Phase 22/23 flow)

```
DigestScheduler tick → SendDigestIfDue
    ↓
listUnnotified(ctx, q) → events []sqlc.Event   (unchanged)
    ↓
distinct artist_ids from events → tagsReader.ListTagsByArtistIDs(ctx, ids)  (NEW, one query)
    ↓
buildDigestChunks(sendable, lastSentAt, tagsByArtist)  (signature gains one param)
    ↓ buildDigestGroups(events, tagsByArtist) → digestLine(eventType, ev, tags)
    ↓ (tags now part of the rendered string BEFORE any rune-counting happens)
chunkDigest(groups)   (UNCHANGED — measures already-final text, no awareness tags exist)
    ↓
per-chunk Send + ackEventsOnly/ackDigestBatch   (UNCHANGED)
```

### Existing Flow, extended: History filter by tag

```
User selects a tag in History's filter combobox (reusing HistoryFilters.tsx's pattern)
    ↓
GET /events?tag_id=7&cursor=...
    ↓
handleListEvents parses tag_id via parseOptionalPositiveInt64 (existing helper, no new parsing idiom)
    ↓
events.Service.List(ctx, ListParams{TagID: &7, ...})
    ↓
ListEvents SQL: existing artist_id/event_type IS-NULL-OR predicates
              + NEW EXISTS(SELECT 1 FROM artist_tags WHERE artist_id = events.artist_id AND tag_id = $tagID)
```

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Current (~50 watched artists, this milestone's stated target) | Everything above as designed: client-side search/sort/filter over a fully-loaded list, correlated subqueries for enrichment, batch-fetch-then-decorate for notifications. No pagination needed anywhere new. |
| A few hundred watched artists | `GET /watchlist`'s correlated subqueries (tags array-agg, latest-release-date) start to matter — add an index on `artist_tags(artist_id)` (the join table's PK already covers this) and consider whether `ListWatchlist`'s implicit full-table scan needs a covering index; client-side filter/sort still fine at this size. |
| Thousands of watched artists (well outside this milestone's stated scope) | `GET /watchlist` would need real server-side pagination/filtering, which is a bigger redesign than v1.6 attempts — not a concern to design for now per PROJECT.md's stated "keep a 50+ artist watchlist manageable" framing, which caps the ambition explicitly. |

### Scaling Priorities

1. **First bottleneck (if it ever mattered): `ListWatchlist`'s per-row correlated subqueries.** At 50–500 rows this is a non-issue; Postgres handles correlated subqueries against small indexed join tables trivially. Not worth optimizing preemptively.
2. **Second bottleneck: digest batch tag-fetch on a very large pending backlog.** `ListTagsByArtistIDs` is bounded by the *distinct artist count* in one notify/digest pass, not the event count — a backlog of hundreds of events from 50 artists still issues one query for at most 50 ids. No adjustment needed at any scale this project targets.

## Anti-Patterns

### Anti-Pattern 1: Fetching tags inside the per-event formatting loop

**What people do:** Call `tagsReader.ListTagsByArtistIDs(ctx, []int64{ev.ArtistID})` once per event inside `NotifyPending`'s or `SendDigestIfDue`'s loop, because it's the easy place to reach for the data.
**Why it's wrong:** Reintroduces an N+1 query pattern this codebase has deliberately avoided everywhere else (`ListUnnotified` is one query for the whole pass; `AdvanceGroupTrackCountBaseline` collapsed a two-round-trip check-then-act into one). At even a modest backlog size this turns one notify pass into dozens of extra DB round trips for no reason.
**Do this instead:** Collect distinct artist ids from the whole batch first, one batch query, build the map, then loop purely over in-memory data (Pattern 2 above).

### Anti-Pattern 2: Injecting tag text into a digest chunk after `chunkDigest` has already split it

**What people do:** Run the existing (unmodified) digest pipeline, then walk the resulting `[]digestChunk` and append tag text to each rendered description string as a "decoration" pass.
**Why it's wrong:** `chunkDigest`'s 4096-rune (minus `chunkOverheadReserve`) budget was computed and proven safe against the text as it exists *before* tags. Appending text afterward can silently push a chunk over Discord's real embed-description limit — exactly the class of bug Phase 23's entire test suite (22 chunker tests + 3 invariant property tests) exists to catch, and this bypasses all of it because the chunker never saw the final text.
**Do this instead:** Decorate at line-render time (`digestLine`/`formatEmbed`), before `buildDigestGroups`/`chunkDigest` ever run — Decision 4 above.

### Anti-Pattern 3: A single server-side "resolve this pasted list" endpoint

**What people do:** Build `POST /watchlist/resolve-bulk { names: string[] }` that loops server-side over MusicBrainz/Deezer search calls and returns everything in one response, because it feels like the more "batch API" shape.
**Why it's wrong:** Collides directly with two already-fixed constraints: the shared per-source rate limiter (MusicBrainz ~1 req/sec, shared with poll traffic) and the 15-second HTTP write timeout `cmd/server/main.go` already sets. Any pasted list longer than ~10-15 names would either time out the request or force raising the server-wide write timeout for every other endpoint just to accommodate this one feature.
**Do this instead:** Client drives per-name calls to the existing `GET /search`, incrementally rendering the review screen as each resolves (Decision 6).

## Integration Points

### External Services

No new external services this milestone — MusicBrainz/Deezer/Discord integrations are all reused exactly as they exist today (search reused as-is for paste-a-list; Discord embeds/digest lines gain a field/line but the client and rate-limit handling in `internal/discord` is untouched).

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `internal/httpserver` ↔ `internal/tags` (NEW) | Direct interface call, narrow `Store`-style seam declared in `httpserver`, mirroring `watchlist.Store`/`events.Store` | New package, same shape as every existing domain package |
| `internal/watchlist` ↔ `internal/tags` (NEW) | `watchlist.Service.List`'s enrichment either calls `tags.Service` directly or issues its own correlated subquery in `ListWatchlist`'s SQL — recommend the SQL-subquery form to keep `List` a single round trip, consistent with how it already joins `artists` in one query | Decide in phase planning: a Go-side second query is simpler to reason about; a SQL subquery is one round trip. Given `ListWatchlist` is already a JOIN-based single query and the dataset is small, the subquery form is recommended but either is architecturally sound |
| `internal/notifier` ↔ `internal/tags` (NEW) | `TagsReader` interface, declared in `notifier`, satisfied by `*tags.Service` — exactly the `SettingsReader`/`*settings.Service` pattern | Wired in `cmd/server/main.go` alongside the existing `notifier.Select(...)` call |
| `internal/events` ↔ `artist_tags` table (NEW) | Direct SQL `EXISTS` join in `queries/events.sql`, no Go-level dependency on `internal/tags` at all | `events.Service` never needs to import `internal/tags` — the filter is expressed entirely in SQL, following `ListEvents`' existing "no dynamic SQL building in Go" rule |
| `web/app/lib/api.ts` ↔ backend | New typed fetch wrappers, same `apiFetch<T>` core, same `ApiError`/401-interceptor/CSRF-header handling — no changes to `apiFetch` itself needed | Every new endpoint is a plain JSON request/response, nothing here needs streaming, SSE, or websockets |

## Suggested Build Order

Ordered by dependency, with the riskiest/highest-blast-radius integration (notifier) deliberately last, following this codebase's own established pattern of landing risky changes after their prerequisites are inert and proven (Phase 18 shipped `RunRecorder` inert before Phase 18.1 wired it live; Phase 20 shipped digest settings before Phase 21 made real-time respect them).

1. **Migration + sqlc regen** — `tags`, `artist_tags`, `watchlist.notes`. Purely additive, no expand/contract split needed (nothing removed/renamed), but still run through `cmd/migration-check` and the existing pre-merge checklist in `internal/db/migrations/README.md`. Nothing downstream can start until this lands.
2. **`internal/tags` package** (Store/Service: CRUD, normalize, rename, delete, get-or-create, attach/detach, `ListTagsByArtistIDs`) — the one new domain package everything else depends on. Fully unit-testable with a fake `sqlc.Querier`, no HTTP/notifier dependency yet.
3. **`internal/watchlist` notes + enrichment** — add `Notes` to `Entry`/the preferences-update axis; extend `ListWatchlist`'s query with tags/latest-release-date. Independently valuable and independently testable before any tag-attach UI exists.
4. **HTTP layer: tags CRUD + attach/detach + notes on existing PATCH** — `internal/httpserver/tags.go` (new), `watchlist.go` (notes field). This is the first point a manual UAT click-path exists (create a tag, attach it, see it on `GET /watchlist`).
5. **Frontend: tag chips + notes editor on the Watchlist card, search/sort/filter (client-side)** — consumes the enriched `GET /watchlist` payload from steps 3-4. Independently shippable slice; the milestone's most immediately visible feature.
6. **Bulk endpoints** (`/watchlist/bulk/{tags,preferences,remove}`) — depends on tags existing (step 2) and the preferences/remove semantics already existing (step-0 baseline); each is a self-contained set-based SQL statement, low risk, testable in isolation.
7. **Frontend: multi-select + bulk-edit toolbar** — depends on step 6's endpoints.
8. **History filter by tag** — `queries/events.sql` `EXISTS` predicate, `events.Service`, `handleListEvents`, `api.ts`, `HistoryFilters.tsx`. Independent of bulk/paste-a-list; can happen in parallel with 6-7 if desired, since it only depends on step 1's `artist_tags` table existing.
9. **Paste-a-list bulk add** — client-driven `GET /search` loop + review screen + `POST /watchlist/bulk/add` confirm endpoint. Depends on step 6's bulk-endpoint pattern being established (reuses the same set-based/transactional shape) and on tags existing if the review screen lets a user tag artists inline during add (optional scope call).
10. **Notifier integration (tags on Discord embeds + digest lines)** — last, deliberately: it's the only change touching Phase 23's heavily-tested chunker/ack invariants. Land the `TagsReader` seam and batch-fetch wiring first with an *empty* decoration (prove the plumbing compiles and the existing chunker tests still pass unchanged with a nil/empty tags map — an "inert" landing mirroring Phase 18's `RunRecorder` pattern), then a second, smaller change that actually renders tags into `formatEmbed`/`digestLine`, re-running the full `digest_chunk_test.go` suite plus a new test asserting a maximally-tagged artist doesn't blow `chunkContentBudget`.

Steps 2-5 and step 8 can run substantially in parallel once step 1 lands, since they don't depend on each other. Steps 6-7 depend on 2 (tags) existing. Step 9 depends on 6. Step 10 should be strictly last regardless of team parallelism, given its risk profile.

## Sources

- `internal/watchlist/service.go`, `queries/watchlist.sql` — Store/Service shape, `ErrDuplicate` translation, hard-delete semantics, `normalizeSet`/`trimAndCap` idiom — read directly, confidence HIGH
- `internal/db/migrations/000003_events.up.sql`, `000004_events_display_fields.up.sql`, `000006_events_watched_artist_name.up.sql`, `000007_backfill_events_watched_artist_name.up.sql` — `events.artist_id` FK target, additive-migration precedent, expand/backfill/contract walkthrough — read directly, confidence HIGH
- `internal/db/migrations/README.md` — N-1 rollback-safety rules, expand/contract discipline, `cmd/migration-check` enforcement scope — read directly, confidence HIGH
- `queries/events.sql` — `ListEvents`/`HasOlderEvents` sqlc.narg "IS NULL OR" filter idiom, release-date lexicographic-sort rationale — read directly, confidence HIGH
- `queries/notification_settings.sql` — `AckEventsOnly`/`AckDigestBatch`'s `= ANY($ids)` set-based bulk-ack precedent — read directly, confidence HIGH
- `internal/notifier/{notifier,digest,digest_format,digest_chunk}.go` — full render/chunk/ack pipeline, `Sender`/`SettingsReader` seam pattern, `chunkContentBudget`/`chunkOverheadReserve` sizing rationale, D-04/D-06/D-16/D-19 invariants — read directly, confidence HIGH
- `internal/httpserver/{search,watchlist,events}.go` — existing handler idioms (`parseOptionalPositiveInt64`, `trimAndCap`, `decodeJSONBody`, shared rate-limited search), the 15-second write-timeout constraint (cited from `internal/watchlist/service.go`'s `matchTimeout` comment) — read directly, confidence HIGH
- `internal/events/service.go` — `ListParams`/`Store` shape, retention-cutoff handling — read directly, confidence HIGH
- `web/app/lib/api.ts`, `web/app/components/history/HistoryFilters.tsx` — existing wire-type/wrapper conventions, existing accessible combobox to reuse for tag filtering — read directly, confidence HIGH
- `.planning/PROJECT.md`, `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/STRUCTURE.md` — milestone scope, existing architectural constraints (single binary, shared rate limiter, seam-based design), directory/naming conventions — read directly, confidence HIGH

---
*Architecture research for: drop-tracker v1.6 Watchlist Organization*
*Researched: 2026-09-22*
