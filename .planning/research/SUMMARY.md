# Project Research Summary

**Project:** drop-tracker v1.6 (Watchlist Organization)
**Domain:** Adding tags, notes, search/sort/filter, bulk operations, and Discord tag display to the v1.5 release tracker
**Researched:** 2026-09-22
**Confidence:** HIGH

## Executive Summary

v1.6 adds user-defined, free-form, globally renameable tags (keyed on `artists.id`, not `watchlist.id`, so History filtering survives removal); per-artist notes; client-side search/sort/filter over the fully loaded watchlist (50–500 rows); bulk edit/remove with explicit confirmation; and a paste-a-list bulk-add review screen (client-orchestrated per-name searches, nothing added until confirmed). Tags surface read-only on both Discord paths (real-time embeds and digest lines) within the existing notification-budget and markdown-safety constraints.

Zero new npm packages, zero new Postgres extensions, no new background API polling. Every piece fits an existing seam: `@base-ui/react` Combobox (multiple + chips), the `sqlc.narg` optional-filter idiom, batch-fetch-then-decorate, and the seam-in-consumer pattern (`SettingsReader` → new `TagsReader`).

The two highest-blast-radius risks: (1) real-time embeds have **no** markdown escaping today (`internal/notifier/format.go`; only the digest path escapes) — tags are the first user-authored text on that path; (2) digest chunker fixtures (`chunkForcingEventCount=75`, `capForcingEventCount=600`) will silently drift once tags lengthen lines. Bulk-add must be client-orchestrated because the 15s server `writeTimeout` collides with MusicBrainz's 1 req/s limiter.

## Key Findings

### Recommended Stack (STACK.md)

- No new npm packages — `@base-ui/react` 1.7.0 Combobox (`multiple`, Chips/Chip/ChipRemove, creatable pattern) covers tag autocomplete.
- No new Postgres extensions — case-insensitive uniqueness via `UNIQUE INDEX ON tags (lower(name))`, not `citext`.
- Additive migration: `tags(id, name, created_at)`, `artist_tags(artist_id → artists ON DELETE CASCADE, tag_id → tags ON DELETE CASCADE, PK(artist_id, tag_id))`, `watchlist.notes TEXT NULL` — length CHECKs baked in at creation.
- Client-side sort/search/filter; avoid `CASE`-based dynamic `ORDER BY` in sqlc (sqlc-dev/sqlc#2061, planner footgun). No virtualization, fuzzy-match, or debounce libraries.

### Expected Features (FEATURES.md)

**Table stakes:** case/whitespace-insensitive tag matching with display casing preserved; global rename (merge-into-existing with explicit confirmation, never silent) and delete; count-stated confirm for destructive bulk remove, apply+toast for reversible bulk edits; newline-delimited paste list, in-batch dedupe, already-watched flagged by exact external ID; review before any add.

**Differentiators:** same tag vocabulary filters Watchlist and History; confidence-tiered review screen (High / Medium / Not found, only High preselected); tags on Discord embeds and digest lines.

**Anti-features:** hierarchical tags; regex-stripping "Artist - extra" noise before search (let ranking absorb it).

Sort rules: always a secondary key; latest-release sort `NULLS LAST` in both directions.

### Architecture Approach (ARCHITECTURE.md)

1. Tags: `tags` + `artist_tags` keyed on `artists.id` — `watchlist.Remove` hard-deletes the watchlist row while `events.artist_id` survives.
2. Normalization: Go-side trim/case-fold/NFC/length-cap in new `internal/tags`, DB `UNIQUE lower(name)` backstop.
3. History-by-tag: `events.artist_id → artist_tags → tags`, a third `sqlc.narg` `EXISTS` predicate inside the same retention-aware `ListEvents`/`HasOlderEvents` queries.
4. Notifier: `TagsReader` seam, batch-fetched once per `NotifyPending`/`SendDigestIfDue` pass, threaded through `formatEmbed` / `digestLine` / `buildDigestGroups` / `buildDigestChunks`; tags rendered **before** `chunkDigest` measures runes. Land inert first, then render.
5. Bulk endpoints: one transactional, set-based endpoint per action (`= ANY($ids)`), precedented by `AckEventsOnly`/`AckDigestBatch`.
6. Paste-a-list: resolve = SPA calls `GET /search` per line (bounded concurrency); confirm = one `POST /watchlist/bulk/add`.
7. `GET /watchlist` enriched with tags, notes, `latest_release_date` (single query, `LEFT JOIN LATERAL`); interactive filter/sort client-side.

### Critical Pitfalls (PITFALLS.md)

1. Real-time embed markdown injection via tags — escape on both paths, table-driven tests.
2. Chunker fixture drift — add precondition assertions / re-derive constants before rendering tags.
3. Bulk add vs 15s `writeTimeout` — client-orchestrated resolve, lock at plan time.
4. New endpoints must sit in the protected chi group (authgate/CSRF); SPA calls via `apiFetch`.
5. Tags must be `artist_id`-scoped or History-by-tag breaks after remove.
6. Re-adding a removed artist skips seed mode (events survive removal) — accept and document in UI copy.
7. Case-folding / Unicode forks — store display casing, unique on `lower(name)`, NFC server-side.
8. Latest-release sort N+1 — single `LEFT JOIN LATERAL` query from the start.
9. Tag filter must compose with retention in the same query, with a regression test.
10. Length caps belong in the initial migration (`cmd/migration-check` blocks later narrowing).

## Implications for Roadmap

Suggested dependency order (phase numbers continue from v1.5, starting at 24):

1. **Tags & notes data foundation** — migration, sqlc regen, `internal/tags` normalization, tag CRUD/attach/rename/delete API, watchlist enrichment (tags, notes, latest_release_date).
2. **Watchlist UI: tags, notes, search/sort/filter** — chip combobox, notes editing, client-side controls, tag management UI.
3. **Bulk edit & remove** — set-based transactional endpoints + multi-select toolbar.
4. **History filter by tag** — can run parallel with bulk work once the foundation lands.
5. **Paste-a-list bulk add** — client-orchestrated resolve + review screen + bulk confirm endpoint.
6. **Discord tag display (last)** — `TagsReader` seam inert first, escaping on both paths, chunker fixture guards, then rendering.

### Research Flags

- Standard patterns: foundation, watchlist UI, History filter.
- Review during planning: bulk endpoints (CSRF/session), paste-a-list (UX spec; consider `/gsd-sketch`).
- Highest risk: notifier integration.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified against installed packages and existing migrations |
| Features | MEDIUM-HIGH | Cross-checked UX conventions (Gmail, GitHub, Lidarr/Sonarr, Letterboxd) |
| Architecture | HIGH | Every pattern precedented in this codebase, file-level citations |
| Pitfalls | HIGH | Direct code reads (notifier, authgate, config, migrations) |

### Gaps to Address (open product decisions)

| Decision | Recommendation |
|----------|----------------|
| Tag length cap | 30–40 runes |
| Tags per artist cap | 10–15 |
| Notes length cap | 500–1000 runes |
| Bulk-add search concurrency | bounded 3–5 |
| "Latest release" definition | `new_release` events only |
| Bulk remove and `artist_tags` | keep tags (artist-scoped) so History filter survives |

## Sources

- `.planning/research/STACK.md`
- `.planning/research/FEATURES.md`
- `.planning/research/ARCHITECTURE.md`
- `.planning/research/PITFALLS.md`

---
*Research completed: 2026-09-22*
*Ready for roadmap: yes*
