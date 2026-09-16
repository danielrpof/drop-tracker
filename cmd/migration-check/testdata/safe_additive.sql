ALTER TABLE events ADD COLUMN foo text;

CREATE INDEX events_foo_idx ON events (foo);

CREATE TABLE widgets (
    id    BIGSERIAL PRIMARY KEY,
    count INT NOT NULL DEFAULT 0 CHECK (count >= 0)
);
