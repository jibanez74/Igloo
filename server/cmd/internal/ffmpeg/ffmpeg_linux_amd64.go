//go:build linux && amd64 && !externalbin

package ffmpeg

import _ "embed"

//go:embed ffmpeg_linux_amd64.zst
var embeddedCompressed []byte
