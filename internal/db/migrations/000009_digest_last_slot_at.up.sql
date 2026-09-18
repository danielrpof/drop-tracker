-- Phase 22 (D-13): the slot record -- "the most recent scheduled fire this
-- singleton row has handled" -- kept distinct from digest_last_sent_at (last
-- successful send). Nullable, no default: never handled until the first
-- due-check writes it.
ALTER TABLE notification_settings ADD COLUMN digest_last_slot_at timestamptz;
