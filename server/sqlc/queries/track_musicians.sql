-- name: CreateTrackMusician :exec
INSERT INTO track_musicians (
  track_id,
  musician_id
)
VALUES
  (?, ?)
ON CONFLICT (track_id, musician_id) DO NOTHING;

-- name: DeleteTrackMusicians :exec
DELETE FROM track_musicians
WHERE track_id = ?;

-- name: DeleteTrackMusiciansExcept :exec
DELETE FROM track_musicians
WHERE track_id = sqlc.arg(track_id)
  AND musician_id NOT IN (sqlc.slice(musician_ids));
