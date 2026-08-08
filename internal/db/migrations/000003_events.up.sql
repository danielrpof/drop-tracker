-- Phase 4 (detection-engine): the seen-store / event-log table (D-09). An
-- event row's existence IS "already seen" for its (event_type, source,
-- external_id) triple -- enforced by the unique constraint below, which
-- backs DTCT-04's idempotency via ON CONFLICT DO NOTHING (D-20).
--
-- source disambiguates the external_id namespace: MusicBrainz MBIDs
-- (UUID-format) vs. Deezer numeric album ids -- new_release events can
-- come from either poll cycle (D-15's per-source-independent seeding
-- implies this); deluxe_change and guest_feature are MusicBrainz-only
-- (D-03, D-08) but still carry source='musicbrainz' for schema
-- consistency and query simplicity.
--
-- release_group_mbid is populated for new_release rows (their own
-- external_id) and, in a later plan, for deluxe_change rows (pointing
-- back to the parent group) -- it is what makes "highest previously-seen
-- track count for this group" (D-02) queryable without a second table.
--
-- track_count is mutable baseline-tracking state (04-01 checkpoint,
-- option-a), NOT part of the D-12 display snapshot (title/artist_name/
-- release_date/cover_art_url), which stays write-once via
-- ON CONFLICT DO NOTHING per D-20. This plan creates the column; plan
-- 04-04 is the first to populate and compare it.
--
-- event_type reuses the exact three-value vocabulary already locked
-- Go-side (watchlist.EventTypes) and DB-side
-- (watchlist_muted_event_types_valid, 000002_watchlist.up.sql).
CREATE TABLE events (
    id                 BIGSERIAL PRIMARY KEY,
    artist_id          BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    source             TEXT NOT NULL,
    event_type         TEXT NOT NULL,
    external_id        TEXT NOT NULL,
    release_group_mbid TEXT,
    title              TEXT NOT NULL,
    artist_name        TEXT NOT NULL,
    release_date       TEXT,
    cover_art_url      TEXT,
    track_count        INT,
    notified_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT events_dedup_key UNIQUE (event_type, source, external_id),
    CONSTRAINT events_source_valid CHECK (source IN ('musicbrainz', 'deezer')),
    CONSTRAINT events_event_type_valid CHECK (event_type IN ('new_release', 'guest_feature', 'deluxe_change'))
);

-- Partial index: Phase 5's D-11 query is SELECT WHERE notified_at IS NULL.
CREATE INDEX events_unnotified_idx ON events (notified_at) WHERE notified_at IS NULL;

-- Speeds up D-14's per-source seed-mode check and D-10's per-source diff
-- lookup (ListExternalIDs).
CREATE INDEX events_artist_source_idx ON events (artist_id, source);

-- Speeds up D-02's per-group baseline lookup, once plan 04-04 adds it.
CREATE INDEX events_release_group_idx ON events (release_group_mbid) WHERE release_group_mbid IS NOT NULL;
