-- migration-check:allow-destructive expand-shipped-in=v1.8.0 reason=table is empty pre-launch, backfill not needed
ALTER TABLE events ADD COLUMN foo text NOT NULL;
