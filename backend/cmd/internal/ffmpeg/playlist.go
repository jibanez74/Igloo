package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (f *ffmpeg) createMasterPlaylist(opts *HlsOpts) error {
	playlistPath := filepath.Join(opts.OutputDir, DefaultMasterPlaylist)

	file, err := os.Create(playlistPath)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_path",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to create master playlist file: %v", err),
		}
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s\n", HlsTagExtM3U)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagExtM3U,
			Msg:   fmt.Sprintf("failed to write EXTM3U tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagVersion)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagVersion,
			Msg:   fmt.Sprintf("failed to write version tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagTargetDuration+"\n", DefaultHlsTime)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagTargetDuration,
			Msg:   fmt.Sprintf("failed to write target duration tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagMediaSequence)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagMediaSequence,
			Msg:   fmt.Sprintf("failed to write media sequence tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagPlaylistType+"\n", DefaultPlaylistType)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagPlaylistType,
			Msg:   fmt.Sprintf("failed to write playlist type tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagIndependentSegments)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagIndependentSegments,
			Msg:   fmt.Sprintf("failed to write independent segments tag: %v", err),
		}
	}

	startTime := time.Now()
	if opts.StartTime > 0 {
		startTime = startTime.Add(time.Duration(-opts.StartTime) * time.Second)
	}

	programDateTime := startTime.UTC().Format(time.RFC3339)
	_, err = fmt.Fprintf(file, HlsTagProgramDateTime+"\n", programDateTime)
	if err != nil {
		return &ffmpegError{
			Field: "program_date_time",
			Value: programDateTime,
			Msg:   fmt.Sprintf("failed to write program date time: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagEndList+"\n")
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_footer",
			Value: HlsTagEndList,
			Msg:   fmt.Sprintf("failed to write end list tag: %v", err),
		}
	}

	return nil
}
