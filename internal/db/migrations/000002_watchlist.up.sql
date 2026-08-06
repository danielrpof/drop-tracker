-- Phase 2 (watchlist-core) introduces the first two domain tables.
--
-- artists is master data (D-03): identity keyed on MusicBrainz's mbid
-- (D-01), independent of whether the artist is currently on anyone's
-- watchlist. Phase 4 (detection events) and Phase 6 (release history) will
-- both hold foreign keys into artists regardless of watchlist membership.
--
-- watchlist is the user's per-artist entry: a foreign key into artists plus
-- two independent preference axes (D-05) -- release_types (which release
-- categories this artist is watched for at all) and muted_event_types
-- (which detected-event categories are suppressed from alerting). Both
-- default to "everything on, nothing muted" (D-08) so a freshly added
-- artist is never silently under-alerted.
--
-- The literal value sets below (album/single/ep/deluxe for release_types;
-- new_release/guest_feature/deluxe_change for muted_event_types) are this
-- phase's working set (02-RESEARCH.md Open Question 1, Assumptions A2/A4).
-- Phase 4 owns the actual detection-event vocabulary and may file a
-- follow-up migration to rename these values once that phase locks its own
-- naming; this migration is not the source of truth for Phase 4 semantics.
CREATE TABLE artists (
    id             BIGSERIAL PRIMARY KEY,
    mbid           TEXT NOT NULL UNIQUE,
    deezer_id      TEXT,
    name           TEXT NOT NULL,
    disambiguation TEXT,
    image_url      TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE watchlist (
    id                BIGSERIAL PRIMARY KEY,
    artist_id         BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    release_types     TEXT[] NOT NULL DEFAULT ARRAY['album','single','ep','deluxe']::text[],
    muted_event_types TEXT[] NOT NULL DEFAULT '{}'::text[],
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT watchlist_artist_id_key UNIQUE (artist_id),
    CONSTRAINT watchlist_release_types_valid
        CHECK (release_types <@ ARRAY['album','single','ep','deluxe']::text[]),
    CONSTRAINT watchlist_muted_event_types_valid
        CHECK (muted_event_types <@ ARRAY['new_release','guest_feature','deluxe_change']::text[])
);
