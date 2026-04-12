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
