-- Session fallback for workspace usage filters.

CREATE INDEX IF NOT EXISTS idx_usage_events_session ON usage_events (session_id);
