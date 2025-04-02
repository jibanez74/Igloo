package ffprobe

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"os/exec"
	"time"
)

type Ffprobe interface {
	GetMovieMetadata(filePath *string) (*movieMetadataResult, error)
	ExtractKeyframes(filePath string) (*KeyframeData, error)
	ComputeSegments(*KeyframeData, time.Duration) []time.Duration
	processStreams(streams []mediaStream) ([]database.CreateVideoStreamParams, []database.CreateAudioStreamParams, []database.CreateSubtitleParams)
	processVideoStream(s mediaStream) database.CreateVideoStreamParams
	processAudioStream(s mediaStream) database.CreateAudioStreamParams
	processSubtitleStream(s mediaStream) database.CreateSubtitleParams
	processChapters(chapters []tmdbChapter) []database.CreateChapterParams
}

type tags struct {
	Title    string `json:"title,omitempty"`
	Language string `json:"language,omitempty"`
}

type disposition struct {
	Default         json.RawMessage `json:"default"`
	Dub             json.RawMessage `json:"dub"`
	Original        json.RawMessage `json:"original"`
	Comment         json.RawMessage `json:"comment"`
	Lyrics          json.RawMessage `json:"lyrics"`
	Karaoke         json.RawMessage `json:"karaoke"`
	Forced          json.RawMessage `json:"forced"`
	HearingImpaired json.RawMessage `json:"hearing_impaired"`
	VisualImpaired  json.RawMessage `json:"visual_impaired"`
	CleanEffects    json.RawMessage `json:"clean_effects"`
	AttachedPic     json.RawMessage `json:"attached_pic"`
	TimedThumbnails json.RawMessage `json:"timed_thumbnails"`
	NonDiegetic     json.RawMessage `json:"non_diegetic"`
	Captions        json.RawMessage `json:"captions"`
	Descriptions    json.RawMessage `json:"descriptions"`
	Metadata        json.RawMessage `json:"metadata"`
	Dependent       json.RawMessage `json:"dependent"`
	StillImage      json.RawMessage `json:"still_image"`
	Multilayer      json.RawMessage `json:"multilayer"`
}

func (d *disposition) GetInt(field json.RawMessage) int {
	if len(field) == 0 {
		return 0
	}
	if string(field) == "false" {
		return 0
	}
	if string(field) == "true" {
		return 1
	}
	var val int
	if err := json.Unmarshal(field, &val); err != nil {
		return 0
	}
	return val
}

type mediaStream struct {
	Index            int             `json:"index"`
	CodecName        string          `json:"codec_name,omitempty"`
	CodecLongName    string          `json:"codec_long_name,omitempty"`
	CodecType        string          `json:"codec_type,omitempty"`
	Profile          string          `json:"profile,omitempty"`
	Height           uint            `json:"height,omitempty"`
	Width            uint            `json:"width,omitempty"`
	CodedHeight      uint            `json:"coded_height,omitempty"`
	CodedWidth       uint            `json:"coded_width,omitempty"`
	AspectRatio      string          `json:"display_aspect_ratio,omitempty"`
	Level            int             `json:"level,omitempty"`
	AvgFrameRate     string          `json:"avg_frame_rate,omitempty"`
	FrameRate        string          `json:"r_frame_rate,omitempty"`
	BitDepth         string          `json:"bits_per_raw_sample,omitempty"`
	BitRate          string          `json:"bit_rate,omitempty"`
	ColorRange       string          `json:"color_range,omitempty"`
	ColorTransfer    string          `json:"color_transfer,omitempty"`
	ColorPrimaries   string          `json:"color_primaries,omitempty"`
	ColorSpace       string          `json:"color_space,omitempty"`
	Channels         int             `json:"channels,omitempty"`
	ChannelLayout    string          `json:"channel_layout,omitempty"`
	Tags             tags            `json:"tags,omitempty"`
	Disposition      *disposition    `json:"disposition"`
	HasBFrames       json.RawMessage `json:"has_b_frames,omitempty"`
	IsAvc            json.RawMessage `json:"is_avc,omitempty"`
	NalLengthSize    json.RawMessage `json:"nal_length_size,omitempty"`
	RFrameRate       json.RawMessage `json:"r_frame_rate,omitempty"`
	StartPts         json.RawMessage `json:"start_pts,omitempty"`
	StartTime        json.RawMessage `json:"start_time,omitempty"`
	Duration         json.RawMessage `json:"duration,omitempty"`
	BitRateRaw       json.RawMessage `json:"bit_rate,omitempty"`
	BitsPerSample    json.RawMessage `json:"bits_per_sample,omitempty"`
	BitsPerRawSample json.RawMessage `json:"bits_per_raw_sample,omitempty"`
	NbFrames         json.RawMessage `json:"nb_frames,omitempty"`
	ExtradataSize    json.RawMessage `json:"extradata_size,omitempty"`
}

type format struct {
	Filename       string          `json:"filename"`
	BitRate        string          `json:"bit_rate,omitempty"`
	Size           string          `json:"size,omitempty"`
	Duration       string          `json:"duration,omitempty"`
	FormatName     string          `json:"format_name,omitempty"`
	FormatLongName string          `json:"format_long_name,omitempty"`
	ProbeScore     json.RawMessage `json:"probe_score,omitempty"`
	Tags           json.RawMessage `json:"tags,omitempty"`
}

type tmdbChapter struct {
	Start     int    `json:"start"`
	StartTime string `json:"start_time"`
	End       int    `json:"end"`
	EndTime   string `json:"end_time"`
	Title     string `json:"title"`
}

type result struct {
	Streams  []mediaStream   `json:"streams"`
	Chapters []tmdbChapter   `json:"chapters"`
	Format   format          `json:"format"`
	Programs json.RawMessage `json:"programs,omitempty"`
}

type movieMetadataResult struct {
	VideoList    []database.CreateVideoStreamParams
	AudioList    []database.CreateAudioStreamParams
	SubtitleList []database.CreateSubtitleParams
	ChapterList  []database.CreateChapterParams
}

type ffprobe struct {
	bin string
}

func New(s *database.GlobalSetting) (Ffprobe, error) {
	var f ffprobe

	if s == nil {
		return nil, errors.New("provided nil value for settings in ffprobe package")
	}

	if s.FfprobePath == "" {
		return nil, errors.New("ffprobe path is empty")
	}

	path, err := exec.LookPath(s.FfprobePath)
	if err != nil {
		return nil, fmt.Errorf("unable to find ffprobe at %s: %w", s.FfprobePath, err)
	}

	_, err = exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe not found or not executable at %s: %w", path, err)
	}

	f.bin = s.FfprobePath

	return &f, nil
}
