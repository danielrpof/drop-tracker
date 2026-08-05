-- Phase 1 creates no domain tables (artists/watchlist) — that schema belongs
-- to Phase 2 (D-01). This no-op migration exists only to prove the
-- go:embed -> iofs -> schema_migrations pipeline end-to-end: Go's embed
-- directive fails to compile against a pattern that matches zero files, so
-- an empty migrations directory is not an option.
SELECT 1;
