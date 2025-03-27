-- name: GetSettingsCount :one
SELECT COUNT(*) FROM global_settings;

-- name: GetSettings :one
SELECT * FROM global_settings LIMIT 1;

-- name: CreateSettings :one
INSERT INTO global_settings (
    port,
    debug,
    base_url,
    movies_dir_list,
    movies_img_dir,
    music_dir_list,
    tvshows_dir_list,
    transcode_dir,
    studios_img_dir,
    static_dir,
    artists_img_dir,
    avatar_img_dir,
    download_images,
    tmdb_api_key,
    ffmpeg_path,
    ffprobe_path,
    enable_hardware_acceleration,
    jellyfin_token,
    issuer,
    audience,
    secret,
    cookie_domain,
    cookie_path
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
)
RETURNING *;

-- name: UpdateSettings :one
UPDATE global_settings SET
    movies_dir_list = $1,
    movies_img_dir = $2,
    music_dir_list = $3,
    tvshows_dir_list = $4,
    transcode_dir = $5,
    studios_img_dir = $6,
    static_dir = $7,
    artists_img_dir = $8,
    avatar_img_dir = $9,
    download_images = $10,
    tmdb_api_key = $11,
    ffmpeg_path = $12,
    ffprobe_path = $13,
    enable_hardware_acceleration = $14,
    jellyfin_token = $15,
    updated_at = NOW()
WHERE id = (SELECT id FROM global_settings LIMIT 1)
RETURNING *;
