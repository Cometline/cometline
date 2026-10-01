-- Rolling context compaction summary state on sessions.

ALTER TABLE sessions ADD COLUMN context_summary TEXT NOT NULL DEFAULT '';

ALTER TABLE sessions ADD COLUMN compacted_until_message_id TEXT;

ALTER TABLE sessions ADD COLUMN context_summary_updated_at TEXT;
