-- Queries for internal/jobs (SYSTEM_DESIGN §11). The clock is a parameter,
-- never now(), so tests control time.

-- name: EnqueueJob :one
INSERT INTO jobs (kind, user_id, payload, run_at, max_attempts, dedupe_key, created_at)
VALUES (sqlc.arg(kind), sqlc.narg(user_id), sqlc.arg(payload), sqlc.arg(run_at),
        sqlc.arg(max_attempts), sqlc.narg(dedupe_key), sqlc.arg(now))
ON CONFLICT (dedupe_key) DO NOTHING
RETURNING id;

-- name: JobIDByDedupeKey :one
SELECT id FROM jobs WHERE dedupe_key = $1;

-- name: ClaimJob :one
-- Takes the oldest ready job. SKIP LOCKED lets several workers poll at once.
UPDATE jobs SET status = 'running', attempts = attempts + 1,
                locked_until = sqlc.arg(locked_until)::timestamptz
WHERE id = (
    SELECT j.id FROM jobs j
    WHERE j.status = 'queued' AND j.run_at <= sqlc.arg(now)::timestamptz
    ORDER BY j.run_at, j.id
    FOR UPDATE SKIP LOCKED
    LIMIT 1)
RETURNING *;

-- name: CompleteJob :exec
UPDATE jobs SET status = 'succeeded', result = sqlc.arg(result), last_error = NULL,
                locked_until = NULL, finished_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: RetryJob :exec
UPDATE jobs SET status = 'queued', run_at = sqlc.arg(run_at), last_error = sqlc.arg(last_error),
                locked_until = NULL
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: FailJob :exec
-- status is 'failed' for a permanent error, 'dead' when attempts ran out.
UPDATE jobs SET status = sqlc.arg(status), last_error = sqlc.arg(last_error),
                locked_until = NULL, finished_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: ReapJobs :execrows
-- Requeues jobs whose worker died: still 'running' after the lock expired.
UPDATE jobs SET status = 'queued', locked_until = NULL,
                last_error = 'lock expired: worker stopped while running the job'
WHERE status = 'running' AND locked_until < sqlc.arg(now)::timestamptz;

-- name: GetJobForUser :one
SELECT * FROM jobs WHERE id = $1 AND user_id = $2;
