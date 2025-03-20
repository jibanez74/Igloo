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
    hardware_acceleration,
    enable_transcoding,
    jellyfin_token,
    issuer,
    audience,
    secret,
    cookie_domain,
    cookie_path
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
)
RETURNING *;

-- name: UpdateSettings :one
UPDATE global_settings SET
    port = $1,
    debug = $2,
    base_url = $3,
    movies_dir_list = $4,
    movies_img_dir = $5,
    music_dir_list = $6,
    tvshows_dir_list = $7,
    transcode_dir = $8,
    studios_img_dir = $9,
    static_dir = $10,
    artists_img_dir = $11,
    avatar_img_dir = $12,
    download_images = $13,
    tmdb_api_key = $14,
    ffmpeg_path = $15,
    ffprobe_path = $16,
    hardware_acceleration = $17,
    enable_transcoding = $18,
    jellyfin_token = $19,
    issuer = $20,
    audience = $21,
    secret = $22,
    cookie_domain = $23,
    cookie_path = $24,
    updated_at = NOW()
WHERE id = $25
RETURNING *;
