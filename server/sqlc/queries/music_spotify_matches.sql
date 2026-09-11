-- name: GetMusicSpotifyMatch :one
SELECT status, reason
FROM music_spotify_matches
WHERE entity_type = ? AND entity_id = ?
LIMIT 1;

-- name: UpsertMusicSpotifyMatch :exec
INSERT INTO music_spotify_matches (entity_type, entity_id, status, reason)
VALUES (?, ?, ?, ?)
ON CONFLICT (entity_type, entity_id) DO UPDATE
SET status = excluded.status, reason = excluded.reason;
