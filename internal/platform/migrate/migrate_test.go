package migrate_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Nich728/fuxi/internal/platform/db"
	"github.com/Nich728/fuxi/internal/platform/migrate"
)

// TestUpDownUp runs every migration against a real Postgres 16, rolls each
// back, and applies them again, so a broken Down section fails CI.
func TestUpDownUp(t *testing.T) {
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
	defer pool.Close()

	var out bytes.Buffer
	if err := migrate.Up(ctx, pool, &out); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}
	assertExtension(t, pool, "pg_trgm", true)
	assertExtension(t, pool, "citext", true)

	// Second Up is a no-op.
	out.Reset()
	if err := migrate.Up(ctx, pool, &out); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if got := out.String(); got != "no pending migrations\n" {
		t.Fatalf("second up output = %q", got)
	}

	// Roll everything back, then forward again.
	for {
		var downOut bytes.Buffer
		if err := migrate.Down(ctx, pool, &downOut); err != nil {
			break // goose returns an error once nothing is left to roll back
		}
	}
	assertExtension(t, pool, "pg_trgm", false)
	if err := migrate.Up(ctx, pool, &out); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	assertExtension(t, pool, "pg_trgm", true)
}

func assertExtension(t *testing.T, pool *pgxpool.Pool, name string, want bool) {
	t.Helper()
	var got bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)`, name).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("extension %s installed = %v, want %v", name, got, want)
	}
}
