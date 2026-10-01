-- Drop unused child-session columns.

ALTER TABLE sessions DROP COLUMN acp_session_id;

ALTER TABLE sessions DROP COLUMN pending_question;
