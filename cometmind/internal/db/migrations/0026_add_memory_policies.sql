-- Replace the overloaded memory pin with explicit application/retention
-- policies, durable task lineage, and summaries.

ALTER TABLE memories ADD COLUMN application_policy TEXT NOT NULL DEFAULT 'relevant' CHECK (application_policy IN ('always', 'relevant'));

ALTER TABLE memories ADD COLUMN retention_policy TEXT NOT NULL DEFAULT 'decaying' CHECK (retention_policy IN ('protected', 'decaying'));

ALTER TABLE memories ADD COLUMN origin_type TEXT NOT NULL DEFAULT '';

ALTER TABLE memories ADD COLUMN origin_id TEXT NOT NULL DEFAULT '';

ALTER TABLE memories ADD COLUMN summary_json TEXT NOT NULL DEFAULT '{}';

UPDATE memories
SET application_policy = CASE
        WHEN kind = 'preference' AND pinned = 1 THEN 'always'
        ELSE 'relevant'
    END,
    retention_policy = CASE
        WHEN pinned = 1 THEN 'protected'
        ELSE 'decaying'
    END;

UPDATE memories
SET origin_type = 'legacy', origin_id = 'legacy:' || id
WHERE kind IN ('task_outcome', 'task_summary')
  AND (origin_type = '' OR origin_id = '');

ALTER TABLE memories DROP COLUMN pinned;

CREATE INDEX IF NOT EXISTS idx_memories_retrieval_pool ON memories (
    archived, kind, application_policy, updated_at DESC
);

CREATE INDEX IF NOT EXISTS idx_memories_lineage ON memories (
    archived, origin_type, origin_id, kind, created_at DESC
);

CREATE INDEX IF NOT EXISTS idx_memories_lifecycle ON memories (
    archived, retention_policy, application_policy, kind, last_accessed_at, created_at
);
