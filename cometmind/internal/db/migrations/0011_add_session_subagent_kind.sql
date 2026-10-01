-- Subagent kind for lifecycle and retention.

ALTER TABLE sessions ADD COLUMN subagent_kind TEXT NOT NULL DEFAULT '';

UPDATE sessions SET subagent_kind = 'acp' WHERE trim(acp_session_id) != '' AND parent_session_id IS NOT NULL;
