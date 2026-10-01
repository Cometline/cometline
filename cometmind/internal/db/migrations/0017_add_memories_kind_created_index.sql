-- Recent memory lookups by kind.

CREATE TABLE IF NOT EXISTS memories (
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
);

CREATE INDEX IF NOT EXISTS idx_memories_kind_created ON memories (archived, kind, created_at DESC);
