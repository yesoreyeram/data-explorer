// Package migrator applies embedded SQL migrations in order, tracking which
// ones have already run in a schema_migrations table. It is intentionally a
// small, dependency-free alternative to a full migration framework: each
// migration is a single forward-only .sql file, applied inside its own
// transaction, so `go build` produces a binary that can migrate itself with
// no external tooling required at deploy time.
package migrator

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version     TEXT PRIMARY KEY,
	applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// Apply runs every *.sql file in dir (an embedded FS) that has not yet been
// recorded in schema_migrations, in lexical filename order (hence the
// 0001_, 0002_... naming convention for migration files).
func Apply(ctx context.Context, pool *pgxpool.Pool, migrations embed.FS, dir string, log *slog.Logger) error {
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrations, dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, name := range files {
		if applied[name] {
			continue
		}

		content, err := fs.ReadFile(migrations, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}

		log.Info("applied migration", "version", name)
	}

	return nil
}
