package music

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"igloo/cmd/internal/ffprobe"
)

// discFolderPattern names the folders a multi-disc album keeps each disc in:
// CD1, Disc 2, Disk_3, Vol. 1, Volume 2, Part 3.
var discFolderPattern = regexp.MustCompile(`(?i)^(cd|disc|disk|vol|volume|part)[ ._-]*[0-9]+$`)

// discNumberPattern is the part of discFolderPattern that numbers a disc. A
// Vol. or Part folder still holds an album's art, but its number is no disc
// number: a single-disc album's own folder may be named "Vol. 3".
var discNumberPattern = regexp.MustCompile(`(?i)^(?:cd|disc|disk)[ ._-]*([0-9]+)$`)

// isDiscFolder reports whether name is a disc folder of an album.
func isDiscFolder(name string) bool {
	return discFolderPattern.MatchString(strings.TrimSpace(name))
}

// discNumberFromFolder reads the disc number a CD, Disc or Disk folder names.
func discNumberFromFolder(name string) (int64, bool) {
	match := discNumberPattern.FindStringSubmatch(strings.TrimSpace(name))
	if match == nil {
		return 0, false
	}
	number, err := strconv.ParseInt(match[1], 10, 64)
	valid := err == nil && number > 0
	if !valid {
		return 0, false
	}
	return number, true
}

// stemTrackPattern is a file name that starts with its track number: "01 -
// Title", "1-Title", "03. Title", "04_Title", two digits and a space ("01
// Title"), or iTunes' disc and track ("2-01 Title"). Two digits keep a title
// such as "7 Rings" whole; a stem of digits alone ("1999") is a title.
var stemTrackPattern = regexp.MustCompile(`^(?:[0-9]-([0-9]{2})[ ]+|([0-9]{1,3})[ ]*(?:-|_|\.[ ])[ ]*|([0-9]{2})[ ]+)(\S.*)$`)

// stemTrack splits a file stem into its leading track number, 0 when it has
// none, and the title that follows.
func stemTrack(stem string) (int64, string) {
	stem = strings.TrimSpace(stem)
	match := stemTrackPattern.FindStringSubmatch(stem)
	if match == nil {
		return 0, stem
	}
	digits := match[1] + match[2] + match[3]
	index, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, stem
	}
	return index, strings.TrimSpace(match[4])
}

// variousArtists is the album artist of a compilation that names none.
const variousArtists = "Various Artists"

// variousArtistsFolders are the artist folders, lowercased, that hold
// compilations.
var variousArtistsFolders = map[string]bool{"compilations": true, "various artists": true}

// albumYearPattern reads a year an album folder carries after its name, "No
// Angel (1999)" or "[1999]", or before it, "1999 - No Angel".
var albumYearPattern = regexp.MustCompile(`^(?:(.+?)[ ]*[(\[]([0-9]{4})[)\]]|([0-9]{4})[ ]*-[ ]*(.+))$`)

// splitAlbumYear splits an album folder name into the album and its year, 0
// when the name carries none.
func splitAlbumYear(name string) (string, int64) {
	match := albumYearPattern.FindStringSubmatch(name)
	if match == nil {
		return name, 0
	}
	album := strings.TrimSpace(match[1] + match[4])
	digits := match[2] + match[3]
	year, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || album == "" {
		return name, 0
	}
	return album, year
}

// splitCreditTitle splits "Artist - Title", the name of a file that carries
// both.
func splitCreditTitle(value string) (string, string, bool) {
	artist, title, found := strings.Cut(value, " - ")
	artist = strings.TrimSpace(artist)
	title = strings.TrimSpace(title)
	valid := found && artist != "" && title != ""
	if !valid {
		return "", "", false
	}
	return artist, title, true
}

// fillTagsFromPath fills the tags of a file that names neither its album nor
// its artist from where it sits under root: <Artist>/<Album (Year)>/[Disc N/]
// gives artist, album and year; an artist folder named Compilations or
// Various Artists makes a compilation whose files are "Artist - Title"; and a
// file in root itself, "Artist - Title", gets its artist and title. title is
// the file stem past its track number. Tagged values are never replaced.
func fillTagsFromPath(tags *ffprobe.FormatTags, root, path, title string) {
	untagged := blankTag(tags.Album) && blankTag(tags.Artist) && blankTag(tags.Artists) && blankTag(tags.AlbumArtist)
	if !untagged {
		return
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Dir(filepath.Clean(path)))
	outside := err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
	if outside {
		return
	}
	if relative == "." {
		artist, creditTitle, split := splitCreditTitle(title)
		if split {
			tags.Artist = artist
			fillBlankTag(&tags.Title, creditTitle)
		}
		return
	}
	folders := strings.Split(relative, string(filepath.Separator))
	inDisc := len(folders) >= 3 && isDiscFolder(folders[len(folders)-1])
	if inDisc {
		folders = folders[:len(folders)-1]
	}
	if len(folders) < 2 {
		return
	}
	album, year := splitAlbumYear(folders[len(folders)-1])
	artist := folders[len(folders)-2]
	tags.Album = album
	if year > 0 {
		fillBlankTag(&tags.Date, strconv.FormatInt(year, 10))
	}
	compilation := variousArtistsFolders[strings.ToLower(strings.TrimSpace(artist))]
	if !compilation {
		tags.Artist = artist
		return
	}
	tags.AlbumArtist = variousArtists
	tags.Compilation = "1"
	credit, creditTitle, split := splitCreditTitle(title)
	if split {
		tags.Artist = credit
		fillBlankTag(&tags.Title, creditTitle)
	}
}

func blankTag(value string) bool {
	return strings.TrimSpace(value) == ""
}

func fillBlankTag(tag *string, value string) {
	if blankTag(*tag) {
		*tag = value
	}
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
