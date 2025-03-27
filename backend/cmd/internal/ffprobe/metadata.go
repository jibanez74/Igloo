package ffprobe

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"os/exec"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
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

	videoStreams := make(map[int]database.CreateVideoStreamParams)
	audioStreams := make(map[int]database.CreateAudioStreamParams)
	subtitleStreams := make(map[int]database.CreateSubtitleParams)

	for _, s := range probeResult.Streams {
		if s.Disposition != nil && s.Disposition.AttachedPic == 1 {
			continue
		}

		switch s.CodecType {
		case "video":
			videoStreams[s.Index] = database.CreateVideoStreamParams{
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
			}
		case "audio":
			audioStreams[s.Index] = database.CreateAudioStreamParams{
				Title:         s.Tags.Title,
				Index:         int32(s.Index),
				Codec:         s.CodecName,
				Language:      s.Tags.Language,
				Channels:      int32(s.Channels),
				ChannelLayout: s.ChannelLayout,
			}
		case "subtitle":
			subtitleStreams[s.Index] = database.CreateSubtitleParams{
				Title:    s.CodecLongName,
				Codec:    s.CodecName,
				Language: s.Tags.Language,
				Index:    int32(s.Index),
			}
		}
	}

	// Convert maps to sorted slices
	result := &movieMetadataResult{
		VideoList:    make([]database.CreateVideoStreamParams, 0, len(videoStreams)),
		AudioList:    make([]database.CreateAudioStreamParams, 0, len(audioStreams)),
		SubtitleList: make([]database.CreateSubtitleParams, 0, len(subtitleStreams)),
		ChapterList:  make([]database.CreateChapterParams, 0),
	}

	// Add streams in order of their index
	for i := 0; i < len(probeResult.Streams); i++ {
		if stream, exists := videoStreams[i]; exists {
			result.VideoList = append(result.VideoList, stream)
		}
		if stream, exists := audioStreams[i]; exists {
			result.AudioList = append(result.AudioList, stream)
		}
		if stream, exists := subtitleStreams[i]; exists {
			result.SubtitleList = append(result.SubtitleList, stream)
		}
	}

	if len(probeResult.Chapters) > 0 {
		result.ChapterList = make([]database.CreateChapterParams, len(probeResult.Chapters))
		for i, chapter := range probeResult.Chapters {
			startTimeMs := int32(0)
			if chapter.StartTime != "" {
				if startTime, err := strconv.ParseFloat(chapter.StartTime, 64); err == nil {
					startTimeMs = int32(startTime * 1000)
				}
			}

			result.ChapterList[i] = database.CreateChapterParams{
				Title:       chapter.Title,
				StartTimeMs: startTimeMs,
				Thumb:       "",
				MovieID:     pgtype.Int4{},
			}
		}
	}

	if len(result.VideoList) == 0 {
		return nil, errors.New("no video streams found")
	}

	duration, err := strconv.ParseFloat(probeResult.Format.Duration, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert duration to float: %w", err)
	}

	// Round to nearest second
	result.VideoList[0].Duration = int64(duration + 0.5)

	return result, nil
}
