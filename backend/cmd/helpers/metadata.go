package helpers

import (
	"encoding/json"
	"fmt"
	"igloo/cmd/database/models"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
)

const ffprobe = "/bin/ffprobe"

func GetMovieMetadata(movie *models.Movie) error {
	fileInfo, err := os.Stat(movie.FilePath)
	if err != nil {
		return err
	}

	movie.Size = uint(fileInfo.Size())
	movie.FileName = fileInfo.Name()

	if movie.Title == "" {
		movie.Title = fileInfo.Name()
	}

	ext := filepath.Ext(movie.FilePath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream" // Fallback if type is unknown
	}

	movie.Container = ext
	movie.ContentType = contentType

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

	movie.Resolution = fmt.Sprintf("%dx%d", movie.VideoList[0].Width, movie.VideoList[0].Height)

	return nil
}
