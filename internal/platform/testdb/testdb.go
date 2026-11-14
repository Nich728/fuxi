// Package testdb starts a throwaway PostgreSQL 16 for integration tests and
// applies every migration. Tests using it are skipped with -short.
package testdb

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Nich728/fuxi/internal/platform/db"
	"github.com/Nich728/fuxi/internal/platform/migrate"
)

// Start returns a pool on a fresh, empty database. If migrated is true, all
// migrations are applied first. The container is removed when the test ends.
func Start(t testing.TB, migrated bool) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: needs Docker")
	}
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("fuxi"),
		postgres.WithUsername("fuxi"),
		postgres.WithPassword("fuxi"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if migrated {
		if err := migrate.Up(ctx, pool, io.Discard); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return pool
}
