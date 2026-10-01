-- Tag user-created vs autonomous operator sessions.

ALTER TABLE sessions ADD COLUMN origin TEXT NOT NULL DEFAULT 'user' CHECK (origin IN ('user', 'autonomy'));

CREATE INDEX IF NOT EXISTS idx_sessions_origin ON sessions (origin);
