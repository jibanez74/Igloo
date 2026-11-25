package music

import "testing"

func TestAllowDirectorySkipsHiddenFoldersAndITunesLPBundles(t *testing.T) {
	cases := map[string]bool{
		"/music/Adele/25":          true,
		"/music/Adele/25 (Deluxe)": true,
		"/music/.Trashes":          false,
		"/music/Adele/.hidden":     false,
		"/music/Kelly Clarkson/Wrapped In Red/iTunes LP.itlp": false,
		"/music/Kelly Clarkson/Booklet.ITLP":                  false,
		"/music/Bundles.itlp.d":                               true,
	}
	for path, want := range cases {
		got := AllowDirectory("/music", path)
		if got != want {
			t.Errorf("AllowDirectory(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSkipFileSkipsHiddenFiles(t *testing.T) {
	cases := map[string]bool{
		"/music/Adele/25/01 Hello.m4a":   false,
		"/music/Adele/25/._01 Hello.m4a": true,
		"/music/.hidden.flac":            true,
	}
	for path, want := range cases {
		got := skipFile(path)
		if got != want {
			t.Errorf("skipFile(%q) = %v, want %v", path, got, want)
		}
	}
}
