package music

import (
	"path/filepath"
	"regexp"
	"strings"
)

// discFolderPattern names the folders a multi-disc album keeps each disc in:
// CD1, Disc 2, Disk_3, Vol. 1, Volume 2, Part 3.
var discFolderPattern = regexp.MustCompile(`(?i)^(cd|disc|disk|vol|volume|part)[ ._-]*[0-9]+$`)

// isDiscFolder reports whether name is a disc folder of an album.
func isDiscFolder(name string) bool {
	return discFolderPattern.MatchString(strings.TrimSpace(name))
}

// iTunesLPExtension names the bundle folders iTunes keeps an album's
// interactive booklet in; their .aif and .m4a files are interface sounds and
// intros, not tracks.
const iTunesLPExtension = ".itlp"

// systemFolders are the folders, lowercased, that operating systems, NAS and
// sync software keep beside a library's music: metadata, or trash and
// snapshots holding copies of its tracks. A leading dot alone says nothing:
// an album folder may be named "...And Justice for All" or ".38 Special".
var systemFolders = map[string]bool{
	".trashes":                true,
	".trash":                  true,
	".fseventsd":              true,
	".spotlight-v100":         true,
	".temporaryitems":         true,
	".documentrevisions-v100": true,
	".appledouble":            true,
	"@eadir":                  true,
	".recycle":                true,
	"#recycle":                true,
	"@recycle":                true,
	"$recycle.bin":            true,
	".stversions":             true,
	".snapshots":              true,
	"lost+found":              true,
}

// AllowDirectory reports whether the scan descends into path: any folder
// under root except system folders, a removable disk's per-user trash
// (.Trash- and a numeric user id) and iTunes LP bundles. Walks and the
// watcher check every folder between root and a file, so the last name is
// all it reads; the signature is the watcher's.
func AllowDirectory(root, path string) bool {
	name := strings.ToLower(filepath.Base(path))
	system := systemFolders[name]
	if system {
		return false
	}
	uid, trashPrefix := strings.CutPrefix(name, ".trash-")
	userTrash := trashPrefix && uid != "" && strings.Trim(uid, "0123456789") == ""
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
