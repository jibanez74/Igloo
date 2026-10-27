package artwork

import (
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"The Matrix", "the_matrix"},
		{"Amélie", "amelie"},
		{"Spider-Man: No Way Home", "spider_man_no_way_home"},
		{"Schindler's List", "schindlers_list"},
		{"Ocean’s Eleven", "oceans_eleven"},
		{"  WALL·E  ", "wall_e"},
		{"2001: A Space Odyssey", "2001_a_space_odyssey"},
		{"千と千尋の神隠し", "untitled"},
		{"", "untitled"},
		{"!!!", "untitled"},
	}
	for _, tt := range tests {
		got := slug(tt.in)
		if got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	long := slug(strings.Repeat("word ", 40))
	if len(long) > maxSlugLength || strings.HasSuffix(long, "_") {
		t.Errorf("slug of a long title = %q (%d bytes), want at most %d without a trailing separator", long, len(long), maxSlugLength)
	}
}

func TestVersion_FollowsTheSourceAndSize(t *testing.T) {
	base := version("w500", "/poster.jpg")
	if len(base) != 8 {
		t.Fatalf("version = %q, want 8 hex digits", base)
	}
	if version("w500", "/poster.jpg") != base {
		t.Fatal("version is not stable for the same source and size")
	}
	if version("w500", "/other.jpg") == base || version("w780", "/poster.jpg") == base {
		t.Fatal("version does not change with the source or the size")
	}
}

func TestLocalURL_RoundTrips(t *testing.T) {
	value := localURL("movies/603/the_matrix_poster.jpg", "1a2b3c4d")
	if value != "/api/static/movies/603/the_matrix_poster.jpg?v=1a2b3c4d" {
		t.Fatalf("localURL = %q", value)
	}

	rel, ver, ok := parseLocalURL(value)
	if !ok || rel != "movies/603/the_matrix_poster.jpg" || ver != "1a2b3c4d" {
		t.Fatalf("parseLocalURL(%q) = %q, %q, %v", value, rel, ver, ok)
	}

	for _, other := range []string{"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg", "https://i.scdn.co/image/ab67", "/api/static/avatars/1-2.jpg", ""} {
		_, _, ok = parseLocalURL(other)
		if ok {
			t.Errorf("parseLocalURL(%q) took a value localURL did not make", other)
		}
	}
}

func TestSourceURL_KeepsMalformedPathsAndSVGRemote(t *testing.T) {
	const base = "https://image.tmdb.org/t/p"
	tests := []struct {
		source string
		want   string
	}{
		{"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg", base + "/w500/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg"},
		{"/logo.svg", ""},
		{"/LOGO.SVG", ""},
		{"no-slash.jpg", ""},
		{"/a/b.jpg", ""},
		{"/../secret.jpg", ""},
		{"/api/static/movies/603/the_matrix_poster.jpg?v=1", ""},
		{"https://image.tmdb.org/t/p/w500/x.jpg", ""},
	}
	for _, tt := range tests {
		got := moviePoster.sourceURL(base, tt.source)
		if got != tt.want {
			t.Errorf("sourceURL(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}

func TestIsManagedFile(t *testing.T) {
	tests := []struct {
		dir  string
		name string
		want bool
	}{
		{"movies", "the_matrix_poster.jpg", true},
		{"movies", "the_matrix_backdrop.webp", true},
		{"movies", "untitled_poster.png", true},
		{"movies", ".tmp-123456", true},
		{"people", "keanu_reeves_profile.jpg", true},
		{"shows", "the_wire_poster.jpg", true},
		{"shows", "season_00_poster.jpg", true},
		{"shows", "s01e02_still.jpg", true},
		{"networks", "hbo_logo.png", true},
		{"networks", "hbo_poster.png", false},
		{"shows", "s01e02_profile.jpg", false},
		{"movies", "keanu_reeves_profile.jpg", false},
		{"movies", "poster.jpg", false},
		{"movies", "The_Matrix_poster.jpg", false},
		{"movies", "the matrix_poster.jpg", false},
		{"movies", "the_matrix_poster.svg", false},
		{"movies", "the_matrix_poster.mkv", false},
		{"movies", "notes.txt", false},
	}
	for _, tt := range tests {
		got := isManagedFile(tt.dir, tt.name)
		if got != tt.want {
			t.Errorf("isManagedFile(%q, %q) = %v, want %v", tt.dir, tt.name, got, tt.want)
		}
	}
}
