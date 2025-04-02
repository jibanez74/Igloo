package ffmpeg

import "fmt"

type ffmpegError struct {
	Field string
	Value any
	Msg   string
}

func (e *ffmpegError) Error() string {
	return fmt.Sprintf("invalid ffmpeg input %s: %v - %s", e.Field, e.Value, e.Msg)
}
