-- Reverse of 000006_events_watched_artist_name.up.sql -- drops the added
-- column.
ALTER TABLE events DROP COLUMN watched_artist_name;
