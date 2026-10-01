-- FTS5 index for hybrid memory retrieval.

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5 (
    memory_id UNINDEXED,
    content
);

INSERT INTO memories_fts (memory_id, content)
SELECT id, content FROM memories WHERE archived = 0;
