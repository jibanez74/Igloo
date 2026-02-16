//go:build linux && amd64

package ffmpeg

import _ "embed"

//go:embed ffmpeg_linux_amd64
var embeddedBinary []byte
