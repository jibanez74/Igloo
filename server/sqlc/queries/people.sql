-- name: GetArtistByTmdbID :one
SELECT
  *
FROM artist
WHERE tmdb_id = ?;

-- name: GetPersonLocalCredits :many
WITH credits AS (
SELECT
  'movie' AS media_type,
  m.id AS library_id,
  m.tmdb_id,
  m.title,
  COALESCE(m.release_date, CAST(m.year AS TEXT), '') AS date,
  COALESCE(m.poster_path, '') AS poster_path,
  'acting' AS kind,
  c.character AS role,
  'Acting' AS department
FROM cast c
JOIN movies m ON m.id = c.movie_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'movie' AS media_type,
  m.id AS library_id,
  m.tmdb_id,
  m.title,
  COALESCE(m.release_date, CAST(m.year AS TEXT), '') AS date,
  COALESCE(m.poster_path, '') AS poster_path,
  'crew' AS kind,
  c.job AS role,
  c.department AS department
FROM crew c
JOIN movies m ON m.id = c.movie_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'acting',
  c.character,
  'Acting'
FROM show_cast c
JOIN shows s ON s.id = c.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'crew',
  c.job,
  c.department
FROM show_crew c
JOIN shows s ON s.id = c.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'crew',
  'Creator',
  'Writing'
FROM show_creators c
JOIN shows s ON s.id = c.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'acting',
  c.character,
  'Acting'
FROM show_season_cast c
JOIN show_seasons ss ON ss.id = c.season_id
JOIN shows s ON s.id = ss.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'crew',
  c.job,
  c.department
FROM show_season_crew c
JOIN show_seasons ss ON ss.id = c.season_id
JOIN shows s ON s.id = ss.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'acting',
  c.character,
  'Acting'
FROM show_episode_guest_cast c
JOIN show_episodes ep ON ep.id = c.episode_id
JOIN show_seasons ss ON ss.id = ep.season_id
JOIN shows s ON s.id = ss.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
UNION ALL
SELECT
  'tv',
  s.id,
  s.tmdb_id,
  s.name,
  COALESCE(s.first_air_date, CAST(s.premiere_year AS TEXT), ''),
  COALESCE(s.poster_path, ''),
  'crew',
  c.job,
  c.department
FROM show_episode_crew c
JOIN show_episodes ep ON ep.id = c.episode_id
JOIN show_seasons ss ON ss.id = ep.season_id
JOIN shows s ON s.id = ss.show_id
WHERE c.artist_id = sqlc.arg(artist_id)
)
SELECT * FROM credits;

-- name: GetPersonLibraryMovies :many
SELECT
  tmdb_id,
  CAST(MIN(id) AS INTEGER) AS id
FROM movies
WHERE tmdb_id IN (sqlc.slice(tmdb_ids))
GROUP BY tmdb_id;

-- name: GetPersonLibraryShows :many
SELECT
  tmdb_id,
  CAST(MIN(id) AS INTEGER) AS id
FROM shows
WHERE tmdb_id IN (sqlc.slice(tmdb_ids))
GROUP BY tmdb_id;
