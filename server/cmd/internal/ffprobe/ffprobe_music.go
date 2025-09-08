package ffprobe

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

func (f *Ffprobe) GetTrackMetadata(trackPath string) (*TrackFfprobeResult, error) {
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

	if len(probeResult.Streams) == 0 {
		return nil, fmt.Errorf("fail to get any streams for %s", trackPath)
	}

	return &probeResult, nil
}
