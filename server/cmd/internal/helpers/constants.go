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
	SCANNER_BATCH_SIZE = 10

	// musicbrainz
	MUSICBRAINZ_ARTIST_MAX_CACHE = 200
	MUSICBRAINZ_ALBUM_MAX_CACHE  = 200
	MUSICBRAINZ_BASE_URL         = "https://musicbrainz.org/ws/2"
	MUSICBRAINZ_USER_AGENT       = "Igloo/1.0 (music media server)"
	MUSICBRAINZ_CACHE_TTL        = 2 * time.Hour
	COVER_ART_ARCHIVE_BASE_URL   = "https://coverartarchive.org/release-group"
	AUDIODB_BASE_URL             = "https://www.theaudiodb.com/api/v1/json"
	AUDIODB_API_KEY              = "2"

	// cookie settings
	COOKIE_USER_ID = "user_id"

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
	HLS_PROFILE_2160P_16MBPS = "2160p_16mbps"
	HLS_PROFILE_1080P_8MBPS  = "1080p_8mbps"
	HLS_PROFILE_1080P_6MBPS  = "1080p_6mbps"
	HLS_PROFILE_1080P_4MBPS  = "1080p_4mbps"
	HLS_PROFILE_720P_3MBPS   = "720p_3mbps"
)
