package helpers

import "testing"

func TestIsMovieReleaseNoiseToken(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"1080p", true},
		{"WEB-DL", true},
		{"x265", true},
		{"remastered", true},
		{"extended", true},
		{"mkv", true},
		{"Moneyball", false},
		{"2011", false},
	}

	for _, tt := range tests {
		got := IsMovieReleaseNoiseToken(tt.token)
		if got != tt.want {
			t.Errorf("IsMovieReleaseNoiseToken(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}

func TestTitleAndYearFromFileName(t *testing.T) {
	tests := []struct {
		name      string
		fileName  string
		wantTitle string
		wantYear  int
	}{
		{
			name:      "parenthesised year",
			fileName:  "Moneyball (2011).mkv",
			wantTitle: "Moneyball",
			wantYear:  2011,
		},
		{
			name:      "parenthesised year with a full path",
			fileName:  "/media/movies/Arrival (2016)/Arrival (2016).mp4",
			wantTitle: "Arrival",
			wantYear:  2016,
		},
		{
			name:      "dotted release name",
			fileName:  "The.Social.Network.2010.1080p.BluRay.x264.mkv",
			wantTitle: "The Social Network",
			wantYear:  2010,
		},
		{
			name:      "dotted release name whose noise tokens look like years",
			fileName:  "Blade.Runner.2049.2017.2160p.WEB-DL.DDP.mkv",
			wantTitle: "Blade Runner 2049",
			wantYear:  2017,
		},
		{
			name:      "trailing year separated by spaces",
			fileName:  "Whiplash 2014.mp4",
			wantTitle: "Whiplash",
			wantYear:  2014,
		},
		{
			name:      "no year at all",
			fileName:  "Some.Untitled.Feature.mkv",
			wantTitle: "Some Untitled Feature",
			wantYear:  0,
		},
		{
			name:      "four-digit trailing number that is not a plausible year",
			fileName:  "Ocean's 1234.mkv",
			wantTitle: "Ocean's 1234",
			wantYear:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TitleAndYearFromFileName(tt.fileName)
			if err != nil {
				t.Fatalf("TitleAndYearFromFileName(%q) returned error: %v", tt.fileName, err)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Year != tt.wantYear {
				t.Errorf("year = %d, want %d", got.Year, tt.wantYear)
			}
		})
	}
}

func TestTitleAndYearFromFileNameRejectsAnEmptyName(t *testing.T) {
	for _, fileName := range []string{"", ".mkv", "   .mkv"} {
		got, err := TitleAndYearFromFileName(fileName)
		if err == nil {
			t.Errorf("TitleAndYearFromFileName(%q) = %+v, want an error", fileName, got)
		}
	}
}

func TestFileExtension(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"movie.mkv", "mkv"},
		{"MOVIE.MKV", "mkv"},
		{"/media/movies/Arrival (2016).mp4", "mp4"},
		{"archive.tar.gz", "gz"},
		{"no-extension", ""},
		{"", ""},
		{".hidden", "hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := FileExtension(tt.path)
			if got != tt.want {
				t.Errorf("FileExtension(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsReasonableYear(t *testing.T) {
	tests := []struct {
		year int
		want bool
	}{
		{1899, false},
		{1900, true},
		{2011, true},
		{2100, true},
		{2101, false},
		{0, false},
		{-2011, false},
	}

	for _, tt := range tests {
		got := IsReasonableYear(tt.year)
		if got != tt.want {
			t.Errorf("IsReasonableYear(%d) = %v, want %v", tt.year, got, tt.want)
		}
	}
}
