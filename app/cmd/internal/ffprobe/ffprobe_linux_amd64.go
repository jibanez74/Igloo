//go:build linux && amd64

package ffprobe

import _ "embed"

//go:embed ffprobe_linux_amd64
var embeddedBinary []byte

