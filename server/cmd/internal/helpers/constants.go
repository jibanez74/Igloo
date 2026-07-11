package helpers

import "time"

const ENV_FILE = ".env"

// Hardware acceleration device identifiers accepted by transcoding settings.
const (
	HARDWARE_ACCELERATION_DEVICE_CPU    = "cpu"
	HARDWARE_ACCELERATION_DEVICE_APPLE  = "apple"
	HARDWARE_ACCELERATION_DEVICE_NVIDIA = "nvidia"
	HARDWARE_ACCELERATION_DEVICE_INTEL  = "intel"
)

const TMDB_HTTP_TIMEOUT = 10 * time.Second

// HLS profile identifiers are URL-visible values accepted by hls_profiles.go.
const (
	HLS_PROFILE_REMUX        = "remux"
	HLS_PROFILE_2160P_16MBPS = "2160p_16mbps"
	HLS_PROFILE_1080P_8MBPS  = "1080p_8mbps"
	HLS_PROFILE_1080P_6MBPS  = "1080p_6mbps"
	HLS_PROFILE_1080P_4MBPS  = "1080p_4mbps"
	HLS_PROFILE_720P_3MBPS   = "720p_3mbps"
)

// HLS segment generation and remux validation settings shared across packages.
const (
	HLS_SEGMENT_TIME_SEC           = 4 // segment duration in seconds for fMP4 HLS
	HLS_REMUX_PREVALIDATE_SEGMENTS = 4
)

// HLS output filenames are shared by FFmpeg, API handlers, and media fixtures.
const (
	HLS_INIT_FILENAME           = "init.mp4"
	HLS_PLAYLIST_FILENAME       = "playlist.m3u8"
	HLS_SEGMENT_FILENAME_PREFIX = "segment_"
	HLS_SEGMENT_FILENAME_SUFFIX = ".m4s"
)
