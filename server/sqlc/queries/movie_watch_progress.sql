-- name: GetMovieWatchProgress :one
SELECT user_id, movie_id, progress_sec, duration_sec, watched, updated_at
FROM movie_watch_progress
WHERE user_id = ? AND movie_id = ?;

-- name: UpsertMovieWatchProgress :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES (?, ?, ?, ?, false, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE SET
  progress_sec = excluded.progress_sec,
  duration_sec = excluded.duration_sec,
  watched = false,
  updated_at = CURRENT_TIMESTAMP;

-- name: DeleteMovieWatchProgress :exec
DELETE FROM movie_watch_progress
WHERE user_id = ? AND movie_id = ?;

-- name: MarkMovieWatched :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES (?, ?, 0, 0, true, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE SET
  progress_sec = 0,
  watched = true,
  updated_at = CURRENT_TIMESTAMP;

-- name: MarkMovieUnwatched :exec
INSERT INTO movie_watch_progress (
  user_id,
  movie_id,
  progress_sec,
  duration_sec,
  watched,
  updated_at
)
VALUES (?, ?, 0, 0, false, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, movie_id) DO UPDATE SET
  watched = false,
  updated_at = CURRENT_TIMESTAMP;
