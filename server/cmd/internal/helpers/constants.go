package helpers

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

	// musicbrainz
	MUSICBRAINZ_ARTIST_MAX_CACHE = 100
	MUSICBRAINZ_ALBUM_MAX_CACHE  = 200

	// auth keys
	COOKIE_USER_ID              = "user_id"
	NOT_AUTHORIZED_MESSAGE      = "not authorized"
	INVALID_CREDENTIALS_MESSAGE = "invalid email or password provided"

	// error messages
	INTERNAL_SERVER_ERROR = "The server encountered an unexpected error"

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
	HLS_PROFILE_1080P_8MBPS = "1080p_8mbps"
	HLS_PROFILE_1080P_4MBPS = "1080p_4mbps"
	HLS_PROFILE_720P_3MBPS = "720p_3mbps"
)
