package music

import (
	"path/filepath"
	"strings"
)

// iTunesLPExtension names the bundle folders iTunes keeps an album's
// interactive booklet in; their .aif files are interface sounds, not tracks.
const iTunesLPExtension = ".itlp"

// AllowDirectory reports whether the scan descends into path: any folder
// under root except hidden ones (macOS keeps .Trashes and .fseventsd on
// external drives) and iTunes LP bundles. root is unused, as every music
// folder may hold tracks; the signature is the watcher's.
func AllowDirectory(root, path string) bool {
	name := filepath.Base(path)
	hidden := strings.HasPrefix(name, ".")
	if hidden {
		return false
	}
	bundle := strings.HasSuffix(strings.ToLower(name), iTunesLPExtension)
	return !bundle
}

// skipFile reports a hidden file, such as the ._ AppleDouble companion macOS
// writes next to each file it copies to a foreign disk, which carries the
// track's extension but no audio.
func skipFile(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}
