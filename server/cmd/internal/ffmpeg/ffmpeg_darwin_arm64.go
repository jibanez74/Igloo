//go:build darwin && arm64 && !externalbin

package ffmpeg

import _ "embed"

//go:embed ffmpeg_darwin_arm64
var embeddedBinary []byte
