-- Model capability cache and opaque assistant provider state.

CREATE TABLE IF NOT EXISTS model_capability_negatives (
    provider_id TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    model_id TEXT NOT NULL,
    feature TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    PRIMARY KEY (provider_id, endpoint, model_id, feature)
);

CREATE INDEX IF NOT EXISTS idx_model_capability_negatives_expiry ON model_capability_negatives (expires_at);

CREATE TABLE IF NOT EXISTS assistant_provider_states (
    message_id TEXT NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    PRIMARY KEY (message_id, provider_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_assistant_provider_states_message ON assistant_provider_states (message_id);
