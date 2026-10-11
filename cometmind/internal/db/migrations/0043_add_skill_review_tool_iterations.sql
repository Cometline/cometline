-- Skill review counts tool-calling iterations. Wiki review keeps the user-turn counter.

ALTER TABLE sessions ADD COLUMN skill_review_tool_iterations INTEGER NOT NULL DEFAULT 0;
