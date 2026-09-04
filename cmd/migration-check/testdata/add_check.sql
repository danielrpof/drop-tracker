ALTER TABLE events ADD CONSTRAINT events_track_count_positive CHECK (track_count >= 0);
