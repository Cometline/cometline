-- name: ListActiveModelCapabilityNegatives :many
SELECT feature
FROM model_capability_negatives
WHERE provider_id = ?
  AND endpoint = ?
  AND model_id = ?
  AND expires_at > ?;

-- name: UpsertModelCapabilityNegative :exec
INSERT INTO model_capability_negatives (provider_id, endpoint, model_id, feature, expires_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (provider_id, endpoint, model_id, feature)
DO UPDATE SET expires_at = excluded.expires_at;

-- name: DeleteExpiredModelCapabilityNegatives :exec
DELETE FROM model_capability_negatives
WHERE expires_at <= ?;
