package ffprobe

import (
	"igloo/cmd/internal/database"
)

type FfprobeInterface interface {
	GetMovieMetadata(filePath *string) (*movieMetadataResult, error)
	GetTrackMetadata(trackPath string) (*database.CreateTrackParams, []string, error)
	processStreams(streams []MediaStream) ([]database.CreateVideoStreamParams, []database.CreateAudioStreamParams, []database.CreateSubtitleParams)
	processVideoStream(s MediaStream) database.CreateVideoStreamParams
	processAudioStream(s MediaStream) database.CreateAudioStreamParams
	processSubtitleStream(s MediaStream) database.CreateSubtitleParams
	processChapters(chapters []tmdbChapter) []database.CreateChapterParams
}

type Tags struct {
	Title    string `json:"title,omitempty"`
	Language string `json:"language,omitempty"`
}

type TrackTags struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	Track     string `json:"track"`
	Genre     string `json:"genre"`
	Composer  string `json:"composer"`
	Date      string `json:"date"`
	Copyright string `json:"copyright"`
}

type MediaStream struct {
	Index          int    `json:"index"`
	CodecName      string `json:"codec_name,omitempty"`
	CodecLongName  string `json:"codec_long_name,omitempty"`
	CodecType      string `json:"codec_type,omitempty"`
	Profile        string `json:"profile,omitempty"`
	Height         uint   `json:"height,omitempty"`
	Width          uint   `json:"width,omitempty"`
	CodedHeight    uint   `json:"coded_height,omitempty"`
	CodedWidth     uint   `json:"coded_width,omitempty"`
	AspectRatio    string `json:"display_aspect_ratio,omitempty"`
	Level          int    `json:"level,omitempty"`
	AvgFrameRate   string `json:"avg_frame_rate,omitempty"`
	FrameRate      string `json:"r_frame_rate,omitempty"`
	BitDepth       string `json:"bits_per_raw_sample,omitempty"`
	BitRate        string `json:"bit_rate,omitempty"`
	SampleRate     string `json:"sample_rate,omitempty"`
	ColorRange     string `json:"color_range,omitempty"`
	ColorTransfer  string `json:"color_transfer,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ColorSpace     string `json:"color_space,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	ChannelLayout  string `json:"channel_layout,omitempty"`
	Tags           Tags   `json:"tags,omitempty"`
}

type format struct {
	Filename       string    `json:"filename"`
	BitRate        string    `json:"bit_rate,omitempty"`
	Size           string    `json:"size,omitempty"`
	Duration       string    `json:"duration,omitempty"`
	FormatName     string    `json:"format_name,omitempty"`
	FormatLongName string    `json:"format_long_name,omitempty"`
	Tags           TrackTags `json:"tags"`
}

type tmdbChapter struct {
	Start     int    `json:"start"`
	StartTime string `json:"start_time"`
	End       int    `json:"end"`
	EndTime   string `json:"end_time"`
	Title     string `json:"title"`
}

type Result struct {
	Streams  []MediaStream `json:"streams"`
	Chapters []tmdbChapter `json:"chapters"`
	Format   format        `json:"format"`
}

type movieMetadataResult struct {
	VideoList    []database.CreateVideoStreamParams
	AudioList    []database.CreateAudioStreamParams
	SubtitleList []database.CreateSubtitleParams
	ChapterList  []database.CreateChapterParams
}

type Ffprobe struct {
	bin string
}
