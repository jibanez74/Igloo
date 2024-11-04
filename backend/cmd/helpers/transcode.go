package helpers

import "os/exec"

const ffmpegPath = "/bin/ffmpeg"

func TranscodeVideo(filePath, outputName string) error {
	cmd := exec.Command(ffmpegPath, "-y", "-i", filePath, "-c", "copy", "-movflgs", "+faststart", outputName)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
