package ffprobe

import "testing"

func TestParseKeyframePacket(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantPts float64
		wantKey bool
		wantOK  bool
	}{
		{name: "keyframe row", line: "12.500000,K__", wantPts: 12.5, wantKey: true, wantOK: true},
		{name: "non-keyframe row", line: "13.250000,___", wantPts: 13.25, wantKey: false, wantOK: true},
		{name: "keyframe with discard flag", line: "0.000000,K_D", wantPts: 0, wantKey: true, wantOK: true},
		{name: "unset pts", line: "N/A,K__", wantOK: false},
		{name: "empty line", line: "", wantOK: false},
		{name: "single field", line: "12.500000", wantOK: false},
		{name: "non-numeric pts", line: "garbage,K__", wantOK: false},
		{name: "surrounding whitespace", line: "  7.000000,K__  \r", wantPts: 7, wantKey: true, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pts, isKeyframe, ok := parseKeyframePacket(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if pts != tt.wantPts {
				t.Errorf("pts = %v, want %v", pts, tt.wantPts)
			}
			if isKeyframe != tt.wantKey {
				t.Errorf("isKeyframe = %v, want %v", isKeyframe, tt.wantKey)
			}
		})
	}
}
