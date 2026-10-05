-- ADR 0004 amendment (CR-01): the skip-existing checks lock the existing link
-- FOR KEY SHARE instead of testing visibility, so an uncommitted concurrent
-- detach can no longer let a writer skip the artist lock and the count.

CREATE OR REPLACE FUNCTION check_artist_tags_max_per_artist() RETURNS trigger AS $$
DECLARE
    link_count int;
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.artist_id = OLD.artist_id THEN
        RETURN NEW;
    END IF;

    PERFORM 1 FROM artist_tags
    WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
    FOR KEY SHARE;
    IF FOUND THEN
        RETURN NEW;
    END IF;

    PERFORM 1 FROM artists WHERE id = NEW.artist_id FOR NO KEY UPDATE;

    PERFORM 1 FROM artist_tags
    WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
    FOR KEY SHARE;
    IF FOUND THEN
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
