-- name: GetRemuxSafetyVerdict :one
-- Persisted remux-safety verdict for one video stream; the caller compares
-- the stored fingerprint and treats a mismatch as a miss.
SELECT
  movie_id,
  stream_index,
  fingerprint,
  safe,
  reason,
  created_at,
  updated_at
FROM remux_safety_verdicts
WHERE movie_id = ?
  AND stream_index = ?;

-- name: UpsertRemuxSafetyVerdict :exec
INSERT INTO remux_safety_verdicts (
  movie_id,
  stream_index,
  fingerprint,
  safe,
  reason
)
VALUES
  (?, ?, ?, ?, ?)
ON CONFLICT (movie_id, stream_index) DO UPDATE
SET
  fingerprint = excluded.fingerprint,
  safe = excluded.safe,
  reason = excluded.reason,
  updated_at = CURRENT_TIMESTAMP;
