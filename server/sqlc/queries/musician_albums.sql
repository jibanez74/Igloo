-- name: CreateMusicianAlbum :exec
INSERT INTO musician_albums (
  musician_id,
  album_id
)
VALUES
  (?, ?)
ON CONFLICT (musician_id, album_id) DO NOTHING;

-- name: GetMusicianIDsByAlbumID :many
SELECT musician_id FROM musician_albums WHERE album_id = ?;

-- name: DeleteAlbumMusiciansWithoutTracks :exec
DELETE FROM musician_albums
WHERE musician_albums.album_id = sqlc.arg(album_id)
  AND NOT EXISTS (
    SELECT 1
    FROM albums
    INNER JOIN musicians
      ON musicians.id = musician_albums.musician_id
    WHERE albums.id = musician_albums.album_id
      AND albums.musician = musicians.name
  )
  AND NOT EXISTS (
    SELECT 1
    FROM tracks
    WHERE tracks.album_id = musician_albums.album_id
      AND tracks.musician_id = musician_albums.musician_id
  )
  AND NOT EXISTS (
    SELECT 1
    FROM tracks
    INNER JOIN track_musicians
      ON track_musicians.track_id = tracks.id
    WHERE tracks.album_id = musician_albums.album_id
      AND track_musicians.musician_id = musician_albums.musician_id
  );
