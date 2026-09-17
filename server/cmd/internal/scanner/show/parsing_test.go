package show

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseFile(t *testing.T) {
	for _, tc := range []struct {
		path     string
		episodes []int
		season   int
	}{
		{"Show (2020)/Season 1/Show.s01e02.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02-E04.mp4", []int{2, 3, 4}, 1},
		{"Show/Season 1/S01E02E03.avi", []int{2, 3}, 1},
		{"Show/season 01/1x02.webm", []int{2}, 1},
		{"Breaking Bad/Season 2/Breaking Bad - S02E09 - 4 Days Out.mkv", []int{9}, 2},
		{"Show/Season 1/Show.S01E02.[1920x1080].mkv", []int{2}, 1},
		{"Show/Season 1/S01E02  -   4 Days Out.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02\t-\t4 Days Out.mkv", []int{2}, 1},
		{"Show/Season 1/1x02 - 4 Days Out.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02-E04 - 4 Days Out.mkv", []int{2, 3, 4}, 1},
		{"Show/Season 1/S01E02E03 - 1984.mkv", []int{2, 3}, 1},
		{"Show/Season 1/S01E02 - 1984.mkv", []int{2}, 1},
		{"Show/Season 1/1920x1080 S01E02.mkv", []int{2}, 1},
		{"Show/Season 1/[1920x1080] S01E02.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02 1920x1080.mkv", []int{2}, 1},
		{"Show/Season 1/1920X1080 1x02 [1280x720].mkv", []int{2}, 1},
		{"Show/Season 1/[640x480][1920x1080]S01E02-E04[3840X2160].mkv", []int{2, 3, 4}, 1},
		{"Show/Season 1/[720x1280] S01E02 [480x640].mkv", []int{2}, 1},
		{"Show/Specials/S00E01.mkv", []int{1}, 0},
		{"Show/Season 0/S00E01-E03.mkv", []int{1, 2, 3}, 0},
		{"Show/Show.S01E01.mkv", []int{1}, 1},
		{"Show (2020)/Show.1x02.mkv", []int{2}, 1},
		{"Show/Show.S00E01.mkv", []int{1}, 0},
		{"Show/S01/S01E01.mkv", []int{1}, 1},
		{"Show/s1/S01E01E02.mkv", []int{1, 2}, 1},
		{"Show/Season 1 - The Beginning/S01E01.mkv", []int{1}, 1},
		{"Show/Season 01 - Title/1x02.mkv", []int{2}, 1},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := parseFile("/tv", filepath.Join("/tv", tc.path))
			if err != nil || !reflect.DeepEqual(got.episodes, tc.episodes) || got.season != tc.season {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	for _, path := range []string{
		"Show/Season 1/1920x1080.mkv",
		"Show/Season 1/[1920X1080].mkv",
		"Show/Season 1/[640x480] [1920x1080].mkv",
		"Show/Season 1/S01E01 1x03 [1920x1080].mkv",
		"Show/Season 1/[1920x1080] S01E01 S01E02.mkv",
		"Show/Season 1/S01E01 - 4 Days Out 1x03.mkv",
		"Show/Season 1/S01E01-03 [1920x1080].mkv",
		"Show/Season 1/S01E01-E [1920x1080].mkv",
		"Show/Season 1/S01E01- 03.mkv",
		"Show/Season 1/S01E01 -03.mkv",
		"Show/Season 1/S01E01\t-03.mkv",
		"Show/Season 1/S01E01-\t03.mkv",
		"Show/Season 1/[1920x1080] S02E01.mkv",
		"Show/Season 1/[1920x1080] S01E04-E02.mkv",
		"Show/Season 1/[1920x1080] S01E01 E03.mkv",
		"Show/Season 1/S01E01E99999.mkv",
		"Show/Season 1/S01E99999 - 4 Days Out.mkv",
		"Show/Season 1/S00001E01.mkv",
		"Show/Season 1/10001x02.mkv",
		"Show/Season 1/1x00002.mkv",
		"Show/Season 1/S01E01 19200x1080.mkv",
		"Show/Season 1/S01E01 1920x10800.mkv",
		"Show/Season 1/S01E01 a1920x1080.mkv",
		"Show/Season 1/S01E01 1920x1080p.mkv",
		"Show/Season 1/S01E01 99x1080.mkv",
		"Show/Season 1/S01E01 1920x99.mkv",
		"Show/Season 1/Season1920x1080.mkv",
		"Show/Season 1/1920x1080p.mkv",
		"Show/Season 1/[1920x1080] S01E01E99999.mkv",
		"Show/Season 1/[1920x1080] aS01E01.mkv",
		"Show/Season 1/S01E04-E02.mkv", "Show/Season 1/S02E01.mkv", "Show/Season 1/S01E01-S02E02.mkv", "Show/Season 1/S01E01 1x03.mkv", "Show/Season 1/S01E01E01.mkv", "Show/Season 1/S01E01-03.mkv", "Show/Season 1/S01E01-E.mkv", "Show/Season 1/S01E00.mkv", "Show/Season 1/S01E01E00.mkv", "Show/Season 1/S01E01 E03.mkv", "Show/Season 1/S01E01_E03.mkv", "Show/Season 1/2020-01-02.mkv", "Show/Season 1/001.mkv", "Show/Season 1/extras/S01E01.mkv", ".backup/Show/Season 1/S01E01.mkv", "Show/Season 1/.S01E01.mkv", "Show/Specials/S01E01.mkv", "Show/S01/S02E01.mkv", "Show/Season1/S01E01.mkv", "Show/Season 1 -Title/S01E01.mkv", "Show/Season/S01E01.mkv", "S01E01.mkv", "Show/.S01E01.mkv", "Show/Season 1/Sub/S01E01.mkv",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := parseFile("/tv", filepath.Join("/tv", path))
			if err == nil {
				t.Fatal("accepted malformed path")
			}
		})
	}
}

func TestParseShowTitle(t *testing.T) {
	for _, tc := range []struct {
		folder, title, full string
		year                int
	}{
		{"Show (2020)", "Show", "", 2020},
		{"Show", "Show", "", 0},
		{"Space 1999", "Space", "Space 1999", 1999},
		{"Show.Name_2020", "Show Name", "Show Name 2020", 2020},
		{"1984", "1984", "", 0},
		{"The Office (US) (2005)", "The Office (US)", "", 2005},
		{"Blade Runner 2049 (2017)", "Blade Runner 2049", "", 2017},
	} {
		t.Run(tc.folder, func(t *testing.T) {
			title, year, full := parseShowTitle(tc.folder)
			if title != tc.title || year != tc.year || full != tc.full {
				t.Fatalf("title=%q year=%d full=%q", title, year, full)
			}
		})
	}
}

func TestParseSeasonDirectoryAndAllowDirectory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		season int
		ok     bool
	}{
		{"Season 1", 1, true}, {"season 01", 1, true}, {"Season 0", 0, true}, {"Specials", 0, true}, {"SPECIALS", 0, true},
		{"S01", 1, true}, {"s1", 1, true}, {"Season 12 - The Long Winter", 12, true},
		{"Season1", 0, false}, {"Season 1 -Title", 0, false}, {"Season 1 - ", 0, false}, {"Seasons 1", 0, false}, {"S", 0, false}, {"Season", 0, false}, {"Extras", 0, false}, {"Series 1", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			season, ok := parseSeasonDirectory(tc.name)
			if ok != tc.ok || season != tc.season {
				t.Fatalf("season=%d ok=%v", season, ok)
			}
			if allowDirectory("/tv", filepath.Join("/tv/Show", tc.name)) != tc.ok {
				t.Fatal("allowDirectory disagrees with parseSeasonDirectory")
			}
		})
	}
	for path, want := range map[string]bool{"Show": true, ".hidden": false, "Show/.git": false, "Show/Season 1/Sub": false, "Show/Specials": true} {
		if allowDirectory("/tv", filepath.Join("/tv", path)) != want {
			t.Fatalf("allowDirectory(%q) != %v", path, want)
		}
	}
}
