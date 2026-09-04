-- migration-check:allow-destructive reason=events.release_type superseded by watched_artist_name
ALTER TABLE events DROP COLUMN release_type;
