-- migration-check:allow-destructive expand-shipped-in=v1.7.0 reason=no destructive statements here, annotation is a no-op
ALTER TABLE events ADD COLUMN foo text;

CREATE INDEX events_foo_idx ON events (foo);
