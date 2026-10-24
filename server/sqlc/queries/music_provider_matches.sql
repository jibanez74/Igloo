-- name: GetMusicProviderMatch :one
SELECT provider, status, reason
FROM music_provider_matches
WHERE entity_type = ? AND entity_id = ?
LIMIT 1;

-- name: UpsertMusicProviderMatch :exec
INSERT INTO music_provider_matches (entity_type, entity_id, provider, status, reason, candidate, score)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (entity_type, entity_id) DO UPDATE
SET provider = excluded.provider, status = excluded.status, reason = excluded.reason, candidate = excluded.candidate, score = excluded.score;

-- name: DeleteMusicProviderMatch :exec
DELETE FROM music_provider_matches WHERE entity_type = ? AND entity_id = ?;
