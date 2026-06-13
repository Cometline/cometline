package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed schema.sql
var schemaSQL string

// Migrate runs DDL from the embedded schema once per fresh database (see [EnsureSchema]).
func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}

	stmts := splitStatements(schemaSQL)
	for _, stmt := range stmts {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate exec: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

// alterStatements contains incremental ALTER TABLE statements for schema upgrades.
// Each entry is a single SQL statement that brings the schema from version N to N+1.
var alterStatements = [][]string{
	// v1 -> v2: add reasoning_content column to messages
	{
		"ALTER TABLE messages ADD COLUMN reasoning_content TEXT NOT NULL DEFAULT '[]'",
	},
}

func splitStatements(sql string) []string {
	var out []string
	rest := strings.TrimSpace(sql)
	for rest != "" {
		if idx := strings.Index(rest, ";"); idx >= 0 {
			stmt := strings.TrimSpace(rest[:idx])
			rest = strings.TrimSpace(rest[idx+1:])
			if stmt == "" {
				continue
			}
			// Skip standalone comments
			if strings.HasPrefix(stmt, "--") {
				continue
			}
			out = append(out, stmt+";")
			continue
		}
		break
	}
	return out
}

const schemaVersion = 2

// EnsureSchema runs [Migrate] once per database file using PRAGMA user_version.
// For existing databases, it applies incremental ALTER statements to upgrade
// the schema to the current version.
func EnsureSchema(ctx context.Context, conn *sql.DB) error {
	var v int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if v == 0 {
		// Fresh database: run full migration.
		if err := Migrate(ctx, conn); err != nil {
			return err
		}
		// Full schema migration already creates the latest shape, so incremental
		// ALTER steps should only run for non-fresh databases.
		v = schemaVersion
	}
	// Apply incremental upgrades.
	for i := v; i < schemaVersion && i < len(alterStatements)+1; i++ {
		stmts := alterStatements[i-1]
		for _, stmt := range stmts {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migrate v%d->v%d exec: %w\nstatement: %s", i, i+1, err, stmt)
			}
		}
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}
