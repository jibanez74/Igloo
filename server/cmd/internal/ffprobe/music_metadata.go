package ffprobe

import (
	"encoding/json"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (f *Ffprobe) GetTrackMetadata(trackPath string) (*database.CreateTrackParams, []string, error) {
	track := database.CreateTrackParams{
		FilePath: trackPath,
	}
	cmd := exec.Command(f.bin,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		track.FilePath)

	output, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	var probeResult Result

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return nil, nil, err
	}

	var audioStream database.CreateAudioStreamParams

	for i := range probeResult.Streams {
		if probeResult.Streams[i].CodecType == "audio" {
			audioStream = f.processAudioStream(probeResult.Streams[i])
			break
		}
	}

	track.Title = probeResult.Format.Tags.Title
	track.Composer = probeResult.Format.Tags.Composer
	track.BitRate = probeResult.Format.BitRate
	track.Channels = audioStream.Channels
	track.Codec = audioStream.Codec
	track.Container = probeResult.Format.FormatName

	// Extract sample rate and bit depth from audio stream
	for i := range probeResult.Streams {
		if probeResult.Streams[i].CodecType == "audio" {
			if probeResult.Streams[i].SampleRate != "" {
				if sampleRate, err := strconv.Atoi(probeResult.Streams[i].SampleRate); err == nil {
					track.SampleRate = pgtype.Int4{Int32: int32(sampleRate), Valid: true}
				}
			}

			if probeResult.Streams[i].BitDepth != "" {
				if bitDepth, err := strconv.Atoi(probeResult.Streams[i].BitDepth); err == nil {
					track.BitDepth = pgtype.Int4{Int32: int32(bitDepth), Valid: true}
				}
			}
			break
		}
	}

	releaseDate, err := helpers.FormatDate(probeResult.Format.Tags.Date)
	if err != nil {
		releaseDate = time.Now()
	}

	track.ReleaseDate = pgtype.Date{
		Time: releaseDate,
	}

	index, err := strconv.Atoi(probeResult.Format.Tags.Track)
	if err != nil {
		return nil, nil, err
	}
	track.Index = int32(index)

	duration, err := strconv.Atoi(probeResult.Format.Duration)
	if err != nil {
		return nil, nil, err
	}
	track.Duration = int32(duration)

	// Extract genres from track metadata
	var genres []string
	if probeResult.Format.Tags.Genre != "" {
		// Split multiple genres if they're comma-separated
		genreList := strings.Split(probeResult.Format.Tags.Genre, ",")
		for _, genre := range genreList {
			trimmedGenre := strings.TrimSpace(genre)
			if trimmedGenre != "" {
				genres = append(genres, trimmedGenre)
			}
		}
	}

	return &track, genres, nil
}
