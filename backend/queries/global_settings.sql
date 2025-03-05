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
    download_images,
    tmdb_api_key,
    ffmpeg_path,
    ffprobe_path,
    hardware_acceleration,
    jellyfin_token
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;
