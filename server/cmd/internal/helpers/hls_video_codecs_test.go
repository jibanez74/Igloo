package helpers

import "testing"

func TestParseHLSVideoCodecs(t *testing.T) {
	tests := []struct {
		value string
		want  HLSVideoCodecs
		ok    bool
	}{
		{value: "hevc", want: HLSVideoCodecs{HEVC: true}, ok: true},
		{value: " hevc ", want: HLSVideoCodecs{HEVC: true}, ok: true},
		{value: "hevc,hevc", want: HLSVideoCodecs{HEVC: true}, ok: true},
		// H.264 is implied, never named.
		{value: "h264"},
		{value: "HEVC"},
		{value: "h265"},
		{value: "hevc,av1"},
		{value: "hevc,"},
		{value: ""},
	}

	for _, tt := range tests {
		got, ok := ParseHLSVideoCodecs(tt.value)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseHLSVideoCodecs(%q) = (%+v, %v), want (%+v, %v)", tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

func TestHLSVideoCodecsForms(t *testing.T) {
	var none HLSVideoCodecs
	hevc := HLSVideoCodecs{HEVC: true}

	if none.QueryValue() != "" || hevc.QueryValue() != "hevc" {
		t.Errorf("QueryValue() = %q, %q; want \"\", \"hevc\"", none.QueryValue(), hevc.QueryValue())
	}
	if none.SessionKey() == hevc.SessionKey() {
		t.Errorf("SessionKey() is %q for both", none.SessionKey())
	}
}

func TestHLSCopyVideoTag(t *testing.T) {
	for codec, want := range map[string]string{"hevc": "hvc1", " HEVC ": "hvc1", "h265": "hvc1", "h264": "", "av1": "", "": ""} {
		got := HLSCopyVideoTag(codec)
		if got != want {
			t.Errorf("HLSCopyVideoTag(%q) = %q, want %q", codec, got, want)
		}
	}
}
