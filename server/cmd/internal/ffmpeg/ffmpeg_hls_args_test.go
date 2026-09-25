package ffmpeg

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
				"libx264", "-preset fast",
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
			notWant:  []string{"libx264", "-hwaccel", "independent_segments"},
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
		wantSS    string
		wantOrder [][2]string
	}{
		{
			name:      "positive offset precedes the input",
			startSec:  3600,
			wantSS:    "3600.000",
			wantOrder: [][2]string{{"-ss", "-i"}},
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
			args := hlsArgs(t, params)

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

// An explicit audio profile maps onto exactly the -c:a/-ac/-b:a/-ar the
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
			want:    []string{"-c:a ac3", "-ac 6", "-b:a 640k", "-ar 48000"},
			notWant: []string{"-c:a aac", "-c:a copy", "320k"},
		},
		{
			name:    "eac3 stereo",
			profile: helpers.HLS_PROFILE_720P_3MBPS,
			audio:   resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 2, 6, "5.1(side)"),
			want:    []string{"-c:a eac3", "-ac 2", "-b:a 384k", "-ar 48000"},
			notWant: []string{"-c:a aac", "-c:a copy"},
		},
		{
			// The legacy copy decision must not leak into explicit mode even if
			// a caller sets both: the explicit profile wins.
			name:      "explicit profile overrides a copy-safe AAC source",
			profile:   helpers.HLS_PROFILE_720P_3MBPS,
			copyAudio: true,
			audio:     resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 6, "5.1(side)"),
			want:      []string{"-c:a ac3", "-ac 6", "-b:a 640k"},
			notWant:   []string{"-c:a copy"},
		},
		{
			// A remux request keeps copying video while the explicit profile
			// encodes audio.
			name:      "remux copies video while encoding explicit audio",
			profile:   helpers.HLS_PROFILE_REMUX,
			copyVideo: true,
			audio:     resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 6, "5.1(side)"),
			want:      []string{"-c:v copy", "-c:a eac3", "-ac 6", "-b:a 768k", "-ar 48000"},
			notWant:   []string{"-c:a copy"},
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

			requireArgSubstrings(t, args, tt.want, tt.notWant, nil)
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
