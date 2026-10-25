-- name: GetSettings :one
SELECT
  *
FROM settings
ORDER BY id
LIMIT 1;

-- name: CreateSettings :one
INSERT INTO settings (
  tmdb_key,
  spotify_client_id,
  spotify_client_secret,
  enable_apple_lookups,
  hardware_acceleration_device,
  enable_watcher,
  download_images,
  movies_dir,
  shows_dir,
  music_dir,
  static_dir,
  transcode_dir
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateGeneralSettings :one
UPDATE settings
SET
  tmdb_key = ?,
  spotify_client_id = ?,
  spotify_client_secret = ?,
  enable_apple_lookups = ?,
  enable_watcher = ?,
  download_images = ?,
  static_dir = ?,
  transcode_dir = ?
WHERE id = (
  SELECT id
  FROM settings
  ORDER BY id
  LIMIT 1
)
RETURNING *;

-- name: UpdateLibrarySettings :one
UPDATE settings
SET
  movies_dir = ?,
  shows_dir = ?,
  music_dir = ?
WHERE id = (
  SELECT id
  FROM settings
  ORDER BY id
  LIMIT 1
)
RETURNING *;

-- name: UpdatePlaybackServerSettings :one
UPDATE settings
SET
  server_upload_mbps = ?,
  hardware_acceleration_device = ?
WHERE id = (
  SELECT id
  FROM settings
  ORDER BY id
  LIMIT 1
)
RETURNING *;
