//go:build externalbin

package ffmpeg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/ffmpeg/subtitletestutil"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/mediabin"
)

// segmentPayloadOffset is where a streamed Segment's children begin: the
// EBML header, then the Segment ID and its 8-byte unknown size. A player that
// reloads the stream from byte 0 rejects a Segment found at another offset.
func segmentPayloadOffset(t *testing.T, stream []byte) int {
	t.Helper()
	segmentAt := bytes.Index(stream, []byte{0x18, 0x53, 0x80, 0x67})
	if segmentAt < 0 {
		t.Fatalf("no Segment in stream %x", stream[:min(len(stream), 64)])
	}
	return segmentAt + 4 + 8
}

type subtitleStreamFixtureTrack struct {
	name    string
	path    string
	stream  int64
	convert bool
	// wantSource lists the packet times a stream from 0 must carry, in source
	// time; nil means the source's own packets of that stream.
	wantSource []float64
}

// TestExternalFFmpegSubtitleStreamsLandOnTheSessionTimeline pins, against the
// real FFmpeg, what the argument tests only describe: a streamed track carries
// exactly the source's packets at or after the session start, rebased by it,
// whether it is copied or converted, read from the media file or from its
// cached copy; and both inputs open with byte-identical headers.
func TestExternalFFmpegSubtitleStreamsLandOnTheSessionTimeline(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolve external FFmpeg: %v", err)
	}
	ffprobePath, err := mediabin.ResolveExternal("ffprobe", "IGLOO_FFPROBE_PATH")
	if err != nil {
		t.Fatalf("resolve external ffprobe: %v", err)
	}
	workspace := t.TempDir()
	fixtures, err := subtitletestutil.BuildFixtures(candidate.path, workspace)
	if err != nil {
		t.Fatalf("build fixtures: %v", err)
	}
	f := &ffmpeg{bin: candidate.path}

	cueStarts := make([]float64, len(subtitletestutil.FixtureCues))
	for i, cue := range subtitletestutil.FixtureCues {
		cueStarts[i] = cue.Start
	}
	tracks := []subtitleStreamFixtureTrack{
		{name: "PGS copied", path: fixtures.Matroska, stream: subtitletestutil.MatroskaPGSStream},
		{name: "SubRip copied", path: fixtures.Matroska, stream: subtitletestutil.MatroskaSubRipStream},
		{name: "DVD copied", path: fixtures.Matroska, stream: subtitletestutil.MatroskaDVDStream},
		// The MP4 track pads its gaps with empty samples, which SubRip drops.
		{name: "mov_text converted", path: fixtures.MP4, stream: subtitletestutil.MP4MovTextStream, convert: true, wantSource: cueStarts},
		{name: "source with a nonzero start", path: fixtures.ShiftedMatroska, stream: subtitletestutil.MatroskaSubRipStream},
	}

	for _, track := range tracks {
		t.Run(track.name, func(t *testing.T) {
			sourceTimes := track.wantSource
			if sourceTimes == nil {
				sourceTimes, err = subtitletestutil.PacketTimes(ffprobePath, track.path, int(track.stream))
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(sourceTimes) == 0 {
				t.Fatalf("fixture track has no packets")
			}

			cachePath := filepath.Join(t.TempDir(), "track.tmp")
			ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
			defer cancel()
			err = f.ExtractSubtitleMatroska(ctx, SubtitleExtractParams{
				SourcePath:      track.path,
				StreamIndex:     track.stream,
				ConvertToSubRip: track.convert,
				OutputPath:      cachePath,
			})
			if err != nil {
				t.Fatalf("ExtractSubtitleMatroska: %v", err)
			}
			cachedTimes, err := subtitletestutil.PacketTimes(ffprobePath, cachePath, 0)
			if err != nil {
				t.Fatal(err)
			}
			requireTimes(t, "cached track", cachedTimes, sourceTimes)

			var firstHeader []byte
			for _, start := range []float64{0, 3.5, 6} {
				want := timesFrom(sourceTimes, start)

				fromSource := streamSubtitleForTest(t, f, SubtitleStreamParams{
					InputPath:       track.path,
					StreamIndex:     track.stream,
					ConvertToSubRip: track.convert,
					StartSec:        start,
				})
				fromCache := streamSubtitleForTest(t, f, SubtitleStreamParams{
					InputPath: cachePath,
					FromCache: true,
					StartSec:  start,
				})

				label := fmt.Sprintf("start %.1f", start)
				requireTimes(t, label+" from the source", streamPacketTimes(t, ffprobePath, fromSource), want)
				requireSameLanguage(t, ffprobePath, track.path, int(track.stream), fromSource)
				requireSameLanguage(t, ffprobePath, track.path, int(track.stream), fromCache)
				requireTimes(t, label+" from the cache", streamPacketTimes(t, ffprobePath, fromCache), want)

				sourceHeader := fromSource[:segmentPayloadOffset(t, fromSource)]
				cacheHeader := fromCache[:segmentPayloadOffset(t, fromCache)]
				if !bytes.Equal(sourceHeader, cacheHeader) {
					t.Fatalf("%s: source and cache streams open differently:\n%x\n%x", label, sourceHeader, cacheHeader)
				}
				if firstHeader == nil {
					firstHeader = sourceHeader
				}
				if !bytes.Equal(sourceHeader, firstHeader) {
					t.Fatalf("%s: stream opens differently from the start-0 stream", label)
				}

				primed, primeErr := helpers.ReadMatroskaHeader(bufio.NewReader(bytes.NewReader(fromSource)))
				if primeErr != nil {
					t.Fatalf("%s: FFmpeg's stream cannot be primed: %v", label, primeErr)
				}
				if !bytes.HasSuffix(primed, helpers.MatroskaPrimerCluster) {
					t.Fatalf("%s: primed header does not end with the primer", label)
				}
			}
		})
	}

	t.Run("primed stream with a keepalive still demuxes", func(t *testing.T) {
		raw := streamSubtitleForTest(t, f, SubtitleStreamParams{
			InputPath:   fixtures.Matroska,
			StreamIndex: subtitletestutil.MatroskaPGSStream,
			StartSec:    3.5,
		})
		src := bufio.NewReader(bytes.NewReader(raw))
		header, primeErr := helpers.ReadMatroskaHeader(src)
		if primeErr != nil {
			t.Fatalf("ReadMatroskaHeader: %v", primeErr)
		}
		rest, readErr := io.ReadAll(src)
		if readErr != nil {
			t.Fatalf("read rest: %v", readErr)
		}
		// A Void between the primer and the first real Cluster, where the
		// relay writes one while FFmpeg reads ahead to a sparse track's cue.
		primed := bytes.Join([][]byte{header, {0xEC, 0x81, 0x00}, rest}, nil)
		want := timesFrom(mustPacketTimes(t, ffprobePath, fixtures.Matroska, subtitletestutil.MatroskaPGSStream), 3.5)
		requireTimes(t, "primed stream", streamPacketTimes(t, ffprobePath, primed), want)
	})

	t.Run("WebVTT from the cache matches WebVTT from the source", func(t *testing.T) {
		textTracks := []subtitleStreamFixtureTrack{
			{name: "SubRip", path: fixtures.Matroska, stream: subtitletestutil.MatroskaSubRipStream},
			{name: "mov_text", path: fixtures.MP4, stream: subtitletestutil.MP4MovTextStream, convert: true},
		}
		for _, track := range textTracks {
			ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
			cachePath := filepath.Join(t.TempDir(), "track.tmp")
			extractErr := f.ExtractSubtitleMatroska(ctx, SubtitleExtractParams{
				SourcePath: track.path, StreamIndex: track.stream, ConvertToSubRip: track.convert, OutputPath: cachePath,
			})
			if extractErr != nil {
				cancel()
				t.Fatalf("%s: ExtractSubtitleMatroska: %v", track.name, extractErr)
			}
			fromSource, sourceErr := f.ExtractSubtitleAsWebVTT(ctx, track.path, track.stream)
			fromCache, cacheErr := f.ExtractCachedSubtitleAsWebVTT(ctx, cachePath)
			cancel()
			if sourceErr != nil || cacheErr != nil {
				t.Fatalf("%s: WebVTT from source: %v, from cache: %v", track.name, sourceErr, cacheErr)
			}
			if !bytes.Equal(fromSource, fromCache) {
				t.Fatalf("%s: WebVTT differs\nsource:\n%s\ncache:\n%s", track.name, fromSource, fromCache)
			}
		}
	})
}

func streamSubtitleForTest(t *testing.T, f *ffmpeg, params SubtitleStreamParams) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	stream, err := f.StreamSubtitleMatroska(ctx, params)
	if err != nil {
		t.Fatalf("StreamSubtitleMatroska(%+v): %v", params, err)
	}
	out, readErr := io.ReadAll(stream)
	waitErr := stream.Wait()
	if readErr != nil || waitErr != nil {
		t.Fatalf("StreamSubtitleMatroska(%+v): read %v, wait %v", params, readErr, waitErr)
	}
	return out
}

func streamPacketTimes(t *testing.T, ffprobePath string, stream []byte) []float64 {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stream.mkv")
	err := os.WriteFile(path, stream, 0o644)
	if err != nil {
		t.Fatalf("write stream: %v", err)
	}
	return mustPacketTimes(t, ffprobePath, path, 0)
}

func mustPacketTimes(t *testing.T, ffprobePath string, path string, streamIndex int) []float64 {
	t.Helper()
	times, err := subtitletestutil.PacketTimes(ffprobePath, path, streamIndex)
	if err != nil {
		t.Fatal(err)
	}
	return times
}

// requireSameLanguage checks that a streamed track keeps its language tag,
// which a player reads to label and pick it.
func requireSameLanguage(t *testing.T, ffprobePath string, sourcePath string, streamIndex int, stream []byte) {
	t.Helper()
	want, err := subtitletestutil.StreamLanguage(ffprobePath, sourcePath, streamIndex)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "stream.mkv")
	err = os.WriteFile(path, stream, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	got, err := subtitletestutil.StreamLanguage(ffprobePath, path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("streamed track language = %q, want the source's %q", got, want)
	}
}

// timesFrom keeps the source times at or after start, rebased onto it.
func timesFrom(sourceTimes []float64, start float64) []float64 {
	var out []float64
	for _, value := range sourceTimes {
		atOrAfter := value >= start-0.0005
		if atOrAfter {
			out = append(out, value-start)
		}
	}
	return out
}

func requireTimes(t *testing.T, label string, got []float64, want []float64) {
	t.Helper()
	matches := len(got) == len(want)
	for i := 0; matches && i < len(got); i++ {
		matches = math.Abs(got[i]-want[i]) <= 0.001
	}
	if !matches {
		t.Fatalf("%s: packet times %v, want %v", label, got, want)
	}
}
