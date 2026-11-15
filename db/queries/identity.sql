-- Queries for internal/identity (SYSTEM_DESIGN §4.1). Every query on devices
-- is scoped to a user.

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: InsertUser :exec
-- First sign-in for this email. A concurrent first request may win the race;
-- the caller reads the row back either way.
INSERT INTO users (id, email, role, created_at, last_seen_at)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (email) DO NOTHING;

-- name: TouchUser :exec
-- Updates last_seen_at at most once an hour, so normal requests don't write.
UPDATE users SET last_seen_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND last_seen_at < sqlc.arg(now)::timestamptz - interval '1 hour';

-- name: UpdateUserSettings :one
UPDATE users
SET settings      = settings || sqlc.arg(settings)::jsonb,
    timezone      = coalesce(sqlc.narg(timezone), timezone),
    day_starts_at = coalesce(sqlc.narg(day_starts_at), day_starts_at)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpsertDevice :one
-- Registers a device or renames it. A device id owned by another user
-- matches no row, so nothing is returned.
INSERT INTO devices (id, user_id, name, last_seen_at, created_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name, last_seen_at = EXCLUDED.last_seen_at
WHERE devices.user_id = EXCLUDED.user_id
RETURNING *;

-- name: ListDevices :many
SELECT * FROM devices WHERE user_id = $1 ORDER BY created_at, id;

-- name: DeviceExists :one
SELECT EXISTS (SELECT 1 FROM devices WHERE id = $1 AND user_id = $2);

-- name: DeleteDevice :execrows
DELETE FROM devices WHERE id = $1 AND user_id = $2;
