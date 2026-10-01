-- Cross-process session run leases and abort requests.

CREATE TABLE IF NOT EXISTS session_runs (
    session_id TEXT PRIMARY KEY REFERENCES sessions (id) ON DELETE CASCADE,
    run_id TEXT NOT NULL UNIQUE,
    owner TEXT NOT NULL CHECK (owner IN ('http', 'gateway')),
    abort_requested INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_session_runs_updated ON session_runs (updated_at);
