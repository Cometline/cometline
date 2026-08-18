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
	// v16 -> v17: recent memory lookups by kind.
	{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			scope TEXT NOT NULL DEFAULT 'global',
			kind TEXT NOT NULL DEFAULT 'fact',
			preference_category TEXT NOT NULL DEFAULT '',
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
		"CREATE INDEX IF NOT EXISTS idx_memories_kind_created ON memories (archived, kind, created_at DESC)",
	},
	// v17 -> v18: scheduled one-shot job definitions.
	{
		`CREATE TABLE IF NOT EXISTS scheduled_jobs (
			id TEXT PRIMARY KEY,
			description TEXT NOT NULL,
			definition_of_done TEXT NOT NULL DEFAULT '',
			workspace_path TEXT,
			created_by TEXT NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'agent')),
			source_session_id TEXT,
			source_platform TEXT NOT NULL DEFAULT '' CHECK (source_platform IN ('', 'desktop', 'discord')),
			source_channel_id TEXT,
			cron_expr TEXT,
			run_at INTEGER,
			next_run_at INTEGER NOT NULL,
			last_run_at INTEGER,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_due ON scheduled_jobs (enabled, next_run_at)",
		"CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_updated ON scheduled_jobs (updated_at DESC)",
	},
	// v18 -> v19: per-session agent plans.
	{
		`CREATE TABLE IF NOT EXISTS session_plans (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
			step_index INTEGER NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'blocked')),
			blocker_reason TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			UNIQUE (session_id, step_index)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_session_plans_session ON session_plans (session_id, step_index)",
	},
	// v19 -> v20: link materialized jobs back to the scheduled job that
	// created them, so a schedule with an outstanding (todo/ongoing) job
	// isn't re-materialized into a duplicate job on the next due tick.
	{
		"ALTER TABLE jobs ADD COLUMN scheduled_job_id TEXT REFERENCES scheduled_jobs (id) ON DELETE SET NULL",
		`CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_job_open ON jobs (scheduled_job_id, status)
			WHERE scheduled_job_id IS NOT NULL`,
	},
	// v20 -> v21: let a session plan be dismissed from the UI once all steps
	// are complete without losing its history in session_plans.
	{
		"ALTER TABLE session_plans ADD COLUMN dismissed_at INTEGER",
	},
	// v21 -> v22: remove session planning storage.
	{
		"DROP INDEX IF EXISTS idx_session_plans_session",
		"DROP TABLE IF EXISTS session_plans",
	},
	// v22 -> v23: agent inbox + sessions.origin allows 'inbox'.
	{
		"PRAGMA foreign_keys = OFF",
		`CREATE TABLE IF NOT EXISTS sessions_new (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces (id),
			title TEXT NOT NULL DEFAULT '',
			model_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
			origin TEXT NOT NULL DEFAULT 'user' CHECK (origin IN ('user', 'autonomy', 'inbox')),
			token_usage TEXT NOT NULL DEFAULT '{}',
			parent_session_id TEXT REFERENCES sessions_new (id) ON DELETE SET NULL,
			purpose TEXT NOT NULL DEFAULT '',
			delegation_status TEXT NOT NULL DEFAULT '' CHECK (
				delegation_status IN (
					'',
					'pending',
					'running',
					'awaiting_user',
					'awaiting_permission',
					'completed',
					'failed',
					'cancelled'
				)
			),
			output_summary TEXT NOT NULL DEFAULT '',
			acp_session_id TEXT NOT NULL DEFAULT '',
			pending_question TEXT NOT NULL DEFAULT '',
			subagent_kind TEXT NOT NULL DEFAULT '' CHECK (subagent_kind IN ('', 'general', 'acp')),
			pinned INTEGER NOT NULL DEFAULT 0,
			context_summary TEXT NOT NULL DEFAULT '',
			compacted_until_message_id TEXT,
			context_summary_updated_at TEXT,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		`INSERT INTO sessions_new (
			id, workspace_id, title, model_id, provider_id, status, origin, token_usage,
			parent_session_id, purpose, delegation_status, output_summary, acp_session_id,
			pending_question, subagent_kind, pinned, context_summary,
			compacted_until_message_id, context_summary_updated_at, created_at, updated_at
		)
		SELECT
			id, workspace_id, title, model_id, provider_id, status, origin, token_usage,
			parent_session_id, purpose, delegation_status, output_summary, acp_session_id,
			pending_question, subagent_kind, pinned, context_summary,
			compacted_until_message_id, context_summary_updated_at, created_at, updated_at
		FROM sessions`,
		"DROP TABLE sessions",
		"ALTER TABLE sessions_new RENAME TO sessions",
		"CREATE INDEX IF NOT EXISTS idx_sessions_workspace ON sessions (workspace_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions (updated_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_origin ON sessions (origin)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions (parent_session_id)",
		`CREATE TABLE IF NOT EXISTS inbox_messages (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			workspace_id TEXT,
			job_id TEXT,
			session_id TEXT,
			status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'archived')),
			archive_reason TEXT CHECK (
				archive_reason IS NULL OR archive_reason IN ('replied', 'dismissed')
			),
			user_reply TEXT,
			processed_at INTEGER,
			process_error TEXT,
			process_attempts INTEGER NOT NULL DEFAULT 0,
			archived_at INTEGER,
			deleted_at INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_inbox_messages_status_created ON inbox_messages (status, created_at DESC)",
		`CREATE INDEX IF NOT EXISTS idx_inbox_messages_process ON inbox_messages (
			status, archive_reason, processed_at, process_attempts
		)`,
		"CREATE INDEX IF NOT EXISTS idx_inbox_messages_archived_at ON inbox_messages (archived_at)",
		"PRAGMA foreign_keys = ON",
	},
	// v23 -> v24: model capability cache and opaque assistant provider state
	{
		`CREATE TABLE IF NOT EXISTS model_capability_negatives (
			provider_id TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			model_id TEXT NOT NULL,
			feature TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			PRIMARY KEY (provider_id, endpoint, model_id, feature)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_model_capability_negatives_expiry ON model_capability_negatives (expires_at)",
		`CREATE TABLE IF NOT EXISTS assistant_provider_states (
			message_id TEXT NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
			provider_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			state TEXT NOT NULL,
			created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
			PRIMARY KEY (message_id, provider_id, model_id)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_assistant_provider_states_message ON assistant_provider_states (message_id)",
	},
	// v24 -> v25: durable memory embedding migration jobs
	{
		`CREATE TABLE IF NOT EXISTS memory_reembed_jobs (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			from_model TEXT NOT NULL DEFAULT '',
			to_provider TEXT NOT NULL DEFAULT '',
			to_model TEXT NOT NULL DEFAULT '',
			to_base_url TEXT NOT NULL DEFAULT '',
			to_api_key TEXT NOT NULL DEFAULT '',
			total INTEGER NOT NULL DEFAULT 0,
			completed INTEGER NOT NULL DEFAULT 0,
			cursor_id TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
	},
	// v25 -> v26: replace the overloaded memory pin with explicit
	// application/retention policies, durable task lineage, and summaries.
	{
		"ALTER TABLE memories ADD COLUMN application_policy TEXT NOT NULL DEFAULT 'relevant' CHECK (application_policy IN ('always', 'relevant'))",
		"ALTER TABLE memories ADD COLUMN retention_policy TEXT NOT NULL DEFAULT 'decaying' CHECK (retention_policy IN ('protected', 'decaying'))",
		"ALTER TABLE memories ADD COLUMN origin_type TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE memories ADD COLUMN origin_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE memories ADD COLUMN summary_json TEXT NOT NULL DEFAULT '{}'",
		`UPDATE memories
		 SET application_policy = CASE
			 WHEN kind = 'preference' AND pinned = 1 THEN 'always'
			 ELSE 'relevant'
		 END,
		 retention_policy = CASE
			 WHEN pinned = 1 THEN 'protected'
			 ELSE 'decaying'
		 END`,
		`UPDATE memories
		 SET origin_type = 'legacy', origin_id = 'legacy:' || id
		 WHERE kind IN ('task_outcome', 'task_summary')
		   AND (origin_type = '' OR origin_id = '')`,
		"ALTER TABLE memories DROP COLUMN pinned",
		`CREATE INDEX IF NOT EXISTS idx_memories_retrieval_pool ON memories (
			archived, kind, application_policy, updated_at DESC
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_lineage ON memories (
			archived, origin_type, origin_id, kind, created_at DESC
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_lifecycle ON memories (
			archived, retention_policy, application_policy, kind, last_accessed_at, created_at
		)`,
	},
	// v26 -> v27: per-session agent mode (auto/plan) for composer mode switching.
	{
		"ALTER TABLE sessions ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'auto' CHECK (agent_mode IN ('auto', 'plan'))",
	},
	// v27 -> v28: preserve sessions whose transcript was cleared or configured,
	// while allowing startup cleanup to remove never-used new chats. Existing
	// sessions are conservatively marked as non-disposable.
	{
		"ALTER TABLE sessions ADD COLUMN is_disposable INTEGER NOT NULL DEFAULT 1",
		"UPDATE sessions SET is_disposable = 0",
	},
	// v28 -> v29: persisted session media catalog for gallery, video, and tombstones.
	{
		`CREATE TABLE IF NOT EXISTS session_media (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
			workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
			kind TEXT NOT NULL CHECK (kind IN ('image', 'video')),
			media_type TEXT NOT NULL,
			alt TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			provider_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'generated' CHECK (
				source IN ('generated', 'presented', 'captured', 'imported', 'user')
			),
			source_media_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'deleted')),
			byte_size INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_session_media_gallery ON session_media (status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_workspace ON session_media (workspace_id, status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_session ON session_media (session_id, status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_kind ON session_media (kind, status, created_at DESC)",
	},
	// v29 -> v30: keep gallery media after the owning session is deleted.
	{
		"PRAGMA foreign_keys = OFF",
		"DROP TABLE IF EXISTS session_media_new",
		`CREATE TABLE session_media_new (
			id TEXT PRIMARY KEY,
			session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL,
			storage_session_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
			kind TEXT NOT NULL CHECK (kind IN ('image', 'video')),
			media_type TEXT NOT NULL,
			alt TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			provider_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'generated' CHECK (
				source IN (
					'generated',
					'presented',
					'captured',
					'imported',
					'user'
				)
			),
			source_media_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'deleted')),
			byte_size INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
		)`,
		`INSERT INTO session_media_new (
			id, session_id, storage_session_id, workspace_id, kind, media_type, alt,
			prompt, model, provider_id, source, source_media_id, status, byte_size,
			duration_ms, created_at
		)
		SELECT
			id, session_id, COALESCE(session_id, id), workspace_id, kind, media_type, alt,
			prompt, model, provider_id, source, source_media_id, status, byte_size,
			duration_ms, created_at
		FROM session_media`,
		"DROP TABLE session_media",
		"ALTER TABLE session_media_new RENAME TO session_media",
		"CREATE INDEX IF NOT EXISTS idx_session_media_gallery ON session_media (status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_workspace ON session_media (workspace_id, status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_session ON session_media (session_id, status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_kind ON session_media (kind, status, created_at DESC)",
		"PRAGMA foreign_keys = ON",
	},
	// v30 -> v31: keep gallery media after the owning workspace is deleted.
	{
		"PRAGMA foreign_keys = OFF",
		"DROP TABLE IF EXISTS session_media_new",
		`CREATE TABLE session_media_new (
			id TEXT PRIMARY KEY,
			session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL,
			storage_session_id TEXT NOT NULL,
			workspace_id TEXT REFERENCES workspaces (id) ON DELETE SET NULL,
			kind TEXT NOT NULL CHECK (kind IN ('image', 'video')),
			media_type TEXT NOT NULL,
			alt TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			provider_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'generated' CHECK (
				source IN (
					'generated',
					'presented',
					'captured',
					'imported',
					'user'
				)
			),
			source_media_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'deleted')),
			byte_size INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER,
			created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
		)`,
		`INSERT INTO session_media_new (
			id, session_id, storage_session_id, workspace_id, kind, media_type, alt,
			prompt, model, provider_id, source, source_media_id, status, byte_size,
			duration_ms, created_at
		)
		SELECT
			id, session_id, storage_session_id, workspace_id, kind, media_type, alt,
			prompt, model, provider_id, source, source_media_id, status, byte_size,
			duration_ms, created_at
		FROM session_media`,
		"DROP TABLE session_media",
		"ALTER TABLE session_media_new RENAME TO session_media",
		"CREATE INDEX IF NOT EXISTS idx_session_media_gallery ON session_media (status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_workspace ON session_media (workspace_id, status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_session ON session_media (session_id, status, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_session_media_kind ON session_media (kind, status, created_at DESC)",
		"PRAGMA foreign_keys = ON",
	},
	// v31 -> v32: usage ledger for the spend dashboard
	{
		`CREATE TABLE IF NOT EXISTS usage_events (
			id TEXT PRIMARY KEY,
			created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
			workspace_id TEXT REFERENCES workspaces (id) ON DELETE SET NULL,
			session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL,
			provider_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			call_kind TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read INTEGER NOT NULL DEFAULT 0,
			cache_write INTEGER NOT NULL DEFAULT 0
		)`,
		"CREATE INDEX IF NOT EXISTS idx_usage_events_created ON usage_events (created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_usage_events_model ON usage_events (provider_id, model_id)",
		"CREATE INDEX IF NOT EXISTS idx_usage_events_workspace ON usage_events (workspace_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_usage_events_kind ON usage_events (call_kind, created_at DESC)",
	},
	// v32 -> v33: session fallback for workspace usage filters
	{
		"CREATE INDEX IF NOT EXISTS idx_usage_events_session ON usage_events (session_id)",
	},
}

func isForeignKeysPragma(stmt string) bool {
	normalized := strings.ToLower(strings.TrimSpace(stmt))
	normalized = strings.TrimSuffix(normalized, ";")
	return strings.HasPrefix(normalized, "pragma foreign_keys")
}

func isTableRebuild(stmts []string) bool {
	for _, stmt := range stmts {
		if strings.Contains(strings.ToUpper(stmt), "DROP TABLE") {
			return true
		}
	}
	return false
}

func setUserVersion(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, version int) error {
	_, err := exec.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
}

func applyAlterVersion(ctx context.Context, conn *sql.DB, fromVersion int, stmts []string) error {
	next := fromVersion + 1
	// A crash after the v30 rebuild but before user_version is checkpointed
	// leaves a v30-shaped table at v29. Replaying the copy would clobber
	// storage_session_id with a now-NULL session_id.
	if fromVersion == 29 {
		hasStorage, err := columnExists(ctx, conn, "session_media", "storage_session_id")
		if err != nil {
			return fmt.Errorf("migrate v%d->v%d inspect: %w", fromVersion, next, err)
		}
		if hasStorage {
			return setUserVersion(ctx, conn, next)
		}
	}
	if isTableRebuild(stmts) {
		return applyRebuildVersion(ctx, conn, fromVersion, stmts)
	}
	for _, stmt := range stmts {
		if err := execAlter(ctx, conn, stmt); err != nil {
			return fmt.Errorf("migrate v%d->v%d exec: %w\nstatement: %s", fromVersion, next, err, stmt)
		}
	}
	if err := setUserVersion(ctx, conn, next); err != nil {
		return fmt.Errorf("migrate v%d->v%d set user_version: %w", fromVersion, next, err)
	}
	return nil
}

func applyRebuildVersion(ctx context.Context, conn *sql.DB, fromVersion int, stmts []string) error {
	next := fromVersion + 1
	var body []string
	flush := func(version int) error {
		if len(body) == 0 {
			if version == 0 {
				return nil
			}
			return setUserVersion(ctx, conn, version)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migrate v%d->v%d begin: %w", fromVersion, next, err)
		}
		defer func() { _ = tx.Rollback() }()
		for _, stmt := range body {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migrate v%d->v%d exec: %w\nstatement: %s", fromVersion, next, err, stmt)
			}
		}
		if version > 0 {
			if err := setUserVersion(ctx, tx, version); err != nil {
				return fmt.Errorf("migrate v%d->v%d set user_version: %w", fromVersion, next, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate v%d->v%d commit: %w", fromVersion, next, err)
		}
		body = nil
		return nil
	}
	for i, stmt := range stmts {
		if isForeignKeysPragma(stmt) {
			version := 0
			if len(body) > 0 && i == len(stmts)-1 {
				version = next
			}
			if err := flush(version); err != nil {
				return err
			}
			if err := execAlter(ctx, conn, stmt); err != nil {
				return fmt.Errorf("migrate v%d->v%d exec: %w\nstatement: %s", fromVersion, next, err, stmt)
			}
			continue
		}
		body = append(body, stmt)
	}
	return flush(next)
}

func columnExists(ctx context.Context, conn *sql.DB, table, column string) (bool, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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

const schemaVersion = 33

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
	// Apply incremental upgrades. Each version checkpoints user_version so a
	// crash cannot replay a completed rebuild. Table rebuilds run in one
	// transaction so DROP + RENAME cannot leave a half-migrated catalog.
	for i := v; i < schemaVersion && i < len(alterStatements)+1; i++ {
		if err := applyAlterVersion(ctx, conn, i, alterStatements[i-1]); err != nil {
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
