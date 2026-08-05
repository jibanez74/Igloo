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
  u.avatar
FROM watch_room_members AS wrm
INNER JOIN users AS u
  ON wrm.user_id = u.id
WHERE wrm.room_id IN (sqlc.slice(room_ids))
ORDER BY wrm.room_id ASC, wrm.created_at ASC;

-- name: GetWatchRoomForMember :one
-- The room row, but only when the user is a member: the auth check every
-- room media request (manifest, each segment, direct stream, websocket)
-- performs. One PK seek plus one (room_id, user_id) unique-index seek;
-- sql.ErrNoRows covers both "no such room" and "not a member", which callers
-- deliberately do not distinguish.
SELECT wr.*
FROM watch_rooms AS wr
INNER JOIN watch_room_members AS wrm
  ON wrm.room_id = wr.id
WHERE wr.id = ?
  AND wrm.user_id = ?
LIMIT 1;

-- name: GetWatchRoomForMemberWithSummary :one
-- GetWatchRoomForMember plus the requesting member's presence summary
-- (name, avatar), for the websocket upgrade only: it needs both and runs
-- once per socket. HTTP media requests keep the leaner GetWatchRoomForMember.
SELECT
  wr.*,
  u.name AS member_name,
  u.avatar AS member_avatar
FROM watch_rooms AS wr
INNER JOIN watch_room_members AS wrm
  ON wrm.room_id = wr.id
INNER JOIN users AS u
  ON u.id = wrm.user_id
WHERE wr.id = ?
  AND wrm.user_id = ?
LIMIT 1;

-- name: IsWatchRoomMember :one
SELECT
  EXISTS (
    SELECT 1
    FROM watch_room_members
    WHERE room_id = ?
      AND user_id = ?
  ) AS is_member;

-- name: DeleteWatchRoom :exec
DELETE FROM watch_rooms
WHERE id = ?;

-- name: CountUsersByIDs :one
SELECT
  COUNT(*)
FROM users
WHERE id IN (sqlc.slice(ids));
