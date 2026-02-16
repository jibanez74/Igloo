-- name: LikeTrack :exec
INSERT INTO user_liked_tracks (user_id, track_id)
VALUES (?, ?)
ON CONFLICT (user_id, track_id) DO NOTHING;

-- name: UnlikeTrack :exec
DELETE FROM user_liked_tracks
WHERE user_id = ? AND track_id = ?;

-- name: IsTrackLiked :one
SELECT COUNT(*) > 0 as is_liked
FROM user_liked_tracks
WHERE user_id = ? AND track_id = ?;

-- name: GetLikedTracksByUserID :many
SELECT
  t.id,
  t.title,
  t.sort_title,
  t.duration,
  t.codec,
  t.bit_rate,
  t.file_path,
  t.track_index,
  t.disc,
  a.id as album_id,
  a.title as album_title,
  a.cover as album_cover,
  m.id as musician_id,
  m.name as musician_name,
  ult.created_at as liked_at
FROM user_liked_tracks ult
INNER JOIN tracks t ON ult.track_id = t.id
LEFT JOIN albums a ON t.album_id = a.id
LEFT JOIN musicians m ON t.musician_id = m.id
WHERE ult.user_id = ?
ORDER BY ult.created_at DESC;

-- name: GetLikedTrackIDsByUserID :many
SELECT track_id
FROM user_liked_tracks
WHERE user_id = ?;
