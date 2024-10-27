package helpers

import (
	"encoding/json"
	"igloo/cmd/database/models"
	"mime"
	"os/exec"
	"path/filepath"
)

const ffprobe = "/bin/ffprobe"

func GetMovieMetadata(movie *models.Movie) error {
	movie.Container = filepath.Ext(movie.FilePath)

	movie.ContentType = mime.TypeByExtension(movie.Container)
	if movie.ContentType == "" {
		movie.ContentType = "application/octet-stream" // Fallback if type is unknown
	}

	cmd := exec.Command(ffprobe, "-i", movie.FilePath,
		"-show_streams",
		"-show_chapters",
		"-show_format",
		"-print_format", "json")

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	var probeResult result

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return err
	}

	for _, s := range probeResult.Streams {
		if s.CodecType == "video" {
			movie.VideoList = append(movie.VideoList, models.VideoStream{
				Index:          uint(s.Index),
				Codec:          s.CodecName,
				Title:          s.CodecLongName,
				Profile:        s.Profile,
				Width:          s.Width,
				Height:         s.Height,
				CodedWidth:     s.CodedWidth,
				CodedHeight:    s.CodedHeight,
				AspectRatio:    s.AspectRatio,
				Level:          s.Level,
				ColorTransfer:  s.ColorTransfer,
				ColorPrimaries: s.ColorPrimaries,
				ColorSpace:     s.ColorSpace,
				ColorRange:     s.ColorRange,
				AvgFrameRate:   s.AvgFrameRate,
				FrameRate:      s.FrameRate,
				BitDepth:       s.BitDepth,
				BitRate:        s.BitRate,
			})
		}

		if s.CodecType == "audio" {
			movie.AudioList = append(movie.AudioList, models.AudioStream{
				Title:         s.Tags.Title,
				Index:         uint(s.Index),
				Codec:         s.CodecName,
				Profile:       s.Profile,
				Language:      s.Tags.Language,
				Channels:      uint(s.Channels),
				ChannelLayout: s.ChannelLayout,
			})
		}

		if s.CodecType == "subtitle" {
			movie.SubtitleList = append(movie.SubtitleList, models.Subtitles{
				Title:    s.CodecLongName,
				Codec:    s.CodecName,
				Language: s.Tags.Language,
				Index:    uint(s.Index),
			})
		}
	}

	if len(probeResult.Chapters) > 0 {
		movie.ChapterList = make([]models.Chapter, len(probeResult.Chapters))
	}

	return nil
}
