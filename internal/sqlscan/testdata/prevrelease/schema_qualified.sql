-- Synthetic fixture for schema-qualified table names (D-15 blind spot B6):
-- "public.events" must resolve to table "events", not a literal
-- "public.events" identifier.

-- name: SelectSchemaQualified :many
SELECT e.title
FROM public.events e
WHERE e.notified_at IS NULL;
