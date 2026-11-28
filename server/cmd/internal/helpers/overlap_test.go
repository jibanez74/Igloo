package helpers

import "testing"

func TestPathsOverlap(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"/srv/static/movies", "/srv/static/movies", true},
		{"/srv/static/movies", "/srv/static", true},
		{"/srv/static", "/srv/static/movies/603", true},
		{"/srv/static/movies", "/", true},
		{"/", "/srv/static/movies", true},
		{"/srv/static/movies", "/srv/static/movies-old", false},
		{"/srv/static/movies", "/srv/media", false},
		{"/srv/static/..movies", "/srv/static", true},
	}
	for _, tt := range tests {
		got := PathsOverlap(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("PathsOverlap(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
