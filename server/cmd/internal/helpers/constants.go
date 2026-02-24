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

	// spotify
	SPOTIFY_ARTIST_MAX_CACHE = 100
	SPOTIFY_ALBUM_MAX_CACHE  = 200

	// auth keys
	COOKIE_USER_ID = "user_id"

	// error messages
	INTERNAL_SERVER_ERROR       = "The server encountered an unexpected error"
	NOT_AUTHORIZED_MESSAGE      = "not authorized"
	INVALID_CREDENTIALS_MESSAGE = "invalid email or password provided"

	// constants for tmdb
	TMDB_BASE_API_URL   = "https://api.themoviedb.org/3"
	TMDB_IMAGE_BASE_URL = "https://image.tmdb.org/t/p"
	TMDB_MAX_ITEMS      = 12
	// TMDB_YEAR_MATCH_SCORE is the score bonus for exact year matches in TMDB search results.
	// This ensures exact year matches are prioritized over popularity/vote average.
	TMDB_YEAR_MATCH_SCORE = 10000.0

	// HLS encoding profiles (allowed list for v1; validate client request against these).
	HLSProfile1080p8Mbps = "1080p_8mbps" // 1080p, 8 Mbps video, AAC audio
	HLSProfile1080p4Mbps = "1080p_4mbps" //
	// 1080p, 4 Mbps video, AAC audio
	HLSProfile720p3Mbps   = "720p_3mbps" // 720p, 3 Mbps video, AAC audio
	HLSSegmentTimeSeconds = 4
	HLS_PLAYLIST_VOD      = "vod"
	HLS_PLAYLIST_SIZE     = "vod"

	// HLS HTTP handler: polling and timeouts for manifest/segment readiness.
	HLS_MANIFEST_POLL_INTERVAL = 200 * time.Millisecond
	HLS_MANIFEST_POLL_TIMEOUT  = 60 * time.Second
	HLS_SEGMENT_WAIT_TIMEOUT   = 60 * time.Second
)
