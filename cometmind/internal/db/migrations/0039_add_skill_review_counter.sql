-- Per-session mutating-call counter for the skill review fork.

ALTER TABLE sessions ADD COLUMN skill_review_mutating_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE sessions ADD COLUMN skill_review_count_reset_at INTEGER NOT NULL DEFAULT 0;
