package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create migration tracking table: %w", err)
	}

	files, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	for _, file := range files {
		if file.IsDir() || file.Name()[0] == '.' {
			continue
		}
		if err := apply(ctx, pool, file.Name()); err != nil {
			return err
		}
	}
	return nil
}

func apply(ctx context.Context, pool *pgxpool.Pool, version string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(735324190)`); err != nil {
		return fmt.Errorf("lock migration %s: %w", version, err)
	}
	var applied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", version, err)
	}
	if applied {
		return nil
	}

	script, err := migrationFiles.ReadFile("sql/" + version)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, string(script)); err != nil {
		return fmt.Errorf("execute migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
