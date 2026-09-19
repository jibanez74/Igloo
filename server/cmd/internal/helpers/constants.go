package helpers

import "time"

// ENV_FILE is read by cmd/api at startup and by the tmdb integration tests,
// which cannot import cmd/api. It stays here for that second consumer.
const ENV_FILE = ".env"

// Hardware acceleration device identifiers accepted by transcoding settings.
const (
	HARDWARE_ACCELERATION_DEVICE_CPU    = "cpu"
	HARDWARE_ACCELERATION_DEVICE_APPLE  = "apple"
	HARDWARE_ACCELERATION_DEVICE_NVIDIA = "nvidia"
	HARDWARE_ACCELERATION_DEVICE_INTEL  = "intel"
)

// PROVIDER_HTTP_TIMEOUT bounds a single request to an external metadata provider.
// It is shared by the TMDB client and the YouTube thumbnail fetcher; Spotify keeps
// its own longer timeout.
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
