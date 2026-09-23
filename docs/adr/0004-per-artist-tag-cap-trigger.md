---
status: accepted
---

# The per-artist tag cap is a locking trigger that skips existing links

## Context

TAG-04 caps an artist at 10 tag links. Phase 24's success criteria require the
database to refuse an 11th link even when the API check is bypassed. A `CHECK`
cannot count rows, and a guarded `INSERT ... WHERE count < 10` only protects
inserts that go through the API's own SQL.

A naive `BEFORE INSERT` count has three traps. Two concurrent attaches can each
see 9 links and both insert. A `BEFORE` row trigger fires *before* `ON CONFLICT`
arbitration, so re-attaching a tag the artist already has fails at 10 instead of
being a no-op, and Phase 26's set-based bulk add relies on that no-op
("artists that already have the tag are unaffected"). A merge done as
insert-target-then-delete-source briefly holds 11 links on a 10-tag artist and
fails a legal merge.

## Decision

A `BEFORE INSERT` row trigger on `artist_tags`:

1. returns the row untouched when `(artist_id, tag_id)` already exists, so
   `ON CONFLICT DO NOTHING` stays a no-op at the cap;
2. locks the artist row with `FOR NO KEY UPDATE`, which serializes concurrent
   attaches for that artist without blocking the foreign-key `KEY SHARE` locks
   that `watchlist` and `events` inserts take on the same row;
3. counts the artist's links and raises
   `USING ERRCODE = 'check_violation', CONSTRAINT = 'artist_tags_max_per_artist'`
   at 10, so the service can map it by constraint name, as it already does for
   `watchlist_artist_id_key`.

Merges never insert. They delete the source links of artists that already carry
the target, `UPDATE artist_tags SET tag_id = target` for the rest, then delete
the source tag. The link count never goes up during a merge.

Links are only ever created by attach and bulk attach. Both go through this
trigger, and no code path inserts `artist_tags` rows any other way.

## Considered options

- **Guarded insert in application SQL.** Rejected: a raw `INSERT` bypasses it,
  which fails the "database refuses it" criterion.
- **`artists.tag_count` counter with `CHECK (tag_count <= 10)`**, maintained by
  insert/delete triggers. Rejected: the declarative check is attractive, but the
  counter is extra state that can drift from `artist_tags` (cascades from tag
  delete, merge rewrites), and it still needs triggers.
- **Plain count trigger, no lock, no skip.** Rejected for the three traps above.
- **Locking, skip-existing trigger.** Chosen.

## Consequences

- Any future writer that moves links between tags must rewrite `tag_id` with an
  `UPDATE` or delete first, never insert-then-delete.
- Tests must pin: two concurrent 10th/11th attaches (one succeeds), a set-based
  `ON CONFLICT` insert against an artist already at 10 that includes an existing
  tag (no error), and a merge on a 10-tag artist that carries only the source.
- The trigger's `$$` body passes `cmd/migration-check`, because `internal/sqlscan`
  handles dollar-quoted bodies (`lex_test.go`).
