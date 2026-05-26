-- name: UpsertMusicSpotifyMatch :exec
INSERT INTO music_spotify_matches (
  entity_type,
  entity_id,
  spotify_id,
  status,
  reason,
  score,
  threshold_value,
  candidate_name,
  candidate_artist,
  search_query,
  strategy,
  error
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (entity_type, entity_id) DO UPDATE
SET
  spotify_id = excluded.spotify_id,
  status = excluded.status,
  reason = excluded.reason,
  score = excluded.score,
  threshold_value = excluded.threshold_value,
  candidate_name = excluded.candidate_name,
  candidate_artist = excluded.candidate_artist,
  search_query = excluded.search_query,
  strategy = excluded.strategy,
  error = excluded.error,
  updated_at = CURRENT_TIMESTAMP;

-- name: GetMusicSpotifyMatch :one
SELECT
  *
FROM music_spotify_matches
WHERE entity_type = ?
  AND entity_id = ?
LIMIT 1;

-- name: DeleteMusicSpotifyMatch :exec
DELETE FROM music_spotify_matches
WHERE entity_type = ?
  AND entity_id = ?;
