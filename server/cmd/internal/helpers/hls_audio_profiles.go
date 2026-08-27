package helpers

import "strings"

// HLSAudioCodec identifies an HLS audio output codec by its public API value.
type HLSAudioCodec string

const (
	// HLSAudioCodecAAC only describes legacy output (copied or stereo-encoded
	// AAC) in diagnostics; ParseHLSAudioCodec never accepts it from a request.
	HLSAudioCodecAAC  HLSAudioCodec = "aac"
	HLSAudioCodecAC3  HLSAudioCodec = "ac3"
	HLSAudioCodecEAC3 HLSAudioCodec = "eac3"
)

const (
	// HLS_AUDIO_MAX_CHANNELS_STEREO and HLS_AUDIO_MAX_CHANNELS_SURROUND are the
	// only values audio_channels accepts: at most stereo, or at most 5.1.
	HLS_AUDIO_MAX_CHANNELS_STEREO   = 2
	HLS_AUDIO_MAX_CHANNELS_SURROUND = 6

	// HLS_EXPLICIT_AUDIO_SAMPLE_RATE is the output sample rate for explicit
	// AC-3/E-AC-3 encodes. Legacy AAC output never sets a sample rate.
	HLS_EXPLICIT_AUDIO_SAMPLE_RATE = 48000

	// HLS_LEGACY_AUDIO_BITRATE is the legacy stereo AAC fallback bitrate. It is
	// deliberately not part of the explicit bitrate table below.
	HLS_LEGACY_AUDIO_BITRATE = "320k"
)

// HLSAudioProfileRequest is the validated audio_codec/audio_channels query
// pair. A nil *HLSAudioProfileRequest means legacy audio behavior.
type HLSAudioProfileRequest struct {
	Codec       HLSAudioCodec
	MaxChannels int
}

// HLSResolvedAudioProfile is the audio configuration a session actually runs:
// the encoder and its typed arguments for explicit encodes, or a description
// of the legacy copy/transcode output for diagnostics. Only resolved values
// ever reach FFmpeg argument construction; raw query strings never do.
type HLSResolvedAudioProfile struct {
	Codec         HLSAudioCodec
	Encoder       string
	Channels      int
	ChannelLayout string
	Bitrate       string
	SampleRate    int
	Copy          bool
}

// hlsAudioEncoderByCodec maps public codec values to FFmpeg encoder names.
// Encoder strings must come from this table after validation, never from a
// query value.
var hlsAudioEncoderByCodec = map[HLSAudioCodec]string{
	HLSAudioCodecAC3:  "ac3",
	HLSAudioCodecEAC3: "eac3",
}

// HLSAudioEncoder returns the FFmpeg encoder for an explicit output codec, or
// "" for values with no explicit encoder (including the legacy AAC marker).
func HLSAudioEncoder(codec HLSAudioCodec) string {
	return hlsAudioEncoderByCodec[codec]
}

// ParseHLSAudioCodec validates a requested audio_codec value. Only the
// explicit Dolby codecs are accepted; AAC stays a legacy-only behavior and is
// rejected like any other unknown value.
func ParseHLSAudioCodec(value string) (HLSAudioCodec, bool) {
	switch strings.TrimSpace(value) {
	case string(HLSAudioCodecAC3):
		return HLSAudioCodecAC3, true
	case string(HLSAudioCodecEAC3):
		return HLSAudioCodecEAC3, true
	default:
		return "", false
	}
}

// IsAllowedHLSAudioMaxChannels reports whether an audio_channels value is one
// of the supported maximums.
func IsAllowedHLSAudioMaxChannels(channels int) bool {
	return channels == HLS_AUDIO_MAX_CHANNELS_STEREO || channels == HLS_AUDIO_MAX_CHANNELS_SURROUND
}

// HLSAudioBitrate selects the encoding bitrate from the output codec and the
// effective (post-resolution) channel count. The value is in FFmpeg's -b:a
// form and doubles as the logging/header representation.
func HLSAudioBitrate(codec HLSAudioCodec, channels int) string {
	eac3 := codec == HLSAudioCodecEAC3
	switch {
	case channels <= 1:
		return "192k"
	case channels == 2:
		return "384k"
	case channels <= 4:
		if eac3 {
			return "512k"
		}
		return "448k"
	default:
		if eac3 {
			return "768k"
		}
		return "640k"
	}
}

// hlsDefaultChannelLayoutName names the standard layout for a channel count,
// used when the source row stored no layout or the output is a downmix.
func hlsDefaultChannelLayoutName(channels int) string {
	switch channels {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	case 3:
		return "3.0"
	case 4:
		return "4.0"
	case 5:
		return "5.0"
	default:
		return "5.1"
	}
}

// ResolveHLSAudioProfile turns a validated explicit request plus the selected
// source stream's stored channel metadata into the profile FFmpeg runs.
//
// MaxChannels is a ceiling, never a target: a mono or stereo source is never
// upmixed, a source within the ceiling keeps its channel count and stored
// layout, and a source above it is downmixed to standard stereo or 5.1.
// FFmpeg's -ac conversion rematrixes through libswresample, so center,
// surround, and LFE content participate in every downmix rather than channels
// being dropped. The caller must have verified sourceChannels > 0.
func ResolveHLSAudioProfile(
	request HLSAudioProfileRequest,
	sourceChannels int,
	sourceChannelLayout string,
) HLSResolvedAudioProfile {
	channels := sourceChannels
	if channels > request.MaxChannels {
		channels = request.MaxChannels
	}

	layout := strings.TrimSpace(sourceChannelLayout)
	downmixed := channels != sourceChannels
	if downmixed || layout == "" {
		layout = hlsDefaultChannelLayoutName(channels)
	}

	return HLSResolvedAudioProfile{
		Codec:         request.Codec,
		Encoder:       HLSAudioEncoder(request.Codec),
		Channels:      channels,
		ChannelLayout: layout,
		Bitrate:       HLSAudioBitrate(request.Codec, channels),
		SampleRate:    HLS_EXPLICIT_AUDIO_SAMPLE_RATE,
	}
}
