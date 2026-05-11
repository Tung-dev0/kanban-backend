package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INT PRIMARY KEY,
    name       TEXT        NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

var migrationFilePattern = regexp.MustCompile(`^(\d+)_(.+)\.sql$`)

type migration struct {
	version int
	name    string
	path    string
}

// Migrate applies any *.sql files in dir whose numeric prefix is not already
// recorded in the schema_migrations table. Each file runs in a single transaction.
func Migrate(ctx context.Context, conn *sql.DB, dir string) error {
	if _, err := conn.ExecContext(ctx, migrationsTable); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	applied, err := loadApplied(ctx, conn)
	if err != nil {
		return err
	}

	pending, err := loadPending(dir, applied)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	for _, m := range pending {
		if err := applyOne(ctx, conn, m); err != nil {
			return fmt.Errorf("apply %d_%s: %w", m.version, m.name, err)
		}
		log.Printf("migration applied: %04d_%s", m.version, m.name)
	}
	return nil
}

func loadApplied(ctx context.Context, conn *sql.DB) (map[int]bool, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	defer rows.Close()

	out := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func loadPending(dir string, applied map[int]bool) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var all []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := migrationFilePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		all = append(all, migration{
			version: v,
			name:    m[2],
			path:    filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].version < all[j].version })

	var pending []migration
	for _, m := range all {
		if !applied[m.version] {
			pending = append(pending, m)
		}
	}
	return pending, nil
}

func applyOne(ctx context.Context, conn *sql.DB, m migration) error {
	body, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("exec sql: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		m.version, m.name); err != nil {
		return fmt.Errorf("record version: %w", err)
	}
	return tx.Commit()
}
