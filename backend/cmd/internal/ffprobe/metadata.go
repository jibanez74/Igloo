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

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run ffprobe: %w, output: %s", err, output)
	}

	var probeResult result

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(probeResult.Streams) == 0 {
		return nil, errors.New("no streams found")
	}

	videoStreams, audioStreams, subtitleStreams := f.processStreams(probeResult.Streams)

	if len(videoStreams) == 0 {
		return nil, errors.New("no video streams found")
	}

	result := &movieMetadataResult{
		VideoList:    videoStreams,
		AudioList:    audioStreams,
		SubtitleList: subtitleStreams,
		ChapterList:  f.processChapters(probeResult.Chapters),
	}

	if probeResult.Format.Duration != "" {
		result.VideoList[0].Duration = probeResult.Format.Duration
	} else {
		result.VideoList[0].Duration = "0"
	}

	return result, nil
}

func (f *ffprobe) processStreams(streams []mediaStream) ([]database.CreateVideoStreamParams, []database.CreateAudioStreamParams, []database.CreateSubtitleParams) {
	videoStreams := make([]database.CreateVideoStreamParams, 0)
	audioStreams := make([]database.CreateAudioStreamParams, 0)
	subtitleStreams := make([]database.CreateSubtitleParams, 0)

	for _, s := range streams {
		if s.Disposition != nil && s.Disposition.AttachedPic == 1 {
			continue
		}

		switch s.CodecType {
		case "video":
			videoStreams = append(videoStreams, f.processVideoStream(s))
		case "audio":
			audioStreams = append(audioStreams, f.processAudioStream(s))
		case "subtitle":
			subtitleStreams = append(subtitleStreams, f.processSubtitleStream(s))
		}
	}

	return videoStreams, audioStreams, subtitleStreams
}

func (f *ffprobe) processVideoStream(s mediaStream) database.CreateVideoStreamParams {
	return database.CreateVideoStreamParams{
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
}

func (f *ffprobe) processAudioStream(s mediaStream) database.CreateAudioStreamParams {
	return database.CreateAudioStreamParams{
		Title:         s.Tags.Title,
		Index:         int32(s.Index),
		Codec:         s.CodecName,
		Language:      s.Tags.Language,
		Channels:      int32(s.Channels),
		ChannelLayout: s.ChannelLayout,
	}
}

func (f *ffprobe) processSubtitleStream(s mediaStream) database.CreateSubtitleParams {
	return database.CreateSubtitleParams{
		Title:    s.CodecLongName,
		Codec:    s.CodecName,
		Language: s.Tags.Language,
		Index:    int32(s.Index),
	}
}

func (f *ffprobe) processChapters(chapters []tmdbChapter) []database.CreateChapterParams {
	if len(chapters) == 0 {
		return nil
	}

	result := make([]database.CreateChapterParams, len(chapters))
	for i, chapter := range chapters {
		startTimeMs := int32(0)
		if chapter.StartTime != "" {
			if startTime, err := strconv.ParseFloat(chapter.StartTime, 64); err == nil {
				startTimeMs = int32(startTime * 1000)
			}
		}

		result[i] = database.CreateChapterParams{
			Title:       chapter.Title,
			StartTimeMs: startTimeMs,
			Thumb:       "",
			MovieID:     pgtype.Int4{},
		}
	}

	return result
}
