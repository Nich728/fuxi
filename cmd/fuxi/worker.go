package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/db"
)

// runWorker connects to the database and stays up until shutdown.
// The job queue and its handlers (SYSTEM_DESIGN §11) are added in later
// milestones; for now the worker only proves it can start, reach Postgres
// and shut down cleanly.
func runWorker(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Info("worker started")
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return nil
		case <-ticker.C:
			if err := pool.Ping(ctx); err != nil {
				log.Warn("database ping failed", "err", err)
			}
		}
	}
}
