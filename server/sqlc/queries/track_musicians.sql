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

-- name: GetMusicianIDsByTrackID :many
SELECT musician_id FROM track_musicians WHERE track_id = ?;
