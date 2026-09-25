-- Paired down migration for 000011 (never run by the app; see
-- internal/db/migrations/README.md -- required to exist for the pair).
DROP TRIGGER IF EXISTS artist_tags_cap_update_trigger ON artist_tags;

CREATE OR REPLACE FUNCTION check_artist_tags_max_per_artist() RETURNS trigger AS $$
DECLARE
    link_count int;
BEGIN
    IF EXISTS (
        SELECT 1 FROM artist_tags
        WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
    ) THEN
        RETURN NEW;
    END IF;

    PERFORM 1 FROM artists WHERE id = NEW.artist_id FOR NO KEY UPDATE;

    IF EXISTS (
        SELECT 1 FROM artist_tags
        WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
    ) THEN
        RETURN NEW;
    END IF;

    SELECT count(*) INTO link_count FROM artist_tags WHERE artist_id = NEW.artist_id;
    IF link_count >= 10 THEN
        RAISE EXCEPTION 'artist already has the maximum of 10 tags'
            USING ERRCODE = 'check_violation', CONSTRAINT = 'artist_tags_max_per_artist';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
