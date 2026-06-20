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
	// v2 -> v3: subagent session fields and gateway_sessions table
	{
		"ALTER TABLE sessions ADD COLUMN parent_session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL",
		"ALTER TABLE sessions ADD COLUMN purpose TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN delegation_status TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN output_summary TEXT NOT NULL DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions (parent_session_id)",
		`CREATE TABLE IF NOT EXISTS gateway_sessions (
			id TEXT PRIMARY KEY,
			platform TEXT NOT NULL,
			platform_user_id TEXT NOT NULL,
			platform_channel_id TEXT NOT NULL,
			thread_id TEXT NOT NULL DEFAULT '',
			cometmind_session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
			workspace_id TEXT NOT NULL REFERENCES workspaces (id),
			last_active_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			UNIQUE (platform, platform_user_id, platform_channel_id, thread_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_sessions_lookup ON gateway_sessions (
			platform, platform_user_id, platform_channel_id, thread_id
		)`,
	},
	// v3 -> v4: subagent ACP session fields
	{
		"ALTER TABLE sessions ADD COLUMN acp_session_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN pending_question TEXT NOT NULL DEFAULT ''",
	},
	// v4 -> v5: global memory tables
	{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			scope TEXT NOT NULL DEFAULT 'global',
			kind TEXT NOT NULL DEFAULT 'fact',
			content TEXT NOT NULL,
			embedding BLOB,
			embedding_model TEXT,
			source TEXT NOT NULL,
			base_weight REAL NOT NULL DEFAULT 1.0,
			access_count INTEGER NOT NULL DEFAULT 0,
			pinned INTEGER NOT NULL DEFAULT 0,
			source_session_id TEXT,
			superseded_by TEXT,
			archived INTEGER NOT NULL DEFAULT 0,
			archived_reason TEXT,
			last_accessed_at INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_active ON memories (archived, scope)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_weight ON memories (archived, base_weight)`,
		`CREATE TABLE IF NOT EXISTS memory_events (
			id TEXT PRIMARY KEY,
			memory_id TEXT,
			action TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
	},
	// v5 -> v6: FTS5 index for hybrid memory retrieval
	{
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5 (
			memory_id UNINDEXED,
			content
		)`,
		`INSERT INTO memories_fts (memory_id, content)
		 SELECT id, content FROM memories WHERE archived = 0`,
	},
	// v6 -> v7: categorize preference memories for lifecycle management
	{
		"ALTER TABLE memories ADD COLUMN preference_category TEXT NOT NULL DEFAULT ''",
		`CREATE INDEX IF NOT EXISTS idx_memories_preference_category ON memories (
			archived,
			kind,
			preference_category,
			updated_at DESC
		)`,
	},
	// v7 -> v8: persist memories injected into a turn so the memory card
	// survives a session reload (previously only emitted live over SSE).
	{
		"ALTER TABLE messages ADD COLUMN injected_memories TEXT NOT NULL DEFAULT '[]'",
	},
	// v8 -> v9: pin sessions to the top of the workspace sidebar group.
	{
		"ALTER TABLE sessions ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0",
	},
	// v9 -> v10: rolling context compaction summary state on sessions.
	{
		"ALTER TABLE sessions ADD COLUMN context_summary TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN compacted_until_message_id TEXT",
		"ALTER TABLE sessions ADD COLUMN context_summary_updated_at TEXT",
	},
	// v10 -> v11: subagent kind for lifecycle and retention.
	{
		"ALTER TABLE sessions ADD COLUMN subagent_kind TEXT NOT NULL DEFAULT ''",
		`UPDATE sessions SET subagent_kind = 'acp' WHERE trim(acp_session_id) != '' AND parent_session_id IS NOT NULL`,
	},
}

// execAlter runs one incremental DDL statement, tolerating idempotent failures
// such as adding a column that already exists on a partially-migrated database.
func execAlter(ctx context.Context, conn *sql.DB, stmt string) error {
	_, err := conn.ExecContext(ctx, stmt)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists") {
		return nil
	}
	return err
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

const schemaVersion = 11

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
			if err := execAlter(ctx, conn, stmt); err != nil {
				return fmt.Errorf("migrate v%d->v%d exec: %w\nstatement: %s", i, i+1, err, stmt)
			}
		}
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}
