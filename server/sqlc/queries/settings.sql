-- name: GetSettings :one
SELECT
  *
FROM settings
ORDER BY id
LIMIT 1;

-- name: CreateSettings :one
INSERT INTO settings (
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
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateGeneralSettings :one
UPDATE settings
SET
  tmdb_key = ?,
  jellyfin_token = ?,
  spotify_client_id = ?,
  spotify_client_secret = ?,
  hardware_acceleration_device = ?,
  enable_logger = ?,
  enable_watcher = ?,
  download_images = ?,
  static_dir = ?,
  logs_dir = ?,
  server_upload_mbps = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = (
  SELECT id
  FROM settings
  ORDER BY id
  LIMIT 1
)
RETURNING *;
