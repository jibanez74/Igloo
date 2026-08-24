-- name: GetMovieWatchProgress :one
SELECT
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  save_session_id,
  save_sequence,
  updated_at
FROM movie_watch_progress
WHERE user_id = ?
  AND movie_id = ?;

-- name: UpsertMovieWatchProgress :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  save_session_id,
  save_sequence,
  updated_at
)
VALUES
  (?, ?, ?, ?, false, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE
SET
  progress_sec = excluded.progress_sec,
  duration_sec = excluded.duration_sec,
  watched = false,
  save_session_id = excluded.save_session_id,
  save_sequence = excluded.save_sequence,
  updated_at = CURRENT_TIMESTAMP
WHERE movie_watch_progress.save_session_id <> excluded.save_session_id
   OR movie_watch_progress.save_sequence < excluded.save_sequence;

-- name: DeleteMovieWatchProgress :exec
DELETE FROM movie_watch_progress
WHERE user_id = ?
  AND movie_id = ?;

-- name: MarkMovieWatched :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES
  (?, ?, 0, 0, true, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE
SET
  progress_sec = 0,
  watched = true,
  updated_at = CURRENT_TIMESTAMP;

-- name: MarkMovieWatchedFromProgress :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  save_session_id,
  save_sequence,
  updated_at
)
VALUES
  (?, ?, 0, 0, true, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE
SET
  progress_sec = 0,
  watched = true,
  save_session_id = excluded.save_session_id,
  save_sequence = excluded.save_sequence,
  updated_at = CURRENT_TIMESTAMP
WHERE movie_watch_progress.save_session_id <> excluded.save_session_id
   OR movie_watch_progress.save_sequence < excluded.save_sequence;

-- name: GetContinueWatchingMovies :many
-- The 30-second floor must match the web client's
-- MOVIE_WATCH_PROGRESS_MIN_SECONDS resume-eligibility floor.
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  mwp.progress_sec,
  mwp.duration_sec
FROM movie_watch_progress AS mwp
JOIN movies AS m ON m.id = mwp.movie_id
WHERE mwp.user_id = ?
  AND mwp.watched = false
  AND mwp.progress_sec >= 30
  AND mwp.duration_sec > 0
  AND mwp.progress_sec < mwp.duration_sec
ORDER BY mwp.updated_at DESC
LIMIT 12;

-- name: MarkMovieUnwatched :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES
  (?, ?, 0, 0, false, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE
SET
  watched = false,
  updated_at = CURRENT_TIMESTAMP;
