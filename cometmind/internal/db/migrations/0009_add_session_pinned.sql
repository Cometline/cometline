-- Pin sessions to the top of the workspace sidebar group.

ALTER TABLE sessions ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
