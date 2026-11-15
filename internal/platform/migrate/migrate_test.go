package migrate_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/platform/migrate"
	"github.com/Nich728/fuxi/internal/platform/testdb"
)

// TestUpDownUp runs every migration against a real Postgres 16, rolls each
// back, and applies them again, so a broken Down section fails CI.
func TestUpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Start(t, false)

	var out bytes.Buffer
	if err := migrate.Up(ctx, pool, &out); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}
	assertExtension(t, pool, "pg_trgm", true)
	assertExtension(t, pool, "citext", true)
	for _, table := range m2Tables {
		assertTable(t, pool, table, true)
	}

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
	for _, table := range m2Tables {
		assertTable(t, pool, table, false)
	}
	if err := migrate.Up(ctx, pool, &out); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	assertExtension(t, pool, "pg_trgm", true)
	for _, table := range m2Tables {
		assertTable(t, pool, table, true)
	}
}

// m2Tables are tables from each migration so far, so every Down is exercised.
var m2Tables = []string{"dict_entries", "users", "devices", "decks", "notes", "cards", "review_logs", "card_states", "fsrs_params", "jobs"}

func assertTable(t *testing.T, pool *pgxpool.Pool, name string, want bool) {
	t.Helper()
	var got bool
	if err := pool.QueryRow(context.Background(),
		`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("table %s exists = %v, want %v", name, got, want)
	}
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
