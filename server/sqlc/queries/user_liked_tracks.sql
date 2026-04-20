-- name: LikeTrack :exec
INSERT INTO user_liked_tracks (
  user_id,
  track_id
)
VALUES
  (?, ?)
ON CONFLICT (user_id, track_id) DO NOTHING;

-- name: UnlikeTrack :exec
DELETE FROM user_liked_tracks
WHERE user_id = ?
  AND track_id = ?;

-- name: IsTrackLiked :one
SELECT
  COUNT(*) > 0 AS is_liked
FROM user_liked_tracks
WHERE user_id = ?
  AND track_id = ?;

-- name: GetLikedTrackIDsByUserID :many
SELECT
  track_id
FROM user_liked_tracks
WHERE user_id = ?;

-- name: GetLikedTracksForUser :many
SELECT
  t.id,
  t.title,
  t.duration,
  t.codec,
  t.bit_rate,
  t.file_path,
  a.id    AS album_id,
  a.title AS album_title,
  a.cover AS album_cover,
  m.id    AS musician_id,
  m.name  AS musician_name
FROM user_liked_tracks AS ult
INNER JOIN tracks AS t ON t.id = ult.track_id
LEFT JOIN albums AS a ON t.album_id = a.id
LEFT JOIN musicians AS m ON t.musician_id = m.id
WHERE ult.user_id = ?
ORDER BY ult.created_at DESC
LIMIT ?
OFFSET ?;

-- name: CountUserLikedTracks :one
SELECT
  COUNT(*)
FROM user_liked_tracks
WHERE user_id = ?;
