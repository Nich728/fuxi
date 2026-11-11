// Package migrate applies the embedded goose migrations.
package migrate

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Nich728/fuxi/db/migrations"
)

func provider(pool *pgxpool.Pool) (*goose.Provider, func() error, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("goose provider: %w", err)
	}
	return p, sqlDB.Close, nil
}

// Up applies every pending migration and reports what ran to out.
func Up(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	p, closeDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	results, err := p.Up(ctx)
	for _, r := range results {
		fmt.Fprintf(out, "applied %s (%s)\n", r.Source.Path, r.Duration)
	}
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "no pending migrations")
	}
	return nil
}

// Down rolls back the most recent migration.
func Down(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	p, closeDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	r, err := p.Down(ctx)
	if r != nil {
		fmt.Fprintf(out, "rolled back %s\n", r.Source.Path)
	}
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Status prints every migration and whether it has been applied.
func Status(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	p, closeDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	statuses, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	for _, s := range statuses {
		fmt.Fprintf(out, "%-8s %s\n", s.State, s.Source.Path)
	}
	return nil
}
