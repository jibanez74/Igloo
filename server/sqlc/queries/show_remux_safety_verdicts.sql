-- name: GetShowRemuxSafetyVerdict :one
-- Persisted remux-safety verdict for one video stream of a show file; the
-- caller compares the stored fingerprint and treats a mismatch as a miss.
SELECT
  fingerprint,
  safe,
  reason
FROM show_remux_safety_verdicts
WHERE file_id = ?
  AND stream_index = ?;

-- name: UpsertShowRemuxSafetyVerdict :exec
INSERT INTO show_remux_safety_verdicts (
  file_id,
  stream_index,
  fingerprint,
  safe,
  reason
)
VALUES
  (?, ?, ?, ?, ?)
ON CONFLICT (file_id, stream_index) DO UPDATE
SET
  fingerprint = excluded.fingerprint,
  safe = excluded.safe,
  reason = excluded.reason;
