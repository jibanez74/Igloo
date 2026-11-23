package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"igloo/cmd/internal/fakebin"
	"igloo/cmd/internal/helpers"
)

func TestBuildHLSArgs_ReadratePacing(t *testing.T) {
	tests := []struct {
		name         string
		caps         Capabilities
		wantReadrate bool
		wantBurst    bool
	}{
		{
			name: "unsupported build omits readrate",
		},
		{
			name: "readrate without burst support",
			caps: Capabilities{
				Probed:     true,
				CLIOptions: map[string]bool{"readrate": true},
			},
			wantReadrate: true,
		},
		{
			name: "supported build paces input reads",
			caps: Capabilities{
				Probed: true,
				CLIOptions: map[string]bool{
					"readrate":               true,
					"readrate_initial_burst": true,
				},
			},
			wantReadrate: true,
			wantBurst:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := basicHLSParams(testHLSOutDir)
			params.Capabilities = tt.caps
			args := hlsArgs(t, params)

			if !tt.wantReadrate {
				if slices.Contains(args, "-readrate") {
					t.Fatalf("readrate must be omitted when the ffmpeg build does not support it: %v", args)
				}
				return
			}

			requireArgumentValue(t, args, "-readrate", fmt.Sprintf("%d", hlsReadrateSpeed))
			// -readrate is an input option, so it must precede -i.
			requireArgumentBefore(t, args, "-readrate", "-i")

			if !tt.wantBurst {
				if slices.Contains(args, "-readrate_initial_burst") {
					t.Fatalf("burst must be omitted when unsupported: %v", args)
				}
				return
			}
			requireArgumentValue(t, args, "-readrate_initial_burst", fmt.Sprintf("%d", hlsReadrateInitialBurstSec))
		})
	}
}

// Codec selection is driven by the profile plus the CopyVideo/CopyAudio flags;
// each case pins the encoder, filter, and audio arguments the combination must
// and must not produce.
func TestBuildHLSArgs_CodecSelection(t *testing.T) {
	type codecCase struct {
		name       string
		sourcePath string
		profile    string
		device     string
		videoIndex int
		audioIndex int
		copyVideo  bool
		copyAudio  bool
		videoTag   string
		want       []string
		notWant    []string
		notFlags   []string
	}

	tests := []codecCase{
		{
			name:       "transcodes video and audio",
			sourcePath: "/safe/source.mkv",
			profile:    helpers.HLS_PROFILE_1080P_4MBPS,
			audioIndex: 1,
			want: []string{
				"-map 0:0", "-map 0:1",
				"libx264", "-preset veryfast",
				"-sc_threshold:v:0 0",
				"-force_key_frames:0 expr:gte(t,n_forced*4)",
				"scale=-2:1080",
				"-c:a aac", "-ac 2", "-b:a 320k",
				"-avoid_negative_ts make_zero", "-fflags +genpts",
				"-hls_segment_type fmp4", "-hls_playlist_type event",
				"-hls_flags independent_segments",
			},
			// No explicit thread cap: libx264 auto-detects its thread count and
			// the concurrency limiter bounds total CPU pressure. A stray
			// -threads before -i would throttle only the decoder while leaving
			// the encoder unbounded.
			notFlags: []string{"-threads"},
		},
		{
			name:       "copies audio and transcodes video",
			profile:    helpers.HLS_PROFILE_1080P_4MBPS,
			audioIndex: 0,
			copyAudio:  true,
			want:       []string{"libx264", "-c:a copy"},
			notWant:    []string{"-b:a"},
		},
		{
			name:       "copies both streams",
			profile:    helpers.HLS_PROFILE_720P_3MBPS,
			audioIndex: 0,
			copyVideo:  true,
			copyAudio:  true,
			want:       []string{"-c:v copy", "-c:a copy"},
			// Copy output splits on whatever keyframes the source carries, and
			// the remux validator only samples four fragments, so FFmpeg must
			// not stamp the playlist as independently decodable throughout.
			notWant:  []string{"libx264", "-hwaccel", "independent_segments", "-tag:v"},
			notFlags: []string{"-hls_flags"},
		},
		{
			name:       "copies video and transcodes audio",
			profile:    helpers.HLS_PROFILE_1080P_8MBPS,
			audioIndex: 1,
			copyVideo:  true,
			want:       []string{"-c:v copy", "-c:a aac", "-ac 2", "-b:a 320k"},
			notWant: []string{
				"libx264", "-hwaccel", "scale=",
				"-b:v", "-maxrate", "-bufsize",
				"-sc_threshold", "-force_key_frames",
			},
		},
		{
			name:       "remux transcodes audio to AAC",
			profile:    helpers.HLS_PROFILE_REMUX,
			audioIndex: 0,
			want:       []string{"-c:v copy", "-c:a aac"},
			notWant: []string{
				"libx264", "h264_videotoolbox", "h264_nvenc", "h264_qsv",
				"-hwaccel", "scale=", "-sc_threshold", "-force_key_frames",
				"independent_segments",
			},
		},
		{
			name:       "remux copies audio when asked",
			profile:    helpers.HLS_PROFILE_REMUX,
			audioIndex: 0,
			copyAudio:  true,
			want:       []string{"-c:v copy", "-c:a copy"},
		},
		{
			// Choosing a non-first audio track on a direct-playable file
			// resolves to remux, so remux must map the selected absolute index
			// while still copying the video.
			name:       "remux maps a selected audio track",
			sourcePath: "/s.mp4",
			profile:    helpers.HLS_PROFILE_REMUX,
			audioIndex: 4,
			copyVideo:  true,
			copyAudio:  true,
			want:       []string{"-map 0:4", "-c:v copy", "-c:a copy"},
		},
		{
			// Safari refuses FFmpeg's default hev1 sample entry.
			name:       "remux tags copied HEVC hvc1",
			profile:    helpers.HLS_PROFILE_REMUX,
			audioIndex: 1,
			videoTag:   "hvc1",
			want:       []string{"-c:v copy -tag:v hvc1"},
		},
		{
			// A transcode writes its own H.264, so the source's tag is moot.
			name:       "transcode ignores the copy tag",
			profile:    helpers.HLS_PROFILE_1080P_8MBPS,
			audioIndex: 1,
			videoTag:   "hvc1",
			want:       []string{"libx264"},
			notFlags:   []string{"-tag:v"},
		},
		{
			name:       "video-only input omits audio options",
			sourcePath: "/media/video only.mkv",
			profile:    helpers.HLS_PROFILE_720P_3MBPS,
			videoIndex: 4,
			audioIndex: -1,
			want:       []string{"-map 0:4"},
			notFlags:   []string{"-c:a", "-b:a", "-ac"},
		},
	}

	// Remux copies the video stream regardless of the configured accelerator.
	for _, device := range []string{
		helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
	} {
		tests = append(tests, codecCase{
			name:       "remux ignores hardware acceleration/" + device,
			profile:    helpers.HLS_PROFILE_REMUX,
			device:     device,
			audioIndex: 1,
			want:       []string{"-c:v copy"},
			notWant: []string{
				"-hwaccel", "h264_videotoolbox", "h264_nvenc", "h264_qsv", "libx264",
			},
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath := tt.sourcePath
			if sourcePath == "" {
				sourcePath = "/s"
			}
			device := tt.device
			if device == "" {
				device = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
			}
			params := basicHLSParams(testHLSOutDir)
			params.SourcePath = sourcePath
			params.Profile = tt.profile
			params.VideoStreamIndex = tt.videoIndex
			params.AudioStreamIndex = tt.audioIndex
			params.HWDevice = device
			params.CopyVideo = tt.copyVideo
			params.CopyAudio = tt.copyAudio
			params.CopyVideoTag = tt.videoTag
			params.Capabilities = Capabilities{}
			args := hlsArgs(t, params)

			requireArgumentValue(t, args, "-i", sourcePath)
			if args[len(args)-1] != filepath.Join(testHLSOutDir, helpers.HLS_PLAYLIST_FILENAME) {
				t.Fatalf("last arg = %q, want the playlist path", args[len(args)-1])
			}
			requireArgSubstrings(t, args, tt.want, tt.notWant, tt.notFlags)
		})
	}
}

// Copy-video sessions cannot filter, so a deinterlace request must not leak a
// -vf into the remux command; the gate keeps interlaced sources off remux in
// the first place.
func TestBuildHLSArgs_RemuxIgnoresDeinterlace(t *testing.T) {
	params := basicHLSParams(testHLSOutDir)
	params.Profile = helpers.HLS_PROFILE_REMUX
	params.Deinterlace = true
	args := hlsArgs(t, params)

	requireArgSubstrings(t, args, []string{"-c:v copy"}, []string{"yadif"}, []string{"-vf"})
}

func TestBuildHLSArgs_SeekOffset(t *testing.T) {
	tests := []struct {
		name      string
		startSec  float64
		device    string
		copyVideo bool
		// remuxOnly sets the remux profile without CopyVideo, which still
		// copies the video.
		remuxOnly bool
		wantSS    string
		wantOrder [][2]string
		// wantInexact expects -noaccurate_seek, which only copy-video sessions
		// use so that encoded audio starts at the copied video's keyframe.
		wantInexact bool
	}{
		{
			name:      "positive offset precedes the input",
			startSec:  3600,
			wantSS:    "3600.000",
			wantOrder: [][2]string{{"-ss", "-i"}},
		},
		{
			name:        "copy video seeks inexactly so audio starts at its keyframe",
			startSec:    1790,
			copyVideo:   true,
			wantSS:      "1790.000",
			wantOrder:   [][2]string{{"-noaccurate_seek", "-ss"}, {"-ss", "-i"}},
			wantInexact: true,
		},
		{
			name:        "remux profile seeks inexactly without CopyVideo",
			startSec:    1790,
			remuxOnly:   true,
			wantSS:      "1790.000",
			wantOrder:   [][2]string{{"-noaccurate_seek", "-ss"}, {"-ss", "-i"}},
			wantInexact: true,
		},
		{
			name:      "copy video from zero needs no seek",
			startSec:  0,
			copyVideo: true,
		},
		{
			name:     "zero offset omits the flag",
			startSec: 0,
		},
		{
			name:     "negative offset omits the flag",
			startSec: -12.5,
		},
		{
			name:      "offset follows hardware decode setup",
			startSec:  120.5,
			device:    helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			wantSS:    "120.500",
			wantOrder: [][2]string{{"-hwaccel", "-ss"}, {"-ss", "-i"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := tt.device
			if device == "" {
				device = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
			}
			params := basicHLSParams(testHLSOutDir)
			params.HWDevice = device
			params.StartSec = tt.startSec
			params.Capabilities = Capabilities{}
			if tt.copyVideo {
				params.Profile = helpers.HLS_PROFILE_REMUX
				params.CopyVideo = true
			}
			if tt.remuxOnly {
				params.Profile = helpers.HLS_PROFILE_REMUX
				params.CopyVideo = false
			}
			args := hlsArgs(t, params)

			if got := slices.Contains(args, "-noaccurate_seek"); got != tt.wantInexact {
				t.Fatalf("-noaccurate_seek present = %v, want %v: %v", got, tt.wantInexact, args)
			}
			if tt.wantSS == "" {
				if slices.Contains(args, "-ss") {
					t.Fatalf("-ss must be omitted for start offset %v: %v", tt.startSec, args)
				}
				return
			}
			requireArgumentValue(t, args, "-ss", tt.wantSS)
			for _, order := range tt.wantOrder {
				requireArgumentBefore(t, args, order[0], order[1])
			}
		})
	}
}

func TestBuildHLSArgs_RejectsInvalidParams(t *testing.T) {
	outDir := testHLSOutDir
	tests := []struct {
		name    string
		params  HLSParams
		wantErr string
	}{
		{
			name: "disallowed profile",
			params: HLSParams{
				SourcePath: "/s", OutDir: outDir, Profile: "4k_20mbps",
				HWDevice: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			},
			wantErr: "invalid HLS profile",
		},
		{
			name: "empty source path",
			params: HLSParams{
				SourcePath: "", OutDir: outDir, Profile: helpers.HLS_PROFILE_720P_3MBPS,
				HWDevice: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			},
			wantErr: "source path is required",
		},
		{
			name: "blank source path",
			params: HLSParams{
				SourcePath: "   ", OutDir: outDir, Profile: helpers.HLS_PROFILE_720P_3MBPS,
				HWDevice: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			},
			wantErr: "source path is required",
		},
		{
			name: "negative video stream index",
			params: HLSParams{
				SourcePath: "/s", OutDir: outDir, Profile: helpers.HLS_PROFILE_720P_3MBPS,
				VideoStreamIndex: -1, HWDevice: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			},
			wantErr: "video stream index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildHLSArgs(tt.params)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// The mapped stream indices are pinned by the codec selection cases; this
// only guards their order, since the first -map decides which stream FFmpeg
// treats as the video track.
func TestBuildHLSArgs_GlobalStreamIndices(t *testing.T) {
	params := basicHLSParams(testHLSOutDir)
	params.VideoStreamIndex = 3
	params.AudioStreamIndex = 7
	args := hlsArgs(t, params)

	requireArgumentBefore(t, args, "0:3", "0:7")
}

func TestBuildHLSArgs_AllProfileConfigs(t *testing.T) {
	for profileID, cfg := range helpers.HLSProfileConfigs {
		t.Run(profileID, func(t *testing.T) {
			params := basicHLSParams(testHLSOutDir)
			params.Profile = profileID
			args := hlsArgs(t, params)

			requireArgSubstrings(t, args, []string{
				fmt.Sprintf("scale=-2:%d", cfg.Height),
				fmt.Sprintf("-b:v %s", cfg.VideoBitrate),
				fmt.Sprintf("-maxrate %s", cfg.VideoBitrate),
				fmt.Sprintf("-bufsize %s", cfg.Bufsize),
			}, nil, nil)
		})
	}
}

func TestBuildHLSArgs_UnknownAndBlankHardwareUseCPU(t *testing.T) {
	for _, device := range []string{"", "not-a-device"} {
		t.Run(device, func(t *testing.T) {
			params := basicHLSParams(testHLSOutDir)
			params.AudioStreamIndex = -1
			params.HWDevice = device
			args := hlsArgs(t, params)
			if !slices.Contains(args, "libx264") || slices.Contains(args, "-hwaccel") {
				t.Fatalf("device %q did not use CPU fallback: %v", device, args)
			}
		})
	}
}

func TestBuildHLSArgs_HLSOutputStructure(t *testing.T) {
	outDir := testHLSOutDir
	tempFileCaps := Capabilities{
		Probed:     true,
		MuxerFlags: map[string]map[string]bool{"hls": {"temp_file": true}},
	}

	params := basicHLSParams(outDir)
	params.Capabilities = tempFileCaps
	args := hlsArgs(t, params)

	requireArgumentValue(t, args, "-f", "hls")
	requireArgumentValue(t, args, "-hls_segment_type", "fmp4")
	requireArgumentValue(t, args, "-hls_playlist_type", "event")
	// One merged -hls_flags occurrence: FFmpeg reads a single value, so a
	// second flag appended as its own -hls_flags would silently replace the
	// first.
	requireArgumentValue(t, args, "-hls_flags", "independent_segments+temp_file")
	if n := countArgument(args, "-hls_flags"); n != 1 {
		t.Errorf("-hls_flags appears %d times, want exactly 1: %v", n, args)
	}
	requireArgumentValue(t, args, "-hls_list_size", "0")
	requireArgumentValue(t, args, "-hls_time", fmt.Sprintf("%d", helpers.HLS_SEGMENT_TIME_SEC))
	requireArgumentValue(t, args, "-hls_fmp4_init_filename", helpers.HLS_INIT_FILENAME)
	requireArgumentValue(t, args, "-hls_segment_filename", filepath.Join(outDir, "segment_%d.m4s"))

	wantPlaylist := filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME)
	if args[len(args)-1] != wantPlaylist {
		t.Errorf("last arg = %q, want playlist path %q", args[len(args)-1], wantPlaylist)
	}

	t.Run("copy-video carries temp_file without the independence tag", func(t *testing.T) {
		params := basicHLSParams(outDir)
		params.Profile = helpers.HLS_PROFILE_REMUX
		params.AudioStreamIndex = -1
		params.Capabilities = tempFileCaps
		args := hlsArgs(t, params)
		requireArgumentValue(t, args, "-hls_flags", "temp_file")
	})

	t.Run("a muxer without temp_file keeps the legacy flags", func(t *testing.T) {
		args := hlsArgs(t, basicHLSParams(outDir))
		requireArgumentValue(t, args, "-hls_flags", "independent_segments")
	})
}

// resolveTestAudioProfile builds a resolved profile the way production does:
// through the server-owned tables, never from raw values.
func resolveTestAudioProfile(codec helpers.HLSAudioCodec, maxChannels, sourceChannels int, layout string) *helpers.HLSResolvedAudioProfile {
	profile := helpers.ResolveHLSAudioProfile(
		helpers.HLSAudioProfileRequest{Codec: codec, MaxChannels: maxChannels},
		sourceChannels,
		layout,
	)
	return &profile
}

// An explicit audio profile maps onto exactly the -c:a/-ch_layout/-b:a/-ar the
// resolved profile carries and wins over the legacy copy decision. How a
// request resolves (downmix, no upmix) is the helpers package's contract.
func TestBuildHLSArgs_ExplicitAudioProfiles(t *testing.T) {
	tests := []struct {
		name      string
		profile   string
		copyVideo bool
		copyAudio bool
		audio     *helpers.HLSResolvedAudioProfile
		want      []string
		notWant   []string
	}{
		{
			name:    "ac3 5.1",
			profile: helpers.HLS_PROFILE_1080P_4MBPS,
			audio:   resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 6, "5.1(side)"),
			want:    []string{"-c:a ac3", "-ch_layout:a 5.1(side)", "-b:a 640k", "-ar 48000"},
			notWant: []string{"-c:a aac", "-c:a copy", "320k", "-ac "},
		},
		{
			name:    "eac3 stereo",
			profile: helpers.HLS_PROFILE_720P_3MBPS,
			audio:   resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 2, 6, "5.1(side)"),
			want:    []string{"-c:a eac3", "-ch_layout:a stereo", "-b:a 384k", "-ar 48000"},
			notWant: []string{"-c:a aac", "-c:a copy", "-ac "},
		},
		{
			// The legacy copy decision must not leak into explicit mode even if
			// a caller sets both: the explicit profile wins.
			name:      "explicit profile overrides a copy-safe AAC source",
			profile:   helpers.HLS_PROFILE_720P_3MBPS,
			copyAudio: true,
			audio:     resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 6, "5.1(side)"),
			want:      []string{"-c:a ac3", "-ch_layout:a 5.1(side)", "-b:a 640k"},
			notWant:   []string{"-c:a copy"},
		},
		{
			// A remux request keeps copying video while the explicit profile
			// encodes audio.
			name:      "remux copies video while encoding explicit audio",
			profile:   helpers.HLS_PROFILE_REMUX,
			copyVideo: true,
			audio:     resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 6, "5.1(side)"),
			want:      []string{"-c:v copy", "-c:a eac3", "-ch_layout:a 5.1(side)", "-b:a 768k", "-ar 48000"},
			notWant:   []string{"-c:a copy"},
		},
		{
			// -ac 3 would let the encoder pick 3.0(back) and drop the LFE.
			name:    "2.1 names its layout instead of a channel count",
			profile: helpers.HLS_PROFILE_1080P_4MBPS,
			audio:   resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 3, "2.1"),
			want:    []string{"-c:a ac3", "-ch_layout:a 2.1", "-b:a 448k", "-ar 48000"},
			notWant: []string{"-ac "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := basicHLSParams(testHLSOutDir)
			params.Profile = tt.profile
			params.CopyVideo = tt.copyVideo
			params.CopyAudio = tt.copyAudio
			params.AudioProfile = tt.audio
			args := hlsArgs(t, params)
			tt.want = append(tt.want, "-af aresample=out_chlayout="+tt.audio.ChannelLayout+":lfe_mix_level=0.70710678:rematrix_maxval=1")

			requireArgSubstrings(t, args, tt.want, tt.notWant, nil)
		})
	}
}

// Explicit Dolby audio is padded back to output time zero, so a source whose
// audio starts late still has a packet when the first segment closes. A
// copy-video seek starts audio below zero, where the pad would trim it, so it
// keeps the plain filter; legacy AAC output never gets an aresample filter.
func TestBuildHLSArgs_ExplicitAudioPadsStart(t *testing.T) {
	const plain = "aresample=out_chlayout=5.1(side):lfe_mix_level=0.70710678:rematrix_maxval=1"
	const padded = plain + ":async=1:first_pts=0:min_hard_comp=1000000"
	tests := []struct {
		name       string
		startSec   float64
		copyVideo  bool
		remuxOnly  bool
		legacy     bool
		copyAudio  bool
		wantFilter string
	}{
		{name: "transcode from zero", wantFilter: padded},
		{name: "transcode seek", startSec: 1790, wantFilter: padded},
		{name: "copy video from zero", copyVideo: true, wantFilter: padded},
		{name: "copy video seek", startSec: 1790, copyVideo: true, wantFilter: plain},
		{name: "remux profile seek without CopyVideo", startSec: 1790, remuxOnly: true, wantFilter: plain},
		{name: "legacy AAC encode", legacy: true},
		{name: "legacy AAC copy", legacy: true, copyAudio: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := basicHLSParams(testHLSOutDir)
			params.StartSec = tt.startSec
			params.CopyAudio = tt.copyAudio
			if tt.copyVideo || tt.remuxOnly {
				params.Profile = helpers.HLS_PROFILE_REMUX
				params.CopyVideo = tt.copyVideo
			}
			if !tt.legacy {
				params.AudioProfile = resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 6, "5.1(side)")
			}
			args := hlsArgs(t, params)

			if tt.wantFilter == "" {
				requireArgSubstrings(t, args, nil, []string{"aresample", "first_pts"}, nil)
				return
			}
			requireArgumentValue(t, args, "-af", tt.wantFilter)
		})
	}
}

// The argument builder only accepts profiles whose fields match the
// server-owned tables, so raw or tampered values can never reach FFmpeg.
func TestBuildHLSArgs_RejectsInvalidAudioProfile(t *testing.T) {
	valid := func() *helpers.HLSResolvedAudioProfile {
		return resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 6, "5.1(side)")
	}

	tests := []struct {
		name    string
		mutate  func(profile *helpers.HLSResolvedAudioProfile)
		wantErr string
	}{
		{
			name:    "empty encoder",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.Encoder = "" },
			wantErr: "invalid HLS audio encoder",
		},
		{
			name:    "encoder not matching the codec table",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.Encoder = "libmp3lame" },
			wantErr: "invalid HLS audio encoder",
		},
		{
			name:    "zero channels",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.Channels = 0 },
			wantErr: "invalid HLS audio channel count",
		},
		{
			name:    "more than six channels",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.Channels = 8 },
			wantErr: "invalid HLS audio",
		},
		{
			name:    "layout not from the layout table",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.ChannelLayout = "5.1:out_sample_rate=8000" },
			wantErr: "invalid HLS audio channel layout",
		},
		{
			name:    "layout with a different channel count",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.ChannelLayout = "stereo" },
			wantErr: "invalid HLS audio channel layout",
		},
		{
			name:    "bitrate not from the profile table",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.Bitrate = "999k" },
			wantErr: "invalid HLS audio bitrate",
		},
		{
			name:    "sample rate other than 48 kHz",
			mutate:  func(p *helpers.HLSResolvedAudioProfile) { p.SampleRate = 44100 },
			wantErr: "invalid HLS audio sample rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := valid()
			tt.mutate(profile)

			params := basicHLSParams(testHLSOutDir)
			params.AudioProfile = profile
			_, err := buildHLSArgs(params)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// Without -nostats FFmpeg's \r-separated progress report reaches the stderr
// tail as one ever-growing line, which buried the real failure in the log.
func TestBuildHLSArgs_SuppressesProgressAndStdin(t *testing.T) {
	for _, profile := range []string{helpers.HLS_PROFILE_720P_3MBPS, helpers.HLS_PROFILE_REMUX} {
		params := basicHLSParams(testHLSOutDir)
		params.Profile = profile
		args := hlsArgs(t, params)
		for _, flag := range []string{"-nostats", "-nostdin"} {
			if countArgument(args, flag) != 1 {
				t.Errorf("%s: %s appears %d times, want 1: %v", profile, flag, countArgument(args, flag), args)
			}
		}
	}
}

// Each case pins the decode, filter, and encode arguments produced for one
// combination of configured device, HDR tone-mapping, and probed capabilities.
// Devices are covered even where the current code path is device-independent,
// so a future device-specific change cannot slip through.
func TestBuildHLSArgs_DevicePaths(t *testing.T) {
	intelRuntimeProbeFailed := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	intelRuntimeProbeFailed.H264QSVRuntimeUsable = false
	intelRuntimeProbeFailed.H264QSVProbeError = "no qsv device"

	sdrScale := fmt.Sprintf("scale=-2:%d", helpers.HLSProfileConfigs[helpers.HLS_PROFILE_720P_3MBPS].Height)

	tests := []struct {
		name        string
		device      string
		profile     string
		tonemap     bool
		deinterlace bool
		bt601       bool
		caps        Capabilities
		want        []string
		notWant     []string
		notFlags    []string
		wantOrder   [][2]string
	}{
		// --- SDR, no probed hardware filter support ---
		{
			name:     "cpu encodes with libx264",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			want:     []string{"libx264", sdrScale, "format=yuv420p"},
			notFlags: []string{"-hwaccel"},
		},
		{
			name:      "apple decodes and encodes with VideoToolbox",
			device:    helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			caps:      hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_APPLE),
			want:      []string{"-hwaccel videotoolbox", "h264_videotoolbox", sdrScale},
			notWant:   []string{"-rc vbr", "-look_ahead"},
			wantOrder: [][2]string{{"-hwaccel", "-i"}},
		},
		{
			// -forced-idr is what turns NVENC's forced keyframes into IDR
			// frames; without it they are plain intra frames that later frames
			// may reference across, which breaks segment-level random access.
			name:   "nvidia decodes with cuda and encodes with NVENC",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:   hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA),
			want: []string{
				"-hwaccel cuda", "h264_nvenc", "-rc vbr", "-preset p4",
				"-forced-idr 1", "-hls_flags independent_segments", sdrScale,
			},
		},
		{
			// A build whose NVENC lacks the option cannot promise IDR frames,
			// so the playlist must drop the independence claim with it.
			name:     "nvidia omits forced-idr and the independence tag when unprobed",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:     nvidiaWithoutForcedIDR(),
			want:     []string{"h264_nvenc", "-rc vbr", "-preset p4"},
			notWant:  []string{"-forced-idr", "independent_segments"},
			notFlags: []string{"-hls_flags"},
		},
		{
			name:   "intel encodes with QSV and software scale",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:   hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL),
			want: []string{
				"h264_qsv", "-look_ahead 1", "-forced_idr 1",
				sdrScale + ",format=nv12",
			},
			// QSV decode is deliberately not enabled, and h264_qsv takes nv12
			// frames rather than a forced yuv420p pixel format.
			notWant:  []string{"-hwaccel qsv", "-pix_fmt yuv420p", "hwupload", "scale_qsv"},
			notFlags: []string{"-init_hw_device", "-filter_hw_device"},
		},

		// --- SDR with probed hardware filter support ---
		{
			name:   "nvidia uses scale_cuda when probed",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:   hlsTestNvidiaCapabilities(false),
			want: []string{
				"-init_hw_device cuda=igloo_cuda", "-filter_hw_device igloo_cuda",
				"-hwaccel cuda",
				"format=nv12,setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709,hwupload,scale_cuda=w=-2:h=720:format=yuv420p",
			},
			notWant:   []string{"zscale"},
			wantOrder: [][2]string{{"-init_hw_device", "-filter_hw_device"}, {"-filter_hw_device", "-i"}},
		},
		{
			name:   "intel uses scale_qsv when probed",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:   hlsTestIntelQSVScaleCapabilities(),
			want: []string{
				"-init_hw_device qsv=igloo_qsv", "-filter_hw_device igloo_qsv",
				"format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=720:format=nv12",
			},
			notWant:   []string{"-hwaccel qsv", sdrScale, "-pix_fmt yuv420p"},
			wantOrder: [][2]string{{"-init_hw_device", "-filter_hw_device"}, {"-filter_hw_device", "-i"}},
		},

		// --- HDR tone-mapping ---
		{
			name:    "cpu tone-maps in software",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			profile: helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap: true,
			want:    []string{"zscale", "tonemap=tonemap=hable", "bt709", "libx264"},
			// The plain scale= filter belongs to the SDR path only.
			notWant:  []string{"scale=-2:"},
			notFlags: []string{"-hwaccel"},
		},
		{
			// Decoded frames land in system memory, and scale_vt only takes
			// hardware frames, so they are uploaded to the VideoToolbox device
			// first; a -pix_fmt after the chain would ask for a software
			// conversion those frames cannot reach.
			name:    "apple tone-maps with scale_vt when probed",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			profile: helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap: true,
			caps:    hlsTestAppleVTCapabilities(),
			want: []string{
				"-init_hw_device videotoolbox=igloo_vt", "-filter_hw_device igloo_vt",
				"-hwaccel videotoolbox", "h264_videotoolbox",
				"format=p010le,hwupload,scale_vt=w=-2:h=1080:color_matrix=bt709:color_primaries=bt709:color_transfer=bt709",
			},
			notWant:   []string{"zscale", "tonemap=", "-pix_fmt yuv420p"},
			wantOrder: [][2]string{{"-init_hw_device", "-filter_hw_device"}, {"-filter_hw_device", "-i"}},
		},
		{
			// An unprobed scale_vt chain is never trusted: the hardware
			// encoder stays, the tone-map runs in software.
			name:     "apple tone-maps in software when scale_vt is unproven",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			profile:  helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap:  true,
			caps:     hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_APPLE),
			want:     []string{"-hwaccel videotoolbox", "zscale", "tonemap=tonemap=hable", "h264_videotoolbox", "-pix_fmt yuv420p"},
			notWant:  []string{"scale_vt", "hwupload"},
			notFlags: []string{"-init_hw_device", "-filter_hw_device"},
		},
		{
			// Software decode is required for the zscale chain, but -hwaccel
			// cuda without -hwaccel_output_format still downloads frames to
			// system memory, so decode acceleration stays on.
			name:    "nvidia falls back to software tone-map without CUDA filters",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			profile: helpers.HLS_PROFILE_720P_3MBPS,
			tonemap: true,
			caps:    hlsTestNvidiaCapabilities(false),
			want:    []string{"-hwaccel cuda", "zscale", "tonemap=tonemap=hable", "h264_nvenc"},
		},
		{
			name:    "nvidia tone-maps on the GPU when probed",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			profile: helpers.HLS_PROFILE_720P_3MBPS,
			tonemap: true,
			caps:    hlsTestNvidiaCapabilities(true),
			want: []string{
				"-init_hw_device cuda=igloo_cuda -filter_hw_device igloo_cuda",
				"-hwaccel cuda",
				"format=p010le,hwupload,scale_cuda=w=-2:h=720:format=p010",
				"tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0",
			},
		},
		{
			name:    "nvidia without probed filters tone-maps in software",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			profile: helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap: true,
			want:    []string{"zscale", "tonemap=tonemap=hable", "h264_nvenc"},
			// No probed capabilities means no cuda hwaccel either.
			notFlags: []string{"-hwaccel"},
		},
		{
			name:     "intel tone-maps in software and encodes with QSV",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			profile:  helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap:  true,
			caps:     hlsTestIntelQSVScaleCapabilities(),
			want:     []string{"zscale", "format=nv12", "h264_qsv"},
			notWant:  []string{"scale_qsv", "hwupload"},
			notFlags: []string{"-hwaccel", "-init_hw_device", "-filter_hw_device"},
		},

		// --- a failed runtime probe puts the arguments on the CPU path ---
		{
			name:     "intel falls back to CPU when the QSV runtime probe fails",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:     intelRuntimeProbeFailed,
			want:     []string{"libx264", sdrScale + ",format=yuv420p"},
			notWant:  []string{"h264_qsv", "-look_ahead", "-forced_idr", "format=nv12", "-hwaccel qsv"},
			notFlags: []string{"-hwaccel"},
		},
		{
			name:     "intel omits QSV encoder options the build does not expose",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:     intelWithoutForcedIDR(),
			want:     []string{"h264_qsv"},
			notWant:  []string{"independent_segments"},
			notFlags: []string{"-preset", "-look_ahead", "-forced_idr", "-hls_flags"},
		},

		// --- deinterlacing: software yadif prepends at the chain head, where
		// decoded frames are in system memory on every path (no chain sets
		// -hwaccel_output_format; CUDA/QSV hwupload from system memory) ---
		{
			name:        "cpu deinterlaces before scaling",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			deinterlace: true,
			want:        []string{"yadif," + sdrScale + ",format=yuv420p", "libx264"},
		},
		{
			name:        "cpu deinterlaces before the software tone-map",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			profile:     helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap:     true,
			deinterlace: true,
			want:        []string{"yadif,zscale", "tonemap=tonemap=hable"},
		},
		{
			name:        "apple SDR deinterlaces on the software chain",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			deinterlace: true,
			caps:        hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_APPLE),
			want:        []string{"yadif," + sdrScale, "h264_videotoolbox"},
		},
		{
			// The scale_vt probe proves only the progressive chain, so
			// interlaced HDR sources are routed to the software zscale chain.
			name:        "apple HDR deinterlace falls back to the software tone-map",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			profile:     helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap:     true,
			deinterlace: true,
			caps:        hlsTestAppleVTCapabilities(),
			want:        []string{"yadif,zscale", "tonemap=tonemap=hable", "h264_videotoolbox"},
			notWant:     []string{"scale_vt"},
			notFlags:    []string{"-init_hw_device"},
		},
		{
			name:        "nvidia deinterlaces before hwupload",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			deinterlace: true,
			caps:        hlsTestNvidiaCapabilities(false),
			want:        []string{"yadif,format=nv12,setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709,hwupload,scale_cuda=w=-2:h=720:format=yuv420p"},
		},
		{
			name:        "nvidia deinterlaces before the GPU tone-map",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			tonemap:     true,
			deinterlace: true,
			caps:        hlsTestNvidiaCapabilities(true),
			want: []string{
				"yadif,format=p010le,hwupload,scale_cuda=w=-2:h=720:format=p010",
				"tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0",
			},
		},
		{
			name:        "intel deinterlaces before hwupload",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			deinterlace: true,
			caps:        hlsTestIntelQSVScaleCapabilities(),
			want:        []string{"yadif,format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=720:format=nv12"},
		},

		// --- a BT.601 source has its matrix converted, not just relabelled ---
		{
			name:     "cpu converts BT.601 to BT.709 while scaling",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			bt601:    true,
			want:     []string{sdrScale + ":in_color_matrix=bt601:out_color_matrix=bt709,format=yuv420p,setparams="},
			notWant:  []string{"scale=in_color_matrix"},
			notFlags: []string{"-hwaccel"},
		},
		{
			name:    "intel software scale converts BT.601 into nv12",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			bt601:   true,
			caps:    hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL),
			want:    []string{sdrScale + ":in_color_matrix=bt601:out_color_matrix=bt709,format=nv12"},
			notWant: []string{"scale=in_color_matrix"},
		},
		{
			name:   "nvidia converts BT.601 before the CUDA upload",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			bt601:  true,
			caps:   hlsTestNvidiaCapabilities(false),
			want: []string{
				"scale=in_color_matrix=bt601:out_color_matrix=bt709,format=nv12,setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709,hwupload,scale_cuda=w=-2:h=720:format=yuv420p",
			},
		},
		{
			name:   "intel converts BT.601 before the QSV upload",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			bt601:  true,
			caps:   hlsTestIntelQSVScaleCapabilities(),
			want: []string{
				"scale=in_color_matrix=bt601:out_color_matrix=bt709,format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=720:format=nv12",
			},
		},
		{
			name:        "deinterlacing precedes the BT.601 conversion",
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			bt601:       true,
			deinterlace: true,
			caps:        hlsTestNvidiaCapabilities(false),
			want:        []string{"yadif,scale=in_color_matrix=bt601:out_color_matrix=bt709,format=nv12,"},
		},
		{
			// zscale reads the source matrix from the frame itself, and an
			// HDR source is BT.2020 in any case.
			name:    "the HDR tone-map ignores the BT.601 flag",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			profile: helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap: true,
			bt601:   true,
			want:    []string{"zscale", "tonemap=tonemap=hable"},
			notWant: []string{"in_color_matrix"},
		},
		{
			name:    "a BT.709 source is scaled without a matrix conversion",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			want:    []string{sdrScale + ",format=yuv420p"},
			notWant: []string{"in_color_matrix"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := tt.profile
			if profile == "" {
				profile = helpers.HLS_PROFILE_720P_3MBPS
			}
			params := basicHLSParams(testHLSOutDir)
			params.Profile = profile
			params.HWDevice = tt.device
			params.TonemapHDR = tt.tonemap
			params.Deinterlace = tt.deinterlace
			params.ConvertBT601 = tt.bt601
			params.Capabilities = tt.caps
			args := hlsArgs(t, params)

			requireArgSubstrings(t, args, tt.want, tt.notWant, tt.notFlags)
			for _, order := range tt.wantOrder {
				requireArgumentBefore(t, args, order[0], order[1])
			}
		})
	}
}

// Segment boundaries must land on IDR frames on every transcode path, and the
// copy paths must not try to force keyframes they cannot produce.
func TestBuildHLSArgs_KeyframeArgs(t *testing.T) {
	forceExpr := fmt.Sprintf("-force_key_frames:0 expr:gte(t,n_forced*%d)", helpers.HLS_SEGMENT_TIME_SEC)

	tests := []struct {
		name      string
		device    string
		profile   string
		copyVideo bool
		frameRate float64
		caps      Capabilities
		want      []string
		notWant   []string
	}{
		{
			name:   "transcode/cpu",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			want:   []string{forceExpr, "-sc_threshold:v:0 0"},
		},
		{
			name:    "transcode/apple",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			want:    []string{forceExpr},
			notWant: []string{"-sc_threshold"},
		},
		{
			name:    "transcode/nvidia",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			want:    []string{forceExpr},
			notWant: []string{"-sc_threshold"},
		},
		{
			name:    "transcode/intel",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			want:    []string{forceExpr},
			notWant: []string{"-sc_threshold"},
		},
		{
			// A 4-second GOP at 23.976fps is 96 frames, which drifts to ~4.004s
			// per segment. -g sizes the GOP; the expression is what pins
			// keyframes to the exact segment timestamps.
			name:      "hardware encoder with a non-integer frame rate",
			device:    helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			profile:   helpers.HLS_PROFILE_1080P_4MBPS,
			frameRate: 23.976,
			caps:      hlsTestNvidiaCapabilities(false),
			want:      []string{"-g:v:0 96", "-keyint_min:v:0 96", forceExpr},
			notWant:   []string{"-sc_threshold"},
		},
		{
			name:      "copy video",
			device:    helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			copyVideo: true,
			notWant:   []string{"-force_key_frames"},
		},
		{
			name:    "remux",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			profile: helpers.HLS_PROFILE_REMUX,
			notWant: []string{"-force_key_frames"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := tt.profile
			if profile == "" {
				profile = helpers.HLS_PROFILE_1080P_4MBPS
			}
			params := basicHLSParams(testHLSOutDir)
			params.Profile = profile
			params.HWDevice = tt.device
			params.CopyVideo = tt.copyVideo
			params.SourceFrameRate = tt.frameRate
			params.Capabilities = tt.caps
			args := hlsArgs(t, params)

			requireArgSubstrings(t, args, tt.want, tt.notWant, nil)
		})
	}
}

// #EXT-X-INDEPENDENT-SEGMENTS asserts that a player may start decoding at any
// segment. That only holds when -force_key_frames yields IDR frames, which
// libx264 and VideoToolbox always do but NVENC and QSV only do with their
// forced-IDR option. Copy-video is never independent: the remux validator
// samples HLS_REMUX_PREVALIDATE_SEGMENTS fragments at the session's start
// offset and cannot rule out a later GOP-structure change in the source.
func TestHLSSegmentsAreIndependent(t *testing.T) {
	nvidiaRuntimeProbeFailed := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA)
	nvidiaRuntimeProbeFailed.H264NVENCRuntimeUsable = false

	tests := []struct {
		name      string
		device    string
		profile   string
		copyVideo bool
		caps      Capabilities
		want      bool
	}{
		{name: "cpu transcode", device: helpers.HARDWARE_ACCELERATION_DEVICE_CPU, want: true},
		{
			name:   "apple transcode",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			caps:   hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_APPLE),
			want:   true,
		},
		{
			name:   "nvidia transcode with forced-idr",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:   hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA),
			want:   true,
		},
		{
			name:   "nvidia transcode without forced-idr",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:   nvidiaWithoutForcedIDR(),
			want:   false,
		},
		{
			name:   "intel transcode with forced_idr",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:   hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL),
			want:   true,
		},
		{
			name:   "intel transcode without forced_idr",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:   intelWithoutForcedIDR(),
			want:   false,
		},
		{
			// The device downgrades to libx264, which does force IDRs.
			name:   "nvidia falls back to CPU when the runtime probe fails",
			device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:   nvidiaRuntimeProbeFailed,
			want:   true,
		},
		{
			name:      "copy video",
			device:    helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			copyVideo: true,
			want:      false,
		},
		{
			name:    "remux profile",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			profile: helpers.HLS_PROFILE_REMUX,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := tt.profile
			if profile == "" {
				profile = helpers.HLS_PROFILE_720P_3MBPS
			}
			params := basicHLSParams(testHLSOutDir)
			params.Profile = profile
			params.HWDevice = tt.device
			params.CopyVideo = tt.copyVideo
			params.Capabilities = tt.caps
			got := HLSSegmentsAreIndependent(params)
			if got != tt.want {
				t.Fatalf("HLSSegmentsAreIndependent() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRunHLSRejectsInvalidOutputPaths(t *testing.T) {
	script := fakebin.Write(t, "fake ffmpeg", "exit 0\n")
	f := &ffmpeg{bin: script}
	filePath := filepath.Join(t.TempDir(), "output-file")
	err := os.WriteFile(filePath, []byte("not a directory"), 0644)
	if err != nil {
		t.Fatalf("write output file: %v", err)
	}

	tests := []struct {
		name    string
		outDir  string
		wantErr string
	}{
		{name: "empty", outDir: "   ", wantErr: "output directory is required"},
		{name: "missing", outDir: filepath.Join(t.TempDir(), "missing"), wantErr: "stat output directory"},
		{name: "file", outDir: filePath, wantErr: "not a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, runErr := f.RunHLS(context.Background(), basicHLSParams(tt.outDir), nil)
			if runErr == nil {
				t.Fatal("expected output path error")
			}
			if !strings.Contains(runErr.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", runErr.Error(), tt.wantErr)
			}
		})
	}
}

// The stderr reader stops quietly when the pipe is closed under it, which is
// what a failed Start does; any other scanner error is reported in the tail.
func TestIsExpectedHLSStderrClose(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "closed file", err: os.ErrClosed, want: true},
		{name: "closed pipe", err: io.ErrClosedPipe, want: true},
		{name: "wrapped path error", err: &fs.PathError{Op: "read", Path: "stderr", Err: os.ErrClosed}, want: true},
		{name: "token too long", err: bufio.ErrTooLong},
		{name: "end of file", err: io.EOF},
		{name: "other", err: errors.New("boom")},
		{name: "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpectedHLSStderrClose(tt.err); got != tt.want {
				t.Fatalf("isExpectedHLSStderrClose(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}

// A start failure returns before the exit goroutine exists, so by the time
// RunHLS has returned any callback would already be in the channel.
func TestRunHLSStartFailureDoesNotDeliverCallback(t *testing.T) {
	results := make(chan hlsExitResult, 1)
	f := &ffmpeg{bin: filepath.Join(t.TempDir(), "missing-ffmpeg")}

	_, err := f.RunHLS(context.Background(), basicHLSParams(t.TempDir()), func(exitErr error, stderrTail []string) {
		results <- hlsExitResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err == nil {
		t.Fatal("expected process start failure")
	}

	select {
	case result := <-results:
		t.Fatalf("callback delivered after start failure: %#v", result)
	default:
	}
}

func TestRunHLSDeliversExactlyOneCallbackForSuccessAndFailure(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantFailure bool
	}{
		{name: "success", body: "printf '%s\\n' success >&2\nexit 0\n"},
		{name: "failure", body: "printf '%s\\n' failure >&2\nexit 7\n", wantFailure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := fakebin.Write(t, "fake ffmpeg", tt.body)
			f := &ffmpeg{bin: script}
			results, _ := startHLS(t, context.Background(), f, basicHLSParams(t.TempDir()))

			result := waitForHLSExit(t, results)
			gotFailure := result.exitErr != nil
			if gotFailure != tt.wantFailure {
				t.Fatalf("exitErr = %v, want failure %v", result.exitErr, tt.wantFailure)
			}
			if len(result.stderrTail) != 1 || result.stderrTail[0] != tt.name {
				t.Fatalf("stderr tail = %v, want [%s]", result.stderrTail, tt.name)
			}

			// The callback runs once after Wait, so a second delivery would
			// already be buffered by the time the first one was received.
			select {
			case duplicate := <-results:
				t.Fatalf("duplicate callback: %#v", duplicate)
			default:
			}
		})
	}
}

func TestRunHLSCancellationTerminatesProcess(t *testing.T) {
	script := fakebin.Write(t, "fake ffmpeg", "printf '%s\\n' started >&2\nexec sleep 30\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &ffmpeg{bin: script}

	results, cmd := startHLS(t, ctx, f, basicHLSParams(t.TempDir()))
	cancel()

	result := waitForHLSExit(t, results)
	if result.exitErr == nil {
		t.Fatal("canceled FFmpeg exited successfully")
	}
	if cmd.ProcessState == nil {
		t.Fatal("canceled FFmpeg process was not reaped before callback")
	}
}

func TestRunHLSAllowsNilCallback(t *testing.T) {
	script := fakebin.Write(t, "fake ffmpeg", "exit 0\n")
	f := &ffmpeg{bin: script}

	cmd, err := f.RunHLS(context.Background(), basicHLSParams(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}
	waitForCommandExit(t, cmd)
}

func TestRunHLSRetainsOrderedLastTwentyStderrLines(t *testing.T) {
	script := fakebin.Write(t, "fake ffmpeg", `
i=1
while [ "$i" -le 25 ]; do
  printf 'line-%02d\n' "$i" >&2
  i=$((i + 1))
done
exit 3
`)
	f := &ffmpeg{bin: script}
	results, _ := startHLS(t, context.Background(), f, basicHLSParams(t.TempDir()))
	result := waitForHLSExit(t, results)

	if result.exitErr == nil {
		t.Fatal("expected nonzero exit")
	}
	if len(result.stderrTail) != hlsStderrTailLines {
		t.Fatalf("stderr tail length = %d, want %d", len(result.stderrTail), hlsStderrTailLines)
	}
	for i, line := range result.stderrTail {
		want := fmt.Sprintf("line-%02d", i+6)
		if line != want {
			t.Fatalf("stderr tail line %d = %q, want %q; tail=%v", i, line, want, result.stderrTail)
		}
	}
}

func TestRunHLSCapturesLongStderrLine(t *testing.T) {
	longLine := strings.Repeat("x", 128*1024)
	body := fmt.Sprintf("printf '%%s\\n' '%s' >&2\n", longLine)
	script := fakebin.Write(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}

	results, _ := startHLS(t, context.Background(), f, basicHLSParams(t.TempDir()))
	result := waitForHLSExit(t, results)
	if result.exitErr != nil {
		t.Fatalf("exitErr = %v, want nil", result.exitErr)
	}
	if len(result.stderrTail) != 1 {
		t.Fatalf("captured stderr line count = %d, want 1", len(result.stderrTail))
	}
	if len(result.stderrTail[0]) != len(longLine) {
		t.Fatalf("captured stderr line length = %d, want %d", len(result.stderrTail[0]), len(longLine))
	}
}

func TestRunHLSReportsStderrScannerErrors(t *testing.T) {
	script := fakebin.Write(t, "fake ffmpeg", "head -c 2000000 /dev/zero | tr '\\000' x >&2\nexit 1\n")
	f := &ffmpeg{bin: script}

	results, _ := startHLS(t, context.Background(), f, basicHLSParams(t.TempDir()))
	result := waitForHLSExit(t, results)
	if result.exitErr == nil {
		t.Fatal("expected nonzero exit")
	}
	hasScannerError := slices.ContainsFunc(result.stderrTail, func(line string) bool {
		return strings.Contains(line, "stderr scan error:") && strings.Contains(line, "token too long")
	})
	if !hasScannerError {
		t.Fatalf("stderr tail missing scanner error: %v", result.stderrTail)
	}
}

func TestRunHLSConcurrentSessionsKeepStateIsolated(t *testing.T) {
	script := fakebin.Write(t, "fake ffmpeg", `
tag=missing
previous=
for argument in "$@"; do
  if [ "$previous" = "-i" ]; then
    tag=$argument
    break
  fi
  previous=$argument
done
printf '%s-first\n' "$tag" >&2
printf '%s-last\n' "$tag" >&2
exit 0
`)
	f := &ffmpeg{bin: script}
	alpha := basicHLSParams(t.TempDir())
	alpha.SourcePath = "alpha source"
	beta := basicHLSParams(t.TempDir())
	beta.SourcePath = "beta source"

	alphaResults, _ := startHLS(t, context.Background(), f, alpha)
	betaResults, _ := startHLS(t, context.Background(), f, beta)

	alphaResult := waitForHLSExit(t, alphaResults)
	betaResult := waitForHLSExit(t, betaResults)
	if alphaResult.exitErr != nil || betaResult.exitErr != nil {
		t.Fatalf("concurrent exits: alpha=%v beta=%v", alphaResult.exitErr, betaResult.exitErr)
	}
	if strings.Join(alphaResult.stderrTail, " ") != "alpha source-first alpha source-last" {
		t.Fatalf("alpha stderr was not isolated: %v", alphaResult.stderrTail)
	}
	if strings.Join(betaResult.stderrTail, " ") != "beta source-first beta source-last" {
		t.Fatalf("beta stderr was not isolated: %v", betaResult.stderrTail)
	}
}

func TestRunHLSUsesAbsoluteOutputDirectoryAndInternalCapabilities(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	parent := t.TempDir()
	outDir := filepath.Join(parent, "output with spaces")
	err = os.Mkdir(outDir, 0755)
	if err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	relativeOutDir, err := filepath.Rel(cwd, outDir)
	if err != nil {
		t.Fatalf("relative output path: %v", err)
	}

	// The fake writes both files relative to its working directory, which is
	// how the test confirms RunHLS resolved and entered the absolute out dir.
	script := fakebin.Write(t, "fake ffmpeg", "printf '%s\\n' \"$PWD\" > pwd.txt\nprintf '%s\\n' \"$@\" > args.txt\n")
	f := &ffmpeg{
		bin:          script,
		capabilities: hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA),
	}
	params := basicHLSParams(relativeOutDir)
	params.HWDevice = helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA
	params.Capabilities = Capabilities{}

	results, _ := startHLS(t, context.Background(), f, params)
	result := waitForHLSExit(t, results)
	if result.exitErr != nil {
		t.Fatalf("exitErr = %v", result.exitErr)
	}

	pwdData, err := os.ReadFile(filepath.Join(outDir, "pwd.txt"))
	if err != nil {
		t.Fatalf("read pwd: %v", err)
	}
	if strings.TrimSpace(string(pwdData)) != outDir {
		t.Fatalf("command directory = %q, want %q", strings.TrimSpace(string(pwdData)), outDir)
	}

	args := fakebin.ReadLog(t, filepath.Join(outDir, "args.txt"))
	if !slices.Contains(args, "h264_nvenc") || !slices.Contains(args, "cuda") {
		t.Fatalf("internal capabilities were not used: %v", args)
	}
	requireArgumentValue(t, args, "-hls_segment_filename", filepath.Join(outDir, "segment_%d.m4s"))
	if args[len(args)-1] != filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME) {
		t.Fatalf("playlist path = %q", args[len(args)-1])
	}
}

func TestHLSSegmentCount(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)

	tests := []struct {
		name        string
		durationSec float64
		frameRate   float64
		want        int
	}{
		{name: "zero duration still yields one segment", durationSec: 0, frameRate: 24, want: 1},
		{name: "shorter than one frame yields one segment", durationSec: 0.01, frameRate: 24, want: 1},
		{name: "one full segment", durationSec: segDur, frameRate: 24, want: 1},
		{name: "one frame into the second segment", durationSec: segDur + 1.0/24.0, frameRate: 24, want: 2},
		{name: "two full segments", durationSec: segDur * 2, frameRate: 24, want: 2},
		{name: "partial last segment", durationSec: segDur*2 + 1, frameRate: 24, want: 3},
		// The container outlasts its 960th frame by five milliseconds of
		// audio; FFmpeg never sees a frame at 40 s and writes ten segments.
		{name: "sub-frame tail does not add a segment", durationSec: segDur*10 + 0.005, frameRate: 24, want: 10},
		{name: "frame exactly on the boundary adds a segment", durationSec: 961.0 / 24.0, frameRate: 24, want: 11},
		{name: "23.976 fps boundary frame", durationSec: 961.0 / (24000.0 / 1001.0), frameRate: 24000.0 / 1001.0, want: 11},
		{name: "60 fps sub-frame tail", durationSec: segDur*10 + 0.01, frameRate: 60, want: 10},
		{name: "unknown frame rate assumes 24 fps", durationSec: segDur*10 + 0.005, frameRate: 0, want: 10},
		{name: "unknown frame rate keeps a real boundary frame", durationSec: segDur*10 + 0.05, frameRate: 0, want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HLSSegmentCount(tt.durationSec, tt.frameRate)
			if got != tt.want {
				t.Fatalf("HLSSegmentCount(%v, %v) = %d, want %d", tt.durationSec, tt.frameRate, got, tt.want)
			}
		})
	}
}
