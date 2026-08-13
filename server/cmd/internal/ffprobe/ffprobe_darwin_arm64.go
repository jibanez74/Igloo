//go:build darwin && arm64 && !externalbin

package ffprobe

import _ "embed"

//go:embed ffprobe_darwin_arm64.zst
var embeddedCompressed []byte
