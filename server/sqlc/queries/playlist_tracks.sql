-- name: AddTrackToPlaylist :execrows
-- Zero rows affected means the track was already in the playlist; the unique
-- constraint replaces a separate membership pre-check.
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
ON CONFLICT (playlist_id, track_id) DO NOTHING;

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
-- A reorder that leaves tracks out can tie their positions with listed ones;
-- id keeps tied tracks in the order they were added, so pages stay stable.
ORDER BY pt.position ASC, pt.id ASC
LIMIT ?
OFFSET ?;

-- name: CountPlaylistTracks :one
SELECT
  COUNT(*) AS count
FROM playlist_tracks
WHERE playlist_id = ?;

-- name: GetPlaylistTrackSummary :one
-- Count, total duration and artwork for the playlist detail response.
-- artwork is built like GetPlaylistsWithCollaboratorAccess's: the album covers
-- of the first 24 tracks in playlist order, taken before the covers are looked
-- at, as a JSON array; the handler keeps the first four distinct ones for the
-- header's collage.
SELECT
  COUNT(*) AS track_count,
  COALESCE(SUM(t.duration), 0) AS total_duration,
  CAST((
    SELECT json_group_array(first_track.cover)
    FROM (
      SELECT a.cover
      FROM (
        SELECT
          head_pt.track_id,
          head_pt.position,
          head_pt.id
        FROM playlist_tracks AS head_pt
        WHERE head_pt.playlist_id = sqlc.arg(playlist_id)
        ORDER BY
          head_pt.position ASC,
          head_pt.id ASC
        LIMIT 24
      ) AS head
      INNER JOIN tracks AS head_t
        ON head_t.id = head.track_id
      INNER JOIN albums AS a
        ON a.id = head_t.album_id
      WHERE a.cover <> ''
      ORDER BY
        head.position ASC,
        head.id ASC
    ) AS first_track
  ) AS TEXT) AS artwork
FROM playlist_tracks AS pt
INNER JOIN tracks AS t
  ON pt.track_id = t.id
WHERE pt.playlist_id = sqlc.arg(playlist_id);

-- name: UpdateTrackPosition :exec
UPDATE playlist_tracks
SET position = ?
WHERE playlist_id = ?
  AND track_id = ?;
