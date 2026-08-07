-- name: GetMusicMetadataMatch :one
SELECT
  *
FROM music_metadata_matches
WHERE entity_type = ?
  AND entity_id = ?
  AND provider = ?
LIMIT 1;

-- name: UpsertMusicMetadataMatch :exec
-- attempts counts consecutive failures for backoff: it grows while the status
-- stays 'failed' and resets on any other outcome.
INSERT INTO music_metadata_matches (
  entity_type,
  entity_id,
  provider,
  external_id,
  status,
  reason,
  score,
  provider_score,
  threshold_value,
  candidate_name,
  candidate_artist,
  search_query,
  strategy,
  error,
  next_retry_at,
  updated_at
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (entity_type, entity_id, provider) DO UPDATE
SET
  external_id = excluded.external_id,
  status = excluded.status,
  reason = excluded.reason,
  score = excluded.score,
  provider_score = excluded.provider_score,
  threshold_value = excluded.threshold_value,
  candidate_name = excluded.candidate_name,
  candidate_artist = excluded.candidate_artist,
  search_query = excluded.search_query,
  strategy = excluded.strategy,
  error = excluded.error,
  attempts = CASE
    WHEN excluded.status = 'failed' THEN music_metadata_matches.attempts + 1
    ELSE 1
  END,
  next_retry_at = excluded.next_retry_at,
  updated_at = CURRENT_TIMESTAMP;
