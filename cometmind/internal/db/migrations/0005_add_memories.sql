-- Global memory tables.

CREATE TABLE IF NOT EXISTS memories (
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
);

CREATE INDEX IF NOT EXISTS idx_memories_active ON memories (archived, scope);

CREATE INDEX IF NOT EXISTS idx_memories_weight ON memories (archived, base_weight);

CREATE TABLE IF NOT EXISTS memory_events (
    id TEXT PRIMARY KEY,
    memory_id TEXT,
    action TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);
