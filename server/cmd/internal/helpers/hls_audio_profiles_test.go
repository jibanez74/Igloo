package helpers

import "testing"

func TestParseHLSAudioCodec(t *testing.T) {
	tests := []struct {
		value string
		want  HLSAudioCodec
		ok    bool
	}{
		{value: "ac3", want: HLSAudioCodecAC3, ok: true},
		{value: "eac3", want: HLSAudioCodecEAC3, ok: true},
		{value: " ac3 ", want: HLSAudioCodecAC3, ok: true},
		// AAC output stays legacy-only and is never a valid explicit request.
		{value: "aac"},
		{value: "AC3"},
		{value: "e-ac-3"},
		{value: "opus"},
		{value: ""},
	}

	for _, tt := range tests {
		got, ok := ParseHLSAudioCodec(tt.value)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseHLSAudioCodec(%q) = (%q, %v), want (%q, %v)", tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsAllowedHLSAudioMaxChannels(t *testing.T) {
	for channels, want := range map[int]bool{0: false, 1: false, 2: true, 4: false, 6: true, 8: false} {
		if got := IsAllowedHLSAudioMaxChannels(channels); got != want {
			t.Errorf("IsAllowedHLSAudioMaxChannels(%d) = %v, want %v", channels, got, want)
		}
	}
}

// The full server-owned bitrate table from the specification.
func TestHLSAudioBitrate(t *testing.T) {
	tests := []struct {
		codec    HLSAudioCodec
		channels int
		want     string
	}{
		{HLSAudioCodecAC3, 1, "192k"},
		{HLSAudioCodecAC3, 2, "384k"},
		{HLSAudioCodecAC3, 3, "448k"},
		{HLSAudioCodecAC3, 4, "448k"},
		{HLSAudioCodecAC3, 5, "640k"},
		{HLSAudioCodecAC3, 6, "640k"},
		{HLSAudioCodecEAC3, 1, "192k"},
		{HLSAudioCodecEAC3, 2, "384k"},
		{HLSAudioCodecEAC3, 3, "512k"},
		{HLSAudioCodecEAC3, 4, "512k"},
		{HLSAudioCodecEAC3, 5, "768k"},
		{HLSAudioCodecEAC3, 6, "768k"},
	}

	for _, tt := range tests {
		if got := HLSAudioBitrate(tt.codec, tt.channels); got != tt.want {
			t.Errorf("HLSAudioBitrate(%s, %d) = %q, want %q", tt.codec, tt.channels, got, tt.want)
		}
	}
}

func TestHLSAudioEncoder(t *testing.T) {
	if got := HLSAudioEncoder(HLSAudioCodecAC3); got != "ac3" {
		t.Errorf("HLSAudioEncoder(ac3) = %q, want ac3", got)
	}
	if got := HLSAudioEncoder(HLSAudioCodecEAC3); got != "eac3" {
		t.Errorf("HLSAudioEncoder(eac3) = %q, want eac3", got)
	}
	// The legacy AAC marker has no explicit encoder mapping.
	if got := HLSAudioEncoder(HLSAudioCodecAAC); got != "" {
		t.Errorf("HLSAudioEncoder(aac) = %q, want empty", got)
	}
}

func TestResolveHLSAudioProfile(t *testing.T) {
	tests := []struct {
		name           string
		request        HLSAudioProfileRequest
		sourceChannels int
		sourceLayout   string
		want           HLSResolvedAudioProfile
	}{
		{
			name:           "5.1 within a maximum of six keeps its layout",
			request:        HLSAudioProfileRequest{Codec: HLSAudioCodecEAC3, MaxChannels: 6},
			sourceChannels: 6,
			sourceLayout:   "5.1(side)",
			want: HLSResolvedAudioProfile{
				Codec: HLSAudioCodecEAC3, Encoder: "eac3", Channels: 6,
				ChannelLayout: "5.1(side)", Bitrate: "768k", SampleRate: 48000,
			},
		},
		{
			name:           "mono is never upmixed",
			request:        HLSAudioProfileRequest{Codec: HLSAudioCodecAC3, MaxChannels: 6},
			sourceChannels: 1,
			sourceLayout:   "mono",
			want: HLSResolvedAudioProfile{
				Codec: HLSAudioCodecAC3, Encoder: "ac3", Channels: 1,
				ChannelLayout: "mono", Bitrate: "192k", SampleRate: 48000,
			},
		},
		{
			name:           "downmix to two produces standard stereo",
			request:        HLSAudioProfileRequest{Codec: HLSAudioCodecAC3, MaxChannels: 2},
			sourceChannels: 6,
			sourceLayout:   "5.1(side)",
			want: HLSResolvedAudioProfile{
				Codec: HLSAudioCodecAC3, Encoder: "ac3", Channels: 2,
				ChannelLayout: "stereo", Bitrate: "384k", SampleRate: 48000,
			},
		},
		{
			name:           "7.1 downmixes to standard 5.1",
			request:        HLSAudioProfileRequest{Codec: HLSAudioCodecEAC3, MaxChannels: 6},
			sourceChannels: 8,
			sourceLayout:   "7.1",
			want: HLSResolvedAudioProfile{
				Codec: HLSAudioCodecEAC3, Encoder: "eac3", Channels: 6,
				ChannelLayout: "5.1", Bitrate: "768k", SampleRate: 48000,
			},
		},
		{
			name:           "a missing stored layout resolves the standard name",
			request:        HLSAudioProfileRequest{Codec: HLSAudioCodecAC3, MaxChannels: 6},
			sourceChannels: 5,
			sourceLayout:   "",
			want: HLSResolvedAudioProfile{
				Codec: HLSAudioCodecAC3, Encoder: "ac3", Channels: 5,
				ChannelLayout: "5.0", Bitrate: "640k", SampleRate: 48000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveHLSAudioProfile(tt.request, tt.sourceChannels, tt.sourceLayout)
			if got != tt.want {
				t.Errorf("ResolveHLSAudioProfile() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
