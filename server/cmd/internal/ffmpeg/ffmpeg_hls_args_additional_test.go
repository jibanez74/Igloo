package ffmpeg

import (
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestBuildHLSArgsOmitsAudioForVideoOnlyInput(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/media/video only.mkv",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 4,
		AudioStreamIndex: -1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	})

	requireArgumentValue(t, args, "-map", "0:4")
	if indexOf(args, "-c:a") >= 0 || indexOf(args, "-b:a") >= 0 || indexOf(args, "-ac") >= 0 {
		t.Fatalf("video-only input received audio options: %v", args)
	}
}

func TestBuildHLSArgsIgnoresNegativeStartOffset(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/media/source.mkv",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: -1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		StartSec:         -12.5,
	})
	if indexOf(args, "-ss") >= 0 {
		t.Fatalf("negative start offset emitted -ss: %v", args)
	}
}

func TestBuildHLSArgsUnknownAndBlankHardwareUseCPU(t *testing.T) {
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
			if !contains(args, "libx264") || indexOf(args, "-hwaccel") >= 0 {
				t.Fatalf("device %q did not use CPU fallback: %v", device, args)
			}
		})
	}
}

func TestBuildHLSArgsPreservesPathsContainingSpaces(t *testing.T) {
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
	if args[len(args)-1] != filepath.Join(outDir, "playlist.m3u8") {
		t.Fatalf("playlist path = %q, want path containing spaces", args[len(args)-1])
	}
	if strings.Contains(sourcePath, " ") && !contains(args, sourcePath) {
		t.Fatal("source path was split into multiple arguments")
	}
}
