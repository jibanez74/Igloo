package ffprobe

import (
	"encoding/json"
	"errors"
	"igloo/cmd/internal/database"
	"os/exec"
)

func (f *ffprobe) GetMovieMetadata(filePath *string) (*movieMetadataResult, error) {
	if filePath == nil {
		return nil, errors.New("file path is required")
	}

	cmd := exec.Command(f.bin, "-i", *filePath,
		"-show_streams",
		"-show_chapters",
		"-show_format",
		"-print_format", "json")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var probeResult result
	if err := json.Unmarshal(output, &probeResult); err != nil {
		return nil, err
	}

	if len(probeResult.Streams) == 0 {
		return nil, errors.New("no streams found")
	}

	// Initialize result with empty slices
	result := &movieMetadataResult{
		VideoList:    make([]database.CreateVideoStreamParams, 0),
		AudioList:    make([]database.CreateAudioStreamParams, 0),
		SubtitleList: make([]database.CreateSubtitleParams, 0),
		ChapterList:  make([]database.CreateChapterParams, 0),
	}

	// Process streams
	for _, s := range probeResult.Streams {
		switch s.CodecType {
		case "video":
			result.VideoList = append(result.VideoList, database.CreateVideoStreamParams{
				Index:          int32(s.Index),
				Codec:          s.CodecName,
				Title:          s.CodecLongName,
				Profile:        s.Profile,
				Width:          int32(s.Width),
				Height:         int32(s.Height),
				CodedWidth:     int32(s.CodedWidth),
				CodedHeight:    int32(s.CodedHeight),
				AspectRatio:    s.AspectRatio,
				Level:          int32(s.Level),
				ColorTransfer:  s.ColorTransfer,
				ColorPrimaries: s.ColorPrimaries,
				ColorSpace:     s.ColorSpace,
				ColorRange:     s.ColorRange,
				AvgFrameRate:   s.AvgFrameRate,
				FrameRate:      s.FrameRate,
				BitDepth:       s.BitDepth,
				BitRate:        s.BitRate,
			})
		case "audio":
			result.AudioList = append(result.AudioList, database.CreateAudioStreamParams{
				Title:         s.Tags.Title,
				Index:         int32(s.Index),
				Codec:         s.CodecName,
				Language:      s.Tags.Language,
				Channels:      int32(s.Channels),
				ChannelLayout: s.ChannelLayout,
			})
		case "subtitle":
			result.SubtitleList = append(result.SubtitleList, database.CreateSubtitleParams{
				Title:    s.CodecLongName,
				Codec:    s.CodecName,
				Language: s.Tags.Language,
				Index:    int32(s.Index),
			})
		}
	}

	if len(probeResult.Chapters) > 0 {
		result.ChapterList = make([]database.CreateChapterParams, len(probeResult.Chapters))
	}

	if len(result.VideoList) == 0 {
		return nil, errors.New("no video streams found")
	}

	return result, nil
}
