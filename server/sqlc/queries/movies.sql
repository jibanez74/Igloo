-- name: GetMovieForDirectStream :one
SELECT
  file_path,
  file_name,
  mime_type
FROM movies
WHERE id = ?
LIMIT 1;

-- name: CheckMovieUnchanged :one
-- Quick check if movie exists with same path and size (likely unchanged)
SELECT
  1
FROM movies
WHERE file_path = ?
  AND size = ?
LIMIT 1;

-- name: GetMovieByID :one
SELECT
  *
FROM movies
WHERE id = ?
LIMIT 1;

-- name: GetMoviesByIDs :many
SELECT
  *
FROM movies
WHERE id IN (sqlc.slice(ids));

-- name: GetMovieByPath :one
SELECT
  *
FROM movies
WHERE file_path = ?
LIMIT 1;

-- name: GetMovieScanIndex :many
SELECT
  id,
  title,
  file_path,
  file_name,
  size,
  tmdb_id,
  year,
  duration
FROM movies
ORDER BY id;

-- name: ReassignMoviePath :exec
UPDATE movies
SET
  file_path = ?,
  file_name = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetLatestMovies :many
SELECT
  id,
  title,
  poster_path,
  year,
  certification
FROM movies
ORDER BY created_at DESC
LIMIT 12;

-- name: UpsertMovie :one
INSERT INTO movies (
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
  run_time,
  duration
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (file_path) DO UPDATE
SET
  title = CASE
    WHEN movies.user_locked_title THEN movies.title
    ELSE excluded.title
  END,
  file_name = excluded.file_name,
  size = excluded.size,
  container = excluded.container,
  mime_type = excluded.mime_type,
  adult = CASE
    WHEN movies.user_locked_adult THEN movies.adult
    ELSE excluded.adult
  END,
  tmdb_id = CASE
    WHEN movies.user_locked_tmdb_id THEN movies.tmdb_id
    ELSE COALESCE(excluded.tmdb_id, movies.tmdb_id)
  END,
  imdb_id = CASE
    WHEN movies.user_locked_imdb_id THEN movies.imdb_id
    ELSE COALESCE(excluded.imdb_id, movies.imdb_id)
  END,
  poster_path = CASE
    WHEN movies.user_locked_poster_path THEN movies.poster_path
    ELSE COALESCE(excluded.poster_path, movies.poster_path)
  END,
  backdrop_path = CASE
    WHEN movies.user_locked_backdrop_path THEN movies.backdrop_path
    ELSE COALESCE(excluded.backdrop_path, movies.backdrop_path)
  END,
  language = CASE
    WHEN movies.user_locked_language THEN movies.language
    ELSE COALESCE(excluded.language, movies.language)
  END,
  year = CASE
    WHEN movies.user_locked_year THEN movies.year
    ELSE COALESCE(excluded.year, movies.year)
  END,
  release_date = CASE
    WHEN movies.user_locked_release_date THEN movies.release_date
    ELSE COALESCE(excluded.release_date, movies.release_date)
  END,
  overview = CASE
    WHEN movies.user_locked_overview THEN movies.overview
    ELSE COALESCE(excluded.overview, movies.overview)
  END,
  tag_line = CASE
    WHEN movies.user_locked_tag_line THEN movies.tag_line
    ELSE COALESCE(excluded.tag_line, movies.tag_line)
  END,
  certification = CASE
    WHEN movies.user_locked_certification THEN movies.certification
    ELSE COALESCE(excluded.certification, movies.certification)
  END,
  critic_rating = CASE
    WHEN movies.user_locked_critic_rating THEN movies.critic_rating
    ELSE COALESCE(excluded.critic_rating, movies.critic_rating)
  END,
  audience_rating = CASE
    WHEN movies.user_locked_audience_rating THEN movies.audience_rating
    ELSE COALESCE(excluded.audience_rating, movies.audience_rating)
  END,
  revenue = CASE
    WHEN movies.user_locked_revenue THEN movies.revenue
    ELSE COALESCE(excluded.revenue, movies.revenue)
  END,
  budget = CASE
    WHEN movies.user_locked_budget THEN movies.budget
    ELSE COALESCE(excluded.budget, movies.budget)
  END,
  run_time = CASE
    WHEN movies.user_locked_run_time THEN movies.run_time
    ELSE COALESCE(excluded.run_time, movies.run_time)
  END,
  duration = COALESCE(excluded.duration, movies.duration),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpsertProductionCompany :one
INSERT INTO production_companies (
  name,
  tmdb_id,
  logo,
  country
)
VALUES
  (?, ?, ?, ?)
ON CONFLICT (tmdb_id) DO UPDATE
SET
  name = excluded.name,
  logo = COALESCE(excluded.logo, production_companies.logo),
  country = COALESCE(excluded.country, production_companies.country),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpsertArtist :one
INSERT INTO artist (
  name,
  tmdb_id,
  profile
)
VALUES
  (?, ?, ?)
ON CONFLICT (tmdb_id) DO UPDATE
SET
  name = excluded.name,
  profile = COALESCE(excluded.profile, artist.profile),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpsertCast :one
INSERT INTO cast (
  movie_id,
  artist_id,
  character,
  cast_order
)
VALUES
  (?, ?, ?, ?)
ON CONFLICT (movie_id, artist_id, cast_order) DO UPDATE
SET
  character = excluded.character,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpsertCrew :one
INSERT INTO crew (
  movie_id,
  artist_id,
  job,
  department
)
VALUES
  (?, ?, ?, ?)
ON CONFLICT (movie_id, artist_id, job, department) DO UPDATE
SET
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: CreateMovieProductionCompany :exec
-- Link movie to production company via junction table
INSERT INTO movie_production_companies (
  movie_id,
  production_company_id
)
VALUES
  (?, ?)
ON CONFLICT (movie_id, production_company_id) DO NOTHING;

-- name: DeleteMovieProductionCompanies :exec
-- Remove all production company links for a movie
DELETE FROM movie_production_companies
WHERE movie_id = ?;

-- name: DeleteMovieVideoStreams :exec
-- Delete all video streams for a movie
DELETE FROM video_streams
WHERE movie_id = ?;

-- name: InsertVideoStream :one
INSERT INTO video_streams (
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
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteMovieAudioStreams :exec
-- Delete all audio streams for a movie
DELETE FROM audio_streams
WHERE movie_id = ?;

-- name: InsertAudioStream :one
INSERT INTO audio_streams (
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
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteMovieSubtitles :exec
-- Delete all subtitles for a movie
DELETE FROM subtitles
WHERE movie_id = ?;

-- name: InsertSubtitle :one
INSERT INTO subtitles (
  movie_id,
  stream_index,
  codec,
  language,
  title,
  is_forced,
  is_default
)
VALUES
  (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteMovieChapters :exec
-- Delete all chapters for a movie
DELETE FROM chapters
WHERE movie_id = ?;

-- name: InsertChapter :one
INSERT INTO chapters (
  movie_id,
  title,
  start_time,
  thumb
)
VALUES
  (?, ?, ?, ?)
RETURNING *;

-- name: CreateMovieGenre :exec
-- Link movie to genre via junction table
INSERT INTO movie_genres (
  movie_id,
  genre_id
)
VALUES
  (?, ?)
ON CONFLICT (movie_id, genre_id) DO NOTHING;

-- name: DeleteMovieGenres :exec
-- Remove all genre links for a movie
DELETE FROM movie_genres
WHERE movie_id = ?;

-- name: UpsertExtraVideo :one
-- Insert or update an extra video by external_id (e.g. TMDB video id). Use for trailers/special features.
-- Call with a non-null external_id so conflicts are detected; then link via CreateMovieExtraVideo.
INSERT INTO extra_videos (
  title,
  external_id,
  key,
  type,
  site,
  official
)
VALUES
  (?, ?, ?, ?, ?, ?)
ON CONFLICT (external_id) DO UPDATE
SET
  title = excluded.title,
  key = excluded.key,
  type = excluded.type,
  site = excluded.site,
  official = excluded.official,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: CreateMovieExtraVideo :exec
-- Link a movie to an extra video (trailer/special feature). Idempotent.
INSERT INTO movie_extra_videos (
  movie_id,
  extra_video_id
)
VALUES
  (?, ?)
ON CONFLICT (movie_id, extra_video_id) DO NOTHING;

-- name: DeleteMovieExtraVideos :exec
-- Remove all extra-video links for a movie (e.g. before re-scanning).
DELETE FROM movie_extra_videos
WHERE movie_id = ?;

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
FROM cast AS c
INNER JOIN artist AS a
  ON a.id = c.artist_id
WHERE c.movie_id = ?
ORDER BY c.cast_order;

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
FROM crew AS c
INNER JOIN artist AS a
  ON a.id = c.artist_id
WHERE c.movie_id = ?
ORDER BY
  c.department,
  c.job;

-- name: GetGenresByMovieID :many
-- Genres linked to a movie (for details view).
SELECT
  g.id,
  g.tag
FROM genres AS g
INNER JOIN movie_genres AS mg
  ON mg.genre_id = g.id
WHERE mg.movie_id = ?
ORDER BY g.tag;

-- name: GetProductionCompaniesByMovieID :many
-- Production companies linked to a movie (for details view).
SELECT
  pc.id,
  pc.name,
  pc.tmdb_id,
  pc.logo,
  pc.country
FROM production_companies AS pc
INNER JOIN movie_production_companies AS mpc
  ON mpc.production_company_id = pc.id
WHERE mpc.movie_id = ?
ORDER BY pc.name;

-- name: GetMovieExtraVideos :many
-- List all extra videos (trailers, special features) linked to a movie.
SELECT
  ev.id,
  ev.title,
  ev.external_id,
  ev.key,
  ev.type,
  ev.site,
  ev.official,
  ev.created_at,
  ev.updated_at
FROM extra_videos AS ev
INNER JOIN movie_extra_videos AS mev
  ON mev.extra_video_id = ev.id
WHERE mev.movie_id = ?
ORDER BY
  ev.type,
  ev.title;

-- name: UpdateMovie :one
-- Dedicated UPDATE for movie metadata (used by Edit feature).
-- Does NOT touch file-level fields (file_path, file_name, size, container, mime_type).
UPDATE movies
SET
  title = ?,
  tmdb_id = ?,
  imdb_id = ?,
  poster_path = ?,
  backdrop_path = ?,
  adult = ?,
  language = ?,
  year = ?,
  release_date = ?,
  overview = ?,
  tag_line = ?,
  certification = ?,
  critic_rating = ?,
  audience_rating = ?,
  revenue = ?,
  budget = ?,
  run_time = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: LockMovieMetadataFields :exec
UPDATE movies
SET
  user_locked_title = user_locked_title OR ?,
  user_locked_tmdb_id = user_locked_tmdb_id OR ?,
  user_locked_imdb_id = user_locked_imdb_id OR ?,
  user_locked_poster_path = user_locked_poster_path OR ?,
  user_locked_backdrop_path = user_locked_backdrop_path OR ?,
  user_locked_adult = user_locked_adult OR ?,
  user_locked_language = user_locked_language OR ?,
  user_locked_year = user_locked_year OR ?,
  user_locked_release_date = user_locked_release_date OR ?,
  user_locked_overview = user_locked_overview OR ?,
  user_locked_tag_line = user_locked_tag_line OR ?,
  user_locked_certification = user_locked_certification OR ?,
  user_locked_critic_rating = user_locked_critic_rating OR ?,
  user_locked_audience_rating = user_locked_audience_rating OR ?,
  user_locked_revenue = user_locked_revenue OR ?,
  user_locked_budget = user_locked_budget OR ?,
  user_locked_run_time = user_locked_run_time OR ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteMovieCast :exec
-- Remove all cast entries for a movie (used before re-identifying with TMDB).
DELETE FROM cast
WHERE movie_id = ?;

-- name: DeleteMovieCrew :exec
-- Remove all crew entries for a movie (used before re-identifying with TMDB).
DELETE FROM crew
WHERE movie_id = ?;

-- name: DeleteMovie :exec
-- Delete a movie by ID. Related data is cascade-deleted via ON DELETE CASCADE.
DELETE FROM movies
WHERE id = ?;

-- name: GetVideoStreamsByMovieID :many
-- Video streams for a movie (for technical details display).
SELECT
  *
FROM video_streams
WHERE movie_id = ?
ORDER BY stream_index;

-- name: GetAudioStreamsByMovieID :many
-- Audio streams for a movie (for technical details and playback settings).
SELECT
  *
FROM audio_streams
WHERE movie_id = ?
ORDER BY stream_index;

-- name: GetSubtitlesByMovieID :many
-- Subtitle tracks for a movie (for technical details display).
SELECT
  *
FROM subtitles
WHERE movie_id = ?
ORDER BY stream_index;

-- name: GetChaptersByMovieID :many
-- Chapters for a movie (for technical details display).
SELECT
  *
FROM chapters
WHERE movie_id = ?
ORDER BY start_time;

-- name: GetMoviesCount :one
SELECT
  COUNT(*)
FROM movies;

-- name: GetMoviesLibraryAsc :many
-- Paginated library A-Z (id tie-breaker so LIMIT/OFFSET is stable when titles match).
SELECT
  id,
  title,
  poster_path,
  year,
  certification
FROM movies
ORDER BY
  LOWER(title) ASC,
  id ASC
LIMIT ?
OFFSET ?;

-- name: GetMoviesLibraryDesc :many
-- Paginated library Z-A (id tie-breaker so LIMIT/OFFSET is stable when titles match).
SELECT
  id,
  title,
  poster_path,
  year,
  certification
FROM movies
ORDER BY
  LOWER(title) DESC,
  id DESC
LIMIT ?
OFFSET ?;

-- name: GetMovieGenresWithCounts :many
-- Movie genres with counts per tag (genre_type movie only).
SELECT
  g.id AS genre_id,
  g.tag AS genre_tag,
  COUNT(mg.movie_id) AS movie_count
FROM genres AS g
INNER JOIN movie_genres AS mg
  ON mg.genre_id = g.id
WHERE g.genre_type = 'movie'
GROUP BY
  g.id,
  g.tag
ORDER BY LOWER(g.tag) ASC;

-- name: CountMoviesForGenre :one
SELECT
  COUNT(*)
FROM movie_genres
WHERE genre_id = ?;

-- name: GetMoviesByGenreAsc :many
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  m.certification
FROM movies AS m
INNER JOIN movie_genres AS mg
  ON mg.movie_id = m.id
WHERE mg.genre_id = ?
ORDER BY
  LOWER(m.title) ASC,
  m.id ASC
LIMIT ?
OFFSET ?;

-- name: GetMoviesByGenreDesc :many
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  m.certification
FROM movies AS m
INNER JOIN movie_genres AS mg
  ON mg.movie_id = m.id
WHERE mg.genre_id = ?
ORDER BY
  LOWER(m.title) DESC,
  m.id DESC
LIMIT ?
OFFSET ?;
