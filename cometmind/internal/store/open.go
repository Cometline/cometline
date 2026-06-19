package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite" // SQLite driver (pure Go)
)

// OpenSQLite opens the global CometMind database and ensures schema v1 is applied.
func OpenSQLite(ctx context.Context, dbPath string) (*sql.DB, error) {
	// _txlock=immediate makes write transactions take the write lock up front
	// instead of lazily upgrading from a read lock. Without it, two connections
	// that both start as readers and then try to write can deadlock into an
	// immediate SQLITE_BUSY that busy_timeout cannot resolve.
	dsn := fmt.Sprintf(
		"file:%s?_txlock=immediate&_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)",
		filepath.ToSlash(dbPath),
	)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	// SQLite allows one writer; cap the pool so database/sql does not open competing connections.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("sql ping: %w", err)
	}
	if err := applySQLitePragmas(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := db.EnsureSchema(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func applySQLitePragmas(ctx context.Context, conn *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 10000",
		"PRAGMA journal_mode = WAL",
	}
	for _, stmt := range pragmas {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}
