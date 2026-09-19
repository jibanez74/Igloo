package helpers

import "testing"

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

		if previousHeight != 0 && cfg.Height > previousHeight {
			t.Fatalf("profile %q (height %d) sorts after height %d", profile, cfg.Height, previousHeight)
		}
		previousHeight = cfg.Height
	}

	for profile := range HLSProfileConfigs {
		if !IsAllowedHLSProfile(profile) {
			t.Errorf("HLSProfileConfigs has entry %q that is not an allowed profile", profile)
		}
	}
}

func TestBestFitHLSFallbackProfile(t *testing.T) {
	tests := []struct {
		name         string
		sourceHeight int64
		want         string
	}{
		{name: "above the tallest profile", sourceHeight: 4320, want: HLS_PROFILE_2160P_16MBPS},
		{name: "exactly 2160p", sourceHeight: 2160, want: HLS_PROFILE_2160P_16MBPS},
		{name: "between 1080p and 2160p", sourceHeight: 1440, want: HLS_PROFILE_1080P_8MBPS},
		{name: "exactly 1080p", sourceHeight: 1080, want: HLS_PROFILE_1080P_8MBPS},
		{name: "between 720p and 1080p", sourceHeight: 900, want: HLS_PROFILE_720P_3MBPS},
		{name: "exactly 720p", sourceHeight: 720, want: HLS_PROFILE_720P_3MBPS},
		{name: "below every profile", sourceHeight: 480, want: HLS_PROFILE_720P_3MBPS},
		{name: "unknown source height", sourceHeight: 0, want: HLS_PROFILE_720P_3MBPS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BestFitHLSFallbackProfile(tt.sourceHeight)
			if got != tt.want {
				t.Fatalf("BestFitHLSFallbackProfile(%d) = %q, want %q", tt.sourceHeight, got, tt.want)
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

	got := BestFitHLSFallbackProfile(1080)
	if got != HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("BestFitHLSFallbackProfile(1080) = %q, want %q", got, HLS_PROFILE_1080P_8MBPS)
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
