-- name: AddTrackToPlaylist :one
INSERT INTO playlist_tracks (playlist_id, track_id, position, added_by)
VALUES (?1, ?2, (SELECT COALESCE(MAX(position), -1) + 1 FROM playlist_tracks pt2 WHERE pt2.playlist_id = ?1), ?3)
RETURNING *;

-- name: RemoveTrackFromPlaylist :exec
DELETE FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?;

-- name: GetPlaylistTracksInfinite :many
SELECT 
    pt.id as playlist_track_id,
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
    a.title as album_title,
    a.cover as album_cover,
    m.name as musician_name
FROM playlist_tracks pt
JOIN tracks t ON pt.track_id = t.id
LEFT JOIN albums a ON t.album_id = a.id
LEFT JOIN musicians m ON t.musician_id = m.id
WHERE pt.playlist_id = ?
ORDER BY pt.position ASC
LIMIT ? OFFSET ?;

-- name: GetAllPlaylistTracks :many
SELECT 
    pt.id as playlist_track_id,
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
    a.title as album_title,
    a.cover as album_cover,
    m.name as musician_name
FROM playlist_tracks pt
JOIN tracks t ON pt.track_id = t.id
LEFT JOIN albums a ON t.album_id = a.id
LEFT JOIN musicians m ON t.musician_id = m.id
WHERE pt.playlist_id = ?
ORDER BY pt.position ASC;

-- name: CountPlaylistTracks :one
SELECT COUNT(*) as count FROM playlist_tracks WHERE playlist_id = ?;

-- name: GetPlaylistDuration :one
SELECT COALESCE(SUM(t.duration), 0) as total_duration
FROM playlist_tracks pt
JOIN tracks t ON pt.track_id = t.id
WHERE pt.playlist_id = ?;

-- name: UpdateTrackPosition :exec
UPDATE playlist_tracks SET position = ? WHERE playlist_id = ? AND track_id = ?;

-- name: ShiftPositionsDown :exec
UPDATE playlist_tracks 
SET position = position + 1 
WHERE playlist_id = ? AND position >= ?;

-- name: ShiftPositionsUp :exec
UPDATE playlist_tracks 
SET position = position - 1 
WHERE playlist_id = ? AND position > ?;

-- name: IsTrackInPlaylist :one
SELECT EXISTS(SELECT 1 FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?) as is_in_playlist;

-- name: ClearPlaylist :exec
DELETE FROM playlist_tracks WHERE playlist_id = ?;

-- name: GetMaxPosition :one
SELECT COALESCE(MAX(position), -1) as max_position FROM playlist_tracks WHERE playlist_id = ?;
