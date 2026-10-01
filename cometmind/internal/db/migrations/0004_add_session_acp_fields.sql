-- Subagent ACP session fields.

ALTER TABLE sessions ADD COLUMN acp_session_id TEXT NOT NULL DEFAULT '';

ALTER TABLE sessions ADD COLUMN pending_question TEXT NOT NULL DEFAULT '';
