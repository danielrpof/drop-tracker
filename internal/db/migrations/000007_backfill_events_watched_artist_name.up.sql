-- Debug session guest-feature-label-missing: 000006 added
-- events.watched_artist_name as a nullable column with no backfill, so every
-- row that already existed carries NULL. On its own that would heal on the
-- next detection sweep -- except InsertEvent is deliberately
-- ON CONFLICT (event_type, source, external_id) DO NOTHING (D-20's write-once
-- snapshot guarantee, explicitly NOT the COALESCE-refresh shape UpsertArtist
-- uses). Those two facts together make the NULL permanent: re-detecting an
-- already-seen recording updates nothing, so the guest-feature watchlist note
-- would never appear for any release detected before 000006 -- 244 of them at
-- the time this was written, versus a trickle of genuinely new ones.
--
-- The value is exactly reconstructible rather than guessed. Both detection
-- call sites build the row from a single watchlist entry, setting
-- ArtistID: entry.ArtistID alongside watchedName := entry.Name, so for every
-- historical row watched_artist_name is by construction the name of the
-- artist events.artist_id already points at. This UPDATE writes precisely
-- what the post-000006 insert path would have written.
--
-- Preserves 000006's stated invariant in both directions, verified against
-- live data before writing this migration: new_release rows had
-- artist_name = artists.name in 1369/1369 cases, so they stay equal and the
-- UI note remains hidden for them; guest_feature rows had
-- artist_name <> artists.name in 244/244 cases, so the note now renders. No
-- event row lacked a matching artist, so coverage is total.
--
-- Scoped to `WHERE watched_artist_name IS NULL` so it is idempotent and can
-- never overwrite a snapshot written by the live insert path -- a row already
-- populated is left exactly as detection recorded it. On a from-scratch
-- database the events table is empty and this updates zero rows.
UPDATE events e
SET watched_artist_name = a.name
FROM artists a
WHERE a.id = e.artist_id
  AND e.watched_artist_name IS NULL;
