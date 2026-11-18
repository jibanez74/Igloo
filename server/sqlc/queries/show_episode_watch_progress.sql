-- name: GetShowEpisodeWatchProgress :one
SELECT
  progress_sec,
  duration_sec,
  watched,
  updated_at
FROM show_episode_watch_progress
WHERE user_id = ?
  AND episode_id = ?;

-- name: UpsertShowEpisodeWatchProgress :exec
INSERT INTO show_episode_watch_progress (
  user_id,
  episode_id,
  progress_sec,
  duration_sec,
  watched,
  save_session_id,
  save_sequence,
  updated_at
)
VALUES
  (?, ?, ?, ?, false, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, episode_id) DO UPDATE
SET
  progress_sec = excluded.progress_sec,
  duration_sec = excluded.duration_sec,
  watched = false,
  save_session_id = excluded.save_session_id,
  save_sequence = excluded.save_sequence,
  updated_at = CURRENT_TIMESTAMP
WHERE show_episode_watch_progress.save_session_id <> excluded.save_session_id
   OR show_episode_watch_progress.save_sequence < excluded.save_sequence;

-- name: DeleteShowEpisodeWatchProgress :exec
DELETE FROM show_episode_watch_progress
WHERE user_id = ?
  AND episode_id = ?;

-- name: MarkShowEpisodeWatched :exec
INSERT INTO show_episode_watch_progress (
  user_id,
  episode_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES
  (?, ?, 0, 0, true, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, episode_id) DO UPDATE
SET
  progress_sec = 0,
  watched = true,
  updated_at = CURRENT_TIMESTAMP;

-- name: MarkShowEpisodeWatchedFromProgress :exec
INSERT INTO show_episode_watch_progress (
  user_id,
  episode_id,
  progress_sec,
  duration_sec,
  watched,
  save_session_id,
  save_sequence,
  updated_at
)
VALUES
  (?, ?, 0, 0, true, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, episode_id) DO UPDATE
SET
  progress_sec = 0,
  watched = true,
  save_session_id = excluded.save_session_id,
  save_sequence = excluded.save_sequence,
  updated_at = CURRENT_TIMESTAMP
WHERE show_episode_watch_progress.save_session_id <> excluded.save_session_id
   OR show_episode_watch_progress.save_sequence < excluded.save_sequence;

-- name: MarkShowEpisodeUnwatched :exec
INSERT INTO show_episode_watch_progress (
  user_id,
  episode_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES
  (?, ?, 0, 0, false, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, episode_id) DO UPDATE
SET
  watched = false,
  updated_at = CURRENT_TIMESTAMP;

-- name: ShowEpisodeExists :one
-- Existence probe for the watch-progress handlers, which only need to 404 on
-- an unknown episode.
SELECT
  EXISTS (
    SELECT 1
    FROM show_episodes
    WHERE id = ?
  );

-- name: GetContinueWatchingEpisodes :many
-- The episode half of the home "Continue Watching" row. Mirrors
-- GetContinueWatchingMovies, including the 30-second floor that must match the
-- web client's WATCH_PROGRESS_MIN_SECONDS, with two differences. An episode the
-- scanner knows but has no file for cannot be resumed, so the same EXISTS guard
-- GetShowNextEpisode uses skips it. And a show contributes one card rather than
-- one per episode: its most recently watched in-progress episode. updated_at
-- has one-second resolution, so the window breaks a tie on the episode id, as
-- the outer order and the movie half do; a GROUP BY with a bare MAX() took
-- whichever tied row the plan reached first. The limit comes from the caller,
-- as GetContinueWatchingMovies' does.
WITH ranked AS (
  SELECT
    wp.updated_at,
    e.id,
    e.name,
    e.episode_number,
    ss.season_number,
    sh.id AS show_id,
    sh.name AS show_name,
    sh.poster_path AS show_poster_path,
    sh.backdrop_path AS show_backdrop_path,
    sh.overview AS show_overview,
    e.overview AS episode_overview,
    sh.premiere_year AS show_premiere_year,
    wp.progress_sec,
    wp.duration_sec,
    ROW_NUMBER() OVER (
      PARTITION BY sh.id
      ORDER BY wp.updated_at DESC, e.id ASC
    ) AS show_rank
  FROM show_episode_watch_progress AS wp
  INNER JOIN show_episodes AS e ON e.id = wp.episode_id
  INNER JOIN show_seasons AS ss ON ss.id = e.season_id
  INNER JOIN shows AS sh ON sh.id = ss.show_id
  WHERE wp.user_id = ?
    AND wp.watched = false
    AND wp.progress_sec >= 30
    AND wp.duration_sec > 0
    AND wp.progress_sec < wp.duration_sec
    AND EXISTS (
      SELECT 1
      FROM show_episode_files AS f
      WHERE f.episode_id = e.id
    )
)
SELECT
  updated_at,
  id,
  name,
  episode_number,
  season_number,
  show_id,
  show_name,
  show_poster_path,
  show_backdrop_path,
  show_overview,
  episode_overview,
  show_premiere_year,
  progress_sec,
  duration_sec
FROM ranked
WHERE show_rank = 1
-- The episode id orders saves in the same second as the handler's merge does,
-- so the limit keeps the rows the merged row would.
ORDER BY updated_at DESC, id ASC
LIMIT ?;
