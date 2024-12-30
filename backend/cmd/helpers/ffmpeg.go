package helpers

type TranscodeOptions struct {
	Bin          string
	InputPath    string
	OutputDir    string
	AudioCodec   string
	AudioBitRate string
	VideoCodec   string
	VideoBitrate string
	VideoHeight  string
}

func CreateHlsStream(opts TranscodeOptions) error {
	cmdArgs := []string{
		"-i", opts.InputPath,
	}

	if opts.AudioCodec == "copy" && opts.VideoCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c", "copy")
	} else {
		if opts.AudioBitRate == "" {
			opts.AudioBitRate = "128k"
		}

		if opts.VideoBitrate == "" {
			opts.VideoBitrate = "1000k"
		}

		if opts.VideoHeight == "" {
			opts.VideoHeight = "480"
		}

		cmdArgs = append(cmdArgs, "-c:a", opts.AudioCodec, "-c:v", opts.VideoCodec, "-b:a", opts.AudioBitRate, "-b:v", opts.VideoBitrate, "-vf", "scale=w="+opts.VideoHeight+":h=-1")
	}

	return nil
}
