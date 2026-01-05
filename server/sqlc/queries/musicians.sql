-- name: GetMusicianBySpotifyID :one
SELECT * FROM musicians WHERE spotify_id = ? LIMIT 1;

-- name: UpsertMusician :one
INSERT INTO musicians (name, sort_name, summary, spotify_popularity, spotify_followers, spotify_id, thumb)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (name) DO UPDATE SET
  sort_name = excluded.sort_name,
  summary = COALESCE(excluded.summary, musicians.summary),
  spotify_popularity = COALESCE(excluded.spotify_popularity, musicians.spotify_popularity),
  spotify_followers = COALESCE(excluded.spotify_followers, musicians.spotify_followers),
  spotify_id = COALESCE(excluded.spotify_id, musicians.spotify_id),
  thumb = COALESCE(excluded.thumb, musicians.thumb),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetMusiciansByAlbumID :many
SELECT
  m.id,
  m.name,
  m.thumb,
  m.spotify_id
FROM
  musicians m
  INNER JOIN musician_albums ma ON m.id = ma.musician_id
WHERE
  ma.album_id = ?
ORDER BY
  m.name ASC;
