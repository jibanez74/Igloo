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
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_720P_3MBPS,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
				Capabilities:     tt.caps,
			})

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
			notWant:    []string{"libx264", "-hwaccel"},
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
			outDir := t.TempDir()

			args := hlsArgs(t, HLSParams{
				SourcePath:       sourcePath,
				OutDir:           outDir,
				Profile:          tt.profile,
				VideoStreamIndex: tt.videoIndex,
				AudioStreamIndex: tt.audioIndex,
				HWDevice:         device,
				CopyVideo:        tt.copyVideo,
				CopyAudio:        tt.copyAudio,
			})

			requireArgumentValue(t, args, "-i", sourcePath)
			if args[len(args)-1] != filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME) {
				t.Fatalf("last arg = %q, want the playlist path", args[len(args)-1])
			}
			requireArgSubstrings(t, args, tt.want, tt.notWant, tt.notFlags)
		})
	}
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
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_720P_3MBPS,
				VideoStreamIndex: 0,
				AudioStreamIndex: 0,
				HWDevice:         device,
				StartSec:         tt.startSec,
			})

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
	outDir := t.TempDir()
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

func TestBuildHLSArgs_GlobalStreamIndices(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/src.mkv",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 3,
		AudioStreamIndex: 7,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	})

	var mapTargets []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-map" {
			mapTargets = append(mapTargets, args[i+1])
		}
	}
	if len(mapTargets) < 2 {
		t.Fatalf("expected two -map targets, got %v", mapTargets)
	}
	if mapTargets[0] != "0:3" || mapTargets[1] != "0:7" {
		t.Errorf("global stream maps = %v, want [0:3 0:7]", mapTargets)
	}
}

func TestBuildHLSArgs_AllProfileConfigs(t *testing.T) {
	for profileID, cfg := range helpers.HLSProfileConfigs {
		t.Run(profileID, func(t *testing.T) {
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          profileID,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			})

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
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/media/source.mkv",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_720P_3MBPS,
				VideoStreamIndex: 0,
				AudioStreamIndex: -1,
				HWDevice:         device,
				Capabilities:     Capabilities{Probed: true},
			})
			if !slices.Contains(args, "libx264") || slices.Contains(args, "-hwaccel") {
				t.Fatalf("device %q did not use CPU fallback: %v", device, args)
			}
		})
	}
}

func TestBuildHLSArgs_HLSOutputStructure(t *testing.T) {
	outDir := t.TempDir()
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	})

	requireArgumentValue(t, args, "-f", "hls")
	requireArgumentValue(t, args, "-hls_segment_type", "fmp4")
	requireArgumentValue(t, args, "-hls_playlist_type", "event")
	requireArgumentValue(t, args, "-hls_list_size", "0")
	requireArgumentValue(t, args, "-hls_time", fmt.Sprintf("%d", helpers.HLS_SEGMENT_TIME_SEC))
	requireArgumentValue(t, args, "-hls_fmp4_init_filename", helpers.HLS_INIT_FILENAME)
	requireArgumentValue(t, args, "-hls_segment_filename", filepath.Join(outDir, "segment_%d.m4s"))

	wantPlaylist := filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME)
	if args[len(args)-1] != wantPlaylist {
		t.Errorf("last arg = %q, want playlist path %q", args[len(args)-1], wantPlaylist)
	}
}

func TestBuildHLSArgs_PreservesPathsContainingSpaces(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "HLS output")
	sourcePath := filepath.Join(t.TempDir(), "movie source.mkv")
	args := hlsArgs(t, HLSParams{
		SourcePath:       sourcePath,
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_REMUX,
		VideoStreamIndex: 2,
		AudioStreamIndex: -1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	})

	requireArgumentValue(t, args, "-i", sourcePath)
	requireArgumentValue(t, args, "-hls_segment_filename", filepath.Join(outDir, "segment_%d.m4s"))
	if args[len(args)-1] != filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME) {
		t.Fatalf("playlist path = %q, want path containing spaces", args[len(args)-1])
	}
	if !slices.Contains(args, sourcePath) {
		t.Fatal("source path was split into multiple arguments")
	}
}
