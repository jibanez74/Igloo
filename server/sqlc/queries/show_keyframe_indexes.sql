-- name: GetShowKeyframeIndex :one
-- Persisted keyframe index for one video stream of a show file; the caller
-- compares the stored fingerprint and treats a mismatch as a miss.
SELECT
  fingerprint,
  duration_sec,
  keyframes
FROM show_keyframe_indexes
WHERE file_id = ?
  AND stream_index = ?;

-- name: UpsertShowKeyframeIndex :exec
INSERT INTO show_keyframe_indexes (
  file_id,
  stream_index,
  fingerprint,
  duration_sec,
  keyframes
)
VALUES
  (?, ?, ?, ?, ?)
ON CONFLICT (file_id, stream_index) DO UPDATE
SET
  fingerprint = excluded.fingerprint,
  duration_sec = excluded.duration_sec,
  keyframes = excluded.keyframes;
