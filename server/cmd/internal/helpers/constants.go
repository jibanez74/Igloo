package helpers

import "time"

const (
	// logger
	LOGGER_MAX_LINES = 500

	// hardware acceleration
	HARDWARE_ACCELERATION_DEVICE_CPU    = "cpu"
	HARDWARE_ACCELERATION_DEVICE_APPLE  = "apple"
	HARDWARE_ACCELERATION_DEVICE_NVIDIA = "nvidia"
	HARDWARE_ACCELERATION_DEVICE_INTEL  = "intel"

	// media scanner
	SCANNER_BATCH_SIZE           = 54
	MOVIE_RENAME_MATCH_THRESHOLD = 50.0
	MOVIE_RENAME_TMDB_ID_SCORE   = 40.0
	MOVIE_RENAME_TITLE_SCORE     = 18.0
	MOVIE_RENAME_YEAR_SCORE      = 10.0
	MOVIE_RENAME_SIZE_SCORE      = 14.0
	MOVIE_RENAME_DURATION_SCORE  = 12.0

	// playlists — content_type discriminator (movies page / unified playlists table)
	PLAYLIST_CONTENT_TYPE_TRACK = "track"
	PLAYLIST_CONTENT_TYPE_MOVIE = "movie"
	// max JSON body size for playlist create/update/add-movies requests
	MAX_PLAYLIST_REQUEST_SIZE = 1024 * 1024 // 1MB

	// movies library API (paginated list; align with music musicians defaults)
	MOVIES_LIBRARY_DEFAULT_PER_PAGE = 24
	MOVIES_LIBRARY_MAX_PER_PAGE     = 48

	// library search (FTS5-backed; used by /api/search and per-entity search endpoints)
	SEARCH_DEFAULT_PER_PAGE = 24
	SEARCH_MAX_PER_PAGE     = 48
	SEARCH_ALL_TOP_N        = 8

	// cookie settings
	COOKIE_USER_ID = "user_id"

	// default admin bootstrap (InitDefaultUser when the database has no admin)
	DEFAULT_ADMIN_NAME     = "Admin"
	DEFAULT_ADMIN_EMAIL    = "admin@sample.com"
	DEFAULT_ADMIN_PASSWORD = "AdminPassword"

	ENV_DEFAULT_ADMIN_NAME     = "DEFAULT_ADMIN_NAME"
	ENV_DEFAULT_ADMIN_EMAIL    = "DEFAULT_ADMIN_EMAIL"
	ENV_DEFAULT_ADMIN_PASSWORD = "DEFAULT_ADMIN_PASSWORD"

	// app startup defaults (used by cmd/api/main.go for InitDB / InitLogger / InitSettings)
	DEFAULT_APP_PORT      = 8080
	DEFAULT_DATA_DIR      = "./data"
	DEFAULT_DB_PATH       = "data/igloo.db"
	DEFAULT_STATIC_DIR    = "data/static"
	DEFAULT_LOGS_DIR      = "data/logs"
	DEFAULT_TRANSCODE_DIR = "data/transcode"
	DEFAULT_MOVIES_DIR    = "/media/movies"
	DEFAULT_SHOWS_DIR     = "/media/shows"
	DEFAULT_MUSIC_DIR     = "/media/music"

	// env vars consumed at startup
	ENV_IGLOO_ENV_FILE        = "IGLOO_ENV_FILE"
	ENV_IGLOO_DATA_DIR        = "IGLOO_DATA_DIR"
	ENV_DB_PATH               = "DB_PATH"
	ENV_STATIC_DIR            = "STATIC_DIR"
	ENV_LOGS_DIR              = "LOGS_DIR"
	ENV_TRANSCODE_DIR         = "TRANSCODE_DIR"
	ENV_SESSION_COOKIE_SECURE = "SESSION_COOKIE_SECURE"
	ENV_LOG_TO_STDOUT         = "LOG_TO_STDOUT"
	ENV_PORT                  = "PORT"

	// error messages
	INTERNAL_SERVER_ERROR       = "The server encountered an unexpected error"
	NOT_AUTHORIZED_MESSAGE      = "not authorized"
	INVALID_CREDENTIALS_MESSAGE = "invalid email or password provided"

	// constants for tmdb
	TMDB_BASE_API_URL          = "https://api.themoviedb.org/3"
	TMDB_IMAGE_BASE_URL        = "https://image.tmdb.org/t/p"
	TMDB_IMAGE_SIZE            = "original"
	TMDB_POSTER_SIZE           = "w500"
	TMDB_PROFILE_SIZE          = "w185"
	TMDB_LOGO_SIZE             = "w92"
	TMDB_MAX_ITEMS             = 12
	TMDB_HTTP_TIMEOUT          = 10 * time.Second
	TMDB_HTTP_MAX_RETRIES      = 3
	TMDB_HTTP_RETRY_BASE_DELAY = 250 * time.Millisecond
	TMDB_HTTP_RETRY_MAX_DELAY  = 2 * time.Second
	// TMDB_YEAR_MATCH_SCORE is the score bonus for exact year matches in TMDB search results.
	// This ensures exact year matches are prioritized over popularity/vote average.
	TMDB_YEAR_MATCH_SCORE = 10000.0

	// HLS encoding profile IDs (allowed list in hls_profiles.go; used in URLs and validation).
	HLS_PROFILE_REMUX        = "remux"
	HLS_PROFILE_2160P_16MBPS = "2160p_16mbps"
	HLS_PROFILE_1080P_8MBPS  = "1080p_8mbps"
	HLS_PROFILE_1080P_6MBPS  = "1080p_6mbps"
	HLS_PROFILE_1080P_4MBPS  = "1080p_4mbps"
	HLS_PROFILE_720P_3MBPS   = "720p_3mbps"

	// HLS transcoding
	HLS_SEGMENT_TIME_SEC = 4 // segment duration in seconds for fMP4 HLS
	// HLS_COPY_VIDEO_TARGET_DURATION is the TARGETDURATION ceiling used when FFmpeg
	// runs in -c:v copy mode. Copy mode splits only at keyframe boundaries, so
	// segments can far exceed HLS_SEGMENT_TIME_SEC. 30s covers all practical GOPs.
	HLS_COPY_VIDEO_TARGET_DURATION = 30
	// HLS remux preflight checks the first few complete segments before the
	// manifest is served so copied H.264 streams can fall back to transcode
	// before playback begins.
	HLS_REMUX_PREVALIDATE_SEGMENTS     = 4
	HLS_REMUX_PREVALIDATE_TIMEOUT      = 30 * time.Second
	HLS_REMUX_SAFETY_CACHE_TTL         = 24 * time.Hour
	HLS_REMUX_SAFETY_CACHE_SWEEP       = 1 * time.Hour
	ENV_HLS_MAX_CPU_TRANSCODES         = "HLS_MAX_CPU_TRANSCODES"
	HLS_CPU_TRANSCODE_DEFAULT_DIVISOR  = 4
	HLS_TRANSCODE_BUSY_RETRY_AFTER_SEC = 5
	HLS_PLAYBACK_SESSION_ID_PATTERN    = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`

	// HDR transfer characteristics as reported by ffprobe (color_transfer field).
	// Used to detect HDR sources that require tone-mapping when transcoding to SDR.
	HDR_TRANSFER_PQ                = "smpte2084"    // HDR10 (Perceptual Quantizer / SMPTE ST 2084)
	HDR_TRANSFER_HLG               = "arib-std-b67" // HLG  (Hybrid Log-Gamma / ARIB STD-B67)
	HLS_STDERR_TAIL_LINES          = 20             // lines of FFmpeg stderr kept for error reporting
	HLS_STDERR_SCANNER_BUFFER_SIZE = 64 * 1024
	HLS_STDERR_SCANNER_MAX_TOKEN   = 1024 * 1024
	HLS_SESSION_TTL                = 30 * time.Minute // TTL for cached HLS session entries
	HLS_SESSION_CACHE_SWEEP        = 10 * time.Minute // interval for removing expired HLS session entries

	// HLS HTTP: manifest polling, response headers, and fMP4 filenames (match FFmpeg output)
	HLS_SEGMENT_WAIT              = 120 * time.Second
	HLS_SEGMENT_POLL              = 250 * time.Millisecond
	HLS_PLAYLIST_CONTENT_TYPE     = "application/vnd.apple.mpegurl"
	HLS_SEGMENT_HTTP_CONTENT_TYPE = "video/mp4"
	HLS_INIT_FILENAME             = "init.mp4"
	HLS_SEGMENT_FILENAME_PREFIX   = "segment_"
	HLS_SEGMENT_FILENAME_SUFFIX   = ".m4s"
	HLS_SDR_COLOR_PARAMS          = "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709"

	// Watch rooms
	WATCH_ROOM_PLAYBACK_MODE_DIRECT = "direct"
	MAX_WATCH_ROOM_REQUEST_SIZE     = 1024 * 1024 // 1 MB

	// Watch progress
	WATCH_COMPLETION_THRESHOLD = 0.98

	// Subtitle extraction
	SUBTITLE_WEBVTT_CONTENT_TYPE = "text/vtt"
	SUBTITLE_EXTRACT_TIMEOUT     = 60 * time.Second
	SUBTITLE_CACHE_TTL           = 1 * time.Hour
	SUBTITLE_CACHE_CLEANUP       = 10 * time.Minute
	SUBTITLE_CACHE_KEY_PREFIX    = "sub:"
)

// Bitmap subtitle codecs cannot be converted to WebVTT.
var BitmapSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvb_subtitle":      true,
}

// CoverArtVideoCodecs lists embedded still-image video tracks (posters) to skip for HLS/direct playback logic.
var CoverArtVideoCodecs = map[string]bool{
	"mjpeg": true,
	"png":   true,
	"gif":   true,
	"bmp":   true,
}
