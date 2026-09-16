-- Phase 20 (D-05, locked): a hard singleton row for the instance-wide digest
-- notification setting. CHECK (id = 1) plus the seed INSERT below is the
-- whole enforcement mechanism -- no app-level ensure-a-row path, no upsert.
-- digest_last_sent_at is nullable with no default: Phase 22 owns writing it.
CREATE TABLE notification_settings (
    id                  int PRIMARY KEY CHECK (id = 1),
    digest_enabled      boolean NOT NULL DEFAULT false,
    digest_cadence      text NOT NULL DEFAULT 'daily'
        CHECK (digest_cadence IN ('daily', 'weekly')),
    digest_last_sent_at timestamptz,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

INSERT INTO notification_settings (id) VALUES (1);
