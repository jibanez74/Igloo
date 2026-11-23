//go:build externalbin

package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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
		params := basicHLSParams(outDir)
		params.SourcePath = sourcePath
		params.CopyAudio = true
		params.SourceFrameRate = 24
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
		params := basicHLSParams(outDir)
		params.SourcePath = sourcePath
		params.Profile = helpers.HLS_PROFILE_REMUX
		params.CopyVideo = true
		params.CopyAudio = true
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

	// Copied HEVC is tagged hvc1 and keeps its HDR signalling, and the
	// validator accepts the CRA pictures Blu-ray-style open GOPs start on.
	t.Run("HEVC HDR10 remux", func(t *testing.T) {
		outDir := filepath.Join(workspace, "HEVC remux HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir HEVC remux output: %v", err)
		}
		// FFmpeg runs in its output directory, so the fixture path is absolute.
		sourcePath, absErr := filepath.Abs(filepath.Join("testdata", "hevc-hdr10-open-gop.mkv"))
		if absErr != nil {
			t.Fatalf("resolve HEVC fixture: %v", absErr)
		}
		params := basicHLSParams(outDir)
		params.SourcePath = sourcePath
		params.Profile = helpers.HLS_PROFILE_REMUX
		params.CopyVideo = true
		params.CopyVideoTag = helpers.HLSCopyVideoTag("hevc")
		params.CopyAudio = true
		runExternalHLSAndWait(t, f, params)
		segments := assertCompleteSequentialHLSOutput(t, outDir)
		if len(segments) < 2 {
			t.Fatalf("HEVC remux produced %d segments, want at least 2", len(segments))
		}
		summary, validationErr := ValidateRemuxSafety(outDir, len(segments))
		if validationErr != nil {
			t.Fatalf("ValidateRemuxSafety: %v", validationErr)
		}
		if summary.CheckedSyncSamples <= len(segments) {
			t.Fatalf("remux validation summary = %#v, want more sync samples than the %d segments", summary, len(segments))
		}

		initData, readErr := os.ReadFile(filepath.Join(outDir, helpers.HLS_INIT_FILENAME))
		if readErr != nil {
			t.Fatalf("read init.mp4: %v", readErr)
		}
		// colr nclx with primaries 9 (BT.2020), transfer 16 (PQ), matrix 9.
		hdr10Colr := []byte{'c', 'o', 'l', 'r', 'n', 'c', 'l', 'x', 0, 9, 0, 16, 0, 9}
		for _, want := range [][]byte{[]byte("hvc1"), []byte("hvcC"), hdr10Colr} {
			if !bytes.Contains(initData, want) {
				t.Errorf("init.mp4 is missing %q", want)
			}
		}
		if bytes.Contains(initData, []byte("hev1")) {
			t.Error("init.mp4 kept FFmpeg's default hev1 sample entry")
		}
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

		sourceVideo := probeExternalStream(t, prober, sourcePath, "video")
		if !isInterlacedFieldOrder(sourceVideo.FieldOrder) {
			t.Fatalf("generated source field_order = %q, want an interlaced value", sourceVideo.FieldOrder)
		}

		outDir := filepath.Join(workspace, "deinterlaced HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir output: %v", err)
		}
		params := basicHLSParams(outDir)
		params.SourcePath = sourcePath
		params.Deinterlace = true
		params.SourceFrameRate = 25
		runExternalHLSAndWait(t, f, params)
		assertCompleteSequentialHLSOutput(t, outDir)

		outVideo := probeExternalStream(t, prober, filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME), "video")
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

		sourceVideo := probeExternalStream(t, prober, rotatedPath, "video")
		rotationDeg, hasMatrix := sourceVideo.Rotation()
		if !hasMatrix || rotationDeg%180 == 0 {
			t.Fatalf("rotated source Rotation() = (%d, %v), want a quarter-turn display matrix", rotationDeg, hasMatrix)
		}

		outDir := filepath.Join(workspace, "rotated HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir output: %v", err)
		}
		params := basicHLSParams(outDir)
		params.SourcePath = rotatedPath
		params.CopyAudio = true
		params.SourceFrameRate = 24
		runExternalHLSAndWait(t, f, params)
		assertCompleteSequentialHLSOutput(t, outDir)

		// The landscape source carries a 90-degree matrix, so a transcode that
		// honors it produces portrait output with the matrix consumed; a
		// leftover matrix would rotate the already-rotated frames again in the
		// player.
		outVideo := probeExternalStream(t, prober, filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME), "video")
		if outVideo.Width >= outVideo.Height {
			t.Fatalf("output = %dx%d, want portrait after autorotation of a landscape source", outVideo.Width, outVideo.Height)
		}
		_, outHasMatrix := outVideo.Rotation()
		if outHasMatrix {
			t.Fatal("output still carries a display matrix, want it consumed by autorotation")
		}
	})
}

// The synthesized transcode playlist lists HLSSegmentCount segments, so the
// prediction has to match what the real muxer writes on both sides of the
// boundary: a container whose audio outlasts its last frame by a few
// milliseconds (the common shape of real files), and one whose video really
// carries a frame on the boundary.
func TestExternalFFmpegHLSSegmentCountMatchesMuxer(t *testing.T) {
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

	tests := []struct {
		name          string
		videoDuration string
		audioDuration string
		wantSegments  int
	}{
		// 960 frames end at 39.958 s; the audio runs to ~40.02 s, so the
		// container duration crosses the 40 s boundary without a frame there.
		{name: "audio tail past the last frame", videoDuration: "40", audioDuration: "40.02", wantSegments: 10},
		// 961 frames put one on the boundary, so the muxer opens segment_10.
		{name: "frame on the boundary", videoDuration: "40.0417", audioDuration: "40.05", wantSegments: 11},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath := filepath.Join(workspace, fmt.Sprintf("count source %d.mkv", index))
			generateLavfiSample(t, candidate.path, sourcePath,
				"testsrc2=size=320x180:rate=24:duration="+tt.videoDuration,
				"sine=frequency=1000:sample_rate=48000:duration="+tt.audioDuration,
				"-c:a", "aac",
			)

			meta, err := prober.GetMetadata(context.Background(), sourcePath)
			if err != nil {
				t.Fatalf("probe source: %v", err)
			}
			durationSec, ok := helpers.ParseDurationSeconds(meta.Format.Duration)
			if !ok {
				t.Fatalf("source duration %q did not parse", meta.Format.Duration)
			}
			video := probeExternalStream(t, prober, sourcePath, "video")
			frameRate := helpers.ParseFrameRate(video.FrameRate)

			outDir := filepath.Join(workspace, fmt.Sprintf("count HLS %d", index))
			err = os.Mkdir(outDir, 0755)
			if err != nil {
				t.Fatalf("mkdir output: %v", err)
			}
			params := basicHLSParams(outDir)
			params.SourcePath = sourcePath
			params.CopyAudio = true
			params.SourceFrameRate = frameRate
			runExternalHLSAndWait(t, f, params)
			segments := assertCompleteSequentialHLSOutput(t, outDir)

			if len(segments) != tt.wantSegments {
				t.Fatalf("muxer wrote %d segments for duration %.3f, want %d", len(segments), durationSec, tt.wantSegments)
			}
			predicted := HLSSegmentCount(durationSec, frameRate)
			if predicted != len(segments) {
				t.Fatalf("HLSSegmentCount(%.3f, %.3f) = %d, muxer wrote %d", durationSec, frameRate, predicted, len(segments))
			}
		})
	}
}

func isInterlacedFieldOrder(fieldOrder string) bool {
	switch fieldOrder {
	case "tt", "bb", "tb", "bt":
		return true
	}
	return false
}

// probeExternalStream returns the first stream of codecType ("video" or
// "audio") that the real ffprobe reports for path.
func probeExternalStream(t *testing.T, prober ffprobe.FfprobeInterface, path string, codecType string) ffprobe.Stream {
	t.Helper()
	meta, err := prober.GetMetadata(context.Background(), path)
	if err != nil {
		t.Fatalf("probe %s: %v", path, err)
	}
	for _, stream := range meta.Streams {
		if stream.CodecType == codecType {
			return stream
		}
	}
	t.Fatalf("no %s stream in %s", codecType, path)
	return ffprobe.Stream{}
}

// lavfiSampleArgs builds the FFmpeg command line for a short libx264 sample
// synthesized from a lavfi video and audio source; extra options land between
// the video encoder and the destination.
func lavfiSampleArgs(destination string, videoSource string, audioSource string, extra ...string) []string {
	args := []string{
		"-y", "-v", "error",
		"-f", "lavfi", "-i", videoSource,
		"-f", "lavfi", "-i", audioSource,
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
	}
	args = append(args, extra...)
	return append(args, destination)
}

func generateLavfiSample(t *testing.T, binary string, destination string, videoSource string, audioSource string, extra ...string) {
	t.Helper()
	runExternalFFmpegCommand(t, binary, lavfiSampleArgs(destination, videoSource, audioSource, extra...)...)
	assertNonemptyFile(t, destination)
}

func generateInterlacedH264AACSource(t *testing.T, binary string, destination string) {
	t.Helper()
	// tinterlace weaves frame pairs (50fps in, 25fps interlaced out) and the
	// +ildct+ilme flags make x264 mark the stream interlaced in the container.
	generateLavfiSample(t, binary, destination,
		"testsrc2=size=320x360:rate=50:duration=5.2",
		"sine=frequency=1000:sample_rate=48000:duration=5.2",
		"-vf", "tinterlace=mode=interleave_top,setfield=tff",
		"-flags", "+ildct+ilme",
		"-c:a", "aac", "-shortest",
	)
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
	generateLavfiSample(t, binary, destination,
		"testsrc2=size=320x180:rate=24:duration=5.2",
		"sine=frequency=1000:sample_rate=48000:duration=5.2",
		"-g", "24", "-keyint_min", "24", "-sc_threshold", "0",
		"-c:a", "aac", "-shortest", "-movflags", "+faststart",
	)
}

// runExternalHLSAndWait runs the real FFmpeg to completion and fails with
// its stderr tail on a nonzero exit.
func runExternalHLSAndWait(t *testing.T, f *ffmpeg, params HLSParams) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	results, _ := startHLS(t, ctx, f, params)

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
		extra := []string{"-g", "24", "-keyint_min", "24", "-sc_threshold", "0", "-c:a", audioCodec}
		extra = append(extra, extraAudioArgs...)
		extra = append(extra, "-shortest")
		args := lavfiSampleArgs(path,
			"testsrc2=size=320x180:rate=24:duration=5.2",
			"anullsrc=channel_layout="+layout+":sample_rate=48000",
			extra...,
		)
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
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 6, "5.1(side)"),
			wantCodec:    "eac3", wantChannels: 6,
		},
		{
			name:       "DTS 5.1 to ac3 5.1",
			sourceName: "dts51", sourceCodec: "dca", sourceLayout: "5.1(side)",
			sourceArgs:   []string{"-strict", "experimental"},
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 6, "5.1(side)"),
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
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 6, "5.1"),
			wantCodec:    "ac3", wantChannels: 6,
		},
		{
			name:       "AAC 5.1 to eac3 5.1",
			sourceName: "aac51", sourceCodec: "aac", sourceLayout: "5.1",
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 6, "5.1"),
			wantCodec:    "eac3", wantChannels: 6,
		},
		{
			name:       "AAC stereo to eac3 is not upmixed",
			sourceName: "aacstereo", sourceCodec: "aac", sourceLayout: "stereo",
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 2, "stereo"),
			wantCodec:    "eac3", wantChannels: 2,
		},
		{
			name:       "mono to ac3 is not upmixed",
			sourceName: "aacmono", sourceCodec: "aac", sourceLayout: "mono",
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecAC3, 6, 1, "mono"),
			wantCodec:    "ac3", wantChannels: 1,
		},
		{
			name:       "7.1 to eac3 downmixes to 5.1",
			sourceName: "aac71", sourceCodec: "aac", sourceLayout: "7.1",
			audioProfile: resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 8, "7.1"),
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
			params := basicHLSParams(outDir)
			params.SourcePath = sourcePath
			params.Profile = helpers.HLS_PROFILE_REMUX
			params.CopyVideo = true
			params.CopyAudio = tt.copyAudio
			params.AudioProfile = tt.audioProfile
			runExternalHLSAndWait(t, f, params)
			assertCompleteSequentialHLSOutput(t, outDir)

			audio := probeExternalStream(t, prober, filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME), "audio")
			if audio.CodecName != tt.wantCodec {
				t.Fatalf("output audio codec = %q, want %q", audio.CodecName, tt.wantCodec)
			}
			if audio.Channels != tt.wantChannels {
				t.Fatalf("output audio channels = %d, want %d", audio.Channels, tt.wantChannels)
			}
		})
	}
}

// The SDR chains tag their output BT.709, and the tag alone only relabels the
// frames: an SD source's BT.601 pixels were shown through the BT.709 matrix,
// which pushed this red from (191,30,62) to (204,48,61). The conversion has to
// change the pixels, which only decoding the real output can show.
func TestExternalFFmpegConvertsBT601SourcesToBT709(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolve external FFmpeg: %v", err)
	}
	workspace := t.TempDir()
	sourcePath := filepath.Join(workspace, "sd source.mp4")
	runExternalFFmpegCommand(t, candidate.path,
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "color=c=0xC02040:s=640x480:r=24:d=4.2",
		"-vf", "scale=out_color_matrix=bt601:out_range=tv,format=yuv420p,"+
			"setparams=color_primaries=smpte170m:color_trc=smpte170m:colorspace=smpte170m",
		"-c:v", "libx264", "-preset", "ultrafast", "-qp", "0",
		sourcePath,
	)
	want := decodeCenterPixel(t, candidate.path, sourcePath, "bt601")

	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	transcode := func(convert bool) [3]int {
		outDir := filepath.Join(workspace, fmt.Sprintf("convert %t", convert))
		mkdirErr := os.Mkdir(outDir, 0755)
		if mkdirErr != nil {
			t.Fatalf("mkdir output: %v", mkdirErr)
		}
		params := basicHLSParams(outDir)
		params.SourcePath = sourcePath
		params.AudioStreamIndex = -1
		params.SourceFrameRate = 24
		params.ConvertBT601 = convert
		runExternalHLSAndWait(t, f, params)

		// A player decodes the output through the BT.709 matrix it is tagged with.
		return decodeCenterPixel(t, candidate.path, joinHLSInitAndFirstSegment(t, outDir), "bt709")
	}

	converted := transcode(true)
	if !pixelsWithin(converted, want, 4) {
		t.Fatalf("converted BT.601 source decodes to %v, want about %v", converted, want)
	}
	relabelled := transcode(false)
	if pixelsWithin(relabelled, want, 4) {
		t.Fatalf("relabelled-only output decodes to %v, the same as the source; the fixture no longer shows the shift", relabelled)
	}
}

// joinHLSInitAndFirstSegment writes init.mp4 followed by segment_0.m4s into
// one file FFmpeg can decode on its own.
func joinHLSInitAndFirstSegment(t *testing.T, outDir string) string {
	t.Helper()
	var joined []byte
	firstSegment := helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
	for _, name := range []string{helpers.HLS_INIT_FILENAME, firstSegment} {
		data, readErr := os.ReadFile(filepath.Join(outDir, name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		joined = append(joined, data...)
	}
	path := filepath.Join(outDir, "first.mp4")
	writeErr := os.WriteFile(path, joined, 0644)
	if writeErr != nil {
		t.Fatalf("write joined segment: %v", writeErr)
	}
	return path
}

// decodeCenterPixel decodes the first frame's center pixel to RGB through the
// given YUV matrix.
func decodeCenterPixel(t *testing.T, binary string, path string, matrix string) [3]int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary,
		"-v", "error", "-i", path, "-frames:v", "1",
		"-vf", "scale=in_color_matrix="+matrix+":in_range=tv,format=rgb24,crop=1:1:iw/2:ih/2",
		"-f", "rawvideo", "pipe:1",
	).Output()
	if err != nil || len(output) < 3 {
		t.Fatalf("decode center pixel of %s: %v (%d bytes)", path, err, len(output))
	}
	return [3]int{int(output[0]), int(output[1]), int(output[2])}
}

func pixelsWithin(got [3]int, want [3]int, tolerance int) bool {
	for i := range got {
		diff := got[i] - want[i]
		if diff < -tolerance || diff > tolerance {
			return false
		}
	}
	return true
}

// Decode real HLS output: channel counts alone cannot detect a discarded LFE
// or surround channel. The isolated fixtures also pin which stereo side hears it.
func TestExternalFFmpegExplicitAudioSignals(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	prober, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	cases := []struct {
		name, layout      string
		channels, ceiling int
		active, audible   []int
		// outLayout is the probed output layout when it is not the standard
		// one for the output channel count.
		outLayout string
	}{
		{"mono", "mono", 1, 6, []int{0}, []int{0}, ""},
		{"stereo", "stereo", 2, 6, []int{0, 1}, []int{0, 1}, ""},
		{"surround", "5.1", 6, 6, []int{0, 1, 2, 3, 4, 5}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"DTS surround", "5.1(side)", 6, 6, []int{0, 1, 2, 3, 4, 5}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"side surround", "5.1(side)", 6, 6, []int{0, 1, 2, 3, 4, 5}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"7.1", "7.1", 8, 6, []int{0, 1, 2, 3, 4, 5, 6, 7}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"7.1 side left", "7.1", 8, 6, []int{6}, []int{4}, ""},
		{"7.1 back right", "7.1", 8, 6, []int{5}, []int{5}, ""},
		{"center downmix", "5.1", 6, 2, []int{2}, []int{0, 1}, ""},
		{"surround left downmix", "5.1", 6, 2, []int{4}, []int{0}, ""},
		{"surround right downmix", "5.1", 6, 2, []int{5}, []int{1}, ""},
		{"LFE downmix", "5.1", 6, 2, []int{3}, []int{0, 1}, ""},
		// Layouts kept as they are: -ac used to let the encoder choose its own
		// layout for the channel count, which dropped these LFE channels.
		{"2.1 LFE", "2.1", 3, 6, []int{2}, []int{2}, "2.1"},
		{"3.1 LFE", "3.1", 4, 6, []int{3}, []int{3}, "3.1"},
		{"4.1 LFE", "4.1", 5, 6, []int{3}, []int{3}, "4.1"},
		{"3.0 center", "3.0", 3, 6, []int{2}, []int{2}, "3.0"},
		{"4.0 back center", "4.0", 4, 6, []int{3}, []int{3}, "4.0"},
		{"2.1 LFE downmix", "2.1", 3, 2, []int{2}, []int{0, 1}, ""},
		// The encoders reject 6.0, so it is remixed to 5.1 with its back
		// center spread over both surrounds.
		{"6.0 back center", "6.0", 6, 6, []int{3}, []int{4, 5}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expressions := make([]string, tc.channels)
			for i := range expressions {
				expressions[i] = "0"
			}
			for _, i := range tc.active {
				expressions[i] = "0.1*sin(2*PI*100*t)"
			}
			source := filepath.Join(t.TempDir(), "signal.mkv")
			// WavPack records the channel mask in Matroska; PCM does not, so a
			// PCM source would decode as FFmpeg's guess for its channel count
			// (2.1 for any three channels) rather than the layout under test.
			// The tone has a duration because the WavPack encoder fails with
			// ENOSPC when -shortest cuts an endless source.
			sourceCodec := "wavpack"
			if tc.name == "DTS surround" {
				sourceCodec = "dca"
			}
			runExternalFFmpegCommand(t, candidate.path, lavfiSampleArgs(source,
				"testsrc2=size=160x90:rate=24:duration=3.2",
				"aevalsrc="+strings.Join(expressions, "|")+":s=44100:d=3.2:c="+tc.layout,
				"-c:a", sourceCodec, "-strict", "experimental", "-g", "24", "-shortest")...)
			for _, codec := range []helpers.HLSAudioCodec{helpers.HLSAudioCodecAC3, helpers.HLSAudioCodecEAC3} {
				t.Run(string(codec), func(t *testing.T) {
					params := basicHLSParams(t.TempDir())
					params.SourcePath = source
					params.CopyVideo = true
					params.Profile = helpers.HLS_PROFILE_REMUX
					params.StartSec = 1
					params.AudioProfile = resolveTestAudioProfile(codec, tc.ceiling, tc.channels, tc.layout)
					runExternalHLSAndWait(t, f, params)
					assertCompleteSequentialHLSOutput(t, params.OutDir)
					playlist := filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME)
					audio := probeExternalStream(t, prober, playlist, "audio")
					channels := min(tc.channels, tc.ceiling)
					layout := tc.outLayout
					if layout == "" {
						layout = helpers.HLSDefaultChannelLayoutName(channels)
						if channels == 6 {
							layout = "5.1(side)"
						}
					}
					if audio.CodecName != string(codec) || audio.Channels != channels || audio.ChannelLayout != layout || audio.SampleRate != "48000" {
						t.Fatalf("unexpected audio: %+v", audio)
					}
					ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
					defer cancel()
					cmd := exec.CommandContext(ctx, candidate.path, "-v", "error", "-i", playlist, "-map", "0:a:0", "-f", "f32le", "-c:a", "pcm_f32le", "pipe:1")
					pcm, err := cmd.Output()
					if err != nil {
						t.Fatalf("decode: %v", err)
					}
					frames := len(pcm) / (4 * channels)
					if frames < 48000 {
						t.Fatalf("only %d decoded frames after nonzero start", frames)
					}
					for ch := range channels {
						var sum float64
						for frame := range frames {
							offset := (frame*channels + ch) * 4
							sample := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[offset : offset+4])))
							sum += sample * sample
						}
						rms := math.Sqrt(sum / float64(frames))
						audible := false
						for _, want := range tc.audible {
							if ch == want {
								audible = true
							}
						}
						if (audible && rms < 0.005) || (!audible && rms > 0.001) || math.IsNaN(rms) {
							t.Errorf("%s channel %d RMS = %g, audible = %t", codec, ch, rms, audible)
						}
					}
				})
			}
		})
	}
}

// A copy-video session that starts past zero copies video from the keyframe
// before the target. An accurate seek trimmed encoded audio to the target
// itself, so audio began up to a GOP after the video. When that gap outlasted
// the first segment (a Matroska file whose Cues skip keyframes, like the
// Captain Marvel remux resumed at 1790 s), the fMP4 muxer wrote init.mp4
// before any E-AC-3 packet was parsed: no dec3, no mvex, and Media3 failed to
// play the session. Encoded audio must start with the copied video.
func TestExternalFFmpegCopyVideoResumeStartsEncodedAudioWithVideo(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}

	// Keyframes only every 10 s, so a start at 5 s copies video from 0 s.
	source := filepath.Join(t.TempDir(), "long-gop.mkv")
	runExternalFFmpegCommand(t, candidate.path, lavfiSampleArgs(source,
		"testsrc2=size=160x90:rate=24:duration=20",
		"anullsrc=channel_layout=5.1(side):sample_rate=48000",
		"-g", "240", "-keyint_min", "240", "-sc_threshold", "0",
		"-c:a", "dca", "-strict", "experimental", "-shortest")...)

	for _, codec := range []helpers.HLSAudioCodec{helpers.HLSAudioCodecEAC3, helpers.HLSAudioCodecAC3} {
		t.Run(string(codec), func(t *testing.T) {
			params := basicHLSParams(t.TempDir())
			params.SourcePath = source
			params.Profile = helpers.HLS_PROFILE_REMUX
			params.CopyVideo = true
			params.StartSec = 5
			params.AudioProfile = resolveTestAudioProfile(codec, 6, 6, "5.1(side)")
			runExternalHLSAndWait(t, f, params)
			assertCompleteSequentialHLSOutput(t, params.OutDir)

			init, err := os.ReadFile(filepath.Join(params.OutDir, helpers.HLS_INIT_FILENAME))
			if err != nil {
				t.Fatal(err)
			}
			if !mp4HasBoxPath(init, "moov", "mvex") {
				t.Fatalf("init.mp4 has no moov/mvex (%d bytes)", len(init))
			}

			first := firstPacketSeconds(t, candidate.path, filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME))
			video, audio := first[0], first[1]
			if math.Abs(audio-video) > 0.2 {
				t.Fatalf("encoded audio starts at %.3f s but copied video at %.3f s", audio, video)
			}
		})
	}
}

// A source whose audio starts 10 s after its video had no Dolby packet when
// the first 4 s segment closed, so the fMP4 muxer wrote init.mp4 without dec3
// or mvex. Encoded audio is padded with silence back to the video's start,
// keeping the tone where the source put it.
func TestExternalFFmpegLateSourceAudioWritesDolbyInit(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}

	const audioDelay = 10.0
	source := filepath.Join(t.TempDir(), "late-audio.mkv")
	runExternalFFmpegCommand(t, candidate.path,
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24:duration=20",
		"-itsoffset", fmt.Sprintf("%g", audioDelay),
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=10",
		"-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "48",
		"-c:a", "ac3", source)

	for _, codec := range []helpers.HLSAudioCodec{helpers.HLSAudioCodecEAC3, helpers.HLSAudioCodecAC3} {
		for _, start := range []struct {
			name      string
			sec       float64
			copyVideo bool
		}{
			{name: "remux from zero", copyVideo: true},
			{name: "transcode seek", sec: 3},
		} {
			t.Run(string(codec)+" "+start.name, func(t *testing.T) {
				params := basicHLSParams(t.TempDir())
				params.SourcePath = source
				params.StartSec = start.sec
				if start.copyVideo {
					params.Profile = helpers.HLS_PROFILE_REMUX
					params.CopyVideo = true
				}
				params.AudioProfile = resolveTestAudioProfile(codec, 2, 1, "mono")
				runExternalHLSAndWait(t, f, params)
				assertCompleteSequentialHLSOutput(t, params.OutDir)

				init, err := os.ReadFile(filepath.Join(params.OutDir, helpers.HLS_INIT_FILENAME))
				if err != nil {
					t.Fatal(err)
				}
				if !mp4HasBoxPath(init, "moov", "mvex") {
					t.Fatalf("init.mp4 has no moov/mvex (%d bytes)", len(init))
				}
				// The codec configuration box sits under stsd, a full box
				// mp4HasBoxPath cannot walk into, so search the bytes.
				configBox := "dac3"
				if codec == helpers.HLSAudioCodecEAC3 {
					configBox = "dec3"
				}
				if !bytes.Contains(init, []byte(configBox)) {
					t.Fatalf("init.mp4 has no %s box (%d bytes)", configBox, len(init))
				}

				playlist := filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME)
				first := firstPacketSeconds(t, candidate.path, playlist)
				video, audio := first[0], first[1]
				if math.Abs(audio-video) > 0.2 {
					t.Fatalf("padded audio starts at %.3f s but video at %.3f s", audio, video)
				}
				wantTone := audioDelay - start.sec
				if tone := firstSoundSeconds(t, candidate.path, playlist); math.Abs(tone-wantTone) > 0.1 {
					t.Fatalf("tone starts at %.3f s, want %.3f s", tone, wantTone)
				}
			})
		}
	}
}

// The start pad must leave a source whose audio starts with its video
// untouched: the same audio packets as the unpadded filter.
func TestExternalFFmpegAudioStartPadLeavesAlignedAudio(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}

	source := filepath.Join(t.TempDir(), "aligned.mkv")
	runExternalFFmpegCommand(t, candidate.path, lavfiSampleArgs(source,
		"testsrc2=size=160x90:rate=24:duration=12",
		"sine=frequency=440:sample_rate=48000:duration=12",
		"-g", "48", "-c:a", "ac3", "-shortest")...)

	for _, start := range []float64{0, 3.4} {
		t.Run(fmt.Sprintf("start %g", start), func(t *testing.T) {
			params := basicHLSParams(t.TempDir())
			params.SourcePath = source
			params.StartSec = start
			params.AudioProfile = resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 2, "stereo")
			runExternalHLSAndWait(t, f, params)

			unpadded := params
			unpadded.OutDir = t.TempDir()
			args := hlsArgs(t, unpadded)
			filter := slices.Index(args, "-af") + 1
			plain, found := strings.CutSuffix(args[filter], ":"+hlsAudioStartPadOptions)
			if !found {
				t.Fatalf("-af %q has no start pad", args[filter])
			}
			args[filter] = plain
			runExternalFFmpegCommand(t, candidate.path, args...)

			got := audioFramecrc(t, candidate.path, filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME))
			want := audioFramecrc(t, candidate.path, filepath.Join(unpadded.OutDir, helpers.HLS_PLAYLIST_FILENAME))
			if got != want {
				t.Fatalf("padding changed aligned audio:\npadded:\n%s\nunpadded:\n%s", got, want)
			}
		})
	}
}

// audioFramecrc lists the HLS output's audio packets with their checksums.
func audioFramecrc(t *testing.T, binary string, playlist string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v", "error", "-i", playlist, "-map", "0:a", "-c", "copy", "-f", "framecrc", "-").Output()
	if err != nil {
		t.Fatalf("framecrc: %v", err)
	}
	return string(out)
}

// firstSoundSeconds returns where the first silence of the HLS output's audio
// ends, which for a tone after leading silence is where the tone begins.
func firstSoundSeconds(t *testing.T, binary string, playlist string) float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-hide_banner", "-nostats", "-i", playlist,
		"-map", "0:a", "-af", "silencedetect=n=-40dB:d=0.5", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("silencedetect: %v: %s", err, out)
	}
	match := regexp.MustCompile(`silence_end: ([0-9.]+)`).FindSubmatch(out)
	if match == nil {
		t.Fatalf("no leading silence in the audio:\n%s", out)
	}
	seconds, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return seconds
}

// mp4HasBoxPath reports whether data holds the nested box path, for example
// moov then mvex, walking only the top level of each container on the way.
func mp4HasBoxPath(data []byte, path ...string) bool {
	if len(path) == 0 {
		return true
	}
	for len(data) >= 8 {
		size := uint64(binary.BigEndian.Uint32(data[:4]))
		header := uint64(8)
		switch size {
		case 0:
			size = uint64(len(data))
		case 1:
			if len(data) < 16 {
				return false
			}
			size = binary.BigEndian.Uint64(data[8:16])
			header = 16
		}
		if size < header || size > uint64(len(data)) {
			return false
		}
		if string(data[4:8]) == path[0] && mp4HasBoxPath(data[header:size], path[1:]...) {
			return true
		}
		data = data[size:]
	}
	return false
}

// firstPacketSeconds returns each stream's first packet timestamp in the HLS
// output, in seconds, from FFmpeg's framecrc listing (decode order, which for
// these streams starts with the earliest timestamp).
func firstPacketSeconds(t *testing.T, binary string, playlist string) map[int]float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v", "error", "-i", playlist, "-map", "0", "-c", "copy", "-f", "framecrc", "-").Output()
	if err != nil {
		t.Fatalf("framecrc: %v", err)
	}
	first := framecrcFirstPTS(out)
	if len(first) < 2 {
		t.Fatalf("framecrc listed %d streams:\n%s", len(first), out)
	}
	return first
}
