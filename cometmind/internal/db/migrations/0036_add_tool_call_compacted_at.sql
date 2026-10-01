-- Mark old tool outputs as pruned from the model prompt.

ALTER TABLE tool_calls ADD COLUMN compacted_at INTEGER;
