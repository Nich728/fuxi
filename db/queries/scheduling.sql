-- Queries for internal/scheduling (SYSTEM_DESIGN §6.5).

-- name: LatestParams :one
-- The user's newest tuned weights. No row means the FSRS defaults (version 0).
SELECT version, weights FROM fsrs_params
WHERE user_id = $1
ORDER BY version DESC
LIMIT 1;
