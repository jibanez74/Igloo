package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func hlsArgs(t *testing.T, p HLSParams) []string {
	t.Helper()
	args, err := buildHLSArgs(p)
	if err != nil {
		t.Fatalf("buildHLSArgs: %v", err)
	}
	return args
}

func hlsTestCapabilitiesForDevice(device string) Capabilities {
	caps := Capabilities{
		Probed:                 true,
		Encoders:               map[string]bool{},
		Filters:                map[string]bool{},
		HWAccels:               map[string]bool{},
		FilterOptions:          map[string]map[string]bool{},
		EncoderOptions:         map[string]map[string]bool{},
		H264NVENCRuntimeUsable: true,
	}

	switch device {
	case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
		caps.Encoders["h264_videotoolbox"] = true
	case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
		caps.Encoders["h264_qsv"] = true
		caps.HWAccels["qsv"] = true
		caps.H264QSVRuntimeUsable = true
		caps.EncoderOptions["h264_qsv"] = map[string]bool{
			"look_ahead": true,
			"forced_idr": true,
			"preset":     true,
		}
	case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
		caps.Encoders["h264_nvenc"] = true
		caps.HWAccels["cuda"] = true
		caps.Filters["scale_cuda"] = true
		caps.FilterOptions["scale_cuda"] = map[string]bool{"format": true}
	}

	return caps
}

func hlsTestIntelQSVScaleCapabilities() Capabilities {
	caps := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	caps.Filters["scale_qsv"] = true
	caps.FilterOptions["scale_qsv"] = map[string]bool{"format": true}
	caps.QSVScaleRuntimeUsable = true
	return caps
}

func hlsTestNvidiaCapabilities(tonemap bool) Capabilities {
	caps := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA)
	if tonemap {
		caps.Filters["tonemap_cuda"] = true
		caps.FilterOptions["tonemap_cuda"] = map[string]bool{
			"format":  true,
			"p":       true,
			"t":       true,
			"m":       true,
			"tonemap": true,
			"desat":   true,
		}
	}
	return caps
}

func TestBuildHLSArgs_TranscodeAll(t *testing.T) {
	sourcePath := "/safe/source.mkv"
	outDir := t.TempDir()
	profile := helpers.HLS_PROFILE_1080P_4MBPS

	args := hlsArgs(t, HLSParams{
		SourcePath:       sourcePath,
		OutDir:           outDir,
		Profile:          profile,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})
	argStr := strings.Join(args, " ")

	for _, want := range []string{
		sourcePath, outDir, "fmp4", "event", "playlist.m3u8",
		"-map 0:0", "-map 0:1",
		"libx264", "-preset", "veryfast",
		"-sc_threshold", "0",
		"-force_key_frames", "expr:gte(t,n_forced*4)",
		"-c:a aac", "-ac", "2", "-b:a", "320k",
		"scale=-2:1080",
		"-avoid_negative_ts", "make_zero",
		"-fflags", "+genpts",
	} {
		if !strings.Contains(argStr, want) {
			t.Errorf("expected %q in args: %s", want, argStr)
		}
	}

	// No explicit -threads cap: libx264 auto-detects its thread count and the
	// concurrency limiter bounds total CPU pressure. A stray -threads before -i
	// would only throttle the decoder while leaving the encoder unbounded, so
	// assert the flag is absent entirely.
	if threadsIdx := indexOf(args, "-threads"); threadsIdx >= 0 {
		t.Errorf("-threads should not be set, found at index %d: %s", threadsIdx, argStr)
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
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
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

func TestBuildHLSArgs_CopyAudioOnly(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_4MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        true,
		StartSec:         0,
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "libx264") {
		t.Error("video should be transcoded with libx264")
	}
	if !strings.Contains(argStr, "-c:a copy") {
		t.Error("audio should use copy when copyAudio=true")
	}
	if strings.Contains(argStr, "-b:a") {
		t.Error("should not set audio bitrate when copying")
	}
}

func TestBuildHLSArgs_CopyBoth(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        true,
		CopyAudio:        true,
		StartSec:         0,
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("video should use copy")
	}
	if !strings.Contains(argStr, "-c:a copy") {
		t.Error("audio should use copy")
	}
	if strings.Contains(argStr, "libx264") || strings.Contains(argStr, "-hwaccel") {
		t.Error("should not use encoder or hwaccel when copying")
	}
}

func TestBuildHLSArgs_HWAccelBeforeInput(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})

	hwIdx := indexOf(args, "-hwaccel")
	iIdx := indexOf(args, "-i")
	if hwIdx < 0 {
		t.Fatal("-hwaccel flag missing for apple device")
	}
	if hwIdx >= iIdx {
		t.Errorf("-hwaccel (pos %d) must come before -i (pos %d)", hwIdx, iIdx)
	}
}

func TestBuildHLSArgs_Remux(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_REMUX,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("remux must use -c:v copy")
	}
	if !strings.Contains(argStr, "-c:a aac") {
		t.Error("remux with copyAudio=false must transcode audio to AAC")
	}
	for _, forbidden := range []string{"libx264", "h264_videotoolbox", "h264_nvenc", "h264_qsv", "-hwaccel", "scale=", "-sc_threshold", "-force_key_frames"} {
		if strings.Contains(argStr, forbidden) {
			t.Errorf("remux must not contain %q in args: %s", forbidden, argStr)
		}
	}
}

func TestBuildHLSArgs_RemuxCopyAudio(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_REMUX,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        true,
		StartSec:         0,
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("remux must use -c:v copy")
	}
	if !strings.Contains(argStr, "-c:a copy") {
		t.Error("remux with copyAudio=true must use -c:a copy")
	}
}

func TestBuildHLSArgs_InvalidProfile(t *testing.T) {
	_, err := buildHLSArgs(HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          "4k_20mbps",
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})
	if err == nil {
		t.Error("expected error for disallowed profile")
	}
}

func TestBuildHLSArgs_EmptySourcePath(t *testing.T) {
	for _, src := range []string{"", "   "} {
		_, err := buildHLSArgs(HLSParams{
			SourcePath:       src,
			OutDir:           t.TempDir(),
			Profile:          helpers.HLS_PROFILE_720P_3MBPS,
			VideoStreamIndex: 0,
			AudioStreamIndex: 0,
			HWDevice:         "cpu",
		})
		if err == nil {
			t.Errorf("expected error for empty SourcePath %q", src)
		}
	}
}

func TestBuildHLSArgs_NegativeVideoStreamIndex(t *testing.T) {
	_, err := buildHLSArgs(HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: -1,
		AudioStreamIndex: 0,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	})
	if err == nil {
		t.Fatal("expected error for negative VideoStreamIndex")
	}
	if !strings.Contains(err.Error(), "video stream index") {
		t.Fatalf("error = %q, want video stream index validation", err.Error())
	}
}

func TestBuildHLSArgs_SegmentFilenameInOutDir(t *testing.T) {
	outDir := t.TempDir()
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_1080P_4MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})
	want := filepath.Join(outDir, "segment_%d.m4s")
	found := false
	for _, a := range args {
		if a == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -hls_segment_filename %q in args", want)
	}
}

func TestBuildHLSArgs_SeekOffsetBeforeInput(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         3600,
	})

	ssIdx := indexOf(args, "-ss")
	iIdx := indexOf(args, "-i")
	if ssIdx < 0 {
		t.Fatal("-ss flag missing when startSec > 0")
	}
	if ssIdx >= iIdx {
		t.Errorf("-ss (pos %d) must come before -i (pos %d)", ssIdx, iIdx)
	}
	if args[ssIdx+1] != "3600.000" {
		t.Errorf("-ss value = %q, want 3600.000", args[ssIdx+1])
	}
}

func TestBuildHLSArgs_NoSeekOffsetWhenZero(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})

	if indexOf(args, "-ss") >= 0 {
		t.Error("-ss should not be present when startSec is 0")
	}
}

func TestBuildHLSArgs_SeekOffsetWithHWAccel(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 0,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         120.5,
	})

	hwIdx := indexOf(args, "-hwaccel")
	ssIdx := indexOf(args, "-ss")
	iIdx := indexOf(args, "-i")
	if hwIdx < 0 {
		t.Fatal("-hwaccel flag missing")
	}
	if ssIdx < 0 {
		t.Fatal("-ss flag missing")
	}
	if !(hwIdx < ssIdx && ssIdx < iIdx) {
		t.Errorf("expected order: -hwaccel(%d) < -ss(%d) < -i(%d)", hwIdx, ssIdx, iIdx)
	}
	if args[ssIdx+1] != "120.500" {
		t.Errorf("-ss value = %q, want 120.500", args[ssIdx+1])
	}
}

func TestBuildHLSArgs_HWAccelDevices(t *testing.T) {
	tests := []struct {
		device      string
		wantHWAccel string
		wantEncoder string
	}{
		{
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantHWAccel: "",
			wantEncoder: "libx264",
		},
		{
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			wantHWAccel: "videotoolbox",
			wantEncoder: "h264_videotoolbox",
		},
		{
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			wantHWAccel: "cuda",
			wantEncoder: "h264_nvenc",
		},
		{
			device:      helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			wantHWAccel: "",
			wantEncoder: "h264_qsv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			caps := Capabilities{}
			if tt.device != helpers.HARDWARE_ACCELERATION_DEVICE_CPU {
				caps = hlsTestCapabilitiesForDevice(tt.device)
			}

			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_720P_3MBPS,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         tt.device,
				CopyVideo:        false,
				CopyAudio:        false,
				StartSec:         0,
				Capabilities:     caps,
			})

			hwIdx := indexOf(args, "-hwaccel")
			if tt.wantHWAccel == "" {
				if hwIdx >= 0 {
					t.Errorf("device %q should not produce -hwaccel, but found %q", tt.device, args[hwIdx+1])
				}
			} else {
				if hwIdx < 0 {
					t.Fatalf("device %q should produce -hwaccel, but flag missing", tt.device)
				}
				if args[hwIdx+1] != tt.wantHWAccel {
					t.Errorf("-hwaccel = %q, want %q", args[hwIdx+1], tt.wantHWAccel)
				}
			}

			argStr := strings.Join(args, " ")
			if !strings.Contains(argStr, tt.wantEncoder) {
				t.Errorf("expected encoder %q in args: %s", tt.wantEncoder, argStr)
			}

			switch tt.device {
			case helpers.HARDWARE_ACCELERATION_DEVICE_CPU:
				if !strings.Contains(argStr, "-sc_threshold:v:0 0") {
					t.Errorf("cpu path must include -sc_threshold 0")
				}
			case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
				if !strings.Contains(argStr, "-rc vbr") {
					t.Errorf("nvidia path must include -rc vbr")
				}
				if !strings.Contains(argStr, "-preset p4") {
					t.Errorf("nvidia path must include -preset p4")
				}
			case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
				if !strings.Contains(argStr, "-look_ahead 1") {
					t.Errorf("intel path must include -look_ahead 1")
				}
				if !strings.Contains(argStr, "-forced_idr 1") {
					t.Errorf("intel path must force IDR frames")
				}
				if !strings.Contains(argStr, "format=nv12") {
					t.Errorf("intel path must feed h264_qsv nv12 frames")
				}
				for _, unexpected := range []string{"-hwaccel qsv", "-pix_fmt yuv420p"} {
					if strings.Contains(argStr, unexpected) {
						t.Errorf("intel encode-only path must not contain %q", unexpected)
					}
				}
			case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
				for _, unexpected := range []string{"-sc_threshold", "-rc vbr", "-look_ahead"} {
					if strings.Contains(argStr, unexpected) {
						t.Errorf("apple path must not contain %q", unexpected)
					}
				}
			}
		})
	}
}

func TestBuildHLSArgs_IntelSDRUsesQSVScaleWhenProbed(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
		CopyVideo:        false,
		CopyAudio:        false,
		Capabilities:     hlsTestIntelQSVScaleCapabilities(),
	})
	argStr := strings.Join(args, " ")

	initIdx := indexOf(args, "-init_hw_device")
	filterIdx := indexOf(args, "-filter_hw_device")
	iIdx := indexOf(args, "-i")
	if initIdx < 0 || filterIdx < 0 {
		t.Fatalf("Intel QSV scale path must initialize and select a QSV device, got: %s", argStr)
	}
	if args[initIdx+1] != "qsv=igloo_qsv" {
		t.Fatalf("-init_hw_device = %q, want qsv=igloo_qsv", args[initIdx+1])
	}
	if args[filterIdx+1] != "igloo_qsv" {
		t.Fatalf("-filter_hw_device = %q, want igloo_qsv", args[filterIdx+1])
	}
	if !(initIdx < filterIdx && filterIdx < iIdx) {
		t.Fatalf("QSV device setup must come before input, got: %s", argStr)
	}

	if !strings.Contains(argStr, "format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=720:format=nv12") {
		t.Fatalf("Intel QSV scale path must upload nv12 frames and use scale_qsv, got: %s", argStr)
	}
	for _, unexpected := range []string{"-hwaccel qsv", "scale=-2:720", "-pix_fmt yuv420p"} {
		if strings.Contains(argStr, unexpected) {
			t.Fatalf("Intel QSV scale path must not contain %q, got: %s", unexpected, argStr)
		}
	}
}

func TestBuildHLSArgs_IntelSDRWithoutQSVScaleUsesSoftwareScale(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
		CopyVideo:        false,
		CopyAudio:        false,
		Capabilities:     hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL),
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "scale=-2:720,format=nv12") {
		t.Fatalf("Intel encode-only path must use software scale into nv12, got: %s", argStr)
	}
	if !strings.Contains(argStr, "h264_qsv") {
		t.Fatalf("Intel encode-only path must use h264_qsv, got: %s", argStr)
	}
	for _, unexpected := range []string{"-init_hw_device", "-filter_hw_device", "hwupload", "scale_qsv", "-hwaccel qsv", "-pix_fmt yuv420p"} {
		if strings.Contains(argStr, unexpected) {
			t.Fatalf("Intel encode-only path must not contain %q, got: %s", unexpected, argStr)
		}
	}
}

func TestBuildHLSArgs_IntelFallsBackToCPUWhenRuntimeProbeFails(t *testing.T) {
	caps := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	caps.H264QSVRuntimeUsable = false
	caps.H264QSVProbeError = "no qsv device"

	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
		CopyVideo:        false,
		CopyAudio:        false,
		Capabilities:     caps,
	})
	argStr := strings.Join(args, " ")

	for _, unexpected := range []string{"h264_qsv", "-look_ahead", "-forced_idr", "format=nv12", "-hwaccel qsv"} {
		if strings.Contains(argStr, unexpected) {
			t.Fatalf("failed QSV runtime probe must fall back to CPU and omit %q, got: %s", unexpected, argStr)
		}
	}
	if !strings.Contains(argStr, "libx264") {
		t.Fatalf("failed QSV runtime probe must use libx264, got: %s", argStr)
	}
	if !strings.Contains(argStr, "scale=-2:720,format=yuv420p") {
		t.Fatalf("failed QSV runtime probe must use CPU software output format, got: %s", argStr)
	}
}

func TestBuildHLSArgs_IntelUnsupportedEncoderOptionsAreOmitted(t *testing.T) {
	caps := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	caps.EncoderOptions["h264_qsv"] = map[string]bool{}

	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
		CopyVideo:        false,
		CopyAudio:        false,
		Capabilities:     caps,
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "h264_qsv") {
		t.Fatalf("Intel path must still use h264_qsv, got: %s", argStr)
	}
	for _, unexpected := range []string{"-preset", "-look_ahead", "-forced_idr"} {
		if indexOf(args, unexpected) >= 0 {
			t.Fatalf("unsupported QSV option %q must be omitted, got: %s", unexpected, argStr)
		}
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
				HWDevice:         "cpu",
				CopyVideo:        false,
				CopyAudio:        false,
				StartSec:         0,
			})
			argStr := strings.Join(args, " ")

			wantScale := fmt.Sprintf("scale=-2:%d", cfg.Height)
			if !strings.Contains(argStr, wantScale) {
				t.Errorf("expected %q in args", wantScale)
			}

			wantBitrate := fmt.Sprintf("-b:v %s", cfg.VideoBitrate)
			if !strings.Contains(argStr, wantBitrate) {
				t.Errorf("expected %q in args", wantBitrate)
			}

			wantMaxrate := fmt.Sprintf("-maxrate %s", cfg.VideoBitrate)
			if !strings.Contains(argStr, wantMaxrate) {
				t.Errorf("expected %q in args", wantMaxrate)
			}

			wantBufsize := fmt.Sprintf("-bufsize %s", cfg.Bufsize)
			if !strings.Contains(argStr, wantBufsize) {
				t.Errorf("expected %q in args", wantBufsize)
			}
		})
	}
}

func TestBuildHLSArgs_CopyVideoTranscodeAudio(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_8MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         "cpu",
		CopyVideo:        true,
		CopyAudio:        false,
		StartSec:         0,
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("video should use copy")
	}
	if !strings.Contains(argStr, "-c:a aac") {
		t.Error("audio should be transcoded to AAC")
	}
	if !strings.Contains(argStr, "-ac 2") {
		t.Error("audio should be downmixed to stereo")
	}
	if !strings.Contains(argStr, "-b:a 320k") {
		t.Error("audio bitrate should be 320k")
	}
	for _, forbidden := range []string{"libx264", "-hwaccel", "scale=", "-b:v", "-maxrate", "-bufsize", "-sc_threshold", "-force_key_frames"} {
		if strings.Contains(argStr, forbidden) {
			t.Errorf("copy video should not contain %q in args", forbidden)
		}
	}
}

func TestBuildHLSArgs_RemuxIgnoresHWAccel(t *testing.T) {
	for _, device := range []string{
		helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
	} {
		t.Run(device, func(t *testing.T) {
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_REMUX,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         device,
				CopyVideo:        false,
				CopyAudio:        false,
				StartSec:         0,
			})
			argStr := strings.Join(args, " ")

			if !strings.Contains(argStr, "-c:v copy") {
				t.Error("remux must use -c:v copy")
			}
			if strings.Contains(argStr, "-hwaccel") {
				t.Errorf("remux should not use -hwaccel even with device %q", device)
			}
			for _, enc := range []string{"h264_videotoolbox", "h264_nvenc", "h264_qsv", "libx264"} {
				if strings.Contains(argStr, enc) {
					t.Errorf("remux should not contain encoder %q", enc)
				}
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
		HWDevice:         "cpu",
		CopyVideo:        false,
		CopyAudio:        false,
		StartSec:         0,
	})

	checks := []struct {
		flag string
		want string
	}{
		{"-f", "hls"},
		{"-hls_segment_type", "fmp4"},
		{"-hls_playlist_type", "event"},
		{"-hls_list_size", "0"},
		{"-hls_time", fmt.Sprintf("%d", helpers.HLS_SEGMENT_TIME_SEC)},
		{"-hls_fmp4_init_filename", "init.mp4"},
		{"-hls_segment_filename", filepath.Join(outDir, "segment_%d.m4s")},
	}

	for _, c := range checks {
		idx := indexOf(args, c.flag)
		if idx < 0 {
			t.Errorf("missing flag %q", c.flag)
			continue
		}
		if idx+1 >= len(args) {
			t.Errorf("flag %q at end of args, no value", c.flag)
			continue
		}
		if args[idx+1] != c.want {
			t.Errorf("%s = %q, want %q", c.flag, args[idx+1], c.want)
		}
	}

	wantPlaylist := filepath.Join(outDir, "playlist.m3u8")
	lastArg := args[len(args)-1]
	if lastArg != wantPlaylist {
		t.Errorf("last arg = %q, want playlist path %q", lastArg, wantPlaylist)
	}
}

func TestRunHLS_UsesAbsoluteOutDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	parentDir := t.TempDir()
	outDir := filepath.Join(parentDir, "out")
	if err := os.Mkdir(outDir, 0755); err != nil {
		t.Fatalf("mkdir outdir: %v", err)
	}

	relOutDir, err := filepath.Rel(cwd, outDir)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}

	scriptPath := filepath.Join(parentDir, "fake-ffmpeg.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf '%s\\n' \"$PWD\" > pwd.txt",
		"printf '%s\\n' \"$@\" > args.txt",
	}, "\n") + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}

	exitCh := make(chan []string, 1)
	f := &ffmpeg{bin: scriptPath}
	_, err = f.RunHLS(context.Background(), HLSParams{
		SourcePath:       "/tmp/source.mkv",
		OutDir:           relOutDir,
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	}, func(exitErr error, stderrTail []string) {
		if exitErr != nil {
			t.Errorf("RunHLS exitErr = %v, want nil", exitErr)
		}
		exitCh <- stderrTail
	})
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}

	<-exitCh

	pwdRaw, err := os.ReadFile(filepath.Join(outDir, "pwd.txt"))
	if err != nil {
		t.Fatalf("read pwd.txt: %v", err)
	}
	gotPwd := strings.TrimSpace(string(pwdRaw))
	if gotPwd != outDir {
		t.Fatalf("command dir = %q, want %q", gotPwd, outDir)
	}

	argsRaw, err := os.ReadFile(filepath.Join(outDir, "args.txt"))
	if err != nil {
		t.Fatalf("read args.txt: %v", err)
	}
	gotArgs := strings.Split(strings.TrimSpace(string(argsRaw)), "\n")

	wantSegmentPattern := filepath.Join(outDir, "segment_%d.m4s")
	if !contains(gotArgs, wantSegmentPattern) {
		t.Fatalf("args missing absolute segment pattern %q: %v", wantSegmentPattern, gotArgs)
	}

	wantPlaylist := filepath.Join(outDir, "playlist.m3u8")
	if gotArgs[len(gotArgs)-1] != wantPlaylist {
		t.Fatalf("playlist arg = %q, want %q", gotArgs[len(gotArgs)-1], wantPlaylist)
	}
}

func TestRunHLS_CapturesLongStderrLines(t *testing.T) {
	outDir := t.TempDir()
	scriptPath := filepath.Join(outDir, "fake-ffmpeg.sh")
	longLine := strings.Repeat("x", 128*1024)
	script := strings.Join([]string{
		"#!/bin/sh",
		fmt.Sprintf("printf '%s\\n' '%s' >&2", "%s", longLine),
	}, "\n") + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}

	type runResult struct {
		exitErr    error
		stderrTail []string
	}
	exitCh := make(chan runResult, 1)

	f := &ffmpeg{bin: scriptPath}
	_, err := f.RunHLS(context.Background(), HLSParams{
		SourcePath:       "/tmp/source.mkv",
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	}, func(exitErr error, stderrTail []string) {
		exitCh <- runResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}

	result := <-exitCh
	if result.exitErr != nil {
		t.Fatalf("exitErr = %v, want nil", result.exitErr)
	}
	if len(result.stderrTail) != 1 {
		t.Fatalf("stderr tail len = %d, want 1", len(result.stderrTail))
	}
	if len(result.stderrTail[0]) != len(longLine) {
		t.Fatalf("captured line length = %d, want %d", len(result.stderrTail[0]), len(longLine))
	}
}

func TestRunHLS_ReportsStderrScannerErrors(t *testing.T) {
	outDir := t.TempDir()
	scriptPath := filepath.Join(outDir, "fake-ffmpeg.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"head -c 2000000 /dev/zero | tr '\\000' 'x' >&2",
		"exit 1",
	}, "\n") + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}

	type runResult struct {
		exitErr    error
		stderrTail []string
	}
	exitCh := make(chan runResult, 1)

	f := &ffmpeg{bin: scriptPath}
	_, err := f.RunHLS(context.Background(), HLSParams{
		SourcePath:       "/tmp/source.mkv",
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	}, func(exitErr error, stderrTail []string) {
		exitCh <- runResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}

	result := <-exitCh
	if result.exitErr == nil {
		t.Fatal("exitErr = nil, want ffmpeg failure")
	}
	if !containsMatching(result.stderrTail, func(line string) bool {
		return strings.Contains(line, "stderr scan error:") && strings.Contains(line, "token too long")
	}) {
		t.Fatalf("stderr tail missing scanner error: %v", result.stderrTail)
	}
}

func TestBuildHLSArgs_TonemapHDR_CPU(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_8MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		CopyVideo:        false,
		CopyAudio:        false,
		TonemapHDR:       true,
	})
	argStr := strings.Join(args, " ")

	if indexOf(args, "-hwaccel") >= 0 {
		t.Error("CPU path must not emit -hwaccel")
	}
	if !strings.Contains(argStr, "zscale") {
		t.Error("CPU tone-mapping must use zscale filter")
	}
	if !strings.Contains(argStr, "tonemap=tonemap=hable") {
		t.Error("CPU tone-mapping must use hable tonemap filter")
	}
	if !strings.Contains(argStr, "bt709") {
		t.Error("tone-mapping output must target bt709 color space")
	}
	if strings.Contains(argStr, "scale=-2:") {
		t.Error("HDR transcode must not use plain scale= filter")
	}
	if !strings.Contains(argStr, "libx264") {
		t.Error("CPU encoder must be libx264")
	}
}

func TestBuildHLSArgs_TonemapHDR_Apple(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_8MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		CopyVideo:        false,
		CopyAudio:        false,
		TonemapHDR:       true,
	})
	argStr := strings.Join(args, " ")

	// Apple keeps hwaccel so VideoToolbox can decode before scale_vt.
	hwIdx := indexOf(args, "-hwaccel")
	if hwIdx < 0 {
		t.Fatal("Apple tone-mapping must keep -hwaccel videotoolbox")
	}
	if args[hwIdx+1] != "videotoolbox" {
		t.Errorf("-hwaccel = %q, want videotoolbox", args[hwIdx+1])
	}
	if !strings.Contains(argStr, "scale_vt") {
		t.Error("Apple tone-mapping must use scale_vt filter")
	}
	if !strings.Contains(argStr, "color_transfer=bt709") {
		t.Error("scale_vt must set color_transfer=bt709")
	}
	if !strings.Contains(argStr, "color_primaries=bt709") {
		t.Error("scale_vt must set color_primaries=bt709")
	}
	if !strings.Contains(argStr, "color_matrix=bt709") {
		t.Error("scale_vt must set color_matrix=bt709")
	}
	if strings.Contains(argStr, "zscale") || strings.Contains(argStr, "tonemap=") {
		t.Error("Apple path must not use software zscale/tonemap filters")
	}
	if !strings.Contains(argStr, "h264_videotoolbox") {
		t.Error("Apple encoder must be h264_videotoolbox")
	}
}

func TestBuildHLSArgs_TonemapHDR_Nvidia(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_8MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		CopyVideo:        false,
		CopyAudio:        false,
		TonemapHDR:       true,
	})
	argStr := strings.Join(args, " ")

	// NVIDIA tone-map uses software decode (no -hwaccel) + hardware encode.
	if indexOf(args, "-hwaccel") >= 0 {
		t.Error("NVIDIA tone-mapping must skip -hwaccel (needs software decode for zscale)")
	}
	if !strings.Contains(argStr, "zscale") {
		t.Error("NVIDIA tone-mapping must use software zscale filter")
	}
	if !strings.Contains(argStr, "tonemap=tonemap=hable") {
		t.Error("NVIDIA tone-mapping must use hable tonemap filter")
	}
	if !strings.Contains(argStr, "h264_nvenc") {
		t.Error("NVIDIA encoder must still be h264_nvenc")
	}
}

func TestBuildHLSArgs_NvidiaSDRUsesCUDAScaleWhenProbed(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		CopyVideo:        false,
		CopyAudio:        false,
		Capabilities:     hlsTestNvidiaCapabilities(false),
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-hwaccel cuda -hwaccel_output_format cuda") {
		t.Fatalf("NVIDIA CUDA scale path must use CUDA hwaccel, got: %s", argStr)
	}
	if !strings.Contains(argStr, "scale_cuda=w=-2:h=720:format=yuv420p") {
		t.Fatalf("NVIDIA CUDA scale path must use scale_cuda, got: %s", argStr)
	}
	if strings.Contains(argStr, "zscale") {
		t.Fatal("SDR CUDA scale path must not use zscale")
	}
}

func TestBuildHLSArgs_NvidiaHDRUsesCUDATonemapWhenProbed(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		CopyVideo:        false,
		CopyAudio:        false,
		TonemapHDR:       true,
		Capabilities:     hlsTestNvidiaCapabilities(true),
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-hwaccel cuda -hwaccel_output_format cuda") {
		t.Fatalf("NVIDIA CUDA tone-map path must use CUDA hwaccel, got: %s", argStr)
	}
	if !strings.Contains(argStr, "scale_cuda=w=-2:h=720:format=p010") {
		t.Fatalf("NVIDIA CUDA tone-map path must scale to p010 before tone-map, got: %s", argStr)
	}
	if !strings.Contains(argStr, "tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0") {
		t.Fatalf("NVIDIA CUDA tone-map path must use tonemap_cuda, got: %s", argStr)
	}
}

func TestBuildHLSArgs_NvidiaHDRFallsBackToSoftwareTonemapWithoutCUDAFilter(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		CopyVideo:        false,
		CopyAudio:        false,
		TonemapHDR:       true,
		Capabilities:     hlsTestNvidiaCapabilities(false),
	})
	argStr := strings.Join(args, " ")

	if indexOf(args, "-hwaccel") >= 0 {
		t.Fatalf("NVIDIA HDR path without tonemap_cuda must skip CUDA hwaccel, got: %s", argStr)
	}
	if !strings.Contains(argStr, "zscale") || !strings.Contains(argStr, "h264_nvenc") {
		t.Fatalf("NVIDIA HDR fallback must use software tone-map with NVENC encode, got: %s", argStr)
	}
}

func TestBuildHLSArgs_NvidiaFallsBackToCPUWhenRuntimeProbeFails(t *testing.T) {
	caps := hlsTestNvidiaCapabilities(false)
	caps.H264NVENCRuntimeUsable = false
	caps.H264NVENCProbeError = "no capable devices"

	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		CopyVideo:        false,
		CopyAudio:        false,
		Capabilities:     caps,
	})
	argStr := strings.Join(args, " ")

	if strings.Contains(argStr, "h264_nvenc") || indexOf(args, "-hwaccel") >= 0 {
		t.Fatalf("failed NVENC runtime probe must fall back to CPU, got: %s", argStr)
	}
	if !strings.Contains(argStr, "libx264") {
		t.Fatalf("failed NVENC runtime probe must use libx264, got: %s", argStr)
	}
}

func TestBuildHLSArgs_TonemapHDR_Intel(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_8MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
		CopyVideo:        false,
		CopyAudio:        false,
		TonemapHDR:       true,
		Capabilities:     hlsTestIntelQSVScaleCapabilities(),
	})
	argStr := strings.Join(args, " ")

	if indexOf(args, "-hwaccel") >= 0 {
		t.Error("Intel tone-mapping must skip -hwaccel (needs software decode for zscale)")
	}
	if !strings.Contains(argStr, "zscale") {
		t.Error("Intel tone-mapping must use software zscale filter")
	}
	if !strings.Contains(argStr, "format=nv12") {
		t.Error("Intel tone-mapping must convert software frames to nv12 for h264_qsv")
	}
	if !strings.Contains(argStr, "h264_qsv") {
		t.Error("Intel encoder must still be h264_qsv")
	}
	for _, unexpected := range []string{"scale_qsv", "hwupload", "-init_hw_device", "-filter_hw_device"} {
		if strings.Contains(argStr, unexpected) {
			t.Errorf("Intel tone-mapping must not use QSV scale path element %q", unexpected)
		}
	}
}

func TestBuildHLSArgs_SDR_FiltersUnchanged(t *testing.T) {
	// SDR sources with TonemapHDR=false must use the original plain scale= filter.
	for _, device := range []string{
		helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
	} {
		t.Run(device, func(t *testing.T) {
			cfg := helpers.HLSProfileConfigs[helpers.HLS_PROFILE_720P_3MBPS]
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_720P_3MBPS,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         device,
				CopyVideo:        false,
				CopyAudio:        false,
				TonemapHDR:       false,
			})
			argStr := strings.Join(args, " ")

			wantScale := fmt.Sprintf("scale=-2:%d", cfg.Height)
			if !strings.Contains(argStr, wantScale) {
				t.Errorf("SDR path must use plain %q filter, got: %s", wantScale, argStr)
			}
			if strings.Contains(argStr, "zscale") || strings.Contains(argStr, "scale_vt") {
				t.Error("SDR path must not emit tone-mapping filters")
			}
		})
	}
}

func TestBuildHLSArgs_ForceKeyframes(t *testing.T) {
	wantFlag := fmt.Sprintf("expr:gte(t,n_forced*%d)", helpers.HLS_SEGMENT_TIME_SEC)

	// All transcode paths must emit -force_key_frames.
	transcodeDevices := []struct {
		name   string
		device string
	}{
		{"cpu", helpers.HARDWARE_ACCELERATION_DEVICE_CPU},
		{"apple", helpers.HARDWARE_ACCELERATION_DEVICE_APPLE},
		{"nvidia", helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA},
		{"intel", helpers.HARDWARE_ACCELERATION_DEVICE_INTEL},
	}
	for _, tc := range transcodeDevices {
		t.Run("transcode/"+tc.name, func(t *testing.T) {
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          helpers.HLS_PROFILE_1080P_4MBPS,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         tc.device,
				CopyVideo:        false,
				CopyAudio:        false,
			})
			argStr := strings.Join(args, " ")
			if !strings.Contains(argStr, "-force_key_frames") {
				t.Errorf("%s transcode path must include -force_key_frames", tc.name)
			}
			if !strings.Contains(argStr, wantFlag) {
				t.Errorf("%s transcode path: -force_key_frames value = want %q, got: %s", tc.name, wantFlag, argStr)
			}
		})
	}

	// Copy-video and remux paths must NOT emit -force_key_frames.
	t.Run("copy_video", func(t *testing.T) {
		args := hlsArgs(t, HLSParams{
			SourcePath:       "/s",
			OutDir:           t.TempDir(),
			Profile:          helpers.HLS_PROFILE_1080P_4MBPS,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			CopyVideo:        true,
			CopyAudio:        false,
		})
		if strings.Contains(strings.Join(args, " "), "-force_key_frames") {
			t.Error("copy-video path must not include -force_key_frames")
		}
	})
	t.Run("remux", func(t *testing.T) {
		args := hlsArgs(t, HLSParams{
			SourcePath:       "/s",
			OutDir:           t.TempDir(),
			Profile:          helpers.HLS_PROFILE_REMUX,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		})
		if strings.Contains(strings.Join(args, " "), "-force_key_frames") {
			t.Error("remux path must not include -force_key_frames")
		}
	})
}

func TestBuildHLSArgs_HardwareFrameRateUsesFixedGOP(t *testing.T) {
	args := hlsArgs(t, HLSParams{
		SourcePath:       "/s",
		OutDir:           t.TempDir(),
		Profile:          helpers.HLS_PROFILE_1080P_4MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		CopyVideo:        false,
		CopyAudio:        false,
		SourceFrameRate:  23.976,
		Capabilities:     hlsTestNvidiaCapabilities(false),
	})
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-g:v:0 96") || !strings.Contains(argStr, "-keyint_min:v:0 96") {
		t.Fatalf("hardware encoder should use a fixed 4-second GOP, got: %s", argStr)
	}
	if strings.Contains(argStr, "-force_key_frames") {
		t.Fatalf("hardware encoder with known frame rate should not also force expression keyframes, got: %s", argStr)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsMatching(values []string, match func(string) bool) bool {
	for _, value := range values {
		if match(value) {
			return true
		}
	}
	return false
}

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}
