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
	// v11 -> v12: global jobs queue
	{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			description TEXT NOT NULL,
			definition_of_done TEXT NOT NULL DEFAULT '',
			progress TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'ongoing', 'done')),
			priority INTEGER NOT NULL DEFAULT 0,
			scheduled_at INTEGER,
			due_at INTEGER,
			workspace_path TEXT,
			assigned_session_id TEXT,
			lease_expires_at INTEGER,
			created_by TEXT NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'agent')),
			source_session_id TEXT,
			source_platform TEXT NOT NULL DEFAULT '' CHECK (source_platform IN ('', 'desktop', 'discord')),
			source_channel_id TEXT,
			deleted_at INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_status_priority ON jobs (status, priority DESC, updated_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_assigned_session ON jobs (assigned_session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_deleted_at ON jobs (deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_at ON jobs (scheduled_at)`,
		`CREATE TABLE IF NOT EXISTS job_events (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
			action TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			actor_session_id TEXT,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_job_events_job ON job_events (job_id, created_at)`,
	},
	// v12 -> v13: drop unused job scheduling/priority columns
	{
		`DROP INDEX IF EXISTS idx_jobs_status_priority`,
		`DROP INDEX IF EXISTS idx_jobs_scheduled_at`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_status_updated ON jobs (status, updated_at ASC)`,
		`ALTER TABLE jobs DROP COLUMN priority`,
		`ALTER TABLE jobs DROP COLUMN scheduled_at`,
		`ALTER TABLE jobs DROP COLUMN due_at`,
	},
	// v13 -> v14: tag user-created vs autonomous operator sessions.
	{
		"ALTER TABLE sessions ADD COLUMN origin TEXT NOT NULL DEFAULT 'user' CHECK (origin IN ('user', 'autonomy'))",
		"CREATE INDEX IF NOT EXISTS idx_sessions_origin ON sessions (origin)",
	},
	// v14 -> v15: archive completed jobs separately from deletion.
	{
		"ALTER TABLE jobs ADD COLUMN archived_at INTEGER",
		"CREATE INDEX IF NOT EXISTS idx_jobs_archived_at ON jobs (archived_at)",
	},
	// v15 -> v16: retry failed job runs and block repeated failures.
	{
		"PRAGMA foreign_keys = OFF",
		`CREATE TABLE IF NOT EXISTS jobs_new (
			id TEXT PRIMARY KEY,
			description TEXT NOT NULL,
			definition_of_done TEXT NOT NULL DEFAULT '',
			progress TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'ongoing', 'done', 'blocked')),
			workspace_path TEXT,
			assigned_session_id TEXT,
			lease_expires_at INTEGER,
			created_by TEXT NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'agent')),
			source_session_id TEXT,
			source_platform TEXT NOT NULL DEFAULT '' CHECK (source_platform IN ('', 'desktop', 'discord')),
			source_channel_id TEXT,
			archived_at INTEGER,
			failure_count INTEGER NOT NULL DEFAULT 0,
			next_retry_at INTEGER,
			last_failure_reason TEXT,
			deleted_at INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		`INSERT INTO jobs_new (
			id, description, definition_of_done, progress, status, workspace_path,
			assigned_session_id, lease_expires_at, created_by, source_session_id,
			source_platform, source_channel_id, archived_at, failure_count,
			next_retry_at, last_failure_reason, deleted_at, created_at, updated_at
		)
		SELECT
			id, description, definition_of_done, progress, status, workspace_path,
			assigned_session_id, lease_expires_at, created_by, source_session_id,
			source_platform, source_channel_id, archived_at, 0,
			NULL, NULL, deleted_at, created_at, updated_at
		FROM jobs`,
		"DROP TABLE jobs",
		"ALTER TABLE jobs_new RENAME TO jobs",
		"CREATE INDEX IF NOT EXISTS idx_jobs_status_updated ON jobs (status, updated_at ASC)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_assigned_session ON jobs (assigned_session_id)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_deleted_at ON jobs (deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_archived_at ON jobs (archived_at)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_next_retry_at ON jobs (next_retry_at)",
		"PRAGMA foreign_keys = ON",
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
	if strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists") || strings.Contains(msg, "no such column") {
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

const schemaVersion = 16

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
