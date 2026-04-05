package main

import (
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestParseSegmentIndex(t *testing.T) {
	tests := []struct {
		filename string
		wantIdx  int64
		wantErr  bool
	}{
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 0, false},
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "42" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 42, false},
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "900" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 900, false},
		{"init.mp4", 0, true},
		{"bad_name.m4s", 0, true},
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "abc" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 0, true},
	}
	for _, tt := range tests {
		idx, err := parseSegmentIndex(tt.filename)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSegmentIndex(%q) expected error, got idx=%d", tt.filename, idx)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSegmentIndex(%q) unexpected error: %v", tt.filename, err)
			continue
		}
		if idx != tt.wantIdx {
			t.Errorf("parseSegmentIndex(%q) = %d, want %d", tt.filename, idx, tt.wantIdx)
		}
	}
}

func TestHLSSessionKey(t *testing.T) {
	key := HLSSessionKey(123, "720p_3mbps", 2)
	want := "123:720p_3mbps:2"
	if key != want {
		t.Errorf("HLSSessionKey = %q, want %q", key, want)
	}
}

func TestIsAllowedHLSFilename(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"init.mp4", true},
		{"segment_0.m4s", true},
		{"segment_999.m4s", true},
		{"segment_.m4s", false},
		{"segment_abc.m4s", false},
		{"other.mp4", false},
		{"../escape.m4s", false},
	}
	for _, tt := range tests {
		got := isAllowedHLSFilename(tt.name)
		if got != tt.ok {
			t.Errorf("isAllowedHLSFilename(%q) = %v, want %v", tt.name, got, tt.ok)
		}
	}
}

func TestStartSegmentComputation(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	tests := []struct {
		startSec     float64
		wantSegment  int64
	}{
		{0, 0},
		{segDur, 1},
		{segDur * 900, 900},
		{segDur*2 + 1, 2},
		{0.5, 0},
	}
	for _, tt := range tests {
		got := int64(tt.startSec / segDur)
		if got != tt.wantSegment {
			t.Errorf("startSec=%.1f -> segment=%d, want %d", tt.startSec, got, tt.wantSegment)
		}
	}
}
