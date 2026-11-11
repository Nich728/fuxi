package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/db"
	"github.com/Nich728/fuxi/internal/platform/migrate"
)

func runMigrate(ctx context.Context, cfg config.Config, sub string) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch sub {
	case "up":
		return migrate.Up(ctx, pool, os.Stdout)
	case "down":
		return migrate.Down(ctx, pool, os.Stdout)
	case "status":
		return migrate.Status(ctx, pool, os.Stdout)
	default:
		return fmt.Errorf("unknown migrate command %q (want up, down or status)", sub)
	}
}
