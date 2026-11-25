package music

import "testing"

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
