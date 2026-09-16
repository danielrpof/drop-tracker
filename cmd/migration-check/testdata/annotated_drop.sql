-- migration-check:allow-destructive expand-shipped-in=v1.7.0 reason=events.release_type superseded by watched_artist_name
ALTER TABLE events DROP COLUMN release_type;
