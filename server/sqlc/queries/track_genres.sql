-- name: CreateTrackGenre :exec
INSERT INTO track_genres (track_id, genre_id)
VALUES (?, ?)
ON CONFLICT (track_id, genre_id) DO NOTHING;

-- name: DeleteTrackGenres :exec
DELETE FROM track_genres WHERE track_id = ?;

-- name: DeleteTrackGenresExcept :exec
-- Deletes all genre relationships for a track except the specified genre.
-- Used to efficiently update genres: only removes stale relationships.
DELETE FROM track_genres WHERE track_id = ? AND genre_id != ?;

-- name: GetGenresByAlbumID :many
SELECT
  tg.track_id,
  g.id AS genre_id,
  g.tag
FROM
  track_genres tg
  INNER JOIN genres g ON tg.genre_id = g.id
  INNER JOIN tracks t ON tg.track_id = t.id
WHERE
  t.album_id = ?
ORDER BY
  tg.track_id,
  g.tag;
