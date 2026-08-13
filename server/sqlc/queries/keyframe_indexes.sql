-- name: GetKeyframeIndex :one
-- Persisted keyframe index for one video stream; the caller compares the
-- stored fingerprint and treats a mismatch as a miss.
SELECT
  movie_id,
  stream_index,
  fingerprint,
  duration_sec,
  keyframes,
  created_at,
  updated_at
FROM keyframe_indexes
WHERE movie_id = ?
  AND stream_index = ?;

-- name: UpsertKeyframeIndex :exec
INSERT INTO keyframe_indexes (
  movie_id,
  stream_index,
  fingerprint,
  duration_sec,
  keyframes
)
VALUES
  (?, ?, ?, ?, ?)
ON CONFLICT (movie_id, stream_index) DO UPDATE
SET
  fingerprint = excluded.fingerprint,
  duration_sec = excluded.duration_sec,
  keyframes = excluded.keyframes,
  updated_at = CURRENT_TIMESTAMP;
