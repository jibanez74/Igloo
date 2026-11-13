-- name: GetMusicProviderMatch :one
SELECT provider, status, reason
FROM music_provider_matches
WHERE entity_type = ? AND entity_id = ?
LIMIT 1;

-- name: UpsertMusicProviderMatch :exec
INSERT INTO music_provider_matches (entity_type, entity_id, provider, status, reason, candidate, score, attempts, failed_at, retry_after)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (entity_type, entity_id) DO UPDATE
SET provider = excluded.provider, status = excluded.status, reason = excluded.reason, candidate = excluded.candidate, score = excluded.score,
  attempts = excluded.attempts, failed_at = excluded.failed_at, retry_after = excluded.retry_after;

-- name: GetMusicProviderMatchFailure :one
-- The failure bookkeeping of an entity whose stored outcome is failed; no row
-- otherwise.
SELECT attempts, failed_at, retry_after
FROM music_provider_matches
WHERE entity_type = ? AND entity_id = ? AND status = 'failed';

-- name: DeleteMusicProviderMatch :exec
DELETE FROM music_provider_matches WHERE entity_type = ? AND entity_id = ?;
