# Stack Research

**Domain:** Watchlist organization (tags, notes, search/sort/filter, bulk edit, bulk add) for an existing Go + React release-tracker
**Researched:** 2026-09-22
**Confidence:** HIGH (frontend — verified against installed `node_modules` + official docs); MEDIUM-HIGH (backend/Postgres — verified against existing migrations/queries + established sqlc/Postgres community sources)

## Headline Finding

**v1.6 needs zero new npm packages and zero new Postgres extensions.** Every capability in the milestone (tag autocomplete with chips, multi-select bulk edit, name search/sort/filter, paste-a-list review) is covered by libraries already in `web/package.json` or by plain Postgres/sqlc patterns already proven elsewhere in this codebase (`internal/events`'s `sqlc.narg` filters, `watchlist.release_types`'s `TEXT[]` column). The only "addition" is *using* an already-installed primitive (`@base-ui/react`'s `Combobox`) that the codebase hasn't reached for yet, plus two new Postgres tables for tags (no new extension).

## Recommended Stack

### Core Technologies (schema/architecture decisions, not new packages)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|------------------|
| Normalized `tags` + `watchlist_tags` join tables (plain Postgres, no extension) | Postgres 16 (already pinned, `docker-compose.yml`) | Store free-form tags with global rename/delete | "Global rename/delete" is the deciding factor over the `TEXT[]`-on-`watchlist` shape the codebase already uses for `release_types`/`muted_event_types` (`internal/db/migrations/000002_watchlist.up.sql`). A `TEXT[]` column *can* be globally renamed via `array_replace()` across every row, but that conflates "rename a tag" (conceptually a single-entity edit) with "rewrite every watchlist row that uses it." A `tags(id, name)` table makes rename/delete a one-row, one-statement operation with a real identity, and `watchlist_tags(watchlist_id, tag_id)` gives History-feed-by-tag a plain join instead of an `= ANY(tags)` array scan. At 50-500 watchlist rows the perf delta between the two shapes is immaterial either way — this is an identity/correctness argument, not a scale argument. |
| `UNIQUE (lower(name))` functional index on `tags`, not the `citext` extension | Postgres 16 built-in | Case-insensitive tag uniqueness ("Hip-Hop" and "hip-hop" don't become two tags) | The project currently has zero `CREATE EXTENSION` statements anywhere in `internal/db/migrations/`. `citext` would be the first, adding a new axis `cmd/migration-check` and the N-1 rollback rule have never had to reason about (extension availability on the target Postgres image, extension-drop-on-rollback edge cases). A plain functional unique index gets the same guarantee — insert/rename does `INSERT ... ON CONFLICT ((lower(name))) DO NOTHING`/`UPDATE` — with zero new surface. Store the user's original casing in `name` for display; match/dedupe on `lower(name)`. |
| Client-side sort/search/filter for the Watchlist view — no `sqlc.narg` dynamic `WHERE`, no `CASE`-based dynamic `ORDER BY` | N/A (React state, existing `listWatchlist()` call) | Name search; sort by name/date-added/latest-release; filter by tag/mute/release-type | `GET /watchlist` already returns the **full, unpaginated** list (`ListWatchlist` has no `LIMIT`/cursor — confirmed in `queries/watchlist.sql`). At the milestone's stated scale (50-500 rows), fetching the whole list once and sorting/filtering/searching it in the browser is strictly simpler than adding a second dynamic-query surface to the backend, and it matches the existing `HistoryFilters.tsx` pattern of "component owns filter state, backend stays dumb." Only extend the backend query to project one new computed column — `latest_release_date` (see below) — everything else (name substring match, tag-set intersection, mute-flag check) is a plain JS `.filter()`/`.sort()` over data already in memory. sqlc's `sqlc.narg` optional-filter pattern (already used in `ListEvents`/`HasOlderEvents`) and a `CASE`-based `ORDER BY` selector are the *right* answer if this list ever needs server-side pagination — deliberately not reaching for them now (see Pitfalls below for why the `CASE`-in-`ORDER BY` version specifically deserves caution even then). |
| Extend `ListWatchlist` with a `latest_release_date` computed column via `LEFT JOIN LATERAL` against `events` | sqlc v1.31.1 (already pinned) | Backs the "sort by latest release" axis, which the client can't compute from data it doesn't have | `events.release_date` is `TEXT` holding zero-padded partial MusicBrainz dates (`YYYY`/`YYYY-MM`/`YYYY-MM-DD`) that sort correctly *lexicographically* — the same property `queries/events.sql`'s `ListEvents` already documents and relies on (`ORDER BY release_date DESC NULLS LAST`). `MAX(release_date)` per `artist_id` via a `LATERAL` subquery reuses that exact property instead of introducing a second date representation. This is a one-column, additive `sqlc` query change — no filter/sort parameters added to the query itself. |

### Supporting Libraries — Frontend (already installed, newly used)

| Library | Version (installed) | Purpose | When to Use |
|---------|---------------------|---------|-------------|
| `@base-ui/react` `Combobox` (+ `Chips`/`Chip`/`ChipRemove` parts) | 1.7.0 (installed; 1.8.0 is current upstream — see Version Compatibility) | Tag input with autocomplete, multi-select chips, free-text tag creation | **Already a runtime dependency** — `web/app/components/ui/select.tsx` already wraps this same package's `Select` primitive for `DigestSettings.tsx` (Phase 20). Verified directly against `node_modules/.pnpm/@base-ui+react@1.7.0.../combobox/`: the package ships `combobox`, `chip`, `chip-remove`, and `chips` parts, and `ComboboxRoot`'s type defs expose a `multiple` prop (`ModeFromMultiple<Multiple>`) plus `filter`/`items`/`onInputValueChange` for exactly the "type to filter existing tags, or type a new one" pattern this milestone needs. Confirmed against the official docs (base-ui.com/react/components/combobox, fetched live): `multiple` renders chips inside `Combobox.Chips`/`Combobox.Chip`/`Combobox.ChipRemove`, and the documented "creatable" pattern (render a "Create ‘X’" option when no exact match exists, add it via `onInputValueChange`) covers free-form tag creation without a second library. |
| `@base-ui/react` `Checkbox` (via existing `checkbox.tsx`) | 1.7.0 (installed) | Multi-select bulk-edit row selection | Already vendored (`web/app/components/ui/checkbox.tsx`), already used elsewhere in the app. A "select all" + per-row checkbox is plain React state (`Set<number>` of selected watchlist ids) — no state-management library needed at this list size. |
| `web/app/components/ui/table.tsx` (existing shadcn/base-ui wrapper) | n/a (already vendored) | Paste-a-list bulk-add review screen; multi-select bulk-edit row list | Already used for other tabular UI in the app; the review screen (best match + alternates per pasted name, nothing added until confirmed) is a straightforward extension, not a new UI pattern. |

### Development Tools

No new dev tools. `sqlc`, `golangci-lint`, `Vitest`/RTL, `prettier` (with `prettier-plugin-tailwindcss`) all already cover the code this milestone adds — the new Go code is more `internal/watchlist`/`internal/events` query and handler surface, and the new TSX is more components in the same tree Vitest/RTL already exercises.

## Installation

```bash
# No new packages required.

# Optional: bump the pinned Base UI version to pick up any interim
# Combobox/Chips fixes since 1.7.0 (not required — 1.7.0 already has the
# `multiple`/chips/creatable surface this milestone needs).
corepack pnpm --dir web update @base-ui/react@1.8.0
```

```sql
-- Backend: no new Go modules. Two new tables via a standard expand
-- migration (internal/db/migrations/READMEsafe — additive only):
--   tags (id, name, created_at)              + UNIQUE (lower(name))
--   watchlist_tags (watchlist_id, tag_id)    + composite PK, FKs ON DELETE CASCADE
-- Notes: one new nullable/DEFAULT-carrying column on watchlist
--   (notes TEXT, length-capped in application code + a CHECK constraint).
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| `@base-ui/react` `Combobox` (already installed) | `cmdk`, `downshift`, `react-select`, `react-tag-input` | Never for this milestone — all three would duplicate a primitive already vendored under the exact "shadcn CLI copies owned source, not a runtime component-library dependency" constraint this project has enforced since `06-CONTEXT.md`'s D-15 (`no heavier component library (Mantine/Chakra, etc.)`). Reach for one of these only if Base UI's `Combobox` genuinely can't express a future requirement (e.g. virtualized options at a scale this project doesn't have). |
| Normalized `tags` + `watchlist_tags` tables | `TEXT[]` column on `watchlist` (matching `release_types`/`muted_event_types`'s existing shape) | If tags were a small, fixed, admin-defined vocabulary (like `release_types` is) rather than free-form and user-renameable, the `TEXT[]` + `CHECK` pattern already in `000002_watchlist.up.sql` would be the better fit — it's simpler and already precedented. Free-form + global rename/delete is what tips this toward a real table. |
| Client-side sort/search/filter | `sqlc.narg`-based dynamic `WHERE` + `CASE`-based dynamic `ORDER BY` on `ListWatchlist` | Once the watchlist can realistically exceed roughly 2,000-5,000 rows (an order of magnitude past this milestone's stated 50-500), or once the UI needs server-side pagination for the Watchlist view the way `ListEvents` already does for History. At that point, follow `ListEvents`'s existing `sqlc.narg('artist_id')::bigint IS NULL OR ...` idiom for the filter axes; for sort, see the Pitfall below before reaching for `CASE ... END` inside `ORDER BY`. |
| Functional `UNIQUE (lower(name))` index | `citext` extension | If case-insensitive comparison needs to happen in many more places across the schema (not just one tag-name column) such that repeating `lower(...)` everywhere becomes genuinely error-prone — at that point a single `citext` column type pays for itself. One column, one place, doesn't clear that bar. |
| Reuse existing `GET /search` result ordering as "best match" for paste-a-list review | A client-side fuzzy-match library (`Fuse.js`, `string-similarity`) to re-rank multi-source search results | If the existing search ranking (MusicBrainz + Deezer merged, Deezer fan-count popularity ranking already shipped in v1.2) proves insufficient in practice for picking a correct "best match" among near-duplicate artist names during UAT. Not assumed necessary up front — the milestone's own spec text says "per-name rate-limited search (via existing GET /search)," implying reuse of the existing ranked result, not a new ranking layer. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `pg_trgm` + a `GIN` trigram index for tag or artist-name autocomplete | Solves a problem this milestone doesn't have. Trigram/GIN indexing exists to make `ILIKE '%substr%'`/fuzzy search fast over tens of thousands-plus rows; at 50-500 watchlist rows and a tag vocabulary that will realistically be dozens-to-low-hundreds of distinct values, an unindexed `ILIKE`/client-side substring filter is sub-millisecond. Adding the extension is pure surface area (first `CREATE EXTENSION` in the migration tree, a new thing `cmd/migration-check`/N-1 rollback reasoning has never covered) for no measurable benefit at this scale. | Plain `WHERE name ILIKE $1 \|\| '%'` (tag-name autocomplete query against the small `tags` table) or client-side `.filter()` (watchlist name search, already-fetched full list). |
| `citext` extension | Same "first extension, no scale justification" argument as `pg_trgm` above — see the Alternatives row for the one case where it would pay for itself. | `UNIQUE (lower(name))` functional index; compare/insert against `lower($1)`. |
| A `CASE WHEN @sort_by = 'name' THEN ... END`-style dynamic `ORDER BY` column selector in a `ListWatchlist`/`ListWatchlistSorted` sqlc query, **right now** | Two independent reasons, not just "unneeded at this scale": (1) sqlc has no first-class support for parameterized *column* selection in `ORDER BY` — it's an open upstream request (sqlc-dev/sqlc#2061) — so this pattern only works by writing raw, sqlc-opaque SQL text and hoping the static analysis passes it through unexamined, which is more fragile than sqlc's usual guarantees. (2) Postgres itself has a documented planner footgun where a `CASE`-in-`ORDER BY` used purely to pick between columns can silently be optimized away/ignored under specific query shapes (real report on pgsql-hackers, "Case in Order By Ignored without warning or error") — a correctness risk, not just a style preference, for something that would ship with much thinner test coverage than `ListEvents`'s keyset-pagination `ORDER BY` already has. | Client-side `Array.prototype.sort()` over the already-fetched full watchlist (this milestone's scale) or, once genuinely needed at larger scale, one **separate, explicit sqlc query per sort axis** (`ListWatchlistByName`, `ListWatchlistByCreatedAt`, `ListWatchlistByLatestRelease`) rather than one query with a dynamic `ORDER BY` — mirrors how `AdvanceGroupTrackCountBaseline` and other queries in this codebase prefer one explicit, fully-typed statement per case over one parameterized "does everything" statement. |
| A new npm tag-input/multi-select/virtualization package (`react-tag-input`, `@tanstack/react-virtual`, `react-window`, etc.) | `@base-ui/react`'s `Combobox` already covers tag input + multi-select (see above), and 50-500 DOM rows is well under any threshold where list virtualization matters — plain `<table>`/mapped rows render instantly at this size, and virtualizing would add scroll-position/focus-management complexity (keyboard nav through a virtualized multi-select list, in particular) for zero perceptible benefit. | Existing `@base-ui/react` primitives + plain mapped React lists. |

## Stack Patterns by Variant

**If the watchlist ever needs to support genuinely large lists (thousands of entries, multiple users' combined watchlists, etc.):**
- Move `ListWatchlist` to `sqlc.narg`-filtered, paginated queries (mirroring `ListEvents`'s keyset pattern) instead of client-side filtering.
- Reconsider `pg_trgm`/`GIN` for tag and name autocomplete at that point — the "why not" above is a scale argument, not a permanent one.
- Because the entire recommendation above is scoped to this milestone's explicit 50-500 target; re-derive it if that assumption changes.

**If tag "merge duplicates" (not just rename) becomes a requirement (e.g. an operator wants to fold "Hip Hop" and "Rap" into one tag after the fact):**
- The `tags`/`watchlist_tags` join-table shape already supports this cleanly: merging is `UPDATE watchlist_tags SET tag_id = $keep WHERE tag_id = $merge ON CONFLICT DO NOTHING; DELETE FROM tags WHERE id = $merge`.
- Because a `TEXT[]`-on-`watchlist` shape would need an `array_replace` scan across every affected row for the same operation — another point in favor of the normalized shape even though this milestone's spec only asks for rename/delete, not merge.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `@base-ui/react` 1.7.0 (installed) | React 19.2.6, `shadcn` 4.16.2 | `Combobox`'s `multiple`/`Chips`/`Chip`/`ChipRemove` surface has been stable since Base UI 1.1.0 (chip-related a11y fix landed then; no breaking change to the multi-select API since) — 1.7.0 is well past that. 1.8.0 is current upstream as of this research; bumping is optional polish, not a requirement for this milestone's features to work. |
| sqlc v1.31.1 (pinned) | Postgres 16 (`docker-compose.yml`), `pgx/v5` output (`sqlc.yaml`) | The `sqlc.narg` optional-filter idiom this doc points to for future scale-up is the same idiom already proven working end-to-end in `queries/events.sql`'s `ListEvents`/`HasOlderEvents` under this exact sqlc/pgx/Postgres version combination — no new compatibility surface. |
| New `tags`/`watchlist_tags` migrations | `internal/db/migrations/README.md`'s expand/contract rule | Both new tables are pure additions (`CREATE TABLE`), and the new `watchlist.notes` column is nullable/`DEFAULT`-carrying — nothing here needs the contract half of expand/contract, and nothing here is a backward-incompatible change against the currently-deployed binary (N-1 rule is a non-issue for additive `CREATE TABLE`/nullable `ADD COLUMN`). |

## Sources

- `web/node_modules/.pnpm/@base-ui+react@1.7.0.../@base-ui/react/combobox/` — direct inspection of the installed package's shipped parts (`chip`, `chips`, `chip-remove`, `combobox`) and `root/ComboboxRoot.d.ts`/`root/AriaCombobox.d.ts` type definitions (`multiple`, `filter`, `items`, `onInputValueChange`) — confidence HIGH (primary source, the exact installed version)
- https://base-ui.com/react/components/combobox — fetched live; confirmed `multiple` + `Combobox.Chips`/`Combobox.Chip`/`Combobox.ChipRemove` multi-select pattern and the documented "creatable" (render-a-Create-option) pattern for free-text tag creation — confidence HIGH (official docs)
- WebSearch, "@base-ui/react npm latest version combobox chips" — confirmed 1.8.0 is current upstream vs. the project's pinned 1.7.0, and that the chips/`toolbar`-role a11y behavior has been present since 1.1.0 — confidence MEDIUM (aggregated search, cross-checked against the installed package's own changelog path)
- `queries/events.sql`, `queries/watchlist.sql`, `internal/db/migrations/000002_watchlist.up.sql` — direct inspection of this codebase's existing sqlc/Postgres patterns (`sqlc.narg` optional filters, `TEXT[]` + `CHECK` preference columns, lexicographically-sortable partial-date `TEXT` column) — confidence HIGH (primary source, this repo)
- https://github.com/sqlc-dev/sqlc/issues/2061 ("Support dynamic order by clause") — confirmed sqlc has no first-class dynamic-`ORDER BY`-column support as of this research — confidence MEDIUM (GitHub issue, community-sourced, cross-checked against sqlc's documented static-analysis design)
- pgsql-hackers thread, "Case in Order By Ignored without warning or error" — confirmed a real, reported Postgres planner behavior where `CASE`-based `ORDER BY` column selection can be silently ignored under some query shapes — confidence MEDIUM (mailing-list report, not a changelog-confirmed bug; treated as a caution, not a blanket prohibition)
- `internal/db/migrations/README.md`, `.planning/codebase/STACK.md`, `.planning/codebase/ARCHITECTURE.md`, `.planning/PROJECT.md` — this project's existing constraints (D-15 no-heavier-component-library, expand/contract migration rule, sqlc/pgx/Postgres pinned versions) — confidence HIGH (primary source, this repo)

---
*Stack research for: drop-tracker v1.6 Watchlist Organization*
*Researched: 2026-09-22*
