package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Nich728/fuxi/internal/jobs"
	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/platform/testdb"
)

func TestBackoff(t *testing.T) {
	tests := []struct {
		attempts int
		r        float64
		want     time.Duration
	}{
		{1, 0.5, 10 * time.Second},
		{2, 0.5, 20 * time.Second},
		{3, 0.5, 40 * time.Second},
		{1, 0, 8 * time.Second},
		{1, 0.999999, 12 * time.Second},
		{30, 0.5, time.Hour},
	}
	for _, tt := range tests {
		got := jobs.Backoff(tt.attempts, tt.r)
		if d := got - tt.want; d < -time.Millisecond || d > time.Millisecond {
			t.Errorf("Backoff(%d, %v) = %v, want %v", tt.attempts, tt.r, got, tt.want)
		}
	}
}

func TestQueue(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Start(t, true)
	clk := clock.NewFake(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	userA, userB := uuid.New(), uuid.New()
	for _, u := range []uuid.UUID{userA, userB} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at, last_seen_at) VALUES ($1, $2, now(), now())`,
			u, u.String()+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}

	var flaky atomic.Int32
	w := jobs.NewWorker(pool, clk, log, map[string]jobs.Handler{
		"ok": func(_ context.Context, j jobs.Job) (any, error) {
			return map[string]string{"user": j.UserID.String()}, nil
		},
		"flaky": func(context.Context, jobs.Job) (any, error) {
			if flaky.Add(1) < 3 {
				return nil, errors.New("temporary")
			}
			return "done", nil
		},
		"bad":   func(context.Context, jobs.Job) (any, error) { return nil, jobs.Permanent(errors.New("bad input")) },
		"never": func(context.Context, jobs.Job) (any, error) { return nil, errors.New("always fails") },
		"panic": func(context.Context, jobs.Job) (any, error) { panic("boom") },
	})
	enqueue := func(kind string, user uuid.UUID, key string, maxAttempts int) int64 {
		t.Helper()
		id, err := jobs.Enqueue(ctx, pool, clk, jobs.New{Kind: kind, UserID: user, Payload: map[string]int{"n": 1}, DedupeKey: key, MaxAttempts: maxAttempts})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	status := func(user uuid.UUID, id int64) jobs.Status {
		t.Helper()
		s, err := jobs.Get(ctx, pool, user, id)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	drain := func() {
		t.Helper()
		for {
			ran, err := w.RunOnce(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !ran {
				return
			}
		}
	}

	t.Run("succeeds and stores the result", func(t *testing.T) {
		id := enqueue("ok", userA, "", 0)
		drain()
		s := status(userA, id)
		if s.Status != "succeeded" || s.Attempts != 1 || string(s.Result) != `{"user": "`+userA.String()+`"}` {
			t.Errorf("status = %+v result=%s", s, s.Result)
		}
	})

	t.Run("dedupe key returns the first job", func(t *testing.T) {
		a := enqueue("ok", userA, "same", 0)
		b := enqueue("ok", userA, "same", 0)
		if a != b {
			t.Errorf("ids %d and %d", a, b)
		}
		drain()
	})

	t.Run("retries with backoff until it works", func(t *testing.T) {
		id := enqueue("flaky", userA, "", 0)
		drain()
		if s := status(userA, id); s.Status != "queued" || s.Attempts != 1 || s.Error == nil {
			t.Fatalf("after first failure: %+v", s)
		}
		clk.Advance(time.Minute) // past the first backoff (≤ 12 s)
		drain()
		clk.Advance(time.Minute) // past the second (≤ 24 s)
		drain()
		if s := status(userA, id); s.Status != "succeeded" || s.Attempts != 3 {
			t.Errorf("final: %+v", s)
		}
	})

	t.Run("permanent error fails at once", func(t *testing.T) {
		id := enqueue("bad", userA, "", 0)
		drain()
		if s := status(userA, id); s.Status != "failed" || s.Attempts != 1 || *s.Error != "bad input" {
			t.Errorf("%+v", s)
		}
	})

	t.Run("dead after max attempts", func(t *testing.T) {
		id := enqueue("never", userA, "", 2)
		drain()
		clk.Advance(time.Hour)
		drain()
		if s := status(userA, id); s.Status != "dead" || s.Attempts != 2 {
			t.Errorf("%+v", s)
		}
	})

	t.Run("a panicking handler doesn't stop the worker", func(t *testing.T) {
		id := enqueue("panic", userA, "", 1)
		drain()
		if s := status(userA, id); s.Status != "dead" || s.Error == nil {
			t.Errorf("%+v", s)
		}
	})

	t.Run("unknown kind fails", func(t *testing.T) {
		id := enqueue("nobody-handles-this", userA, "", 0)
		drain()
		if s := status(userA, id); s.Status != "failed" {
			t.Errorf("%+v", s)
		}
	})

	t.Run("reaper requeues a job whose worker died", func(t *testing.T) {
		id := enqueue("ok", userA, "", 0)
		// Claim it the way a worker would, then "crash".
		if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'running', attempts = 1, locked_until = $2 WHERE id = $1`,
			id, clk.Now().Add(jobs.Lock)); err != nil {
			t.Fatal(err)
		}
		if n, _ := w.Reap(ctx); n != 0 {
			t.Fatalf("reaped %d before the lock expired", n)
		}
		clk.Advance(jobs.Lock + time.Second)
		if n, _ := w.Reap(ctx); n != 1 {
			t.Fatalf("reaped %d, want 1", n)
		}
		drain()
		if s := status(userA, id); s.Status != "succeeded" || s.Attempts != 2 {
			t.Errorf("%+v", s)
		}
	})

	t.Run("a user can't see another user's job", func(t *testing.T) {
		id := enqueue("ok", userA, "", 0)
		if _, err := jobs.Get(ctx, pool, userB, id); !errors.Is(err, jobs.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		drain()
	})

	t.Run("jobs scheduled later wait", func(t *testing.T) {
		id := enqueue("ok", userA, "", 0)
		if _, err := pool.Exec(ctx, `UPDATE jobs SET run_at = $2 WHERE id = $1`, id, clk.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if ran, _ := w.RunOnce(ctx); ran {
			t.Error("ran a job before its run_at")
		}
		clk.Advance(time.Hour)
		drain()
		if s := status(userA, id); s.Status != "succeeded" {
			t.Errorf("%+v", s)
		}
	})
}
