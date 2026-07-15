package ffmpeg

import (
	"fmt"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

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

	initIdx := indexOf(args, "-init_hw_device")
	filterIdx := indexOf(args, "-filter_hw_device")
	iIdx := indexOf(args, "-i")
	if initIdx < 0 || filterIdx < 0 {
		t.Fatalf("NVIDIA CUDA scale path must initialize and select a CUDA filter device, got: %s", argStr)
	}
	if args[initIdx+1] != "cuda=igloo_cuda" {
		t.Fatalf("-init_hw_device = %q, want cuda=igloo_cuda", args[initIdx+1])
	}
	if args[filterIdx+1] != "igloo_cuda" {
		t.Fatalf("-filter_hw_device = %q, want igloo_cuda", args[filterIdx+1])
	}
	if !(initIdx < filterIdx && filterIdx < iIdx) {
		t.Fatalf("CUDA device setup must come before input, got: %s", argStr)
	}
	hwIdx := indexOf(args, "-hwaccel")
	if hwIdx < 0 || args[hwIdx+1] != "cuda" {
		t.Fatalf("NVIDIA CUDA scale path must decode with -hwaccel cuda, got: %s", argStr)
	}
	if !strings.Contains(argStr, "format=nv12,hwupload,scale_cuda=w=-2:h=720:format=yuv420p") {
		t.Fatalf("NVIDIA CUDA scale path must upload software frames and use scale_cuda, got: %s", argStr)
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

	if !strings.Contains(argStr, "-init_hw_device cuda=igloo_cuda -filter_hw_device igloo_cuda") {
		t.Fatalf("NVIDIA CUDA tone-map path must initialize and select a CUDA filter device, got: %s", argStr)
	}
	hwIdx := indexOf(args, "-hwaccel")
	if hwIdx < 0 || args[hwIdx+1] != "cuda" {
		t.Fatalf("NVIDIA CUDA tone-map path must decode with -hwaccel cuda, got: %s", argStr)
	}
	if !strings.Contains(argStr, "format=p010le,hwupload,scale_cuda=w=-2:h=720:format=p010") {
		t.Fatalf("NVIDIA CUDA tone-map path must upload p010 frames before tone-map, got: %s", argStr)
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

	// Decode acceleration is independent of the filter path: -hwaccel cuda
	// (without -hwaccel_output_format) downloads frames to system memory, so
	// the software zscale tone-map chain still applies.
	hwIdx := indexOf(args, "-hwaccel")
	if hwIdx < 0 || args[hwIdx+1] != "cuda" {
		t.Fatalf("NVIDIA HDR fallback must still decode with -hwaccel cuda, got: %s", argStr)
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
	// The GOP size alone drifts on non-integer frame rates (96 frames at
	// 23.976fps ≈ 4.004s), so expression keyframes stay on to pin segment
	// boundaries exactly.
	if !strings.Contains(argStr, "-force_key_frames:0 expr:gte(t,n_forced*4)") {
		t.Fatalf("hardware encoder must still force expression keyframes on segment boundaries, got: %s", argStr)
	}
	if strings.Contains(argStr, "-sc_threshold") {
		t.Fatalf("-sc_threshold is libx264-only and must not be set for hardware encoders, got: %s", argStr)
	}
}
