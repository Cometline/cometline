-- Categorize preference memories for lifecycle management.

ALTER TABLE memories ADD COLUMN preference_category TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_memories_preference_category ON memories (
    archived,
    kind,
    preference_category,
    updated_at DESC
);
