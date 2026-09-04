ALTER TABLE events DROP COLUMN release_type;
ALTER TABLE events ADD COLUMN foo text NOT NULL;
ALTER TABLE events DROP COLUMN notified_at;
