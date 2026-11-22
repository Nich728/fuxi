-- What each user knows about individual words (SYSTEM_DESIGN §4.6, §9.7).
-- Owned by internal/learner. M3 writes only manual rows ("I already know
-- this"); the vocabulary check (M4) adds placement rows and the rest of §4.6.

-- +goose Up
CREATE TABLE word_knowledge (
    id          uuid PRIMARY KEY,            -- UUIDv5(user_id, entry_id): identical on every device
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_id    bigint NOT NULL REFERENCES dict_entries(id),
    status      text NOT NULL CHECK (status IN ('known', 'learning', 'unknown')),
    source      text NOT NULL CHECK (source IN ('placement', 'manual', 'review')),
    confidence  real NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    field_hlc   jsonb NOT NULL,
    updated_at  timestamptz NOT NULL,
    UNIQUE (user_id, entry_id)
);

-- +goose Down
DROP TABLE word_knowledge;
