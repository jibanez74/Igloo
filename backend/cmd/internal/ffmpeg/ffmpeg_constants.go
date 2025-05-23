package ffmpeg

const (
	DefaultHlsTime        = 6
	DefaultHlsListSize    = "0"
	DefaultSegmentType    = "fmp4"
	DefaultHlsFlags       = "independent_segments+program_date_time+append_list+discont_start+omit_endlist"
	DefaultPlaylistType   = "event"
	DefaultSegmentPattern = "segment_%d.m4s"
	DefaultPlaylistName   = "playlist.m3u8"
	DefaultInitFileName   = "init.mp4"

	AudioCodecCopy = "copy"
	AudioCodecAAC  = "aac"
	AudioCodecAC3  = "ac3"
	AudioCodecFLAC = "flac"
	AudioCodecMP3  = "mp3"

	VideoCodecCopy       = "copy"
	VideoCodecH264       = "h264"
	VideoCodecH265       = "h265"
	VideoPresetUltrafast = "ultrafast"
	VideoPresetSuperfast = "superfast"
	VideoPresetVeryfast  = "veryfast"
	VideoPresetFaster    = "faster"
	VideoPresetFast      = "fast"
	VideoPresetMedium    = "medium"
	VideoPresetSlow      = "slow"
	VideoPresetSlower    = "slower"
	VideoPresetVeryslow  = "veryslow"
	VideoProfileBaseline = "baseline"
	VideoProfileMain     = "main"
	VideoProfileHigh     = "high"
	VideoProfileHigh10   = "high10"

	VodPlaylistName = "vod.m3u8"
)
