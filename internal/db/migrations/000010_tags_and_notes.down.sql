-- Paired down migration for 000010 (never run by the app; see
-- internal/db/migrations/README.md -- required to exist for the pair).
ALTER TABLE watchlist DROP COLUMN IF EXISTS note;
DROP TRIGGER IF EXISTS artist_tags_cap_trigger ON artist_tags;
DROP FUNCTION IF EXISTS check_artist_tags_max_per_artist();
DROP TABLE IF EXISTS artist_tags;
DROP TABLE IF EXISTS tags;
