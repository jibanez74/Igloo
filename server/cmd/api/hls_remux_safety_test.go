package main

import (
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
)

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
