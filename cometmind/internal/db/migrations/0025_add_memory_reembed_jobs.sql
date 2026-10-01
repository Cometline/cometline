-- Durable memory embedding migration jobs.

CREATE TABLE IF NOT EXISTS memory_reembed_jobs (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    from_model TEXT NOT NULL DEFAULT '',
    to_provider TEXT NOT NULL DEFAULT '',
    to_model TEXT NOT NULL DEFAULT '',
    to_base_url TEXT NOT NULL DEFAULT '',
    to_api_key TEXT NOT NULL DEFAULT '',
    total INTEGER NOT NULL DEFAULT 0,
    completed INTEGER NOT NULL DEFAULT 0,
    cursor_id TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
