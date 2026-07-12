package helpers

import "testing"

func TestIsBitmapSubtitleCodec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		codec string
		want  bool
	}{
		{name: "PGS", codec: "hdmv_pgs_subtitle", want: true},
		{name: "DVD", codec: "dvd_subtitle", want: true},
		{name: "DVB case insensitive", codec: "DVB_SUBTITLE", want: true},
		{name: "text subtitle", codec: "subrip", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsBitmapSubtitleCodec(tt.codec)
			if got != tt.want {
				t.Fatalf("IsBitmapSubtitleCodec(%q) = %v, want %v", tt.codec, got, tt.want)
			}
		})
	}
}
