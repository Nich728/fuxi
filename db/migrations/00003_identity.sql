-- Users and devices (SYSTEM_DESIGN §4.1). Owned by internal/identity.
--
-- Sign-in is handled by Cloudflare Access (ADR-0018): the API receives a
-- signed JWT carrying the user's email, so there are no passwords, refresh
-- tokens or invite codes here. A user row is created the first time a valid
-- token for a new email arrives; who may get a token is the Access policy.

-- +goose Up
CREATE TABLE users (
    id              uuid PRIMARY KEY,                -- server-generated UUIDv7
    email           citext UNIQUE NOT NULL,          -- from the Access token
    timezone        text NOT NULL DEFAULT 'Asia/Jakarta',
    day_starts_at   smallint NOT NULL DEFAULT 4 CHECK (day_starts_at BETWEEN 0 AND 23),
    sync_seq        bigint NOT NULL DEFAULT 0,       -- per-user change counter (§7, from M5)
    role            text NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'owner')),
    settings        jsonb NOT NULL DEFAULT '{"explanation_language": "en", "tone_colours": false}',
    created_at      timestamptz NOT NULL,
    last_seen_at    timestamptz NOT NULL
);

CREATE TABLE devices (
    id               uuid PRIMARY KEY,               -- client-generated UUIDv7, kept in the PWA's storage
    user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             text NOT NULL,
    last_pulled_seq  bigint NOT NULL DEFAULT 0,
    last_seen_at     timestamptz NOT NULL,
    created_at       timestamptz NOT NULL
);
CREATE INDEX devices_user_idx ON devices (user_id);

-- +goose Down
DROP TABLE devices;
DROP TABLE users;
