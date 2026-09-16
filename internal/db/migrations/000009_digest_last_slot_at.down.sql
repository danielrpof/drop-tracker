-- Paired down migration for 000009 (never run by the app; see
-- internal/db/migrations/README.md -- required to exist for the pair).
ALTER TABLE notification_settings DROP COLUMN digest_last_slot_at;
