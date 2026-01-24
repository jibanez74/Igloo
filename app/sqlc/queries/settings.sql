-- name: GetSettings :one
SELECT
  *
FROM
  settings
LIMIT
  1;

-- name: CreateSettings :one
INSERT INTO
  settings (
    tmdb_key,
    jellyfin_token,
    spotify_client_id,
    spotify_client_secret,
    hardware_acceleration_device,
    enable_logger,
    enable_watcher,
    download_images,
    movies_dir,
    shows_dir,
    music_dir,
    static_dir,
    logs_dir
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;