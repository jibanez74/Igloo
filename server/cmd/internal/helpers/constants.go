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
	SCANNER_BATCH_SIZE = 54

	// spotify constants
	SPOTIFY_ARTIST_MAX_CACHE = 200
	SPOTIFY_ALBUM_MAX_CACHE  = 200

	// playlists — content_type discriminator (movies page / unified playlists table)
	PLAYLIST_CONTENT_TYPE_TRACK = "track"
	PLAYLIST_CONTENT_TYPE_MOVIE = "movie"
	// max JSON body size for playlist create/update/add-movies requests
	MAX_PLAYLIST_REQUEST_SIZE = 1024 * 1024 // 1MB

	// movies library API (paginated list; align with music musicians defaults)
	MOVIES_LIBRARY_DEFAULT_PER_PAGE = 24
	MOVIES_LIBRARY_MAX_PER_PAGE     = 48

	// cookie settings
	COOKIE_USER_ID = "user_id"

	// default admin bootstrap (InitDefaultUser when the database has no admin)
	DEFAULT_ADMIN_NAME     = "Admin"
	DEFAULT_ADMIN_EMAIL    = "admin@sample.com"
	DEFAULT_ADMIN_PASSWORD = "AdminPassword"

	ENV_DEFAULT_ADMIN_NAME     = "DEFAULT_ADMIN_NAME"
	ENV_DEFAULT_ADMIN_EMAIL    = "DEFAULT_ADMIN_EMAIL"
	ENV_DEFAULT_ADMIN_PASSWORD = "DEFAULT_ADMIN_PASSWORD"

	// error messages
	INTERNAL_SERVER_ERROR       = "The server encountered an unexpected error"
	NOT_AUTHORIZED_MESSAGE      = "not authorized"
	INVALID_CREDENTIALS_MESSAGE = "invalid email or password provided"

	// constants for tmdb
	TMDB_BASE_API_URL   = "https://api.themoviedb.org/3"
	TMDB_IMAGE_BASE_URL = "https://image.tmdb.org/t/p"
	TMDB_IMAGE_SIZE     = "original"
	TMDB_POSTER_SIZE    = "w500"
	TMDB_PROFILE_SIZE   = "w185"
	TMDB_LOGO_SIZE      = "w92"
	TMDB_MAX_ITEMS      = 12
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
	HLS_SEGMENT_TIME_SEC    = 4                // segment duration in seconds for fMP4 HLS
	HLS_STDERR_TAIL_LINES   = 20               // lines of FFmpeg stderr kept for error reporting
	HLS_SESSION_TTL         = 30 * time.Minute // TTL for cached HLS session entries
	HLS_SESSION_CACHE_SWEEP = 10 * time.Minute // interval for removing expired HLS session entries

	// HLS HTTP: manifest polling, response headers, and fMP4 filenames (match FFmpeg output)
	HLS_SEGMENT_WAIT              = 120 * time.Second
	HLS_SEGMENT_POLL              = 250 * time.Millisecond
	HLS_PLAYLIST_CONTENT_TYPE     = "application/vnd.apple.mpegurl"
	HLS_SEGMENT_HTTP_CONTENT_TYPE = "video/mp4"
	HLS_INIT_FILENAME             = "init.mp4"
	HLS_SEGMENT_FILENAME_PREFIX   = "segment_"
	HLS_SEGMENT_FILENAME_SUFFIX   = ".m4s"

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
