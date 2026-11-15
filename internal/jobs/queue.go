// Package jobs is a small job queue on Postgres (SYSTEM_DESIGN §11, ADR-0002).
//
// Jobs are enqueued in the caller's transaction, claimed with
// FOR UPDATE SKIP LOCKED, retried with exponential backoff and jitter, and
// marked dead after max_attempts. Handlers must be idempotent: a job whose
// worker crashed is run again.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Nich728/fuxi/internal/jobs/jobsdb"
	"github.com/Nich728/fuxi/internal/platform/clock"
)

// ErrNotFound is returned by Get for a job that doesn't exist or isn't the user's.
var ErrNotFound = errors.New("job not found")

// DBTX is a pool or a transaction.
type DBTX = jobsdb.DBTX

// Lock is how long a claimed job is reserved. A handler must finish within
// it; otherwise the reaper hands the job to another worker.
const Lock = 5 * time.Minute

// New describes a job to enqueue.
type New struct {
	Kind        string
	UserID      uuid.UUID // zero for system jobs
	Payload     any
	DedupeKey   string // optional; a second enqueue with the same key returns the first job
	MaxAttempts int    // 0 means 5
}

// Enqueue adds a job using db, which may be the caller's transaction so the
// job commits or rolls back with the data it refers to.
func Enqueue(ctx context.Context, db DBTX, clk clock.Clock, j New) (int64, error) {
	payload, err := json.Marshal(j.Payload)
	if err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", j.Kind, err)
	}
	if j.MaxAttempts == 0 {
		j.MaxAttempts = 5
	}
	p := jobsdb.EnqueueJobParams{
		Kind: j.Kind, Payload: payload, RunAt: clk.Now(), Now: clk.Now(),
		MaxAttempts: int32(j.MaxAttempts),
	}
	if j.UserID != uuid.Nil {
		p.UserID = uuid.NullUUID{UUID: j.UserID, Valid: true}
	}
	if j.DedupeKey != "" {
		p.DedupeKey = &j.DedupeKey
	}
	q := jobsdb.New(db)
	id, err := q.EnqueueJob(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) && p.DedupeKey != nil {
		id, err = q.JobIDByDedupeKey(ctx, p.DedupeKey)
	}
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", j.Kind, err)
	}
	return id, nil
}

// Job is a claimed job, as a handler sees it.
type Job struct {
	ID       int64
	Kind     string
	UserID   uuid.UUID
	Payload  json.RawMessage
	Attempts int
}

// Handler runs one job and returns its result (stored as JSON), or an error.
// Wrap an error with Permanent when retrying can't help.
type Handler func(ctx context.Context, j Job) (any, error)

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks err as not worth retrying: the job fails at once.
func Permanent(err error) error { return permanentError{err} }

// Worker claims and runs jobs.
type Worker struct {
	db       DBTX
	q        *jobsdb.Queries
	clock    clock.Clock
	log      *slog.Logger
	handlers map[string]Handler
	// Poll is the pause between polls when the queue is empty.
	Poll time.Duration
	// rand drives backoff jitter; replaceable in tests.
	rand func() float64
}

// NewWorker returns a worker on db with the given handlers, keyed by job kind.
func NewWorker(db DBTX, clk clock.Clock, log *slog.Logger, handlers map[string]Handler) *Worker {
	return &Worker{
		db: db, q: jobsdb.New(db), clock: clk, log: log, handlers: handlers,
		Poll: 2 * time.Second, rand: rand.Float64,
	}
}

// Run processes jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	reap := time.NewTicker(time.Minute)
	defer reap.Stop()
	if _, err := w.Reap(ctx); err != nil {
		w.log.Warn("reap jobs", "err", err)
	}
	for {
		ran, err := w.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			w.log.Error("run job", "err", err)
		}
		if ran && err == nil {
			continue // drain the queue before sleeping
		}
		select {
		case <-ctx.Done():
			return nil
		case <-reap.C:
			if _, err := w.Reap(ctx); err != nil {
				w.log.Warn("reap jobs", "err", err)
			}
		case <-time.After(w.Poll):
		}
	}
}

// Reap requeues jobs whose worker stopped mid-run.
func (w *Worker) Reap(ctx context.Context) (int64, error) {
	n, err := w.q.ReapJobs(ctx, w.clock.Now())
	if err != nil {
		return 0, fmt.Errorf("reap: %w", err)
	}
	if n > 0 {
		w.log.Warn("requeued jobs with expired locks", "count", n)
	}
	return n, nil
}

// RunOnce claims one ready job and runs it. It reports whether a job was found.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	now := w.clock.Now()
	row, err := w.q.ClaimJob(ctx, jobsdb.ClaimJobParams{Now: now, LockedUntil: now.Add(Lock)})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim: %w", err)
	}
	log := w.log.With("job_id", row.ID, "kind", row.Kind, "attempt", row.Attempts)

	if row.Attempts > row.MaxAttempts {
		// Reaped too often: its handler keeps outliving the lock.
		return true, w.fail(ctx, row.ID, "dead", "attempts exhausted")
	}
	h, ok := w.handlers[row.Kind]
	if !ok {
		log.Error("no handler for job kind")
		return true, w.fail(ctx, row.ID, "failed", "no handler for kind "+row.Kind)
	}

	runCtx, cancel := context.WithTimeout(ctx, Lock-30*time.Second)
	result, runErr := safeRun(runCtx, h, Job{
		ID: row.ID, Kind: row.Kind, UserID: row.UserID.UUID, Payload: row.Payload, Attempts: int(row.Attempts),
	})
	cancel()
	done := w.clock.Now()

	if runErr == nil {
		b, err := json.Marshal(result)
		if err != nil {
			return true, w.fail(ctx, row.ID, "failed", "encode result: "+err.Error())
		}
		log.Info("job succeeded")
		return true, w.q.CompleteJob(ctx, jobsdb.CompleteJobParams{ID: row.ID, Result: b, Now: done})
	}

	msg := runErr.Error()
	var perm permanentError
	switch {
	case errors.As(runErr, &perm):
		log.Warn("job failed", "err", runErr)
		return true, w.fail(ctx, row.ID, "failed", msg)
	case row.Attempts >= row.MaxAttempts:
		log.Error("job dead after last attempt", "err", runErr)
		return true, w.fail(ctx, row.ID, "dead", msg)
	default:
		delay := Backoff(int(row.Attempts), w.rand())
		log.Warn("job will retry", "err", runErr, "in", delay)
		return true, w.q.RetryJob(ctx, jobsdb.RetryJobParams{ID: row.ID, RunAt: done.Add(delay), LastError: &msg})
	}
}

func (w *Worker) fail(ctx context.Context, id int64, status, msg string) error {
	return w.q.FailJob(ctx, jobsdb.FailJobParams{ID: id, Status: status, LastError: &msg, Now: w.clock.Now()})
}

// safeRun turns a handler panic into an error, so one bad job can't stop the worker.
func safeRun(ctx context.Context, h Handler, j Job) (result any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("handler panicked: %v", p)
		}
	}()
	return h(ctx, j)
}

// Backoff is the delay before retry number attempts+1: 10 s doubling to at
// most an hour, with ±20% jitter so failed jobs don't retry in lockstep.
// r is a random number in [0, 1).
func Backoff(attempts int, r float64) time.Duration {
	d := 10 * time.Second
	for i := 1; i < attempts && d < time.Hour; i++ {
		d *= 2
	}
	d = min(d, time.Hour)
	return time.Duration(float64(d) * (0.8 + 0.4*r))
}

// Status is a job as its owner sees it (GET /jobs/{id}).
type Status struct {
	ID         int64           `json:"id"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"` // queued | running | succeeded | failed | dead
	Attempts   int             `json:"attempts"`
	Error      *string         `json:"error,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

// Get returns the user's job.
func Get(ctx context.Context, db DBTX, userID uuid.UUID, id int64) (Status, error) {
	row, err := jobsdb.New(db).GetJobForUser(ctx, jobsdb.GetJobForUserParams{
		ID: id, UserID: uuid.NullUUID{UUID: userID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, fmt.Errorf("load job: %w", err)
	}
	return Status{
		ID: row.ID, Kind: row.Kind, Status: row.Status, Attempts: int(row.Attempts),
		Error: row.LastError, Result: row.Result, CreatedAt: row.CreatedAt, FinishedAt: row.FinishedAt,
	}, nil
}
