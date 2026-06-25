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
