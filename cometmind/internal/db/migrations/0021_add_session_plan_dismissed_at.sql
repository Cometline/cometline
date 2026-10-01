-- Let a session plan be dismissed from the UI once all steps are complete
-- without losing its history in session_plans.

ALTER TABLE session_plans ADD COLUMN dismissed_at INTEGER;
