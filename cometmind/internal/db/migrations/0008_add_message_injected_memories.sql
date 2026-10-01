-- Persist memories injected into a turn so the memory card survives a
-- session reload (previously only emitted live over SSE).

ALTER TABLE messages ADD COLUMN injected_memories TEXT NOT NULL DEFAULT '[]';
