-- name: GetSettings :one
SELECT * FROM global_settings LIMIT 1;

-- name: CreateSettings :one
INSERT INTO global_settings (
    port,
    debug,
    enable_logger,
    base_url,
    logs_dir,
    enable_watcher,
    movies_dir,
    music_dir,
    tvshows_dir,
    transcode_dir,
    movies_img_dir,
    studios_img_dir,
    artists_img_dir,
    avatar_img_dir,
    static_dir,
    download_images,
    tmdb_api_key,
    ffmpeg_path,
    ffprobe_path,
    enable_hardware_acceleration,
    hardware_acceleration_method,
    jellyfin_token,
    plex_token,
    spotify_client_id,
    spotify_client_secret
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25
)
RETURNING *;

-- name: UpdateSettings :one
UPDATE global_settings SET
    port = $1,
    debug = $2,
    enable_logger = $3,
    base_url = $4,
    logs_dir = $5,
    enable_watcher = $6,
    movies_dir = $7,
    music_dir = $8,
    tvshows_dir = $9,
    transcode_dir = $10,
    movies_img_dir = $11,
    studios_img_dir = $12,
    artists_img_dir = $13,
    avatar_img_dir = $14,
    static_dir = $15,
    download_images = $16,
    tmdb_api_key = $17,
    ffmpeg_path = $18,
    ffprobe_path = $19,
    enable_hardware_acceleration = $20,
    hardware_acceleration_method = $21,
    jellyfin_token = $22,
    plex_token = $23,
    spotify_client_id = $24,
    spotify_client_secret = $25,
    updated_at = CURRENT_TIMESTAMP
WHERE id = (SELECT id FROM global_settings LIMIT 1)
RETURNING *;
