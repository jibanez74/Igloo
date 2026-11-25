package music

import (
	"path/filepath"
	"strings"
)

// iTunesLPExtension names the bundle folders iTunes keeps an album's
// interactive booklet in; their .aif files are interface sounds, not tracks.
const iTunesLPExtension = ".itlp"

// systemFolders are the folders, lowercased, that operating systems and NAS
// software keep beside a library's music. None holds tracks, and a leading
// dot alone says nothing: an album folder may be named "...And Justice for
// All" or ".38 Special".
var systemFolders = map[string]bool{
	".trashes":                true,
	".trash":                  true,
	".fseventsd":              true,
	".spotlight-v100":         true,
	".temporaryitems":         true,
	".documentrevisions-v100": true,
	".appledouble":            true,
	"@eadir":                  true,
}

// AllowDirectory reports whether the scan descends into path: any folder
// under root except system folders, a removable disk's per-user trash
// (.Trash-1000) and iTunes LP bundles. Walks and the watcher check every
// folder between root and a file, so the last name is all it reads; the
// signature is the watcher's.
func AllowDirectory(root, path string) bool {
	name := strings.ToLower(filepath.Base(path))
	system := systemFolders[name]
	if system {
		return false
	}
	userTrash := strings.HasPrefix(name, ".trash-")
	if userTrash {
		return false
	}
	bundle := strings.HasSuffix(name, iTunesLPExtension)
	return !bundle
}

// skipFile reports the ._ AppleDouble companion macOS writes next to each
// file it copies to a disk that cannot hold its metadata: it carries the
// track's name and extension but no audio.
func skipFile(path string) bool {
	return strings.HasPrefix(filepath.Base(path), "._")
}
