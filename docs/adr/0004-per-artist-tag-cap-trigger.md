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
- The existence check runs again after the artist lock, so two concurrent
  attaches of the same tag to a 9-tag artist both succeed instead of the
  second racing into a spurious cap violation.

## Amendment: UPDATE OF artist_id (migration 000011)

The gap: the trigger above was `BEFORE INSERT` only, so a raw
`UPDATE artist_tags SET artist_id = ...` gave an artist an 11th link with no
error (24-VERIFICATION gap 1, review WR-03). The fix: 000011 attaches the same
function to `artist_tags_cap_update_trigger BEFORE UPDATE OF artist_id`, and
the function returns early when `artist_id` is unchanged. Merges rewrite only
`tag_id`, so this column-scoped trigger never fires for them (D-19 holds). The
database guarantee now covers every way a link can reach an artist -- INSERT
and UPDATE of `artist_id` -- replacing the earlier application-level promise
that no code path moves links. Pinned by `TestSchema_TagCapTrigger_RawUpdateRefused`,
`TestSchema_TagCapTrigger_UpdatePaths`, and `TestSchema_Migration000011_DownUpRoundTrip`.

## Amendment: concurrent detach (migration 000012)

The gap: the skip-existing checks only tested visibility, so a link being deleted by an uncommitted detach still counted as existing (24-VERIFICATION re-verification gap 1, review CR-01). A re-attach or move of it skipped the artist lock and the count, and a third attach counted 9 and landed an 11th link.
The fix: 000012 locks the existing link `FOR KEY SHARE`. That lock waits for an in-flight delete or key-changing update, and when the row is gone it falls through to the artist lock and the count. Uncommitted inserts stay invisible, so two same-tag attaches at 9 still both succeed.
With this, the 000011 amendment's "every way a link can reach an artist" also holds under concurrency.
Pinned by `TestSchema_TagCapTrigger_ConcurrentDetachRace`, `TestSchema_TagCapTrigger_DistinctTagConcurrentAt9`, and `TestSchema_Migration000012_DownUpRoundTrip`.
