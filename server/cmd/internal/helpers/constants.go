package helpers

import "time"

// Process configuration and filesystem defaults used while bootstrapping the API.
const (
	ENV_FILE = ".env"

	DEFAULT_APP_PORT      = 8080
	DEFAULT_DB_PATH       = "db/igloo.db"
	DEFAULT_STATIC_DIR    = "static"
	DEFAULT_LOGS_DIR      = "logs"
	DEFAULT_TRANSCODE_DIR = "transcode"

	ENV_DB_PATH               = "DB_PATH"
	ENV_STATIC_DIR            = "STATIC_DIR"
	ENV_LOGS_DIR              = "LOGS_DIR"
	ENV_TRANSCODE_DIR         = "TRANSCODE_DIR"
	ENV_SESSION_COOKIE_SECURE = "SESSION_COOKIE_SECURE"
	ENV_LOG_TO_STDOUT         = "LOG_TO_STDOUT"
	ENV_PORT                  = "PORT"
)

// Auth cookie names and the bootstrap admin used when no admin exists yet.
const (
	COOKIE_USER_ID = "user_id"

	DEFAULT_ADMIN_NAME     = "Admin"
	DEFAULT_ADMIN_EMAIL    = "admin@sample.com"
	DEFAULT_ADMIN_PASSWORD = "AdminPassword"

	ENV_DEFAULT_ADMIN_NAME     = "DEFAULT_ADMIN_NAME"
	ENV_DEFAULT_ADMIN_EMAIL    = "DEFAULT_ADMIN_EMAIL"
	ENV_DEFAULT_ADMIN_PASSWORD = "DEFAULT_ADMIN_PASSWORD"
)

// Shared API messages kept consistent across handlers.
const (
	INTERNAL_SERVER_ERROR       = "The server encountered an unexpected error"
	NOT_AUTHORIZED_MESSAGE      = "not authorized"
	INVALID_CREDENTIALS_MESSAGE = "invalid email or password provided"
)

// Hardware acceleration device identifiers accepted by transcoding settings.
const (
	HARDWARE_ACCELERATION_DEVICE_CPU    = "cpu"
	HARDWARE_ACCELERATION_DEVICE_APPLE  = "apple"
	HARDWARE_ACCELERATION_DEVICE_NVIDIA = "nvidia"
	HARDWARE_ACCELERATION_DEVICE_INTEL  = "intel"
)

// Media scanner and library pagination limits keep scans and list responses bounded.
const (
	SCANNER_BATCH_SIZE = 54

	MOVIES_LIBRARY_DEFAULT_PER_PAGE = 24
	MOVIES_LIBRARY_MAX_PER_PAGE     = 48
)

// Library search limits are shared by FTS-backed search and per-entity search endpoints.
const (
	SEARCH_DEFAULT_PER_PAGE = 24
	SEARCH_MAX_PER_PAGE     = 48
	SEARCH_ALL_TOP_N        = 8
)

// Playlist constants cover the unified playlist table discriminator and request limits.
const (
	PLAYLIST_CONTENT_TYPE_TRACK = "track"
	PLAYLIST_CONTENT_TYPE_MOVIE = "movie"

	MAX_PLAYLIST_REQUEST_SIZE = 1024 * 1024 // 1 MB
)

// Notification title values are persisted in notifications.title.
const (
	NOTIFICATION_TITLE_MOVIE_REQUEST = "movie_request"
	NOTIFICATION_TITLE_ALBUM_REQUEST = "album_request"
	NOTIFICATION_TITLE_TRACK_REQUEST = "track_request"
	NOTIFICATION_TITLE_OTHER         = "other"
)

// TMDB image sizing and shared HTTP settings used by metadata enrichment.
const (
	TMDB_IMAGE_BASE_URL = "https://image.tmdb.org/t/p"
	TMDB_IMAGE_SIZE     = "original"
	TMDB_BACKDROP_SIZE  = "w1280"
	TMDB_POSTER_SIZE    = "w500"
	TMDB_PROFILE_SIZE   = "w185"
	TMDB_LOGO_SIZE      = "w92"
	TMDB_MAX_ITEMS      = 12
	TMDB_HTTP_TIMEOUT   = 10 * time.Second
	// TMDB_YEAR_MATCH_SCORE is the score bonus for exact year matches in TMDB search results.
	// This ensures exact year matches are prioritized over popularity/vote average.
	TMDB_YEAR_MATCH_SCORE = 10000.0
)

// HLS profile identifiers are URL-visible values accepted by hls_profiles.go.
const (
	HLS_PROFILE_REMUX        = "remux"
	HLS_PROFILE_2160P_16MBPS = "2160p_16mbps"
	HLS_PROFILE_1080P_8MBPS  = "1080p_8mbps"
	HLS_PROFILE_1080P_6MBPS  = "1080p_6mbps"
	HLS_PROFILE_1080P_4MBPS  = "1080p_4mbps"
	HLS_PROFILE_720P_3MBPS   = "720p_3mbps"
)

// HLS segment generation and remux validation settings.
const (
	HLS_SEGMENT_TIME_SEC = 4 // segment duration in seconds for fMP4 HLS

	// HLS_COPY_VIDEO_TARGET_DURATION is the TARGETDURATION ceiling used when FFmpeg
	// runs in -c:v copy mode. Copy mode splits only at keyframe boundaries, so
	// segments can far exceed HLS_SEGMENT_TIME_SEC. 30s covers all practical GOPs.
	HLS_COPY_VIDEO_TARGET_DURATION = 30

	// HLS remux preflight checks the first few complete segments before the
	// manifest is served so copied H.264 streams can fall back to transcode
	// before playback begins.
	HLS_REMUX_PREVALIDATE_SEGMENTS = 4
	HLS_REMUX_PREVALIDATE_TIMEOUT  = 30 * time.Second
	HLS_REMUX_SAFETY_CACHE_TTL     = 24 * time.Hour
	HLS_REMUX_SAFETY_CACHE_SWEEP   = 1 * time.Hour
)

// HLS concurrency settings keep local transcodes predictable.
const (
	ENV_HLS_MAX_CPU_TRANSCODES         = "HLS_MAX_CPU_TRANSCODES"
	HLS_CPU_TRANSCODE_DEFAULT_DIVISOR  = 4
	HLS_TRANSCODE_BUSY_RETRY_AFTER_SEC = 5
)

// HDR transfer characteristics reported by ffprobe.
const (
	HDR_TRANSFER_PQ  = "smpte2084"    // HDR10 (Perceptual Quantizer / SMPTE ST 2084)
	HDR_TRANSFER_HLG = "arib-std-b67" // HLG  (Hybrid Log-Gamma / ARIB STD-B67)
)

// HLS session cache and playback-session request validation.
const (
	HLS_PLAYBACK_SESSION_ID_PATTERN = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	HLS_SESSION_TTL                 = 30 * time.Minute // TTL for cached HLS session entries
	HLS_SESSION_CACHE_SWEEP         = 10 * time.Minute // interval for removing expired HLS session entries
)

// HLS HTTP serving constants for manifest polling, content types, and FFmpeg fMP4 filenames.
const (
	HLS_SEGMENT_WAIT              = 120 * time.Second
	HLS_SEGMENT_POLL              = 250 * time.Millisecond
	HLS_PLAYLIST_CONTENT_TYPE     = "application/vnd.apple.mpegurl"
	HLS_SEGMENT_HTTP_CONTENT_TYPE = "video/mp4"
	HLS_INIT_FILENAME             = "init.mp4"
	HLS_SEGMENT_FILENAME_PREFIX   = "segment_"
	HLS_SEGMENT_FILENAME_SUFFIX   = ".m4s"
)

// Watch-room playback modes and request limits for shared viewing sessions.
const (
	WATCH_ROOM_PLAYBACK_MODE_DIRECT = "direct"
	MAX_WATCH_ROOM_REQUEST_SIZE     = 1024 * 1024 // 1 MB
)

// Watch progress thresholds define when playback progress counts as completed.
const (
	WATCH_COMPLETION_THRESHOLD = 0.98
)

// Subtitle extraction and cache settings for generated WebVTT subtitle tracks.
const (
	SUBTITLE_WEBVTT_CONTENT_TYPE = "text/vtt"
	SUBTITLE_EXTRACT_TIMEOUT     = 60 * time.Second
	SUBTITLE_CACHE_TTL           = 1 * time.Hour
	SUBTITLE_CACHE_CLEANUP       = 10 * time.Minute
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
