package helpers

var ValidAudioExtensions = map[string]bool{
	"mp3":  true,
	"flac": true,
	"m4a":  true,
	"opus": true,
	"ogg":  true,
	"aiff": true,
	"aif":  true,
	"wav":  true,
}

// AudioMimeTypes is the Content-Type each track is streamed with. Tracks are
// served as stored: an AIFF track plays only where the browser decodes AIFF.
var AudioMimeTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
	"opus": "audio/ogg",
	"ogg":  "audio/ogg",
	"aiff": "audio/aiff",
	"aif":  "audio/aiff",
	"wav":  "audio/wav",
}

var ValidVideoExtensions = map[string]bool{
	"mp4":  true,
	"avi":  true,
	"mkv":  true,
	"mov":  true,
	"m4v":  true,
	"webm": true,
}

// VideoMimeTypes pins the container→MIME mapping for movie files. Deriving it
// with mime.TypeByExtension is host-dependent (/etc/mime.types overrides Go's
// table and maps .webm to audio/webm; minimal images have no table at all),
// which made playback eligibility and Content-Type vary by machine — see
// "Direct Play Eligibility and Fallback" in docs/ffmpeg.md. Keys must match
// ValidVideoExtensions exactly.
var VideoMimeTypes = map[string]string{
	"mp4":  "video/mp4",
	"m4v":  "video/mp4",
	"mkv":  "video/x-matroska",
	"webm": "video/webm",
	"avi":  "video/x-msvideo",
	"mov":  "video/quicktime",
}
