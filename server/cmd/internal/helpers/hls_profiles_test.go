package helpers

import (
	"fmt"
	"testing"
)

func TestIsAllowedHLSProfile(t *testing.T) {
	for _, profile := range HLSAllowedProfiles {
		if !IsAllowedHLSProfile(profile) {
			t.Errorf("IsAllowedHLSProfile(%q) = false, want true", profile)
		}
	}

	rejected := []string{"", "1080p", "4320p_40mbps", "REMUX", " remux "}
	for _, profile := range rejected {
		if IsAllowedHLSProfile(profile) {
			t.Errorf("IsAllowedHLSProfile(%q) = true, want false", profile)
		}
	}
}

// Every non-remux allowed profile must have a usable config entry, and the list
// must run from the tallest profile to the shortest. BestFitHLSFallbackProfile
// walks the list in order and compares against the config's height, so an id
// with no entry — or a list out of order — would hand the wrong profile to
// every playback session.
func TestHLSAllowedProfilesAreConfiguredAndOrderedByDescendingHeight(t *testing.T) {
	previousHeight := 0
	previousMbps := 0
	for i, profile := range HLSAllowedProfiles {
		if profile == HLS_PROFILE_REMUX {
			if i != 0 {
				t.Fatalf("HLS_PROFILE_REMUX is at index %d, want 0", i)
			}
			continue
		}

		cfg, ok := HLSProfileConfigs[profile]
		if !ok {
			t.Fatalf("allowed profile %q has no HLSProfileConfigs entry", profile)
		}
		if cfg.ID != profile {
			t.Errorf("HLSProfileConfigs[%q].ID = %q, want %q", profile, cfg.ID, profile)
		}
		if cfg.Height <= 0 {
			t.Fatalf("HLSProfileConfigs[%q].Height = %d, want > 0", profile, cfg.Height)
		}
		if cfg.VideoBitrate == "" || cfg.Bufsize == "" {
			t.Errorf("HLSProfileConfigs[%q] is missing a bitrate or bufsize", profile)
		}

		// The catalog handler and the fallback's bitrate comparison read
		// VideoMbps while FFmpeg gets VideoBitrate, so the two must agree.
		if cfg.VideoBitrate != fmt.Sprintf("%dM", cfg.VideoMbps) {
			t.Errorf("HLSProfileConfigs[%q] bitrate %q does not match VideoMbps %d", profile, cfg.VideoBitrate, cfg.VideoMbps)
		}

		if previousHeight != 0 && cfg.Height > previousHeight {
			t.Fatalf("profile %q (height %d) sorts after height %d", profile, cfg.Height, previousHeight)
		}

		// BestFitHLSFallbackProfile walks one height's profiles in list order
		// and takes the first whose bitrate fits the source, so equal heights
		// must run from the richest bitrate to the cheapest. Out of order, it
		// would hand back a lower profile than the source can justify.
		if cfg.Height == previousHeight && cfg.VideoMbps >= previousMbps {
			t.Fatalf("profile %q (%d Mbps) sorts after %d Mbps at the same height", profile, cfg.VideoMbps, previousMbps)
		}

		previousHeight = cfg.Height
		previousMbps = cfg.VideoMbps
	}

	for profile := range HLSProfileConfigs {
		if !IsAllowedHLSProfile(profile) {
			t.Errorf("HLSProfileConfigs has entry %q that is not an allowed profile", profile)
		}
	}
}

func TestBestFitHLSFallbackProfile(t *testing.T) {
	const mbps = 1_000_000

	tests := []struct {
		name          string
		sourceHeight  int64
		sourceBitRate int64
		want          string
	}{
		{name: "above the tallest profile", sourceHeight: 4320, sourceBitRate: 40 * mbps, want: HLS_PROFILE_2160P_16MBPS},
		{name: "exactly 2160p", sourceHeight: 2160, sourceBitRate: 20 * mbps, want: HLS_PROFILE_2160P_16MBPS},
		{name: "between 1080p and 2160p", sourceHeight: 1440, sourceBitRate: 12 * mbps, want: HLS_PROFILE_1080P_8MBPS},
		{name: "1080p above every profile bitrate", sourceHeight: 1080, sourceBitRate: 25 * mbps, want: HLS_PROFILE_1080P_8MBPS},
		{name: "1080p just above the richest profile", sourceHeight: 1080, sourceBitRate: 8*mbps + 500_000, want: HLS_PROFILE_1080P_8MBPS},
		{name: "1080p exactly at the richest profile", sourceHeight: 1080, sourceBitRate: 8 * mbps, want: HLS_PROFILE_1080P_8MBPS},
		{name: "1080p at the middle profile", sourceHeight: 1080, sourceBitRate: 6 * mbps, want: HLS_PROFILE_1080P_6MBPS},
		{name: "1080p between the middle and cheapest profiles", sourceHeight: 1080, sourceBitRate: 5 * mbps, want: HLS_PROFILE_1080P_4MBPS},
		{name: "1080p below every profile bitrate", sourceHeight: 1080, sourceBitRate: 2 * mbps, want: HLS_PROFILE_1080P_4MBPS},
		{name: "unknown bitrate keeps the tallest profile at that height", sourceHeight: 1080, sourceBitRate: 0, want: HLS_PROFILE_1080P_8MBPS},
		{name: "a single-profile height ignores bitrate", sourceHeight: 2160, sourceBitRate: 1 * mbps, want: HLS_PROFILE_2160P_16MBPS},
		{name: "between 720p and 1080p", sourceHeight: 900, sourceBitRate: 5 * mbps, want: HLS_PROFILE_720P_3MBPS},
		{name: "exactly 720p", sourceHeight: 720, sourceBitRate: 4 * mbps, want: HLS_PROFILE_720P_3MBPS},
		{name: "below every profile", sourceHeight: 480, sourceBitRate: 1 * mbps, want: HLS_PROFILE_720P_3MBPS},
		{name: "unknown source height", sourceHeight: 0, sourceBitRate: 0, want: HLS_PROFILE_720P_3MBPS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BestFitHLSFallbackProfile(tt.sourceHeight, tt.sourceBitRate)
			if got != tt.want {
				t.Fatalf("BestFitHLSFallbackProfile(%d, %d) = %q, want %q", tt.sourceHeight, tt.sourceBitRate, got, tt.want)
			}
		})
	}
}

// A profile id in the allowed list with no config must be skipped, not treated
// as height 0 — which would match every source and win for all playback.
func TestBestFitHLSFallbackProfileSkipsUnconfiguredProfiles(t *testing.T) {
	original := HLSAllowedProfiles
	t.Cleanup(func() { HLSAllowedProfiles = original })

	HLSAllowedProfiles = append([]string{"4320p_40mbps"}, original...)

	got := BestFitHLSFallbackProfile(1080, 0)
	if got != HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("BestFitHLSFallbackProfile(1080, 0) = %q, want %q", got, HLS_PROFILE_1080P_8MBPS)
	}
}

func TestIsBrowserCompatibleH264(t *testing.T) {
	t.Parallel()

	tests := []struct {
		codec string
		want  bool
	}{
		{"h264", true},
		{"H264", true},
		{"h.264", true},
		{"avc", true},
		{"avc1", true},
		{" avc1 ", true},
		{"hevc", false},
		{"h265", false},
		{"av1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			t.Parallel()

			got := IsBrowserCompatibleH264(tt.codec)
			if got != tt.want {
				t.Errorf("IsBrowserCompatibleH264(%q) = %v, want %v", tt.codec, got, tt.want)
			}
		})
	}
}
