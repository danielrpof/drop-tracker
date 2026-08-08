-- Reverse of 000004_events_display_fields.up.sql -- drops in reverse order
-- of the adds.
ALTER TABLE events DROP COLUMN release_type;
ALTER TABLE events DROP COLUMN previous_track_count;
