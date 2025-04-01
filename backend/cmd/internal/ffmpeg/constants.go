package ffmpeg

const (
	DefaultHlsTime        = "6"
	DefaultHlsListSize    = "0"
	DefaultSegmentType    = "fmp4"
	DefaultHlsFlags       = "independent_segments+program_date_time+append_list+discont_start"
	DefaultPlaylistType   = "vod"
	DefaultSegmentPattern = "segment_%d.m4s"
	DefaultPlaylistName   = "playlist.m3u8"
	DefaultMasterPlaylist = "master.m3u8"
	DefaultInitFileName   = "init.mp4"

	HlsTagExtM3U              = "#EXTM3U"
	HlsTagVersion             = "#EXT-X-VERSION:7"
	HlsTagTargetDuration      = "#EXT-X-TARGETDURATION:%s"
	HlsTagMediaSequence       = "#EXT-X-MEDIA-SEQUENCE:0"
	HlsTagPlaylistType        = "#EXT-X-PLAYLIST-TYPE:%s"
	HlsTagIndependentSegments = "#EXT-X-INDEPENDENT-SEGMENTS"
	HlsTagProgramDateTime     = "#EXT-X-PROGRAM-DATE-TIME:%s"
	HlsTagMap                 = "#EXT-X-MAP:URI=\"%s\",BANDWIDTH=0"
	HlsTagStreamInf           = "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",FRAME-RATE=%s"
	HlsTagStreamInfAudio      = "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",FRAME-RATE=%s,AUDIO=\"audio\""
	HlsTagMedia               = "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",DEFAULT=YES,URI=\"%s\",LANGUAGE=\"%s\",CODECS=\"%s\",CHANNELS=\"%d\""
	HlsTagEndList             = "#EXT-X-ENDLIST"

	DefaultAudioRate     = 128
	DefaultAudioChannels = 2
	DefaultAudioCodec    = "aac"
	AudioCodecCopy       = "copy"
	AudioCodecAAC        = "aac"
	AudioCodecAC3        = "ac3"
	AudioCodecFLAC       = "flac"
	AudioCodecMP3        = "mp3"

	DefaultVideoCodec    = "libx264"
	DefaultVideoHeight   = 480
	DefaultVideoRate     = 1000
	DefaultPreset        = "fast"
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
)
