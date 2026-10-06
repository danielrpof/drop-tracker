-- ADR 0004 amendment (gap 1, review WR-03): the cap must also fire when an
-- UPDATE moves a link onto another artist, not just INSERT. Merges rewrite
-- only tag_id, so the column-scoped trigger below never fires for them (D-19).

CREATE OR REPLACE FUNCTION check_artist_tags_max_per_artist() RETURNS trigger AS $$
DECLARE
    link_count int;
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.artist_id = OLD.artist_id THEN
        RETURN NEW;
    END IF;

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

CREATE TRIGGER artist_tags_cap_update_trigger
    BEFORE UPDATE OF artist_id ON artist_tags
    FOR EACH ROW
    EXECUTE FUNCTION check_artist_tags_max_per_artist();
