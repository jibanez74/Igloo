//go:build linux && amd64 && !externalbin

package ffprobe

import _ "embed"

//go:embed ffprobe_linux_amd64.zst
var embeddedCompressed []byte
