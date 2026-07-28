package main

import (
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
)

func TestIsBrowserSafeH264RemuxCandidate_PixelFormats(t *testing.T) {
	tests := []struct {
		pixelFormat string
		wantSafe    bool
	}{
		{"yuv420p", true},
		{"yuvj420p", true},
		// 8-bit 4:2:0 names that a "contains 10/12" marker list misread.
		{"nv12", true},
		{"nv21", true},
		{"", true},
		{"yuv420p10le", false},
		{"yuv422p", false},
		{"yuv444p", false},
		{"gray", false},
	}

	for _, tt := range tests {
		t.Run(tt.pixelFormat, func(t *testing.T) {
			stream := database.VideoStream{
				Codec:        "h264",
				CodecProfile: sql.NullString{String: "High", Valid: true},
				BitDepth:     sql.NullInt64{Int64: 8, Valid: true},
				PixelFormat:  sql.NullString{String: tt.pixelFormat, Valid: tt.pixelFormat != ""},
			}

			safe, reason := isBrowserSafeH264RemuxCandidate(&stream)
			if safe != tt.wantSafe {
				t.Fatalf("pixel format %q: got safe=%t (%s), want %t", tt.pixelFormat, safe, reason, tt.wantSafe)
			}
		})
	}
}

func TestRemuxSafetyFingerprint_ChangesWithStreamProperties(t *testing.T) {
	baseMovie := database.Movie{ID: 7, Size: 1_000_000, UpdatedAt: "2026-07-01"}
	baseVideo := database.VideoStream{
		StreamIndex:  0,
		Codec:        "h264",
		CodecProfile: sql.NullString{String: "High", Valid: true},
		BitDepth:     sql.NullInt64{Int64: 8, Valid: true},
		PixelFormat:  sql.NullString{String: "yuv420p", Valid: true},
	}

	baseKey := remuxSafetyFingerprint(&baseMovie, &baseVideo)
	if got := remuxSafetyFingerprint(&baseMovie, &baseVideo); got != baseKey {
		t.Fatalf("fingerprint not stable: %q vs %q", got, baseKey)
	}

	tests := []struct {
		name   string
		mutate func(m *database.Movie, v *database.VideoStream)
	}{
		{"codec", func(_ *database.Movie, v *database.VideoStream) { v.Codec = "hevc" }},
		{"codec profile", func(_ *database.Movie, v *database.VideoStream) {
			v.CodecProfile = sql.NullString{String: "High 10", Valid: true}
		}},
		{"bit depth", func(_ *database.Movie, v *database.VideoStream) {
			v.BitDepth = sql.NullInt64{Int64: 10, Valid: true}
		}},
		{"pixel format", func(_ *database.Movie, v *database.VideoStream) {
			v.PixelFormat = sql.NullString{String: "yuv420p10le", Valid: true}
		}},
		{"movie size", func(m *database.Movie, _ *database.VideoStream) { m.Size = 2_000_000 }},
		{"updated at", func(m *database.Movie, _ *database.VideoStream) { m.UpdatedAt = "2026-07-02" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movie := baseMovie
			video := baseVideo
			tt.mutate(&movie, &video)
			if got := remuxSafetyFingerprint(&movie, &video); got == baseKey {
				t.Fatalf("fingerprint unchanged after %s change", tt.name)
			}
		})
	}
}
