-- ALTER TABLE events DROP COLUMN release_type;

/*
ALTER TABLE events DROP COLUMN release_type;
*/

ALTER TABLE events ADD COLUMN note text DEFAULT 'ALTER TABLE events DROP COLUMN release_type;';
