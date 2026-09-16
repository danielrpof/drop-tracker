-- migration-check:allow-destructive expand-shipped-in=v1.7.0
ALTER TABLE events DROP COLUMN release_type;
