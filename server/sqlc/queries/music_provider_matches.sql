-- name: GetMusicProviderMatch :one
SELECT provider, status, reason
FROM music_provider_matches
WHERE entity_type = ? AND entity_id = ?
LIMIT 1;

-- name: UpsertMusicProviderMatch :exec
INSERT INTO music_provider_matches (entity_type, entity_id, provider, status, reason, candidate, score, attempts, retry_after)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (entity_type, entity_id) DO UPDATE
SET provider = excluded.provider, status = excluded.status, reason = excluded.reason, candidate = excluded.candidate, score = excluded.score,
  attempts = excluded.attempts, retry_after = excluded.retry_after;

-- name: GetMusicProviderMatchAttempts :one
-- The failed lookups in a row an entity has recorded; 0 when it has no outcome.
SELECT CAST(COALESCE((SELECT attempts FROM music_provider_matches WHERE entity_type = ? AND entity_id = ? AND status = 'failed'), 0) AS INTEGER) AS attempts;

-- name: DeleteMusicProviderMatch :exec
DELETE FROM music_provider_matches WHERE entity_type = ? AND entity_id = ?;
