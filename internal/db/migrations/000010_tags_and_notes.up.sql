-- Phase 24 (D-28, ADR 0004): free-form artist tags plus a per-watchlist-entry
-- note. tags/artist_tags key on artists.id so links survive a watchlist
-- remove/re-add (TAG-07); the cap trigger is the non-bypassable backstop
-- SC2 requires (docs/adr/0004-per-artist-tag-cap-trigger.md).

CREATE TABLE tags (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tags_name_length CHECK (char_length(name) BETWEEN 1 AND 32),
    CONSTRAINT tags_name_trimmed CHECK (name = btrim(name))
);

-- Case/accent-insensitive identity (TAG-03, D-29's ON CONFLICT target). Not
-- citext, not CONCURRENTLY -- golang-migrate wraps this file in one
-- transaction.
CREATE UNIQUE INDEX tags_name_lower_idx ON tags (lower(name));

CREATE TABLE artist_tags (
    artist_id BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    tag_id    BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (artist_id, tag_id)
);

CREATE INDEX artist_tags_tag_id_idx ON artist_tags (tag_id);

-- ADR 0004 / D-18: BEFORE INSERT row trigger enforcing the 10-tag-per-artist
-- cap even against a raw INSERT that bypasses the API. Skips an
-- already-existing link so ON CONFLICT DO NOTHING stays a no-op at the cap;
-- locks the artist row FOR NO KEY UPDATE (never FOR UPDATE, which would
-- conflict with watchlist/events' own FOR KEY SHARE FK locks on artists);
-- re-checks existence after the lock so two concurrent same-tag attaches at
-- 9 links both succeed (D-21 under concurrency, ADR 0004 Consequences).
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

CREATE TRIGGER artist_tags_cap_trigger
    BEFORE INSERT ON artist_tags
    FOR EACH ROW
    EXECUTE FUNCTION check_artist_tags_max_per_artist();

-- D-10/D-28: note lives on the watchlist row (removed with the entry, never
-- with the artist). Nullable, no default, both CHECKs inline in this same
-- ADD COLUMN clause -- the only shape cmd/migration-check does not flag.
ALTER TABLE watchlist ADD COLUMN note TEXT CONSTRAINT watchlist_note_length CHECK (char_length(note) <= 500) CONSTRAINT watchlist_note_not_blank CHECK (note IS NULL OR btrim(note) <> '');
