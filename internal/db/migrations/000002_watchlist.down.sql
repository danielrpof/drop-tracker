-- Reverse of 000002_watchlist.up.sql: drop watchlist first (it holds the FK
-- into artists), then artists.
DROP TABLE watchlist;
DROP TABLE artists;
