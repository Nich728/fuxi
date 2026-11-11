-- Extensions used across the schema (SYSTEM_DESIGN §4).
-- Tables arrive with the milestone that owns them; this migration only
-- proves the migration pipeline end to end.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;   -- case-insensitive email (users)
CREATE EXTENSION IF NOT EXISTS pg_trgm;  -- trigram pinyin search (dict_entries)

-- +goose Down
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS citext;
