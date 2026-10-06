# Phase 24: Artist Tags & Notes - Research

**Researched:** 2026-09-22
**Domain:** PostgreSQL triggers/upsert semantics, sqlc/pgx codegen, Go service-layer design, base-ui Combobox, migration-safety tooling
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Tag chips render under the artist name/disambiguation, wrapping inside the name column of `WatchlistRow`. Preference toggles and the remove button stay where they are.
- **D-02:** A compact "+ tag" button after the chips opens the autocomplete input in place; Esc or blur closes it. No always-visible input on every row.
- **D-03:** Chip add/remove is optimistic with rollback and an error toast, mirroring `PreferenceToggles`' optimistic-update-then-rollback pattern. **Amended by D-24:** the rollback mechanism is functional add/remove updaters, not whole-array snapshots.
- **D-04:** Chips are plain labels with an × remove control. Clicking a chip does nothing in this phase (tag filtering arrives in Phase 25).
- **D-05:** The note shows inline under the chips as muted text, clamped to 2 lines with a "more" expander. An empty note shows only an "add note" affordance.
- **D-06:** The note is edited in place: textarea with a character counter and explicit Save / Cancel; Esc cancels. Clearing the text and saving clears the note.
- **D-07:** Global tag management is a "Manage tags" dialog opened from a button in the Watchlist header. No new route, no /system section.
- **D-08:** Each row shows the tag name and its watched-artist count (e.g. "reggaeton · 12 artists"), with Rename and Delete actions, sorted by name.
- **D-09:** Rename is inline in the list (name becomes an input with Save/Cancel). If the normalized new name collides with a different existing tag, a merge confirmation opens naming both tags; nothing merges without it. A case-only rename of the same tag is a plain rename, not a merge.
- **D-10:** Notes live on the `watchlist` row: removing an artist deletes the note, and a re-add starts blank. Tags live on `artists.id` and survive remove/re-add (TAG-07). — **Reversibility:** one-way — moving notes to the artist later needs a migration plus a data backfill.
- **D-11:** Counts in Manage tags and in the delete confirmation include only artists currently on the watchlist. Deleting a tag still removes its links to removed artists too; they just aren't counted.
- **D-12:** A tag with zero carriers stays in the vocabulary (shown as "0 artists") until explicitly deleted. No auto-cleanup.
- **D-13:** Autocomplete suggests every tag in the vocabulary, including tags only removed artists carry — autocomplete and Manage tags show the same set.
- **D-14:** At 10 tags the "+ tag" button is replaced by a muted "max 10 tags" hint; removing a chip restores it. API and DB still refuse an 11th tag.
- **D-15:** The tag input hard-stops at 32 characters (`maxLength`) with a small live counter that appears near the limit. API and DB still refuse >32.
- **D-16:** Merge and delete confirmations are modal alert dialogs built on `@base-ui/react`'s AlertDialog: title, one-sentence consequence naming the tag(s) and artist count, Cancel + primary/destructive action. Build it as a reusable component — Phase 26's bulk-remove confirm reuses it.
- **D-17:** After a successful merge/delete: a sonner toast (e.g. "Deleted "X" from 12 artists"), and both the Manage tags list and the Watchlist cards update without a page reload. No undo.
- **D-18:** The 10-per-artist cap is a `BEFORE INSERT` trigger on `artist_tags` that (1) returns early if the `(artist_id, tag_id)` link already exists, (2) locks the artist row `FOR NO KEY UPDATE`, and (3) raises `check_violation` with `CONSTRAINT = 'artist_tags_max_per_artist'` at 10. The service maps errors by constraint name. A guarded insert can't satisfy SC2 ("DB refuses even when the API check is bypassed"). See `docs/adr/0004-per-artist-tag-cap-trigger.md`. `internal/sqlscan` already handles `$$` bodies (`lex_test.go:60`).
- **D-19:** Merges never insert. They delete the source links of artists that already carry the target, run `UPDATE artist_tags SET tag_id = target` for the rest, then delete the source tag, all in one transaction. Insert-then-delete would briefly give a 10-tag artist 11 links and trip D-18.
- **D-20:** Tag attach/detach routes live under the watchlist entry: `POST /watchlist/{id}/tags {name}` and `DELETE /watchlist/{id}/tags/{tag_id}`. The server resolves `artist_id` from the entry and returns 404 when the entry doesn't exist. This uses the same id space as PATCH, the note, and Phase 26's bulk `ids`.
- **D-21:** Attach and detach are idempotent. Attaching a tag the artist already has returns 200 with the tag. Detaching a link that doesn't exist returns 204. Phase 26 depends on these semantics.
- **D-22:** Rename/merge/delete API: `GET /tags` returns `[{id, name, carrier_count}]` (carriers only, D-11); `PATCH /tags/{id} {name}` returns 200 with `{id, name}` for a plain rename, including a case-only one; a collision returns **409** with `{target: {id, name}, carrier_count_after_merge}`, detected from the `lower(name)` unique violation (SQLSTATE plus index name, the `watchlist_artist_id_key` pattern), never by checking first; `POST /tags/{id}/merge {into: <target_id>}` merges by **id**, returning 404/409 if the target was renamed or deleted meanwhile; `DELETE /tags/{id}` returns 200 with `{carrier_count}` for the toast.
- **D-23:** A merge keeps the target's stored casing (TAG-03). Renaming `rap` to `TRAP` when `trap` exists results in `trap`.
- **D-24:** Optimistic chips use functional route-level updaters (`addTag(entryId, tag)`, `removeTag(entryId, tagId)`). Pending chips live in a per-row pending set under a temporary id and render after the server's tags, so a `refresh()` can't wipe them. A rollback removes only its own item. This supersedes D-03's whole-array patch.
- **D-25:** The note has its own endpoint, `PUT /watchlist/{id}/note {note: string|null}`. It is trimmed, and empty text saves as `null`. `PATCH /watchlist/{id}` stays preferences-only, so Phase 26's bulk preferences request can never set notes.
- **D-26:** `POST /watchlist` and `PATCH /watchlist/{id}` return `tags` and `note` through the same enrichment projection as `GET /watchlist`, so `WatchlistEntry` is accurate on every route. `tags` is never `null`.
- **D-27:** The remove toast's Undo re-adds the note: `POST /watchlist` accepts an optional `note`. This closes the silent note loss D-10 would otherwise add to Undo. Phase 26/27 copy reuses it.
- **D-28:** Migration `000010` adds these on top of the ROADMAP schema: the column is **`watchlist.note`** (singular), `CHECK (name = btrim(name))` on `tags`, and `CHECK (note IS NULL OR btrim(note) <> '')`. All of them are free now and cost an expand/contract cycle after release.
- **D-29:** Get-or-create is one statement, `INSERT ... ON CONFLICT (lower(name)) DO UPDATE SET name = tags.name RETURNING id`. It runs in the same transaction as the link insert, and there is no fallback `SELECT`.
- **D-30:** The tag vocabulary loads on first use: when "+ tag" is first opened or Manage tags opens. It is then kept in route state and updated on create, rename, merge, and delete. Until it has loaded, the combobox offers only `Create "{q}"`, and attaching still works. Opening the Watchlist stays one request (Phase 25 SC5).
- **D-31:** Test notes: the case/whitespace identity tests include a non-ASCII pair (`REGGAETÓN` vs `reggaetón`), because Postgres `lower()` depends on the database collation. The client's exact-match pinning applies `.normalize("NFC")` before lower-casing. ADR 0004 lists the three required trigger tests.

### Claude's Discretion

- Where the counter threshold for D-15 starts, exact copy for hints/errors/toasts, and chip visual styling (the UI phase pinned these already, per 24-UI-SPEC.md). For chip styling, tag-vocabulary layout, and any palette/font-pairing choices, consult the `ui-ux-pro-max` skill rather than deciding from scratch.
- `GET /watchlist` enrichment query form (correlated `array_agg` subquery recommended) — must stay one query, no per-artist follow-up. **This research recommends two parallel `array_agg` subqueries (ids + names), not `json_agg`** — see Pattern 5 and Pitfall 1 below.

### Deferred Ideas (OUT OF SCOPE)

- Clicking a tag chip to filter the Watchlist — Phase 25.
- Six reviewed pending todos matched only on generic keywords and are unrelated to tags/notes (tooling chores, notifier chores, or already assigned to Phase 28) — see 24-CONTEXT.md's Reviewed Todos list for the full accounting.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| TAG-01 | Add free-form tags with autocomplete; new tags created on the fly | Pattern 4 (D-29 get-or-create statement verified against Postgres `ON CONFLICT` expression-index syntax); Standard Stack / base-ui Combobox verified directly against installed `node_modules` |
| TAG-02 | Remove a tag from an artist | D-21 idempotent detach (existing `DeleteWatchlistEntry`-style `:execrows` idiom applies identically) |
| TAG-03 | Case/whitespace-insensitive matching, first-entered casing preserved | Pattern 4 (`lower(name)` expression index); Pitfall 3 / Assumption A1 (non-ASCII collation, flagged as unverified in this sandbox) |
| TAG-04 | 32-char / 10-per-artist caps, enforced by API and DB | Patterns 1-3 (ADR 0004 trigger timing, row visibility, lock compatibility, all verified against PostgreSQL docs) |
| TAG-05 | Global rename; rename-onto-existing asks to confirm merge | Pattern 4 (constraint-name collision detection via `pgconn.PgError.ConstraintName`, verified by reading pgx source) |
| TAG-06 | Global delete with a carrier-count confirmation | Architectural Responsibility Map (DB owns cascade via `ON DELETE CASCADE`; API computes D-11's watched-only count) |
| TAG-07 | Tags belong to the artist, survive remove/re-add | Architectural Responsibility Map; Code Examples (mirrors `TestService_Remove_LeavesArtistRowIntact` precedent, read directly this session) |
| NOTE-01 | Add/edit/clear a ≤500-char plain-text note on a watchlist entry | D-25/D-28 (nullable `watchlist.note` column, inline `CHECK`, confirmed **not** a migration-check finding by reading `internal/sqlscan/parse.go` directly) |

</phase_requirements>

## Summary

This research verifies the technical foundations 24-CONTEXT.md's locked decisions (D-01…D-31) and ADR 0004 depend on, using PostgreSQL's own documentation, this repository's actual source (`internal/sqlscan`, `pgx/v5@v5.10.0`, `internal/watchlist`, `internal/httpserver`), and the installed `@base-ui/react@1.7.0` package — not blog posts. Every item the phase context flagged as needing verification is confirmed:

- The ADR 0004 trigger design (`BEFORE INSERT` row trigger, skip-if-exists, `FOR NO KEY UPDATE` row lock, `RAISE ... USING CONSTRAINT`) is sound against primary PostgreSQL documentation: a `BEFORE INSERT` row trigger fires and its effects are visible before `ON CONFLICT` arbitration runs; `FOR NO KEY UPDATE` does not conflict with the `FOR KEY SHARE` lock a `watchlist`/`events` foreign-key insert takes on the same `artists` row; `RAISE EXCEPTION ... USING ERRCODE, CONSTRAINT` is documented PL/pgSQL syntax; and `pgconn.PgError` (pgx v5.10.0, the pinned version) carries a `ConstraintName` field read directly from source.
- `internal/sqlscan.Parse` only structurally recognizes `CREATE TABLE`/`ALTER TABLE`/`DROP TABLE`; a `CREATE FUNCTION ... $$ ... $$ LANGUAGE plpgsql;` and `CREATE TRIGGER ...` statement falls through as an unclassified `RawStatement` and triggers **no** backward-incompatible or unsafe-forward finding. sqlc v1.31.1 (pinned, well past the v1.8.0 release that fixed sqlc-dev/sqlc#305) parses `CREATE FUNCTION`/`CREATE TRIGGER` schema statements without erroring, ignoring them for codegen purposes.
- `watchlist.note TEXT NULL CHECK (...)` added via `ALTER TABLE ... ADD COLUMN` is **not** flagged as a migration-check finding: it is nullable (no `NOT NULL`), so `unsafe-forward` never fires, and the inline `CHECK` sits inside the same `ADD COLUMN` clause (not a separate `ADD CONSTRAINT ... CHECK` clause against an *existing* column), so `backward-incompatible`'s `AddCheck` classification never fires either — confirmed by reading `internal/sqlscan/parse.go`'s regex/clause-ordering directly.
- `GET /watchlist`'s tag enrichment should use **two parallel `array_agg` correlated subqueries** (`tag_ids bigint[]`, `tag_names text[]`), not `json_agg`/`COALESCE(json_agg(...), '[]')` — a documented, still-open sqlc bug (sqlc-dev/sqlc#3438) makes a coalesced `json_agg` column emit `interface{}` instead of a usable Go type under `pgx/v5`. The parallel-array-agg approach has a direct, working precedent in this codebase (`watchlist.release_types text[]` → `[]string`).
- `@base-ui/react@1.7.0`'s installed `Combobox` (read directly from `node_modules`) supports single-value controlled mode with `value={null}`, `autoHighlight`, and ships a `combobox/empty` export — confirming the 24-UI-SPEC.md's Interaction Contract is implementable as written, not just as documented upstream.
- Postgres's non-ASCII `lower()` folding (`REGGAETÓN` → `reggaetón`) depends on the database's collation, which this repo's `docker-compose.yml` and CI service container leave at the `postgres:16` image default — **not directly observable in this sandbox** (no Docker daemon reachable here). This is flagged as an assumption requiring a live check at execution time, not a verified fact.

**Primary recommendation:** Implement ADR 0004's trigger exactly as specified; use the get-or-create statement `INSERT INTO tags (name) VALUES ($1) ON CONFLICT ((lower(name))) DO UPDATE SET name = tags.name RETURNING id` (D-29, confirmed valid `conflict_target` syntax against an expression index); enrich `GET /watchlist` with two parallel `array_agg` subqueries zipped into `[]TagRef` in Go, never `json_agg`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Tag identity/normalization (trim, NFC, lower-fold) | API / Backend (`internal/tags`) | Database (`UNIQUE (lower(name))`, `CHECK (name = btrim(name))`) | Go does the compare-shaping (trim/NFC) before any query; DB is the non-bypassable backstop (three-layer validation, this repo's established pattern) |
| Per-artist tag cap enforcement | Database (BEFORE INSERT trigger) | API / Backend (client-facing error mapping) | ADR 0004: only a DB-level mechanism satisfies "refuses even when the API check is bypassed"; the service layer's job is only to translate the resulting `PgError` into a typed error |
| Tag chip UI state (optimistic add/remove, pending set) | Browser / Client (`web/app/routes/watchlist.tsx`, `WatchlistRow.tsx`) | API / Backend (attach/detach idempotency, D-21) | Optimism requires client-side rollback machinery; its correctness depends on the server honoring D-21's idempotent semantics so a retry/duplicate pick is never an error |
| Tag vocabulary listing / rename / merge / delete | API / Backend (`internal/tags`, `GET/PATCH/DELETE /tags`, `POST /tags/{id}/merge`) | Database (unique-violation-as-collision-signal) | Global identity operations belong server-side; the collision detection strategy (D-22) is explicitly "from the unique violation, never check-then-insert" — DB is the source of truth for "does this name already exist" |
| `GET /watchlist` tags/note enrichment | Database (`ListWatchlist`'s single query) | API / Backend (Go-side zip of parallel arrays into `[]TagRef`) | Correlated subqueries keep this one round trip (D-30's "opening the Watchlist stays one request"); the zip step is pure Go with no I/O |
| Note storage/lifetime | Database (`watchlist.note`) | — | Single scalar column, no join table; D-10 ties its lifetime to the `watchlist` row directly |
| Migration safety gating | CI / Tooling (`cmd/migration-check`) | — | Enforced automatically pre-merge; not a runtime concern |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| PostgreSQL | 16 (pinned, `docker-compose.yml`) | Row trigger for the tag cap, expression unique index, `ON CONFLICT` upsert | Already the project's DB; every mechanism this phase needs (`BEFORE INSERT` triggers, `FOR NO KEY UPDATE`, expression indexes, `RAISE ... USING`) is stock PostgreSQL 16, no extension |
| `github.com/jackc/pgx/v5` | v5.10.0 (pinned, confirmed via `go.mod`/module cache) | Driver; `pgconn.PgError.ConstraintName` is what the service layer reads to map the trigger's `check_violation` to a typed error | Already locked project-wide; `ConstraintName` field verified by reading `pgconn/errors.go` directly in this session |
| `sqlc` | v1.31.1 (pinned, `Makefile`'s `SQLC_VERSION`) | Codegen for `queries/tags.sql` and the modified `watchlist.sql`/`events.sql` | Already locked; confirmed to handle `CREATE FUNCTION`/`CREATE TRIGGER` schema statements without erroring since v1.8.0 (sqlc-dev/sqlc#305, closed) |
| `golang.org/x/text` | v0.39.0 (already a **direct** dependency per `go.mod`) | `unicode/norm` for NFC normalization (D-31), `collate` for locale-aware tag-name sort in Manage tags (already used the same way in `internal/notifier/digest_format.go`/`digest_chunk.go`) | No new dependency — already imported elsewhere in this codebase for the identical purpose (case-insensitive, locale-aware sort) |
| `@base-ui/react` | 1.7.0 (installed, `web/package.json`) | `Combobox` (creatable, single-value), `Dialog`, `AlertDialog` | Already a runtime dependency; UI-SPEC's Combobox contract verified directly against the installed package's type definitions in this session |

### Supporting

No new supporting libraries. `internal/sqlscan` (already in-tree, `cmd/migration-check`'s parser) requires no changes for this phase's migration to pass.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Two parallel `array_agg(bigint)`/`array_agg(text)` subqueries for tag enrichment | `json_agg(json_build_object('id', t.id, 'name', t.name))` | Rejected: sqlc-dev/sqlc#3438 (open, no workaround documented) makes `COALESCE(json_agg(...), ...)` emit an untyped `interface{}` field under `pgx/v5`, not `[]byte`/a usable struct — this would need extra sqlc override config with no confirmed fix, whereas parallel `array_agg` on scalar columns has a working precedent in this exact codebase (`release_types text[]`) |
| `FOR NO KEY UPDATE` row lock in the trigger | `FOR UPDATE` | Rejected in ADR 0004: `FOR UPDATE` conflicts with the `FOR KEY SHARE` lock that `watchlist`/`events`' own FK inserts take on the same `artists` row (per the PostgreSQL 13.3 lock-compatibility matrix), which would serialize unrelated writes against `artists` during every tag attach — `FOR NO KEY UPDATE` only conflicts with itself, `FOR SHARE`, and `FOR UPDATE`, never `FOR KEY SHARE` |
| Get-or-create via `INSERT ... ON CONFLICT ((lower(name))) DO UPDATE SET name = tags.name RETURNING id` | `INSERT ... ON CONFLICT DO NOTHING RETURNING id`, fallback `SELECT` | D-29 already locks the one-statement `DO UPDATE` form specifically to avoid a fallback `SELECT`; confirmed valid syntax (expression `conflict_target` inference is documented, exact form `( index_expression )`) |

**Installation:** No new packages for either Go or the frontend. This phase's only "installation" is vendoring four shadcn components already enumerated in 24-UI-SPEC.md (`dialog`, `alert-dialog`, `combobox`, `textarea`), all first-party shadcn `base-maia` registry, already safety-gated by the UI-SPEC's Registry Safety section.

**Version verification performed this session:**
- `pgx/v5` — read `pgconn/errors.go` from the module cache at the exact pinned path `github.com/jackc/pgx/v5@v5.10.0`; confirmed present.
- `golang.org/x/text` — confirmed `v0.39.0` present in `go.mod` as a direct (not indirect) dependency.
- `@base-ui/react` — confirmed `1.7.0` via `web/node_modules/@base-ui/react/package.json`.
- sqlc — not reinstalled/reinvoked this session (no local sqlc binary invocation was run); version pin (`v1.31.1`) taken from `Makefile`'s `SQLC_VERSION` variable and cross-checked against the closed GitHub issue's fix version (v1.8.0), not independently re-run against a synthetic trigger migration in this sandbox. Recommend the executor run `sqlc generate` against the real migration as the actual gate, per `make sqlc-check`.

## Package Legitimacy Audit

Not applicable — this phase adds zero new npm packages and zero new Go modules (`golang.org/x/text` is already a direct dependency; `@base-ui/react` is already installed).

## Architecture Patterns

### System Architecture Diagram

```
Watchlist card (React)                Manage tags dialog (React)
   │ "+ tag" opens Combobox               │ list / rename / merge / delete
   ▼                                       ▼
apiFetch (CSRF header, 401 interceptor) ──┴──────────────┐
   │                                                       │
   ▼                                                       ▼
POST/DELETE /watchlist/{id}/tags            GET/PATCH/DELETE /tags, POST /tags/{id}/merge
PUT /watchlist/{id}/note                    (registerDataRoutes, gate.Authenticate + RequireCSRFHeader)
   │                                                       │
   ▼                                                       ▼
internal/tags.Service                        internal/tags.Service
  Attach: get-or-create (D-29) then            Rename: UPDATE ... RETURNING, or
  INSERT artist_tags ON CONFLICT DO NOTHING    409 collision (lower(name) unique violation)
  → BEFORE INSERT trigger (ADR 0004)           Merge: DELETE dup-links, UPDATE rest, DELETE tag
  → check_violation at 10 → typed error        Delete: DELETE tags (cascades artist_tags)
   │                                                       │
   ▼                                                       ▼
Postgres: tags, artist_tags (artist_id-scoped, ON DELETE CASCADE from artists)
   │
   ▼
GET /watchlist → ListWatchlist: watchlist JOIN artists
  + correlated array_agg(tag_id), array_agg(tag_name) subqueries
  + w.note column
  → internal/watchlist.Service zips arrays into []TagRef, builds Entry{Tags, Note}
```

### Recommended Project Structure

Matches `.planning/research/ARCHITECTURE.md`'s recommended structure exactly (already reviewed and endorsed in ROADMAP.md); this research adds no deviation. Key paths for this phase:

```
internal/
├── tags/                    # NEW: Service, Store interface, normalization
│   ├── service.go           #   GetOrCreateAndAttach, Detach, List, Rename, Merge, Delete
│   └── service_test.go
├── watchlist/
│   └── service.go           # MODIFIED: Entry gains Tags []TagRef, Note *string;
│                             #   List's row-to-Entry zips the two parallel arrays
internal/httpserver/
├── tags.go                  # NEW: GET/PATCH/DELETE /tags, POST /tags/{id}/merge,
│                             #   POST/DELETE /watchlist/{id}/tags
└── watchlist.go             # MODIFIED: PUT /watchlist/{id}/note (D-25); enrichment
                              #   fields flow through the existing toEntry-style mapper
queries/
├── tags.sql                 # NEW
└── watchlist.sql            # MODIFIED: ListWatchlist gains array_agg subqueries + note;
                              #   CreateWatchlistEntry gains an optional note param (D-27)
internal/db/migrations/
└── 000010_tags_and_notes.up.sql / .down.sql   # NEW
```

### Pattern 1: BEFORE INSERT trigger fires (and its effects are visible) before ON CONFLICT arbitration

**What:** A row-level `BEFORE INSERT` trigger executes, and any changes it makes are reflected, before Postgres decides whether an `ON CONFLICT` clause's arbiter index has a conflict.
**When to use:** ADR 0004's cap trigger, which must run and potentially abort the statement before a same-artist duplicate-tag `ON CONFLICT DO NOTHING` is resolved as a no-op.
**Verification:** `[CITED: postgresql.org/docs/current/sql-insert.html, postgresql.org/docs/current/trigger-definition.html]` — "BEFORE row insert triggers fire before the ON CONFLICT arbitration occurs... the effects of all per-row BEFORE INSERT triggers are reflected in excluded values, since those effects may have contributed to the row being excluded from insertion." An `INSERT ... ON CONFLICT DO UPDATE` executes statement-level `BEFORE INSERT` triggers, then `BEFORE UPDATE` on conflicting rows, meaning the row-level `BEFORE INSERT` trigger this ADR specifies always gets a chance to run and raise before conflict handling silently no-ops the row.

```sql
-- Source: ADR 0004 (docs/adr/0004-per-artist-tag-cap-trigger.md), verified
-- against PostgreSQL's documented trigger/ON CONFLICT execution order.
CREATE OR REPLACE FUNCTION check_artist_tags_max_per_artist() RETURNS trigger AS $$
DECLARE
    link_count int;
BEGIN
    IF EXISTS (
        SELECT 1 FROM artist_tags
        WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
    ) THEN
        RETURN NEW; -- skip-existing: keeps ON CONFLICT DO NOTHING a true no-op at the cap
    END IF;

    PERFORM 1 FROM artists WHERE id = NEW.artist_id FOR NO KEY UPDATE;

    SELECT count(*) INTO link_count FROM artist_tags WHERE artist_id = NEW.artist_id;
    IF link_count >= 10 THEN
        RAISE EXCEPTION 'artist already has the maximum of 10 tags'
            USING ERRCODE = 'check_violation', CONSTRAINT = 'artist_tags_max_per_artist';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER artist_tags_cap_trigger
    BEFORE INSERT ON artist_tags
    FOR EACH ROW
    EXECUTE FUNCTION check_artist_tags_max_per_artist();
```

### Pattern 2: Row visibility within a single multi-row INSERT (bulk attach, Phase 26)

**What:** SQL run inside a `BEFORE ROW` trigger during a multi-row `INSERT` sees the effects of rows already processed earlier in the *same* statement — Postgres increments the command counter between per-row trigger firings.
**When to use:** This is what makes the trigger's `SELECT count(*)` correct when Phase 26's set-based bulk tag-attach inserts several `artist_tags` rows for the same artist in one `INSERT` statement — each row's trigger invocation sees the count already updated by earlier rows in that same statement, so the cap is enforced cumulatively, not per-row-in-isolation.
**Verification:** `[CITED: postgresql.org/docs/17/trigger-datachanges.html, postgresql.org/docs/16/trigger-datachanges.html]` — "SQL commands executed in a row-level BEFORE trigger will see the effects of data changes for rows previously processed in the same outer command."

### Pattern 3: FOR NO KEY UPDATE does not block a concurrent FK insert's FOR KEY SHARE lock

**What:** Locking a referenced row with `FOR NO KEY UPDATE` (ADR 0004's artist-row lock) does not block another transaction's `INSERT` into `watchlist`/`events` that takes a `FOR KEY SHARE` lock on the same `artists` row to satisfy its foreign key.
**Verification:** `[VERIFIED: postgresql.org/docs/16/explicit-locking.html Table 13.3, fetched and quoted this session]` — the row-level lock compatibility matrix shows no conflict between "Requested: FOR NO KEY UPDATE" and "Current: FOR KEY SHARE" (conflicts are only NO KEY UPDATE↔NO KEY UPDATE, NO KEY UPDATE↔FOR SHARE, NO KEY UPDATE↔FOR UPDATE). Quoted directly from the fetched table:
```
Requested Lock Mode | Current Lock Mode
                    | FOR KEY SHARE | FOR SHARE | FOR NO KEY UPDATE | FOR UPDATE
FOR NO KEY UPDATE   |               | X         | X                 | X
```
No `X` in the `FOR KEY SHARE` column for a `FOR NO KEY UPDATE` request — this is exactly why `FOR NO KEY UPDATE`/`FOR KEY SHARE` exist as of Postgres 9.3: to let a row be updated (non-key columns) or locked-for-cap-check without blocking unrelated FK inserts referencing it.

### Pattern 4: Expression-index `ON CONFLICT` target and `RAISE ... USING CONSTRAINT`

**What:** `INSERT ... ON CONFLICT ((lower(name))) DO UPDATE SET name = tags.name RETURNING id` — the parenthesized `(lower(name))` is a valid `index_expression` conflict target matching `CREATE UNIQUE INDEX ... ON tags (lower(name))`.
**Verification:** `[VERIFIED: postgresql.org/docs/16/sql-insert.html, fetched and quoted this session]` — grammar: `conflict_target ::= ( { index_column_name | ( index_expression ) } [...] )`; "`index_expression` ... used to infer expressions on table_name columns appearing within index definitions ... Follows CREATE INDEX format." This directly supports D-29's one-statement get-or-create.
`RAISE EXCEPTION ... USING ERRCODE = 'check_violation', CONSTRAINT = 'artist_tags_max_per_artist'` — `[VERIFIED: postgresql.org/docs/16/plpgsql-errors-and-messages.html, fetched and quoted this session]` — `CONSTRAINT` is a documented `RAISE ... USING` option ("Supplies the name of a related object"). This constraint name surfaces in Go as `pgconn.PgError.ConstraintName` — `[VERIFIED: github.com/jackc/pgx/v5@v5.10.0 pgconn/errors.go:32-51]`:
```go
// Source: pgconn/errors.go, jackc/pgx v5.10.0 (module cache, read directly this session)
type PgError struct {
	Severity            string
	SeverityUnlocalized string
	Code                string
	Message             string
	Detail              string
	Hint                string
	Position            int32
	InternalPosition    int32
	InternalQuery       string
	Where               string
	SchemaName          string
	TableName           string
	ColumnName          string
	DataTypeName        string
	ConstraintName      string
	File                string
	Line                int32
	Routine             string
}
```
The existing `internal/watchlist/service.go` pattern (matching `pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key"`) is the direct precedent `internal/tags` should mirror for both the cap violation (`Code == "23514"` i.e. `pgerrcode.CheckViolation`, `ConstraintName == "artist_tags_max_per_artist"`) and the name-collision case (`Code == pgerrcode.UniqueViolation`, index name for `tags (lower(name))` — confirm the actual generated index/constraint name once the migration is written, following the `watchlist_artist_id_key` naming precedent D-22 already references).

### Pattern 5: sqlc/pgx tag-array enrichment — use parallel array_agg, not json_agg

**What:** `GET /watchlist`'s single-query enrichment (D-26, D-30) should project two correlated `array_agg` subqueries, not a JSON-aggregated composite array.
**Why:** `[CITED: github.com/sqlc-dev/sqlc/issues/3438, open as of this research]` — `COALESCE(json_agg(...), '[]'::jsonb)` under sqlc + `sql_package: pgx/v5` (this project's exact `sqlc.yaml` config) generates a Go field typed `interface{}` instead of a usable `[]byte`/struct, and the issue reporter found no working override. This project's `sqlc.yaml` has no `overrides:` section today (confirmed by reading the file), so this bug would surface as-is.
**Recommended pattern instead**, extending `queries/watchlist.sql`'s existing `ListWatchlist`:
```sql
-- Source: pattern derived from this repo's existing watchlist.release_types
-- (text[] column -> []string, confirmed working precedent) plus the D-26
-- "single query" and D-11 "carrier-only counts don't apply here" constraints.
SELECT w.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id,
       a.disambiguation, a.image_url,
       w.release_types, w.muted_event_types, w.note, w.created_at, w.updated_at,
       COALESCE(
         (SELECT array_agg(t.id ORDER BY t.name)
          FROM artist_tags at JOIN tags t ON t.id = at.tag_id
          WHERE at.artist_id = a.id),
         '{}'
       ) AS tag_ids,
       COALESCE(
         (SELECT array_agg(t.name ORDER BY t.name)
          FROM artist_tags at JOIN tags t ON t.id = at.tag_id
          WHERE at.artist_id = a.id),
         '{}'
       ) AS tag_names
FROM watchlist w
JOIN artists a ON a.id = w.artist_id
ORDER BY a.name ASC, a.id ASC;
```
sqlc emits `TagIds []int64` and `TagNames []string` (both non-null thanks to `COALESCE`, matching how `ReleaseTypes []string` already comes back non-null today). `internal/watchlist.Service.List` zips them index-for-index into `[]TagRef{ID, Name}` (both arrays are built from the same `ORDER BY t.name` subquery shape against the same row set, so their lengths and ordering always match — this is safe zip, not a heuristic). D-26 requires `tags` to never be `null` in the JSON response: initialize the Go slice with `make([]TagRef, 0, len(tagIds))` the same way `List` already does for `Entry` itself.

### Anti-Patterns to Avoid

- **`json_agg`/`COALESCE(json_agg(...), ...)` for the tag enrichment column:** hits the open sqlc/pgx `interface{}`-typing bug (Pattern 5). Use parallel `array_agg` instead.
- **Guarded `INSERT ... WHERE (SELECT count(*) ...) < 10` in application SQL for the cap:** ADR 0004 explicitly rejects this — bypassable by any raw `INSERT`, and doesn't hold under concurrency without the same row lock the trigger already needs.
- **Insert-then-delete for merge:** ADR 0004 explicitly rejects this — briefly gives a 10-tag artist 11 links and trips the trigger's own cap on a *legal* merge. Always `DELETE` the source's duplicate links, `UPDATE ... SET tag_id = target` for the rest, then `DELETE` the source tag (D-19).
- **Check-then-insert for get-or-create:** D-29 already locks the single `ON CONFLICT DO UPDATE` statement; a separate `SELECT` fallback reopens the exact TOCTOU race `AdvanceGroupTrackCountBaseline`'s existing doc comment warns against elsewhere in this codebase.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Case-insensitive tag uniqueness | A custom normalization table or app-level dedup pass | `UNIQUE INDEX ON tags (lower(name))` + Go-side `strings.ToLower`/NFC pre-normalization | Already the exact pattern `watchlist_artist_id_key`-style unique-violation mapping uses elsewhere in this codebase; a functional index is enough, no `citext` extension needed (matches STACK.md's "no new Postgres extensions" finding) |
| Per-artist tag count enforcement | A `tag_count` counter column maintained by separate triggers | The single `BEFORE INSERT` locking/counting trigger (ADR 0004) | ADR 0004 already evaluated and rejected the counter-column approach: it's extra state that can drift (cascades from tag delete, merge rewrites) |
| Locale-aware tag-name sort in Manage tags | A hand-rolled `Intl.Collator` on the frontend, or a naive Go `sort.Strings` | `golang.org/x/text/collate` server-side (if sorting server-side) or `Intl.Collator` client-side (already the plan per UI-SPEC's `localeCompare` with `sensitivity: "base"`) | `internal/notifier/digest_format.go`/`digest_chunk.go` already solved "locale-aware, case-insensitive sort" with `collate.New(language.Und, collate.IgnoreCase)`, built fresh per call (not package-level, since `collate.Collator` isn't concurrency-safe) — reuse that exact idiom if any server-side tag sort is ever needed |
| Unicode NFC normalization | A custom Unicode-combining-character stripper | `golang.org/x/text/unicode/norm` (`norm.NFC.String(...)`), already a direct module dependency | No new dependency; standard library-adjacent, battle-tested normalization |

**Key insight:** Every "don't hand-roll" item in this phase already has a working precedent somewhere else in this exact codebase (`ErrDuplicate`-style constraint mapping, `collate.Collator` per-call construction, `x/text` as a direct dependency). The work is applying the existing idiom to a new domain, not inventing anything new.

## Common Pitfalls

### Pitfall 1: `json_agg` enrichment silently produces an untyped sqlc field
**What goes wrong:** `COALESCE(json_agg(json_build_object('id', t.id, 'name', t.name)), '[]'::json)` compiles as SQL but sqlc generates a Go struct field typed `interface{}` under this project's `sql_package: pgx/v5` config, not a `[]byte`/usable type — the code won't compile against a `[]TagRef` field without an extra unmarshal step sqlc doesn't generate for you, and per the open upstream issue there's no confirmed override.
**Why it happens:** sqlc's JSON-column type inference has a known gap specifically in the `COALESCE(json_agg(...), ...)` shape under the pgx driver.
**How to avoid:** Use two parallel `array_agg` subqueries (Pattern 5) and zip them in Go — this is a scalar-array type sqlc already handles correctly and identically to `release_types`.
**Warning signs:** `sqlc generate` producing an `interface{}`-typed field where a typed slice/struct was expected; `git diff --exit-code -- internal/db/sqlc/` (the `sqlc-check` target) showing a surprising diff after adding the enrichment query.

### Pitfall 2: `internal/sqlscan.Parse` does not need any change for this migration, but a *later* `ALTER TABLE ... ADD CONSTRAINT ... CHECK` against `tags`/`artist_tags`/`watchlist.note` in a *future* release would be flagged
**What goes wrong:** Someone reads ADR 0004/D-28 and assumes any future CHECK tightening on these new columns follows the same "no finding" path as the initial migration.
**Why it happens:** The initial migration's CHECKs are inline in `CREATE TABLE`/`ADD COLUMN` clauses (never flagged, confirmed this session by reading `parse.go`'s clause classification order). A *later* migration adding `ALTER TABLE tags ADD CONSTRAINT ... CHECK (...)` against the by-then-already-shipped `tags` table **is** a `backward-incompatible` finding (`AddCheck` case), per the same code path.
**How to avoid:** D-28 already gets this right — "All of them are free now and cost an expand/contract cycle after release." Nothing to fix in this phase; just don't let a future phase add a length-cap tightening pass without re-reading `internal/db/migrations/README.md`.
**Warning signs:** `cmd/migration-check` going red on a future PR that "just" tightens a tag/note constraint.

### Pitfall 3: Postgres collation for `lower()` non-ASCII folding is not independently verified in this sandbox
**What goes wrong:** D-31's `REGGAETÓN` vs `reggaetón` test assumes `lower()` folds `Ó`→`ó` correctly under whatever collation the `postgres:16` Docker image initializes with by default. This session's sandbox has no reachable Docker daemon (`docker ps` failed: "failed to connect to the docker API"), so this could not be directly queried (`SHOW lc_collate; SHOW lc_ctype;`) against a live container this session.
**Why it happens:** `docker-compose.yml` sets no explicit `POSTGRES_INITDB_ARGS`/locale, so the effective collation is whatever the base image defaults to — commonly `en_US.utf8` for the official `postgres:16` image, which folds `Ó`→`ó` correctly, but this is the image's *default* behavior, not something this repo's config pins explicitly.
**How to avoid:** Before or during the phase's own test-writing, run `SHOW lc_collate; SHOW lc_ctype;` against the actual `make db-up` container (or the CI Postgres service) and record the result. If it comes back `C`/`POSIX` (unlikely but not excluded), `lower()`'s non-ASCII folding would NOT work correctly for `Ó`, and the D-31 non-ASCII test would need to either force a locale-aware comparison in SQL or document the gap.
**Warning signs:** The `REGGAETÓN`/`reggaetón` identity test failing specifically for the accented character while the plain-ASCII case-fold test passes.

**Assumption logged as A1** below — this is a probe this environment could not run, not a declared absent finding; it costs a confirmation checkpoint at the start of implementation (a two-line `SHOW lc_collate; SHOW lc_ctype;` against the dev/CI Postgres), not a blocked plan.

### Pitfall 4: Zipping two parallel `array_agg` arrays assumes matched ordering/length
**What goes wrong:** If a future edit changes one subquery's `ORDER BY` independently of the other (e.g. someone "simplifies" the names subquery to skip `ORDER BY t.name`), the two arrays could return in different orders for the same artist and the Go-side zip would silently pair the wrong id with the wrong name.
**Why it happens:** The two subqueries are textually independent SQL fragments that happen to be written identically today.
**How to avoid:** Keep both subqueries byte-for-byte identical in their `FROM`/`JOIN`/`WHERE`/`ORDER BY` clauses (differing only in the aggregated column), and add a unit test that asserts `len(tagIds) == len(tagNames)` and that a specific known multi-tag artist zips to the expected `{id, name}` pairs, not just a length check.
**Warning signs:** A `TagRef{ID: 7, Name: "wrong-name"}` showing up in `GET /watchlist` for an artist who does have tag id 7, just under a different name than the response shows.

### Pitfall 5 (carried from PITFALLS.md, restated for planning): the merge transaction must delete-then-update-then-delete, in that literal order, inside one transaction
**What goes wrong:** Any ordering other than ADR 0004's specified sequence (delete duplicate links → update remaining links → delete source tag) risks either a transient cap violation (insert-then-delete) or leaving the source tag's links dangling if the transaction is interrupted between steps.
**How to avoid:** Wrap all three statements in one DB transaction (`pgx.Tx`), matching this codebase's existing single-transaction multi-statement precedent style.
**Warning signs:** A merge test that doesn't explicitly assert the source tag's row is gone AND the target's link count includes both the union count.

## Code Examples

### Merge transaction (ADR 0004 / D-19)
```go
// Source: pattern derived from ADR 0004's literal ordering requirement;
// no existing multi-statement transaction precedent in this codebase uses
// this exact shape, so this is new code following the repo's general
// pgx.Tx usage conventions (not a verified copy of an existing function).
func (s *Service) Merge(ctx context.Context, sourceID, targetID int64) (carrierCount int64, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin merge tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	q := s.q.WithTx(tx)
	// 1. Delete source links for artists that already carry the target
	//    (these would otherwise become duplicate (artist_id, target) rows).
	if _, err := q.DeleteDuplicateSourceLinks(ctx, sourceID, targetID); err != nil {
		return 0, fmt.Errorf("delete duplicate source links: %w", err)
	}
	// 2. Rewrite remaining source links to point at target — never insert.
	if _, err := q.RepointSourceLinks(ctx, sourceID, targetID); err != nil {
		return 0, fmt.Errorf("repoint source links: %w", err)
	}
	// 3. Delete the now-empty source tag.
	if _, err := q.DeleteTag(ctx, sourceID); err != nil {
		return 0, fmt.Errorf("delete source tag: %w", err)
	}
	count, err := q.CountCarriers(ctx, targetID) // watched artists only, D-11
	if err != nil {
		return 0, fmt.Errorf("count carriers: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit merge tx: %w", err)
	}
	return count, nil
}
```

### Route registration (mirrors `internal/httpserver/server.go:261`)
```go
// Source: internal/httpserver/server.go:261-278, registerDataRoutes,
// read directly this session — new routes follow the identical pattern.
func registerDataRoutes(r chi.Router, s *Server) {
	// ... existing routes unchanged ...
	r.Get("/tags", s.handleListTags)
	r.Patch("/tags/{id}", s.handleRenameTag)
	r.Delete("/tags/{id}", s.handleDeleteTag)
	r.Post("/tags/{id}/merge", s.handleMergeTag)
	r.Post("/watchlist/{id}/tags", s.handleAttachTag)
	r.Delete("/watchlist/{id}/tags/{tag_id}", s.handleDetachTag)
	r.Put("/watchlist/{id}/note", s.handleUpdateNote)
}
```
Every one of these inherits `gate.Authenticate` + `gate.RequireCSRFHeader` automatically because `registerDataRoutes` is called on the protected sub-router (`server.go:242`) — no new middleware wiring needed, matching PITFALLS.md #4's warning.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Check-then-insert for get-or-create | Single `ON CONFLICT ... DO UPDATE RETURNING` statement | Already this codebase's established idiom (`UpsertArtist`) | D-29 just extends the existing house style to tags |
| `citext` for case-insensitive text | Functional `UNIQUE (lower(name))` index | STACK.md already settled this for v1.6 | Zero new extensions, consistent with "first CREATE EXTENSION would be new surface" reasoning already documented |

**Deprecated/outdated:** None specific to this phase — no library versions in the Standard Stack table are behind current pins.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `postgres:16` Docker image (as configured in this repo's `docker-compose.yml`, no explicit locale args) folds `lower('Ó')` → `'ó'` correctly, i.e. its default collation is not `C`/`POSIX` | Pitfall 3, D-31's non-ASCII test | If wrong, the `REGGAETÓN`/`reggaetón` identity test fails, and either the migration needs an explicit `LC_COLLATE`/ICU collation choice, or the phase needs a documented gap for non-ASCII tag folding. Low probability (official image default is `en_US.utf8`) but genuinely unverified this session — no Docker daemon was reachable. |
| A2 | The unique-violation constraint/index name for `tags (lower(name))` will be `tags_name_lower_idx` (or whatever name the actual `CREATE UNIQUE INDEX` statement in the written migration uses) — D-22's 409-collision detection needs to match on this exact name, mirroring the `watchlist_artist_id_key` precedent | Pattern 4, D-22 | If the executor names the index differently than what the service-layer constraint-name check expects, the collision-detection code silently falls through to the generic error path instead of returning 409 — must be kept in lockstep by writing both in the same PR and testing the actual constraint name, not assuming it |
| A3 | sqlc v1.31.1 parses this phase's specific `CREATE FUNCTION ... $$ ... $$ LANGUAGE plpgsql;` + `CREATE TRIGGER` migration text without error — confirmed via the closed upstream issue (fixed in v1.8.0) and this project's pin being well past that version, but **not independently re-run against the actual migration file in this sandbox** (no `sqlc generate` invocation happened this session) | Standard Stack, Version verification | If sqlc still stumbles on some specific syntax detail (e.g. a particular `RAISE ... USING` form), `make sqlc-check` fails at implementation time — low risk given the issue's closure, but the executor should run `sqlc generate` against the real file as the actual gate, not rely on this research alone |

## Open Questions

1. **Exact naming of the `tags (lower(name))` unique index/constraint**
   - What we know: D-22 requires detecting the collision "from the lower(name) unique violation (SQLSTATE plus index name, the watchlist_artist_id_key pattern), never by checking first."
   - What's unclear: The literal string the executor should match on isn't fixed by any locked decision — it depends on how the `CREATE UNIQUE INDEX` statement is named in the actual migration file.
   - Recommendation: Name it explicitly and predictably when writing migration `000010` (e.g. `tags_name_lower_idx`), and add a test asserting the service layer's constraint-name string literal matches the real migrated index name — don't let the two drift by writing them in the same commit and testing against a real Postgres instance (per this repo's `internal/watchlist/service_test.go` convention, which already does this for `watchlist_artist_id_key`).

2. **Postgres collation for non-ASCII folding (A1 above)**
   - What we know: The official `postgres:16` Docker image's default is commonly `en_US.utf8`, which folds `Ó`→`ó` correctly.
   - What's unclear: Not independently observed against this repo's actual `docker-compose.yml`/CI Postgres service container this session (no reachable Docker daemon).
   - Recommendation: Run `docker compose exec postgres psql -U drop_tracker -d drop_tracker -c "SHOW lc_collate; SHOW lc_ctype;"` once at the start of implementation and record the output in the plan's verification notes; this is a two-minute check, not a blocker.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker (for live Postgres collation check, A1) | Pitfall 3 verification | ✗ (this research session's sandbox — `docker ps` failed: no daemon reachable) | — | Executor runs the `SHOW lc_collate` check locally/in CI at implementation time; not a blocker for planning |
| PostgreSQL 16 | Migration 000010, trigger, expression index | Not directly queried this session (no reachable instance) | pinned `postgres:16` per `docker-compose.yml` | Same as above |
| `sqlc` CLI v1.31.1 | Codegen for `queries/tags.sql` | Not invoked this session | pinned per `Makefile SQLC_VERSION` | Executor runs `make sqlc`/`make sqlc-check` as the real gate |
| `@base-ui/react` 1.7.0 | Combobox/Dialog/AlertDialog | ✓ confirmed installed and read directly | 1.7.0 | — |
| `golang.org/x/text` v0.39.0 | NFC normalization, collation | ✓ confirmed direct dependency in `go.mod` | v0.39.0 | — |

**Missing dependencies with no fallback:** None — every gap above (Docker/live Postgres, sqlc invocation) has a documented fallback: the executor runs the real tool/gate at implementation time, which is the normal Definition-of-Done flow (`go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check`) already required by this project's `CLAUDE.md` before every commit.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework (backend) | Go stdlib `testing` + real-Postgres integration tests via `internal/testutil.NewTestPool` |
| Framework (frontend) | Vitest 4.x + React Testing Library, jsdom environment |
| Config file (backend) | none — plain `go test`, driven by `Makefile`'s `test`/`test-integration`/`test-short` targets |
| Config file (frontend) | `web/vitest.config.ts` (coverage: v8 provider, `text` + `json-summary` reporters) |
| Quick run command | `go test ./internal/tags/... ./internal/watchlist/... ./internal/httpserver/... -short` (unit-only, skips DB-backed tests per `testutil.RequirePostgresDSN`'s `testing.Short()` gate) |
| Full suite command | `make db-up && make test` (backend); `corepack pnpm --dir web test` (frontend) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TAG-01 | Attach existing tag via autocomplete; create-on-the-fly | integration (real Postgres) | `go test ./internal/tags/... -run TestService_Attach` | ❌ Wave 0 |
| TAG-01 | Combobox creatable UX (chip appears, input stays open) | component (Vitest/RTL) | `corepack pnpm --dir web test -- TagChips` | ❌ Wave 0 |
| TAG-02 | Detach a tag; idempotent 204 on already-gone link (D-21) | integration | `go test ./internal/tags/... -run TestService_Detach` | ❌ Wave 0 |
| TAG-03 | Case/whitespace identity, including non-ASCII (D-31) | integration | `go test ./internal/tags/... -run TestService_Identity` | ❌ Wave 0 |
| TAG-04 | 32-char / 10-per-artist caps, API + DB layer, concurrent race (ADR 0004) | integration, real-Postgres concurrency (mirrors `TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404`, `internal/httpserver/watchlist_test.go:610`) | `go test ./internal/httpserver/... -run TestTags_Attach_Concurrent` | ❌ Wave 0 |
| TAG-05 | Rename; rename-onto-existing triggers 409 + merge confirm | integration | `go test ./internal/tags/... -run TestService_Rename` | ❌ Wave 0 |
| TAG-06 | Delete with carrier count (D-11: watched-only) | integration | `go test ./internal/tags/... -run TestService_Delete` | ❌ Wave 0 |
| TAG-07 | Tags survive `watchlist.Service.Remove` + re-add (mirrors `TestService_Remove_LeavesArtistRowIntact`) | integration | `go test ./internal/watchlist/... -run TestService_Remove_LeavesArtistTagsIntact` | ❌ Wave 0 |
| NOTE-01 | Add/edit/clear note, 500-char cap, trims to null (D-25) | integration | `go test ./internal/watchlist/... -run TestService_Note` | ❌ Wave 0 |
| ADR-0004 test 1 | Two concurrent 10th/11th attaches — one succeeds | integration, goroutine race | `go test ./internal/httpserver/... -run TestTags_Attach_ConcurrentCapRace -count=25` (loop, per this repo's `TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive` precedent of looping an unforced race 25×) | ❌ Wave 0 |
| ADR-0004 test 2 | Set-based `ON CONFLICT` insert against an already-at-10 artist including an existing tag — no error | integration | `go test ./internal/tags/... -run TestTrigger_SkipExisting` | ❌ Wave 0 |
| ADR-0004 test 3 | Merge on a 10-tag artist carrying only the source tag — succeeds | integration | `go test ./internal/tags/... -run TestService_Merge_AtCap` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** unit-level tests for the file(s) touched (`go test ./internal/tags/... -short`, or the relevant Vitest file)
- **Per wave merge:** `make db-up && make test && make coverage-gate` (backend); `corepack pnpm --dir web test` (frontend, 70% gate)
- **Phase gate:** Full suite green (backend + frontend) before `/gsd-verify-work`, plus `make sqlc-check` (no CI counterpart — must be run and committed locally) and `cmd/migration-check`'s CI job on the migration PR

### Wave 0 Gaps
- [ ] `internal/tags/service_test.go` — new file, covers TAG-01…07 service-layer behavior
- [ ] `internal/tags/service.go` — new file, the package itself (no framework install needed, pure Go stdlib `testing`)
- [ ] `internal/httpserver/tags_test.go` — new file, covers the HTTP layer (400/404/409 mapping, CSRF/auth inheritance per PITFALLS.md #4)
- [ ] A real-Postgres concurrency test file/function proving ADR 0004's three required tests — can live in `internal/tags/service_test.go` or `internal/httpserver/tags_test.go` depending on whether the race is driven at the service or HTTP layer (HTTP layer matches this repo's existing concurrency-test precedent most closely)
- [ ] Frontend: `web/app/components/watchlist/TagChips.test.tsx`, `NoteEditor.test.tsx`, `ManageTagsDialog.test.tsx`, `ConfirmDialog.test.tsx` — new files; Vitest/RTL framework is already installed and configured, no new install needed

*(No framework installs needed — `go test`, Vitest, and React Testing Library are all already wired for this repo.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no (this phase adds no auth surface) | inherited `gate.Authenticate` on all new routes |
| V3 Session Management | no | inherited, unchanged |
| V4 Access Control | yes | every new route registered inside `registerDataRoutes` (protected group), never a new top-level route — this is the whole access-control surface for a single-operator, single-session app |
| V5 Input Validation | yes | Go-side trim/length-cap (32 tags, 500 notes) before any DB call, backed by DB `CHECK` constraints — three-layer pattern already established (handler → service → DB) |
| V6 Cryptography | no | not applicable — no secrets/crypto surface in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Tag/note text rendered as HTML | Tampering (stored XSS) | Plain JSX text nodes only, never `dangerouslySetInnerHTML` — already this repo's blanket rule (Phase 06 XSS posture), and 24-UI-SPEC.md's Copywriting Contract explicitly reaffirms it for tag names and artist names |
| New write routes registered outside the protected chi group | Elevation of Privilege | Every new route added inside `registerDataRoutes` (verified this session by reading `server.go:257-278`) — mirrors PITFALLS.md #4's warning exactly |
| Client bypassing `apiFetch` for a new call shape (e.g. a hand-rolled fetch for tag attach) | Tampering (missing CSRF header) | Route every new SPA call through the existing `apiFetch` wrapper (`web/app/lib/api.ts:184`), which already carries the CSRF header and the 401 interceptor |
| Trigger-based cap bypassed via a raw `INSERT` outside the API | Tampering | ADR 0004: DB-level trigger, not an application-only guard — this is precisely what SC2 in the phase's success criteria requires |

## Sources

### Primary (HIGH confidence)
- `postgresql.org/docs/current/sql-insert.html`, `postgresql.org/docs/current/trigger-definition.html` — BEFORE INSERT trigger timing relative to ON CONFLICT arbitration
- `postgresql.org/docs/17/trigger-datachanges.html`, `postgresql.org/docs/16/trigger-datachanges.html` — row visibility for BEFORE triggers within the same multi-row command
- `postgresql.org/docs/16/explicit-locking.html` (Table 13.3, fetched and quoted verbatim this session) — `FOR NO KEY UPDATE` / `FOR KEY SHARE` non-conflict
- `postgresql.org/docs/16/sql-insert.html` (fetched and quoted verbatim this session) — `ON CONFLICT` expression `conflict_target` grammar
- `postgresql.org/docs/16/plpgsql-errors-and-messages.html` (fetched and quoted verbatim this session) — `RAISE ... USING CONSTRAINT` syntax
- `github.com/jackc/pgx/v5@v5.10.0` `pgconn/errors.go` (read directly from the local module cache this session) — `PgError.ConstraintName` field
- `internal/sqlscan/parse.go`, `internal/sqlscan/lex.go`, `internal/sqlscan/lex_test.go` (read directly this session) — migration-check's statement classification; confirms `CREATE FUNCTION`/`CREATE TRIGGER` fall through as unclassified `RawStatement`, and `ADD COLUMN ... CHECK(...)` inline does not trigger the `AddCheck` finding class
- `web/node_modules/.../@base-ui/react/combobox/root/ComboboxRoot.d.ts`, `AriaCombobox.d.ts` (read directly this session) — confirmed `value?: ... | null`, `autoHighlight`, `multiple` props and the `combobox/empty` export exist in the installed 1.7.0 package
- `internal/httpserver/server.go` (read directly this session) — `registerDataRoutes` registration pattern, protected-group inheritance
- `internal/watchlist/service.go`, `queries/watchlist.sql` (read directly this session) — `ErrDuplicate` constraint-name-mapping precedent, `ListWatchlist`'s existing query shape
- `internal/httpserver/watchlist_test.go` (read directly this session) — `TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404` / `TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive` — real-Postgres concurrency test precedent for ADR 0004's required race tests
- `internal/db/migrations/README.md` (read directly this session) — N-1 rollback rule, `cmd/migration-check` finding classes
- `go.mod` (read directly this session) — `golang.org/x/text v0.39.0` confirmed as a direct dependency

### Secondary (MEDIUM confidence)
- `github.com/sqlc-dev/sqlc/issues/305` (fetched this session) — CREATE TRIGGER/CREATE FUNCTION parsing fixed in sqlc v1.8.0 (this project pins v1.31.1, well past)
- `github.com/sqlc-dev/sqlc/issues/3438` (fetched this session, open as of research date) — `COALESCE(json_agg(...), ...)` under pgx/v5 emits `interface{}`, no documented workaround — basis for recommending parallel `array_agg` instead

### Tertiary (LOW confidence / unverified this session)
- The exact `lc_collate`/`lc_ctype` the running `postgres:16` container/CI service uses (A1) — not independently queried, no Docker daemon reachable in this sandbox; standard official-image default (`en_US.utf8`) assumed but not confirmed

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version pin verified against `go.mod`/`package.json`/module cache directly, not training memory
- Architecture: HIGH — every claim traces to either a primary PostgreSQL doc fetched this session, this repo's own source read this session, or the pre-existing v1.6 milestone research (already HIGH-confidence and endorsed in ROADMAP.md)
- Pitfalls: HIGH for the sqlc/json_agg and migration-check findings (both verified against primary sources this session); MEDIUM for the collation pitfall (documented as an open assumption, not verified live)

**Research date:** 2026-09-22
**Valid until:** 30 days (stable domain — PostgreSQL/sqlc/pgx semantics don't shift quickly; re-verify the collation assumption (A1) and the sqlc issue #3438 status specifically if this research is reused past that window)
