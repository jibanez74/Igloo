-- name: CreateWatchRoom :one
INSERT INTO watch_rooms (
  owner_user_id,
  movie_id,
  playback_mode,
  audio_track,
  subtitle_track
)
VALUES
  (?, ?, ?, ?, ?)
RETURNING *;

-- name: AddWatchRoomMember :exec
INSERT INTO watch_room_members (
  room_id,
  user_id
)
VALUES
  (?, ?);

-- name: GetWatchRoomByID :one
SELECT
  *
FROM watch_rooms
WHERE id = ?
LIMIT 1;

-- name: GetWatchRoomsForUser :many
SELECT
  wr.*
FROM watch_rooms AS wr
INNER JOIN watch_room_members AS wrm
  ON wr.id = wrm.room_id
WHERE wrm.user_id = ?
ORDER BY wr.created_at DESC;

-- name: GetWatchRoomMembers :many
SELECT
  u.id,
  u.name,
  u.email,
  u.avatar
FROM watch_room_members AS wrm
INNER JOIN users AS u
  ON wrm.user_id = u.id
WHERE wrm.room_id = ?
ORDER BY wrm.created_at ASC;

-- name: GetWatchRoomMembersByRoomIDs :many
SELECT
  wrm.room_id,
  u.id,
  u.name,
  u.email,
  u.avatar
FROM watch_room_members AS wrm
INNER JOIN users AS u
  ON wrm.user_id = u.id
WHERE wrm.room_id IN (sqlc.slice(room_ids))
ORDER BY wrm.room_id ASC, wrm.created_at ASC;

-- name: IsWatchRoomOwner :one
SELECT
  1
FROM watch_rooms
WHERE id = ?
  AND owner_user_id = ?
LIMIT 1;

-- name: IsWatchRoomMember :one
SELECT
  1
FROM watch_room_members
WHERE room_id = ?
  AND user_id = ?
LIMIT 1;

-- name: DeleteWatchRoom :exec
DELETE FROM watch_rooms
WHERE id = ?;

-- name: CountUsersByIDs :one
SELECT
  COUNT(*)
FROM users
WHERE id IN (sqlc.slice(ids));
