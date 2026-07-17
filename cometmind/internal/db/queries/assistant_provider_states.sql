-- name: CreateAssistantProviderState :exec
INSERT INTO assistant_provider_states (message_id, provider_id, model_id, state)
VALUES (?, ?, ?, ?)
ON CONFLICT (message_id, provider_id, model_id)
DO UPDATE SET state = excluded.state;

-- name: ListAssistantProviderStatesBySession :many
SELECT assistant_provider_states.message_id,
       assistant_provider_states.provider_id,
       assistant_provider_states.model_id,
       assistant_provider_states.state
FROM assistant_provider_states
JOIN messages ON messages.id = assistant_provider_states.message_id
WHERE messages.session_id = ?;

-- name: DeleteAssistantProviderStatesBySession :exec
DELETE FROM assistant_provider_states
WHERE message_id IN (SELECT id FROM messages WHERE session_id = ?);
