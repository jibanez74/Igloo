//go:build darwin && arm64 && !systembin

package ffprobe

import _ "embed"

//go:embed ffprobe_darwin_arm64
var embeddedBinary []byte

