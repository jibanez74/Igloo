//go:build darwin && arm64

package ffmpeg

import _ "embed"

//go:embed ffmpeg_mac_arm
var embeddedBinary []byte
