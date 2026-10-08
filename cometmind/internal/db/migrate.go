package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// The schema ships two ways that must stay equivalent (see
// TestMigrationsFromV1MatchFreshSchema):
//
//   - schema.sql builds a fresh database at schemaVersion in one pass.
//   - migrations/NNNN_description.sql upgrades an existing database from
//     user_version NNNN-1 to NNNN. Every file checkpoints user_version, so a
//     crash never replays a finished step.
//
// A file containing DROP TABLE is a table rebuild. Each run of statements
// between its PRAGMA foreign_keys lines commits in one transaction, and the
// last run sets user_version in that same transaction, so DROP + RENAME cannot
// leave a half-migrated catalog. The pragmas run outside any transaction
// because SQLite ignores foreign_keys changes inside one. Other files run
// statement by statement and tolerate errors from partially migrated
// databases (see [execAlter]).

// skipIfApplied is the only Go-side migration logic. Keyed by target version,
// each check reports that the step already ran, so it is checkpointed without
// replaying its SQL.
var skipIfApplied = map[int]func(context.Context, *sql.DB) (bool, error){
	// A crash after the 0030 rebuild but before user_version was checkpointed
	// left a v30-shaped table at v29. Replaying the copy would clobber
	// storage_session_id with a now-NULL session_id.
	30: func(ctx context.Context, conn *sql.DB) (bool, error) {
		return columnExists(ctx, conn, "session_media", "storage_session_id")
	},
	// 0038 rebuilds sessions. Upgrade tests seed partial sessions tables, so
	// missing columns are added before the copy. The SQL still runs.
	38: func(ctx context.Context, conn *sql.DB) (bool, error) {
		return false, ensureSessionRebuildColumns(ctx, conn)
	},
}

// sessionRebuildColumns are the pre-0038 sessions columns the rebuild copies.
// New skill-review columns are filled by the migration SQL itself.
var sessionRebuildColumns = []struct {
	name string
	ddl  string
}{
	{"workspace_id", "ALTER TABLE sessions ADD COLUMN workspace_id TEXT NOT NULL DEFAULT ''"},
	{"title", "ALTER TABLE sessions ADD COLUMN title TEXT NOT NULL DEFAULT ''"},
	{"model_id", "ALTER TABLE sessions ADD COLUMN model_id TEXT NOT NULL DEFAULT ''"},
	{"provider_id", "ALTER TABLE sessions ADD COLUMN provider_id TEXT NOT NULL DEFAULT ''"},
	{"status", "ALTER TABLE sessions ADD COLUMN status TEXT NOT NULL DEFAULT 'active'"},
	{"origin", "ALTER TABLE sessions ADD COLUMN origin TEXT NOT NULL DEFAULT 'user'"},
	{"is_disposable", "ALTER TABLE sessions ADD COLUMN is_disposable INTEGER NOT NULL DEFAULT 1"},
	{"token_usage", "ALTER TABLE sessions ADD COLUMN token_usage TEXT NOT NULL DEFAULT '{}'"},
	{"parent_session_id", "ALTER TABLE sessions ADD COLUMN parent_session_id TEXT"},
	{"purpose", "ALTER TABLE sessions ADD COLUMN purpose TEXT NOT NULL DEFAULT ''"},
	{"delegation_status", "ALTER TABLE sessions ADD COLUMN delegation_status TEXT NOT NULL DEFAULT ''"},
	{"output_summary", "ALTER TABLE sessions ADD COLUMN output_summary TEXT NOT NULL DEFAULT ''"},
	{"subagent_kind", "ALTER TABLE sessions ADD COLUMN subagent_kind TEXT NOT NULL DEFAULT ''"},
	{"agent_mode", "ALTER TABLE sessions ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'auto'"},
	{"pinned", "ALTER TABLE sessions ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0"},
	{"context_summary", "ALTER TABLE sessions ADD COLUMN context_summary TEXT NOT NULL DEFAULT ''"},
	{"compacted_until_message_id", "ALTER TABLE sessions ADD COLUMN compacted_until_message_id TEXT"},
	{"context_summary_updated_at", "ALTER TABLE sessions ADD COLUMN context_summary_updated_at TEXT"},
	{"created_at", "ALTER TABLE sessions ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0"},
	{"updated_at", "ALTER TABLE sessions ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0"},
}

func ensureSessionRebuildColumns(ctx context.Context, conn *sql.DB) error {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'sessions'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := conn.ExecContext(ctx, `CREATE TABLE sessions (id TEXT PRIMARY KEY)`); err != nil {
			return err
		}
	}
	for _, col := range sessionRebuildColumns {
		ok, err := columnExists(ctx, conn, "sessions", col.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := conn.ExecContext(ctx, col.ddl); err != nil {
			return err
		}
	}
	return nil
}

// Migrate runs DDL from the embedded schema once per fresh database (see [EnsureSchema]).
func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}
	for _, stmt := range splitStatements(schemaSQL) {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate exec: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

// EnsureSchema runs [Migrate] once per database file using PRAGMA user_version.
// For existing databases, it applies the embedded migrations newer than the
// stored version to upgrade the schema to schemaVersion.
func EnsureSchema(ctx context.Context, conn *sql.DB) error {
	var v int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if v == 0 {
		// Fresh database: schema.sql already has the latest shape.
		if err := Migrate(ctx, conn); err != nil {
			return err
		}
		v = schemaVersion
	}
	for _, m := range migrations {
		if m.version <= v {
			continue
		}
		if err := applyMigration(ctx, conn, m); err != nil {
			return err
		}
	}
	if err := setUserVersion(ctx, conn, schemaVersion); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}
	return nil
}

func applyMigration(ctx context.Context, conn *sql.DB, m migration) error {
	if applied := skipIfApplied[m.version]; applied != nil {
		ok, err := applied(ctx, conn)
		if err != nil {
			return m.wrap("inspect", err)
		}
		if ok {
			return setUserVersion(ctx, conn, m.version)
		}
	}
	if isTableRebuild(m.stmts) {
		return applyRebuild(ctx, conn, m)
	}
	for _, stmt := range m.stmts {
		if err := execAlter(ctx, conn, stmt); err != nil {
			return m.execError(err, stmt)
		}
	}
	if err := setUserVersion(ctx, conn, m.version); err != nil {
		return m.wrap("set user_version", err)
	}
	return nil
}

func applyRebuild(ctx context.Context, conn *sql.DB, m migration) error {
	var run []string
	for i, stmt := range m.stmts {
		if !isForeignKeysPragma(stmt) {
			run = append(run, stmt)
			continue
		}
		// Only a closing pragma follows the final run, so it checkpoints.
		if err := execInTx(ctx, conn, m, run, i == len(m.stmts)-1); err != nil {
			return err
		}
		run = nil
		if err := execAlter(ctx, conn, stmt); err != nil {
			return m.execError(err, stmt)
		}
	}
	if len(run) == 0 {
		if err := setUserVersion(ctx, conn, m.version); err != nil {
			return m.wrap("set user_version", err)
		}
		return nil
	}
	return execInTx(ctx, conn, m, run, true)
}

func execInTx(ctx context.Context, conn *sql.DB, m migration, stmts []string, checkpoint bool) error {
	if len(stmts) == 0 {
		return nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return m.wrap("begin", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return m.execError(err, stmt)
		}
	}
	if checkpoint {
		if err := setUserVersion(ctx, tx, m.version); err != nil {
			return m.wrap("set user_version", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return m.wrap("commit", err)
	}
	return nil
}

func (m migration) wrap(step string, err error) error {
	return fmt.Errorf("migrate v%d->v%d (%s) %s: %w", m.version-1, m.version, m.name, step, err)
}

func (m migration) execError(err error, stmt string) error {
	return fmt.Errorf("%w\nstatement: %s", m.wrap("exec", err), stmt)
}

// execAlter runs one incremental DDL statement, tolerating idempotent failures
// such as adding a column that already exists on a partially-migrated database.
func execAlter(ctx context.Context, conn *sql.DB, stmt string) error {
	_, err := conn.ExecContext(ctx, stmt)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	for _, benign := range []string{"duplicate column name", "already exists", "no such column", "no such table"} {
		if strings.Contains(msg, benign) {
			return nil
		}
	}
	return err
}

func setUserVersion(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, version int) error {
	_, err := exec.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
}

func columnExists(ctx context.Context, conn *sql.DB, table, column string) (bool, error) {
	var n int
	err := conn.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&n)
	return n > 0, err
}
