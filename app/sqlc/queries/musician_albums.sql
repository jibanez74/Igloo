-- name: CreateMusicianAlbum :exec
INSERT INTO
  musician_albums (musician_id, album_id)
VALUES
  (?, ?) ON CONFLICT (musician_id, album_id) DO NOTHING;