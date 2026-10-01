-- Remove session planning storage.

DROP INDEX IF EXISTS idx_session_plans_session;

DROP TABLE IF EXISTS session_plans;
