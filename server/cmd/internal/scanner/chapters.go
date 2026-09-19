package scanner

import (
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

// ChapterStartTimeSeconds reads ffprobe's start_time. The raw start field is in
// the chapter's own time base and is not a usable fallback, so an unparsable
// start time places the chapter at zero.
func ChapterStartTimeSeconds(chapter ffprobe.Chapter) int64 {
	if chapter.StartTime == "" {
		return 0
	}
	seconds, ok := helpers.ParseDurationSeconds(chapter.StartTime)
	if !ok {
		return 0
	}
	return int64(seconds)
}
