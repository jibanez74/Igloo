package helpers

import "time"

// Hardware acceleration device identifiers accepted by transcoding settings.
const (
	HARDWARE_ACCELERATION_DEVICE_CPU    = "cpu"
	HARDWARE_ACCELERATION_DEVICE_APPLE  = "apple"
	HARDWARE_ACCELERATION_DEVICE_NVIDIA = "nvidia"
	HARDWARE_ACCELERATION_DEVICE_INTEL  = "intel"
)

// PROVIDER_HTTP_TIMEOUT bounds a single request to an external metadata provider.
// It is shared by the TMDB client, the iTunes lookup client and the YouTube
// thumbnail fetcher; Spotify keeps its own longer timeout.
const PROVIDER_HTTP_TIMEOUT = 10 * time.Second

// HLS segment generation and remux validation settings shared across packages.
const (
	HLS_SEGMENT_TIME_SEC = 4 // segment duration in seconds for fMP4 HLS

	// HLS_REMUX_PREVALIDATE_SEGMENTS is driven from cmd/api but is also what
	// ffmpeg's remux validator tests validate against, so it is shared.
	HLS_REMUX_PREVALIDATE_SEGMENTS = 4
)

// HLS output filenames are shared by FFmpeg, API handlers, and media fixtures.
const (
	HLS_INIT_FILENAME           = "init.mp4"
	HLS_PLAYLIST_FILENAME       = "playlist.m3u8"
	HLS_SEGMENT_FILENAME_PREFIX = "segment_"
	HLS_SEGMENT_FILENAME_SUFFIX = ".m4s"
)

// TMDB image sizes. The image proxy accepts exactly these, and the artwork
// syncer downloads each kind of image at the size the clients request it.
const (
	TMDB_IMAGE_BASE_URL      = "https://image.tmdb.org/t/p"
	TMDB_ORIGINAL_IMAGE_SIZE = "original"
	TMDB_BACKDROP_SIZE       = "w1280"
	TMDB_POSTER_SIZE         = "w500"
	TMDB_PROFILE_SIZE        = "w185"
	TMDB_LOGO_SIZE           = "w92"
	// TMDB_STILL_SIZE is the poster size, which the clients show stills at.
	TMDB_STILL_SIZE = TMDB_POSTER_SIZE
)

// STATIC_URL_PREFIX is the URL path the static directory is served under.
const STATIC_URL_PREFIX = "/api/static/"

// TEMP_FILE_PREFIX starts the name of every temporary file WriteFileAtomic
// creates beside its target, so one a crash left behind is recognizable.
const TEMP_FILE_PREFIX = ".tmp-"
