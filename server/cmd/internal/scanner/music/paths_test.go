package music

import (
	"testing"

	"igloo/cmd/internal/ffprobe"
)

func TestAllowDirectorySkipsSystemFoldersAndITunesLPBundles(t *testing.T) {
	cases := map[string]bool{
		"/music/Adele/25":                                     true,
		"/music/Metallica/...And Justice for All":             true,
		"/music/.38 Special":                                  true,
		"/music/Bundles.itlp.d":                               true,
		"/music/.Trashes":                                     false,
		"/music/.Trash-1000":                                  false,
		"/music/.Trash-Metal":                                 true,
		"/music/#recycle":                                     false,
		"/music/$RECYCLE.BIN":                                 false,
		"/music/@Recycle":                                     false,
		"/music/.stversions":                                  false,
		"/music/lost+found":                                   false,
		"/music/.fseventsd":                                   false,
		"/music/.Spotlight-V100":                              false,
		"/music/Adele/25/.AppleDouble":                        false,
		"/music/Adele/@eaDir":                                 false,
		"/music/Kelly Clarkson/Wrapped In Red/iTunes LP.itlp": false,
		"/music/Kelly Clarkson/Booklet.ITLP":                  false,
	}
	for path, want := range cases {
		got := AllowDirectory("/music", path)
		if got != want {
			t.Errorf("AllowDirectory(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSkipFileSkipsOnlyAppleDoubleCompanions(t *testing.T) {
	cases := map[string]bool{
		"/music/Adele/25/01 Hello.m4a":                     false,
		"/music/Britney Spears/...Baby One More Time.flac": false,
		"/music/.38 Special - Hold On Loosely.mp3":         false,
		"/music/Adele/25/._01 Hello.m4a":                   true,
	}
	for path, want := range cases {
		got := skipFile(path)
		if got != want {
			t.Errorf("skipFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDiscNumberFromFolderReadsOnlyDiscFolders(t *testing.T) {
	cases := map[string]int64{
		"CD1":       1,
		"cd 2":      2,
		"Disc 2":    2,
		"Disk_3":    3,
		"DISC-10":   10,
		"Vol. 3":    0,
		"Volume 2":  0,
		"Part 1":    0,
		"Disc 0":    0,
		"The River": 0,
		"Disc":      0,
		"CD1 Bonus": 0,
	}
	for name, want := range cases {
		got, ok := discNumberFromFolder(name)
		if got != want || ok != (want > 0) {
			t.Errorf("discNumberFromFolder(%q) = %d, %v, want %d", name, got, ok, want)
		}
	}
}

func TestStemTrackSplitsALeadingTrackNumber(t *testing.T) {
	cases := []struct {
		stem  string
		index int64
		title string
	}{
		{"01 - Here With Me", 1, "Here With Me"},
		{"1-Title", 1, "Title"},
		{"03. Thank You", 3, "Thank You"},
		{"04_Hunter", 4, "Hunter"},
		{"01 Singer Alpha - Hit One", 1, "Singer Alpha - Hit One"},
		{"2-01 Disc Two Opener", 1, "Disc Two Opener"},
		{"112 - Long Album Track", 112, "Long Album Track"},
		{"1999", 0, "1999"},
		{"7 Rings", 0, "7 Rings"},
		{"3.14 Pi", 0, "3.14 Pi"},
		{"Norah Jones - Don't Know Why", 0, "Norah Jones - Don't Know Why"},
		{"  ", 0, ""},
	}
	for _, tc := range cases {
		index, title := stemTrack(tc.stem)
		if index != tc.index || title != tc.title {
			t.Errorf("stemTrack(%q) = %d, %q, want %d, %q", tc.stem, index, title, tc.index, tc.title)
		}
	}
}

func TestSplitAlbumYearReadsAYearBeforeOrAfterTheName(t *testing.T) {
	cases := []struct {
		name  string
		album string
		year  int64
	}{
		{"No Angel (1999)", "No Angel", 1999},
		{"No Angel [1999]", "No Angel", 1999},
		{"1999 - No Angel", "No Angel", 1999},
		{"No Angel", "No Angel", 0},
		{"(1999)", "(1999)", 0},
		{"1999", "1999", 0},
		{"Greatest Hits 1990-1999", "Greatest Hits 1990-1999", 0},
	}
	for _, tc := range cases {
		album, year := splitAlbumYear(tc.name)
		if album != tc.album || year != tc.year {
			t.Errorf("splitAlbumYear(%q) = %q, %d, want %q, %d", tc.name, album, year, tc.album, tc.year)
		}
	}
}

func TestFillTagsFromPathFillsOnlyUntaggedFiles(t *testing.T) {
	root := "/music"
	cases := []struct {
		name  string
		path  string
		title string
		tags  ffprobe.FormatTags
		want  ffprobe.FormatTags
	}{
		{"artist and album folders", "/music/Dido/No Angel (1999)/01 - Here With Me.mp3", "Here With Me",
			ffprobe.FormatTags{}, ffprobe.FormatTags{Artist: "Dido", Album: "No Angel", Date: "1999"}},
		{"a disc folder under the album", "/music/Bruce Springsteen/The River/Disc 2/01 Song.mp3", "Song",
			ffprobe.FormatTags{}, ffprobe.FormatTags{Artist: "Bruce Springsteen", Album: "The River"}},
		{"an album folder named like a disc", "/music/Traveling Wilburys/Vol. 1/01 Song.mp3", "Song",
			ffprobe.FormatTags{}, ffprobe.FormatTags{Artist: "Traveling Wilburys", Album: "Vol. 1"}},
		{"deeper folders take the last two", "/music/Pop/Dido/No Angel/01 Song.mp3", "Song",
			ffprobe.FormatTags{}, ffprobe.FormatTags{Artist: "Dido", Album: "No Angel"}},
		{"a tagged date wins", "/music/Dido/No Angel (1999)/01 Song.mp3", "Song",
			ffprobe.FormatTags{Date: "2000-05-01"}, ffprobe.FormatTags{Artist: "Dido", Album: "No Angel", Date: "2000-05-01"}},
		{"a compilations folder", "/music/Compilations/Pure Dance/02 Singer Beta - Hit Two.mp3", "Singer Beta - Hit Two",
			ffprobe.FormatTags{}, ffprobe.FormatTags{Title: "Hit Two", Artist: "Singer Beta", AlbumArtist: variousArtists, Album: "Pure Dance", Compilation: "1"}},
		{"a various artists folder keeps a tagged title", "/music/various artists/Pure Dance/Singer Beta - Hit Two.mp3", "Singer Beta - Hit Two",
			ffprobe.FormatTags{Title: "Tagged"}, ffprobe.FormatTags{Title: "Tagged", Artist: "Singer Beta", AlbumArtist: variousArtists, Album: "Pure Dance", Compilation: "1"}},
		{"a root file names artist and title", "/music/Norah Jones - Don't Know Why.mp3", "Norah Jones - Don't Know Why",
			ffprobe.FormatTags{}, ffprobe.FormatTags{Title: "Don't Know Why", Artist: "Norah Jones"}},
		{"a root file without a credit", "/music/Untitled.mp3", "Untitled",
			ffprobe.FormatTags{}, ffprobe.FormatTags{}},
		{"one folder names no artist", "/music/No Angel/01 Song.mp3", "Song",
			ffprobe.FormatTags{}, ffprobe.FormatTags{}},
		{"a file outside the root", "/elsewhere/Dido/No Angel/01 Song.mp3", "Song",
			ffprobe.FormatTags{}, ffprobe.FormatTags{}},
		{"an artist tag keeps the folders out", "/music/Dido/No Angel/01 Song.mp3", "Song",
			ffprobe.FormatTags{Artist: "Tagged"}, ffprobe.FormatTags{Artist: "Tagged"}},
		{"an album tag keeps the folders out", "/music/Dido/No Angel/01 Song.mp3", "Song",
			ffprobe.FormatTags{Album: "Tagged"}, ffprobe.FormatTags{Album: "Tagged"}},
		{"an ARTISTS tag keeps the folders out", "/music/Dido/No Angel/01 Song.mp3", "Song",
			ffprobe.FormatTags{Artists: "A;B"}, ffprobe.FormatTags{Artists: "A;B"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tags := tc.tags
			fillTagsFromPath(&tags, root, tc.path, tc.title)
			if tags != tc.want {
				t.Fatalf("fillTagsFromPath() = %+v, want %+v", tags, tc.want)
			}
		})
	}
}
