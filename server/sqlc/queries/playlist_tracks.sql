-- name: AddTrackToPlaylist :one
INSERT INTO playlist_tracks (
  playlist_id,
  track_id,
  position,
  added_by
)
VALUES
  (
    ?1,
    ?2,
    (
      SELECT COALESCE(MAX(position), -1) + 1
      FROM playlist_tracks AS pt2
      WHERE pt2.playlist_id = ?1
    ),
    ?3
  )
RETURNING *;

-- name: RemoveTrackFromPlaylist :exec
DELETE FROM playlist_tracks
WHERE playlist_id = ?
  AND track_id = ?;

-- name: GetPlaylistTracksInfinite :many
SELECT
  pt.id AS playlist_track_id,
  pt.position,
  pt.added_at,
  pt.added_by,
  t.id,
  t.title,
  t.duration,
  t.file_path,
  t.codec,
  t.bit_rate,
  t.album_id,
  t.musician_id,
  a.title AS album_title,
  a.cover AS album_cover,
  m.name AS musician_name
FROM playlist_tracks AS pt
INNER JOIN tracks AS t
  ON pt.track_id = t.id
LEFT JOIN albums AS a
  ON t.album_id = a.id
LEFT JOIN musicians AS m
  ON t.musician_id = m.id
WHERE pt.playlist_id = ?
ORDER BY pt.position ASC
LIMIT ?
OFFSET ?;

-- name: CountPlaylistTracks :one
SELECT
  COUNT(*) AS count
FROM playlist_tracks
WHERE playlist_id = ?;

-- name: GetPlaylistDuration :one
SELECT
  COALESCE(SUM(t.duration), 0) AS total_duration
FROM playlist_tracks AS pt
INNER JOIN tracks AS t
  ON pt.track_id = t.id
WHERE pt.playlist_id = ?;

-- name: UpdateTrackPosition :exec
UPDATE playlist_tracks
SET position = ?
WHERE playlist_id = ?
  AND track_id = ?;

-- name: IsTrackInPlaylist :one
SELECT
  EXISTS (
    SELECT 1
    FROM playlist_tracks
    WHERE playlist_id = ?
      AND track_id = ?
  ) AS is_in_playlist;
