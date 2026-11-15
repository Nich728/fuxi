package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/dictionary"
	"github.com/Nich728/fuxi/internal/identity"
	"github.com/Nich728/fuxi/internal/jobs"
	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/db"
	"github.com/Nich728/fuxi/internal/scheduling"
	"github.com/Nich728/fuxi/internal/study"
)

// runWorker runs the background job queue (SYSTEM_DESIGN §11) until shutdown.
func runWorker(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Info("worker started")
	err = newWorker(pool, clock.System{}, log).Run(ctx)
	log.Info("worker stopped")
	return err
}

// newWorker registers every job handler.
func newWorker(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) *jobs.Worker {
	dictSvc := dictionary.NewService(pool)
	studySvc := study.NewService(pool, clk, studyDictionary{dictSvc},
		identity.NewService(pool, clk, ""), scheduling.NewStore(pool))
	return jobs.NewWorker(pool, clk, log.With("module", "jobs"), map[string]jobs.Handler{
		study.JobImportCSV: studySvc.ImportCSVJob,
	})
}
