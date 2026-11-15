-- Background job queue on Postgres (SYSTEM_DESIGN §4.7, §11; ADR-0002).
-- Owned by internal/jobs.

-- +goose Up
CREATE TABLE jobs (
    id            bigserial PRIMARY KEY,
    kind          text NOT NULL,
    user_id       uuid REFERENCES users(id) ON DELETE CASCADE,   -- owner, for jobs a user started
    payload       jsonb NOT NULL,
    status        text NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'dead')),
    run_at        timestamptz NOT NULL DEFAULT now(),
    attempts      integer NOT NULL DEFAULT 0,
    max_attempts  integer NOT NULL DEFAULT 5,
    locked_until  timestamptz,
    last_error    text,
    result        jsonb,                                         -- handler output, e.g. rows imported
    dedupe_key    text UNIQUE,                                   -- optional: prevents duplicate enqueues
    created_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz
);
CREATE INDEX jobs_ready_idx ON jobs (run_at) WHERE status = 'queued';
CREATE INDEX jobs_running_idx ON jobs (locked_until) WHERE status = 'running';
CREATE INDEX jobs_user_idx ON jobs (user_id, id);

-- +goose Down
DROP TABLE jobs;
