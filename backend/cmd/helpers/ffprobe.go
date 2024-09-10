package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/models"
	"os/exec"
)

var ffprobe string = "/bin/ffprobe"

func GetAudioTracks(filePath string) ([]models.Audio, error) {
	var audioList []models.Audio

	cmd := exec.Command(ffprobe, "-v", "error", "-print_format", "json", "-show_streams", "-select_streams", "a", "-i", filePath)

	output, err := cmd.Output()
	if err != nil {
		return audioList, err
	}

	var probeResult models.AudioProbe

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return audioList, err
	}

	for _, stream := range probeResult.Streams {
		audio := models.Audio{
			Title:         stream.Tags.Title,
			Codec:         stream.Codec,
			Profile:       stream.Profile,
			Channels:      uint(stream.Channels),
			ChannelLayout: stream.ChannelLayout,
			Language:      stream.Tags.Language,
			Index:         uint(stream.Index),
			BitRate:       stream.Tags.BitRate,
		}

		audioList = append(audioList, audio)
	}

	return audioList, nil
}

func GetVideoTrack(filePath string) ([]models.Video, error) {
	var videos []models.Video

	cmd := exec.Command(ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "v", filePath)

	output, err := cmd.Output()
	if err != nil {
		return videos, err
	}

	var probeResult models.VideoProbe

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return videos, err
	}

	if len(probeResult.Streams) == 0 {
		return videos, errors.New("no video stream found")
	}

	for _, stream := range probeResult.Streams {
		video := models.Video{
			Title:          stream.Title,
			Index:          stream.Index,
			BitRate:        stream.Tags.BitRate,
			BitDepth:       stream.BitDepth,
			Codec:          stream.CodecName,
			ColorSpace:     stream.ColorSpace,
			ColorPrimaries: stream.ColorPrimaries,
			Width:          uint(stream.Width),
			Height:         uint(stream.Height),
			CodedHeight:    uint(stream.CodedHeight),
			CodedWidth:     uint(stream.CodedWidth),
			AvgFrameRate:   stream.AvgFrameRate,
			FrameRate:      stream.FrameRate,
			NumberOfFrames: stream.Tags.NumberOfFrames,
			NumberOfBytes:  stream.Tags.NumberOfBytes,
			AspectRatio:    stream.AspectRatio,
		}

		video.Duration, _ = ConvertFFmpegDurationToSeconds(stream.Tags.Duration)

		video.Chapters, err = GetChapters(filePath)
		if err != nil {
			return videos, err
		}

		videos = append(videos, video)
	}

	return videos, nil
}

func GetSubtitles(filePath string) ([]models.Subtitles, error) {
	var subtitles []models.Subtitles

	cmd := exec.Command(ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "s", filePath)

	output, err := cmd.Output()
	if err != nil {
		return subtitles, err
	}

	var probeResult models.SubtitlesProbe

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return subtitles, err
	}

	for _, stream := range probeResult.Streams {
		subtitle := models.Subtitles{
			Codec:    stream.Codec,
			Index:    uint(stream.Index),
			Language: stream.Tags.Language,
		}

		subtitles = append(subtitles, subtitle)
	}

	return subtitles, nil
}

func GetChapters(filePath string) ([]models.Chapter, error) {
	var chapters []models.Chapter

	cmd := exec.Command(ffprobe, "-v", "quiet", "-print_format", "json", "-show_chapters", "-i", filePath)

	output, err := cmd.Output()
	if err != nil {
		return chapters, err
	}

	var probeResult models.ChaptersProbe

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return chapters, err
	}

	if len(probeResult.Chapters) == 0 {
		return chapters, nil
	}

	for _, ch := range probeResult.Chapters {
		chapter := models.Chapter{
			Title:     ch.Tags.Title,
			TimeBase:  ch.TimeBase,
			Start:     uint(ch.Start),
			StartTime: ch.StartTime,
			End:       uint(ch.End),
			EndTime:   ch.EndTime,
		}

		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func ConvertFFmpegDurationToSeconds(duration string) (float64, error) {
	var hours, minutes, seconds, milliseconds float64

	_, err := fmt.Sscanf(duration, "%f:%f:%f.%f", &hours, &minutes, &seconds, &milliseconds)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	totalSeconds := hours*3600 + minutes*60 + seconds + milliseconds/1000

	return totalSeconds, nil
}
