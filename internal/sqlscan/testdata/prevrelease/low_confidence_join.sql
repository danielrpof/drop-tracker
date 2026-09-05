-- Synthetic fixture (not derived from a real repo query): a bare
-- unqualified column referenced inside a two-table join. The extractor
-- cannot tell which table "status" belongs to, so it must land in the
-- low-confidence set only -- never the high-confidence set (RESEARCH D-15
-- Pitfall E).

-- name: ListJoinedRows :many
SELECT id
FROM widgets_a
JOIN widgets_b ON widgets_a.widget_id = widgets_b.id
WHERE status = 'ok';
