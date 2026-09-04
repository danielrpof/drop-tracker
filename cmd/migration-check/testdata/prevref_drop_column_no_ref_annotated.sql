-- migration-check:allow-destructive expand-shipped-in=v1.7.0 reason=events.some_column_no_query_touches never read by any query
ALTER TABLE events DROP COLUMN some_column_no_query_touches;
