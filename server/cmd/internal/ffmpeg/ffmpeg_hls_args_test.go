package ffmpeg

import (
	"fmt"
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
		caps.Filters["hwupload"] = true
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
	caps.NvidiaCUDAScaleRuntimeUsable = true
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
		caps.NvidiaCUDATonemapRuntimeUsable = true
	}
	return caps
}

func TestBuildHLSArgs_ReadratePacing(t *testing.T) {
	base := HLSParams{
		SourcePath:       "/s",
		OutDir:           "",
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	}

	t.Run("unsupported build omits readrate", func(t *testing.T) {
		p := base
		p.OutDir = t.TempDir()
		argStr := strings.Join(hlsArgs(t, p), " ")
		if strings.Contains(argStr, "-readrate") {
			t.Fatalf("readrate must be omitted when the ffmpeg build does not support it, got: %s", argStr)
		}
	})

	t.Run("supported build paces input reads", func(t *testing.T) {
		p := base
		p.OutDir = t.TempDir()
		p.Capabilities = Capabilities{
			Probed: true,
			CLIOptions: map[string]bool{
				"readrate":               true,
				"readrate_initial_burst": true,
			},
		}
		args := hlsArgs(t, p)
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, fmt.Sprintf("-readrate %d", hlsReadrateSpeed)) {
			t.Fatalf("expected -readrate %d, got: %s", hlsReadrateSpeed, argStr)
		}
		if !strings.Contains(argStr, fmt.Sprintf("-readrate_initial_burst %d", hlsReadrateInitialBurstSec)) {
			t.Fatalf("expected -readrate_initial_burst %d, got: %s", hlsReadrateInitialBurstSec, argStr)
		}
		readrateIdx := indexOf(args, "-readrate")
		iIdx := indexOf(args, "-i")
		if readrateIdx > iIdx {
			t.Fatalf("-readrate is an input option and must come before -i, got: %s", argStr)
		}
	})

	t.Run("readrate without burst support", func(t *testing.T) {
		p := base
		p.OutDir = t.TempDir()
		p.Capabilities = Capabilities{
			Probed:     true,
			CLIOptions: map[string]bool{"readrate": true},
		}
		argStr := strings.Join(hlsArgs(t, p), " ")
		if !strings.Contains(argStr, "-readrate") {
			t.Fatalf("expected -readrate, got: %s", argStr)
		}
		if strings.Contains(argStr, "-readrate_initial_burst") {
			t.Fatalf("burst must be omitted when unsupported, got: %s", argStr)
		}
	})
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
		"libx264", "-preset", "fast",
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
	threadsIdx := indexOf(args, "-threads")
	if threadsIdx >= 0 {
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
