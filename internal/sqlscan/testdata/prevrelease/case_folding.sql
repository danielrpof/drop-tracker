-- Synthetic fixture for identifier case folding (D-15, Task 2): an
-- unquoted identifier folds to lower case (Postgres's own rule), so
-- "IMAGE_URL" and "Image_Url" both match a lookup for "image_url"; a
-- double-quoted identifier is byte-exact and must NOT fold.

-- name: SelectCaseFolding :many
SELECT a.image_url, a."Mixed"
FROM artists a
WHERE a.image_url IS NOT NULL;
