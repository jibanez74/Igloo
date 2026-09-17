//go:build externalbin

package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

const externalFFmpegIntegrationTimeout = 45 * time.Second

func TestExternalFFmpegCPUHLSRemuxAndSubtitles(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolve external FFmpeg: %v", err)
	}
	workspace := filepath.Join(t.TempDir(), "media workspace")
	err = os.Mkdir(workspace, 0755)
	if err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	sourcePath := filepath.Join(workspace, "tiny source.mp4")
	generateTinyH264AACSource(t, candidate.path, sourcePath)

	f := &ffmpeg{
		bin:          candidate.path,
		capabilities: Capabilities{Probed: true},
	}

	t.Run("CPU transcode", func(t *testing.T) {
		outDir := filepath.Join(workspace, "CPU HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir CPU output: %v", err)
		}
		params := HLSParams{
			SourcePath:       sourcePath,
			OutDir:           outDir,
			Profile:          helpers.HLS_PROFILE_720P_3MBPS,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			CopyAudio:        true,
			SourceFrameRate:  24,
			Capabilities:     Capabilities{Probed: true},
		}
		runExternalHLSAndWait(t, f, params)
		segments := assertCompleteSequentialHLSOutput(t, outDir)
		if len(segments) < 2 {
			t.Fatalf("CPU transcode produced %d segments, want at least 2", len(segments))
		}
		// libx264 honors -force_key_frames as IDR, so the transcode may claim
		// independence — and this proves -hls_flags actually reaches FFmpeg's
		// playlist rather than just appearing in the argument list.
		assertHLSPlaylistIndependence(t, outDir, true)
	})

	t.Run("H264 AAC remux", func(t *testing.T) {
		outDir := filepath.Join(workspace, "remux HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir remux output: %v", err)
		}
		params := HLSParams{
			SourcePath:       sourcePath,
			OutDir:           outDir,
			Profile:          helpers.HLS_PROFILE_REMUX,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			CopyVideo:        true,
			CopyAudio:        true,
			Capabilities:     Capabilities{Probed: true},
		}
		runExternalHLSAndWait(t, f, params)
		segments := assertCompleteSequentialHLSOutput(t, outDir)
		summary, validationErr := ValidateRemuxSafety(outDir, len(segments))
		if validationErr != nil {
			t.Fatalf("ValidateRemuxSafety: %v", validationErr)
		}
		if summary.CheckedSegments != len(segments) || summary.CheckedSyncSamples < len(segments) {
			t.Fatalf("remux validation summary = %#v, segments=%d", summary, len(segments))
		}
		// Copy-video is served FFmpeg's own playlist verbatim, and validation
		// only ever samples the leading fragments, so the playlist must not
		// claim independence even when this fixture happens to validate clean.
		assertHLSPlaylistIndependence(t, outDir, false)
	})

	t.Run("subtitle conversion", func(t *testing.T) {
		subtitlePath := filepath.Join(workspace, "subtitle source.srt")
		subtitle := "1\n00:00:00,000 --> 00:00:01,500\nIgloo subtitle\n"
		err := os.WriteFile(subtitlePath, []byte(subtitle), 0644)
		if err != nil {
			t.Fatalf("write subtitle source: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
		defer cancel()
		output, extractErr := f.ExtractSubtitleAsWebVTT(ctx, subtitlePath, 0)
		if extractErr != nil {
			t.Fatalf("ExtractSubtitleAsWebVTT: %v", extractErr)
		}
		if !strings.HasPrefix(string(output), "WEBVTT") || !strings.Contains(string(output), "Igloo subtitle") {
			t.Fatalf("real subtitle output = %q", output)
		}
	})
}

// The argument tests pin where yadif lands in each chain; this proves the
// deinterlace chain actually runs against real FFmpeg, and that autorotation
// (which Igloo relies on instead of explicit transpose filters) applies a
// source's display matrix during transcode. Both sources also validate the
// field_order/side_data_list parse in the ffprobe package against a real
// binary.
func TestExternalFFmpegDeinterlaceAndAutorotation(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolve external FFmpeg: %v", err)
	}
	prober, err := ffprobe.New()
	if err != nil {
		t.Fatalf("resolve external ffprobe: %v", err)
	}
	workspace := t.TempDir()

	f := &ffmpeg{
		bin:          candidate.path,
		capabilities: Capabilities{Probed: true},
	}

	t.Run("interlaced source transcodes through yadif", func(t *testing.T) {
		sourcePath := filepath.Join(workspace, "interlaced source.mkv")
		generateInterlacedH264AACSource(t, candidate.path, sourcePath)

		sourceVideo := probeExternalVideoStream(t, prober, sourcePath)
		if !isInterlacedFieldOrder(sourceVideo.FieldOrder) {
			t.Fatalf("generated source field_order = %q, want an interlaced value", sourceVideo.FieldOrder)
		}

		outDir := filepath.Join(workspace, "deinterlaced HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir output: %v", err)
		}
		params := HLSParams{
			SourcePath:       sourcePath,
			OutDir:           outDir,
			Profile:          helpers.HLS_PROFILE_720P_3MBPS,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			Deinterlace:      true,
			SourceFrameRate:  25,
			Capabilities:     Capabilities{Probed: true},
		}
		runExternalHLSAndWait(t, f, params)
		assertCompleteSequentialHLSOutput(t, outDir)

		outVideo := probeExternalVideoStream(t, prober, filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME))
		if isInterlacedFieldOrder(outVideo.FieldOrder) {
			t.Fatalf("output field_order = %q, want progressive after yadif", outVideo.FieldOrder)
		}
		// yadif's default send_frame mode must keep the frame rate; a doubled
		// rate would break the GOP math in appendHLSKeyframeArgs.
		outRate := helpers.ParseFrameRate(outVideo.AvgFrameRate)
		if outRate < 24 || outRate > 26 {
			t.Fatalf("output avg frame rate = %v (%q), want ~25 (send_frame must not double fps)", outRate, outVideo.AvgFrameRate)
		}
	})

	t.Run("rotated source is autorotated during transcode", func(t *testing.T) {
		plainPath := filepath.Join(workspace, "plain source.mp4")
		generateTinyH264AACSource(t, candidate.path, plainPath)
		rotatedPath := filepath.Join(workspace, "rotated source.mp4")
		runExternalFFmpegCommand(t, candidate.path,
			"-y", "-v", "error",
			"-display_rotation", "90", "-i", plainPath,
			"-c", "copy", rotatedPath,
		)

		sourceVideo := probeExternalVideoStream(t, prober, rotatedPath)
		rotationDeg, hasMatrix := sourceVideo.Rotation()
		if !hasMatrix || rotationDeg%180 == 0 {
			t.Fatalf("rotated source Rotation() = (%d, %v), want a quarter-turn display matrix", rotationDeg, hasMatrix)
		}

		outDir := filepath.Join(workspace, "rotated HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir output: %v", err)
		}
		params := HLSParams{
			SourcePath:       rotatedPath,
			OutDir:           outDir,
			Profile:          helpers.HLS_PROFILE_720P_3MBPS,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			CopyAudio:        true,
			SourceFrameRate:  24,
			Capabilities:     Capabilities{Probed: true},
		}
		runExternalHLSAndWait(t, f, params)
		assertCompleteSequentialHLSOutput(t, outDir)

		// The landscape source carries a 90-degree matrix, so a transcode that
		// honors it produces portrait output with the matrix consumed; a
		// leftover matrix would rotate the already-rotated frames again in the
		// player.
		outVideo := probeExternalVideoStream(t, prober, filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME))
		if outVideo.Width >= outVideo.Height {
			t.Fatalf("output = %dx%d, want portrait after autorotation of a landscape source", outVideo.Width, outVideo.Height)
		}
		_, outHasMatrix := outVideo.Rotation()
		if outHasMatrix {
			t.Fatal("output still carries a display matrix, want it consumed by autorotation")
		}
	})
}

func isInterlacedFieldOrder(fieldOrder string) bool {
	switch fieldOrder {
	case "tt", "bb", "tb", "bt":
		return true
	}
	return false
}

func probeExternalVideoStream(t *testing.T, prober ffprobe.FfprobeInterface, path string) ffprobe.Stream {
	t.Helper()
	meta, err := prober.GetMetadata(context.Background(), path)
	if err != nil {
		t.Fatalf("probe %s: %v", path, err)
	}
	for _, stream := range meta.Streams {
		if stream.CodecType == "video" {
			return stream
		}
	}
	t.Fatalf("no video stream in %s", path)
	return ffprobe.Stream{}
}

func generateInterlacedH264AACSource(t *testing.T, binary string, destination string) {
	t.Helper()
	// tinterlace weaves frame pairs (50fps in, 25fps interlaced out) and the
	// +ildct+ilme flags make x264 mark the stream interlaced in the container.
	args := []string{
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x360:rate=50:duration=5.2",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=5.2",
		"-vf", "tinterlace=mode=interleave_top,setfield=tff",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-flags", "+ildct+ilme",
		"-c:a", "aac", "-shortest",
		destination,
	}
	runExternalFFmpegCommand(t, binary, args...)
}

func runExternalFFmpegCommand(t *testing.T, binary string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external FFmpeg %v: %v: %s", args, err, strings.TrimSpace(string(output)))
	}
}

func generateTinyH264AACSource(t *testing.T, binary string, destination string) {
	t.Helper()
	args := []string{
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=5.2",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=5.2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-g", "24", "-keyint_min", "24", "-sc_threshold", "0",
		"-c:a", "aac", "-shortest", "-movflags", "+faststart",
		destination,
	}
	runExternalFFmpegCommand(t, binary, args...)

	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat generated source: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated source is empty")
	}
}

func runExternalHLSAndWait(t *testing.T, f *ffmpeg, params HLSParams) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	results := make(chan hlsExitResult, 1)
	_, err := f.RunHLS(ctx, params, func(exitErr error, stderrTail []string) {
		results <- hlsExitResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}

	select {
	case result := <-results:
		if result.exitErr != nil {
			t.Fatalf("external FFmpeg exit: %v\nstderr tail:\n%s", result.exitErr, strings.Join(result.stderrTail, "\n"))
		}
	case <-ctx.Done():
		t.Fatalf("external FFmpeg HLS timed out: %v", ctx.Err())
	}
}

func assertCompleteSequentialHLSOutput(t *testing.T, outDir string) []string {
	t.Helper()
	playlistPath := filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME)
	playlistData, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("read HLS playlist: %v", err)
	}
	playlist := string(playlistData)
	if !strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Fatalf("playlist is incomplete:\n%s", playlist)
	}
	wantMap := fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"", helpers.HLS_INIT_FILENAME)
	if !strings.Contains(playlist, wantMap) {
		t.Fatalf("playlist missing %q:\n%s", wantMap, playlist)
	}
	assertNonemptyFile(t, filepath.Join(outDir, helpers.HLS_INIT_FILENAME))

	segmentPattern := regexp.MustCompile(`(?m)^segment_([0-9]+)\.m4s$`)
	matches := segmentPattern.FindAllStringSubmatch(playlist, -1)
	if len(matches) == 0 {
		t.Fatalf("playlist has no media segments:\n%s", playlist)
	}
	segments := make([]string, 0, len(matches))
	for index, match := range matches {
		sequence, conversionErr := strconv.Atoi(match[1])
		if conversionErr != nil {
			t.Fatalf("parse segment sequence %q: %v", match[1], conversionErr)
		}
		if sequence != index {
			t.Fatalf("segment sequence = %d at position %d, want %d", sequence, index, index)
		}
		segments = append(segments, match[0])
		assertNonemptyFile(t, filepath.Join(outDir, match[0]))
	}
	return segments
}

// assertHLSPlaylistIndependence checks FFmpeg's own playlist, which copy-video
// sessions are served verbatim and transcode sessions are served once the
// session exits.
func assertHLSPlaylistIndependence(t *testing.T, outDir string, want bool) {
	t.Helper()
	playlistData, err := os.ReadFile(filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME))
	if err != nil {
		t.Fatalf("read HLS playlist: %v", err)
	}
	got := strings.Contains(string(playlistData), "#EXT-X-INDEPENDENT-SEGMENTS")
	if got != want {
		t.Fatalf("playlist contains #EXT-X-INDEPENDENT-SEGMENTS = %t, want %t:\n%s", got, want, playlistData)
	}
}

func assertNonemptyFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("file %s is empty", path)
	}
}

// Explicit audio profiles against the real external FFmpeg build: each case
// produces fMP4 HLS whose fragments ffprobe verifies for the expected codec
// and channel count. Video is copied so the runs stay cheap; the audio path is
// identical for transcoded video.
//
// DTS-HD MA cannot be generated by FFmpeg (its dca encoder produces DTS core
// only), so the DTS cases stand in with core DTS 5.1 — the decode side is the
// same family — and TrueHD 7.1 stands in with an AAC 7.1 source for the
// downmix case.
func TestExternalFFmpegExplicitAudioProfiles(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolve external FFmpeg: %v", err)
	}
	prober, err := ffprobe.New()
	if err != nil {
		t.Fatalf("resolve external ffprobe: %v", err)
	}
	workspace := t.TempDir()

	f := &ffmpeg{
		bin:          candidate.path,
		capabilities: Capabilities{Probed: true},
	}

	sources := map[string]string{}
	sourceFor := func(t *testing.T, name, audioCodec, layout string, extraAudioArgs ...string) string {
		t.Helper()
		if path, ok := sources[name]; ok {
			if path == "" {
				t.Skipf("source %s could not be generated by this FFmpeg build", name)
			}
			return path
		}
		path := filepath.Join(workspace, name+".mkv")
		args := []string{
			"-y", "-v", "error",
			"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=5.2",
			"-f", "lavfi", "-i", "anullsrc=channel_layout=" + layout + ":sample_rate=48000",
			"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
			"-g", "24", "-keyint_min", "24", "-sc_threshold", "0",
			"-c:a", audioCodec,
		}
		args = append(args, extraAudioArgs...)
		args = append(args, "-shortest", path)
		ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
		defer cancel()
		output, genErr := exec.CommandContext(ctx, candidate.path, args...).CombinedOutput()
		if genErr != nil {
			sources[name] = ""
			t.Skipf("cannot generate %s source: %v: %s", name, genErr, strings.TrimSpace(string(output)))
		}
		sources[name] = path
		return path
	}

	explicit := func(codec helpers.HLSAudioCodec, maxChannels, sourceChannels int, layout string) *helpers.HLSResolvedAudioProfile {
		profile := helpers.ResolveHLSAudioProfile(
			helpers.HLSAudioProfileRequest{Codec: codec, MaxChannels: maxChannels},
			sourceChannels,
			layout,
		)
		return &profile
	}

	tests := []struct {
		name         string
		sourceName   string
		sourceCodec  string
		sourceLayout string
		sourceArgs   []string
		audioProfile *helpers.HLSResolvedAudioProfile
		copyAudio    bool
		wantCodec    string
		wantChannels int
	}{
		{
			name:       "DTS 5.1 to eac3 5.1",
			sourceName: "dts51", sourceCodec: "dca", sourceLayout: "5.1(side)",
			sourceArgs:   []string{"-strict", "experimental"},
			audioProfile: explicit(helpers.HLSAudioCodecEAC3, 6, 6, "5.1(side)"),
			wantCodec:    "eac3", wantChannels: 6,
		},
		{
			name:       "DTS 5.1 to ac3 5.1",
			sourceName: "dts51", sourceCodec: "dca", sourceLayout: "5.1(side)",
			sourceArgs:   []string{"-strict", "experimental"},
			audioProfile: explicit(helpers.HLSAudioCodecAC3, 6, 6, "5.1(side)"),
			wantCodec:    "ac3", wantChannels: 6,
		},
		{
			// Legacy mode for an incompatible source stays stereo AAC.
			name:       "DTS 5.1 legacy fallback",
			sourceName: "dts51", sourceCodec: "dca", sourceLayout: "5.1(side)",
			sourceArgs: []string{"-strict", "experimental"},
			wantCodec:  "aac", wantChannels: 2,
		},
		{
			// Legacy copy keeps multichannel AAC-LC intact.
			name:       "AAC 5.1 legacy copy",
			sourceName: "aac51", sourceCodec: "aac", sourceLayout: "5.1",
			copyAudio: true,
			wantCodec: "aac", wantChannels: 6,
		},
		{
			name:       "AAC 5.1 to ac3 5.1",
			sourceName: "aac51", sourceCodec: "aac", sourceLayout: "5.1",
			audioProfile: explicit(helpers.HLSAudioCodecAC3, 6, 6, "5.1"),
			wantCodec:    "ac3", wantChannels: 6,
		},
		{
			name:       "AAC 5.1 to eac3 5.1",
			sourceName: "aac51", sourceCodec: "aac", sourceLayout: "5.1",
			audioProfile: explicit(helpers.HLSAudioCodecEAC3, 6, 6, "5.1"),
			wantCodec:    "eac3", wantChannels: 6,
		},
		{
			name:       "AAC stereo to eac3 is not upmixed",
			sourceName: "aacstereo", sourceCodec: "aac", sourceLayout: "stereo",
			audioProfile: explicit(helpers.HLSAudioCodecEAC3, 6, 2, "stereo"),
			wantCodec:    "eac3", wantChannels: 2,
		},
		{
			name:       "mono to ac3 is not upmixed",
			sourceName: "aacmono", sourceCodec: "aac", sourceLayout: "mono",
			audioProfile: explicit(helpers.HLSAudioCodecAC3, 6, 1, "mono"),
			wantCodec:    "ac3", wantChannels: 1,
		},
		{
			name:       "7.1 to eac3 downmixes to 5.1",
			sourceName: "aac71", sourceCodec: "aac", sourceLayout: "7.1",
			audioProfile: explicit(helpers.HLSAudioCodecEAC3, 6, 8, "7.1"),
			wantCodec:    "eac3", wantChannels: 6,
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath := sourceFor(t, tt.sourceName, tt.sourceCodec, tt.sourceLayout, tt.sourceArgs...)

			outDir := filepath.Join(workspace, fmt.Sprintf("audio HLS %d", index))
			err := os.Mkdir(outDir, 0755)
			if err != nil {
				t.Fatalf("mkdir output: %v", err)
			}
			params := HLSParams{
				SourcePath:       sourcePath,
				OutDir:           outDir,
				Profile:          helpers.HLS_PROFILE_REMUX,
				VideoStreamIndex: 0,
				AudioStreamIndex: 1,
				HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
				CopyVideo:        true,
				CopyAudio:        tt.copyAudio,
				AudioProfile:     tt.audioProfile,
				Capabilities:     Capabilities{Probed: true},
			}
			runExternalHLSAndWait(t, f, params)
			assertCompleteSequentialHLSOutput(t, outDir)

			audio := probeExternalAudioStream(t, prober, filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME))
			if audio.CodecName != tt.wantCodec {
				t.Fatalf("output audio codec = %q, want %q", audio.CodecName, tt.wantCodec)
			}
			if audio.Channels != tt.wantChannels {
				t.Fatalf("output audio channels = %d, want %d", audio.Channels, tt.wantChannels)
			}
		})
	}
}

func probeExternalAudioStream(t *testing.T, prober ffprobe.FfprobeInterface, path string) ffprobe.Stream {
	t.Helper()
	meta, err := prober.GetMetadata(context.Background(), path)
	if err != nil {
		t.Fatalf("probe %s: %v", path, err)
	}
	for _, stream := range meta.Streams {
		if stream.CodecType == "audio" {
			return stream
		}
	}
	t.Fatalf("no audio stream in %s", path)
	return ffprobe.Stream{}
}
