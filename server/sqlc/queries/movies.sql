-- name: CheckMovieUnchanged :one
-- Quick check if movie exists with same path and size (likely unchanged)
SELECT
  1
FROM
  movies
WHERE
  file_path = ?
  AND size = ?
LIMIT
  1;

-- name: GetMovieByFilePath :one
SELECT
  *
FROM
  movies
WHERE
  file_path = ?
LIMIT
  1;

-- name: GetMovieByID :one
SELECT
  *
FROM
  movies
WHERE
  id = ?
LIMIT
  1;

-- name: GetMovieByTmdbID :one
-- When multiple rows share the same tmdb_id, returns the one with smallest id.
SELECT
  *
FROM
  movies
WHERE
  tmdb_id = ?
ORDER BY
  id ASC
LIMIT
  1;

-- name: GetLatestMovies :many
SELECT
  id,
  title,
  poster_path,
  year,
  certification
FROM
  movies
ORDER BY
  created_at DESC
LIMIT
  12;

-- name: UpsertMovie :one
INSERT INTO
  movies (
    title,
    file_path,
    file_name,
    size,
    container,
    mime_type,
    adult,
    tmdb_id,
    imdb_id,
    poster_path,
    backdrop_path,
    language,
    year,
    release_date,
    overview,
    tag_line,
    certification,
    critic_rating,
    audience_rating,
    revenue,
    budget,
    run_time
  )
VALUES
  (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
  ) ON CONFLICT (file_path) DO
UPDATE
SET
  title = excluded.title,
  file_name = excluded.file_name,
  size = excluded.size,
  container = excluded.container,
  mime_type = excluded.mime_type,
  adult = excluded.adult,
  tmdb_id = COALESCE(excluded.tmdb_id, movies.tmdb_id),
  imdb_id = COALESCE(excluded.imdb_id, movies.imdb_id),
  poster_path = COALESCE(excluded.poster_path, movies.poster_path),
  language = COALESCE(excluded.language, movies.language),
  year = COALESCE(excluded.year, movies.year),
  release_date = COALESCE(excluded.release_date, movies.release_date),
  overview = COALESCE(excluded.overview, movies.overview),
  tag_line = COALESCE(excluded.tag_line, movies.tag_line),
  certification = COALESCE(excluded.certification, movies.certification),
  critic_rating = COALESCE(excluded.critic_rating, movies.critic_rating),
  audience_rating = COALESCE(excluded.audience_rating, movies.audience_rating),
  revenue = COALESCE(excluded.revenue, movies.revenue),
  budget = COALESCE(excluded.budget, movies.budget),
  run_time = COALESCE(excluded.run_time, movies.run_time),
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: UpsertProductionCompany :one
INSERT INTO
  production_companies (name, tmdb_id, logo, country)
VALUES
  (?, ?, ?, ?) ON CONFLICT (tmdb_id) DO
UPDATE
SET
  name = excluded.name,
  logo = COALESCE(excluded.logo, production_companies.logo),
  country = COALESCE(excluded.country, production_companies.country),
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: UpsertArtist :one
INSERT INTO
  artist (name, tmdb_id, profile)
VALUES
  (?, ?, ?) ON CONFLICT (tmdb_id) DO
UPDATE
SET
  name = excluded.name,
  profile = COALESCE(excluded.profile, artist.profile),
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: UpsertCast :one
INSERT INTO
  cast(movie_id, artist_id, character, cast_order)
VALUES
  (?, ?, ?, ?) ON CONFLICT (movie_id, artist_id, cast_order) DO
UPDATE
SET
  character = excluded.character,
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: UpsertCrew :one
INSERT INTO
  crew (movie_id, artist_id, job, department)
VALUES
  (?, ?, ?, ?) ON CONFLICT (movie_id, artist_id, job, department) DO
UPDATE
SET
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: CreateMovieProductionCompany :exec
-- Link movie to production company via junction table
INSERT INTO
  movie_production_companies (movie_id, production_company_id)
VALUES
  (?, ?) ON CONFLICT (movie_id, production_company_id) DO NOTHING;

-- name: DeleteMovieProductionCompanies :exec
-- Remove all production company links for a movie
DELETE FROM movie_production_companies
WHERE
  movie_id = ?;

-- name: DeleteMovieVideoStreams :exec
-- Delete all video streams for a movie
DELETE FROM video_streams
WHERE
  movie_id = ?;

-- name: InsertVideoStream :one
INSERT INTO
  video_streams (
    movie_id,
    stream_index,
    codec,
    codec_profile,
    codec_level,
    bit_rate,
    width,
    height,
    coded_width,
    coded_height,
    aspect_ratio,
    frame_rate,
    avg_frame_rate,
    bit_depth,
    color_range,
    color_space,
    color_primaries,
    color_transfer,
    language,
    title
  )
VALUES
  (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
  ) RETURNING *;

-- name: DeleteMovieAudioStreams :exec
-- Delete all audio streams for a movie
DELETE FROM audio_streams
WHERE
  movie_id = ?;

-- name: InsertAudioStream :one
INSERT INTO
  audio_streams (
    movie_id,
    stream_index,
    codec,
    codec_profile,
    bit_rate,
    sample_rate,
    channels,
    channel_layout,
    language,
    title
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: DeleteMovieSubtitles :exec
-- Delete all subtitles for a movie
DELETE FROM subtitles
WHERE
  movie_id = ?;

-- name: InsertSubtitle :one
INSERT INTO
  subtitles (
    movie_id,
    stream_index,
    codec,
    language,
    title,
    is_forced,
    is_default
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: DeleteMovieChapters :exec
-- Delete all chapters for a movie
DELETE FROM chapters
WHERE
  movie_id = ?;

-- name: InsertChapter :one
INSERT INTO
  chapters (movie_id, title, start_time, thumb)
VALUES
  (?, ?, ?, ?) RETURNING *;

-- name: CreateMovieGenre :exec
-- Link movie to genre via junction table
INSERT INTO
  movie_genres (movie_id, genre_id)
VALUES
  (?, ?) ON CONFLICT (movie_id, genre_id) DO NOTHING;

-- name: DeleteMovieGenres :exec
-- Remove all genre links for a movie
DELETE FROM movie_genres
WHERE
  movie_id = ?;

-- name: UpsertExtraVideo :one
-- Insert or update an extra video by external_id (e.g. TMDB video id). Use for trailers/special features.
-- Call with a non-null external_id so conflicts are detected; then link via CreateMovieExtraVideo.
INSERT INTO
  extra_videos (title, external_id, key, type, site, official)
VALUES
  (?, ?, ?, ?, ?, ?) ON CONFLICT (external_id) DO
UPDATE
SET
  title = excluded.title,
  key = excluded.key,
  type = excluded.type,
  site = excluded.site,
  official = excluded.official,
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: CreateMovieExtraVideo :exec
-- Link a movie to an extra video (trailer/special feature). Idempotent.
INSERT INTO
  movie_extra_videos (movie_id, extra_video_id)
VALUES
  (?, ?) ON CONFLICT (movie_id, extra_video_id) DO NOTHING;

-- name: DeleteMovieExtraVideos :exec
-- Remove all extra-video links for a movie (e.g. before re-scanning).
DELETE FROM movie_extra_videos
WHERE
  movie_id = ?;

-- name: GetCastByMovieID :many
-- Cast for a movie with artist name and profile (for details view).
SELECT
  c.id,
  c.movie_id,
  c.artist_id,
  c.character,
  c.cast_order,
  a.name AS artist_name,
  a.profile AS artist_profile
FROM
  cast c
  INNER JOIN artist a ON a.id = c.artist_id
WHERE
  c.movie_id = ?
ORDER BY
  c.cast_order;

-- name: GetCrewByMovieID :many
-- Crew for a movie with artist name and profile (for details view).
SELECT
  c.id,
  c.movie_id,
  c.artist_id,
  c.job,
  c.department,
  a.name AS artist_name,
  a.profile AS artist_profile
FROM
  crew c
  INNER JOIN artist a ON a.id = c.artist_id
WHERE
  c.movie_id = ?
ORDER BY
  c.department,
  c.job;

-- name: GetGenresByMovieID :many
-- Genres linked to a movie (for details view).
SELECT
  g.id,
  g.tag
FROM
  genres g
  INNER JOIN movie_genres mg ON mg.genre_id = g.id
WHERE
  mg.movie_id = ?
ORDER BY
  g.tag;

-- name: GetProductionCompaniesByMovieID :many
-- Production companies linked to a movie (for details view).
SELECT
  pc.id,
  pc.name,
  pc.tmdb_id,
  pc.logo,
  pc.country
FROM
  production_companies pc
  INNER JOIN movie_production_companies mpc ON mpc.production_company_id = pc.id
WHERE
  mpc.movie_id = ?
ORDER BY
  pc.name;

-- name: GetMovieExtraVideos :many
-- List all extra videos (trailers, special features) linked to a movie.
SELECT
  extra_videos.id,
  extra_videos.title,
  extra_videos.external_id,
  extra_videos.key,
  extra_videos.type,
  extra_videos.site,
  extra_videos.official,
  extra_videos.created_at,
  extra_videos.updated_at
FROM
  extra_videos
  INNER JOIN movie_extra_videos ON movie_extra_videos.extra_video_id = extra_videos.id
WHERE
  movie_extra_videos.movie_id = ?
ORDER BY
  extra_videos.type,
  extra_videos.title;