-- Phase 13 (bug #3 / D-12, grilling round Q4): the artist-art backfill sweep
-- (sibling plan 13-03) re-visits every watchlisted, image-less artist on
-- every process restart unless something bounds the re-attempt rate. This
-- column exists solely to bound that re-attempt cost across restarts, not to
-- track anything about the match result itself -- image_url and deezer_id
-- already carry that. Nullable with no DEFAULT: existing rows and any
-- never-swept artist read as NULL, which is D-12's "never attempted" state
-- and ListArtistsMissingImage's own eligibility predicate.
ALTER TABLE artists ADD COLUMN art_match_attempted_at timestamptz;
