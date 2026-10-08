-- Curator state for self-improvement skills, plus the hidden pass clock.

CREATE TABLE IF NOT EXISTS skill_curator_state (
    skill_name TEXT PRIMARY KEY,
    origin TEXT NOT NULL DEFAULT 'self-improvement',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'stale', 'archived')),
    pinned INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL DEFAULT 0,
    unused_since INTEGER NOT NULL DEFAULT 0,
    archived_at INTEGER NOT NULL DEFAULT 0,
    pinned_at INTEGER NOT NULL DEFAULT 0,
    delete_notified_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS skill_curator_pass (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    last_pass_at INTEGER NOT NULL DEFAULT 0,
    last_merge_at INTEGER NOT NULL DEFAULT 0,
    runs_idle_since INTEGER NOT NULL DEFAULT 0
);
