package ffprobe

import (
	"encoding/json"
	"errors"
	"os/exec"
)

func (f *Ffprobe) GetTrackMetadata(trackPath string) (*TrackFfprobeResult, error) {
	if trackPath == "" {
		return nil, errors.New("got empty track file path in GetTrackMetadata function")
	}

	cmd := exec.Command(f.bin,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		trackPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var probeResult TrackFfprobeResult

	err = json.Unmarshal(output, &probeResult)
	if err != nil {
		return nil, err
	}

	return &probeResult, nil
}
