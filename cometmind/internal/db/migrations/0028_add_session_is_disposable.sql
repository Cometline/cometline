-- Preserve sessions whose transcript was cleared or configured, while
-- allowing startup cleanup to remove never-used new chats. Existing sessions
-- are conservatively marked as non-disposable.

ALTER TABLE sessions ADD COLUMN is_disposable INTEGER NOT NULL DEFAULT 1;

UPDATE sessions SET is_disposable = 0;
