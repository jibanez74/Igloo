package ffmpeg

import (
	"fmt"
	"testing"

	"igloo/cmd/internal/helpers"
)

// Each case pins the decode, filter, and encode arguments produced for one
// combination of configured device, HDR tone-mapping, and probed capabilities.
// Devices are covered even where the current code path is device-independent,
// so a future device-specific change cannot slip through.
func TestBuildHLSArgs_DevicePaths(t *testing.T) {
	intelQSVWithoutEncoderOptions := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	intelQSVWithoutEncoderOptions.EncoderOptions["h264_qsv"] = map[string]bool{}

	intelRuntimeProbeFailed := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	intelRuntimeProbeFailed.H264QSVRuntimeUsable = false
	intelRuntimeProbeFailed.H264QSVProbeError = "no qsv device"

	nvidiaRuntimeProbeFailed := hlsTestNvidiaCapabilities(false)
	nvidiaRuntimeProbeFailed.H264NVENCRuntimeUsable = false
	nvidiaRuntimeProbeFailed.H264NVENCProbeError = "no capable devices"

	sdrScale := fmt.Sprintf("scale=-2:%d", helpers.HLSProfileConfigs[helpers.HLS_PROFILE_720P_3MBPS].Height)

	tests := []struct {
		name      string
		device    string
		profile   string
		tonemap   bool
		caps      Capabilities
		want      []string
		notWant   []string
		notFlags  []string
		wantOrder [][2]string
	}{
		// --- SDR, no probed hardware filter support ---
		{
			name:     "cpu encodes with libx264",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			want:     []string{"libx264", "-sc_threshold:v:0 0", sdrScale, "format=yuv420p"},
			notFlags: []string{"-hwaccel"},
		},
		{
			name:      "apple decodes and encodes with VideoToolbox",
			device:    helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			caps:      hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_APPLE),
			want:      []string{"-hwaccel videotoolbox", "h264_videotoolbox", sdrScale},
			notWant:   []string{"-sc_threshold", "-rc vbr", "-look_ahead"},
			wantOrder: [][2]string{{"-hwaccel", "-i"}},
		},
		{
			name:    "nvidia decodes with cuda and encodes with NVENC",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:    hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA),
			want:    []string{"-hwaccel cuda", "h264_nvenc", "-rc vbr", "-preset p4", sdrScale},
			notWant: []string{"-sc_threshold"},
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
			notWant:  []string{"-hwaccel qsv", "-pix_fmt yuv420p", "-sc_threshold", "hwupload", "scale_qsv"},
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
				"format=nv12,hwupload,scale_cuda=w=-2:h=720:format=yuv420p",
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
			// Apple keeps hwaccel so VideoToolbox can decode before scale_vt.
			name:    "apple tone-maps with scale_vt",
			device:  helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			profile: helpers.HLS_PROFILE_1080P_8MBPS,
			tonemap: true,
			want: []string{
				"-hwaccel videotoolbox", "h264_videotoolbox", "scale_vt",
				"color_transfer=bt709", "color_primaries=bt709", "color_matrix=bt709",
			},
			notWant: []string{"zscale", "tonemap="},
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

		// --- runtime probe failures fall back to CPU ---
		{
			name:     "intel falls back to CPU when the QSV runtime probe fails",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:     intelRuntimeProbeFailed,
			want:     []string{"libx264", sdrScale + ",format=yuv420p"},
			notWant:  []string{"h264_qsv", "-look_ahead", "-forced_idr", "format=nv12", "-hwaccel qsv"},
			notFlags: []string{"-hwaccel"},
		},
		{
			name:     "nvidia falls back to CPU when the NVENC runtime probe fails",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:     nvidiaRuntimeProbeFailed,
			want:     []string{"libx264"},
			notWant:  []string{"h264_nvenc"},
			notFlags: []string{"-hwaccel"},
		},
		{
			name:     "intel omits QSV encoder options the build does not expose",
			device:   helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:     intelQSVWithoutEncoderOptions,
			want:     []string{"h264_qsv"},
			notFlags: []string{"-preset", "-look_ahead", "-forced_idr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := tt.profile
			if profile == "" {
				profile = helpers.HLS_PROFILE_720P_3MBPS
			}
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          profile,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         tt.device,
				TonemapHDR:       tt.tonemap,
				Capabilities:     tt.caps,
			})

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
			args := hlsArgs(t, HLSParams{
				SourcePath:       "/s",
				OutDir:           t.TempDir(),
				Profile:          profile,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         tt.device,
				CopyVideo:        tt.copyVideo,
				SourceFrameRate:  tt.frameRate,
				Capabilities:     tt.caps,
			})

			requireArgSubstrings(t, args, tt.want, tt.notWant, nil)
		})
	}
}
