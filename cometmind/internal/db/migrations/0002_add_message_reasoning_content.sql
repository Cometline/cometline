-- Add reasoning_content column to messages.

ALTER TABLE messages ADD COLUMN reasoning_content TEXT NOT NULL DEFAULT '[]';
