-- Per-session agent mode (auto/plan) for composer mode switching.

ALTER TABLE sessions ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'auto' CHECK (agent_mode IN ('auto', 'plan'));
