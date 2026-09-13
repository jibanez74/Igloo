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
