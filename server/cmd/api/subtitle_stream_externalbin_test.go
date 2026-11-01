//go:build externalbin

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffmpeg/subtitletestutil"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/mediabin"
)

// TestSubtitleStream_RealFFmpegEndToEnd drives the routes over HTTP with the
// real FFmpeg: the primed stream carries the source's packets on the session
// timeline, the cached copy serves the same packets, and WebVTT read from the
// cache matches WebVTT read from the media file.
func TestSubtitleStream_RealFFmpegEndToEnd(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	realFFmpeg, err := ffmpeg.New()
	if err != nil {
		t.Fatalf("ffmpeg.New: %v", err)
	}
	ffmpegPath, err := mediabin.ResolveExternal("ffmpeg", "IGLOO_FFMPEG_PATH")
	if err != nil {
		t.Fatalf("resolve FFmpeg: %v", err)
	}
	ffprobePath, err := mediabin.ResolveExternal("ffprobe", "IGLOO_FFPROBE_PATH")
	if err != nil {
		t.Fatalf("resolve ffprobe: %v", err)
	}
	fixtures, err := subtitletestutil.BuildFixtures(ffmpegPath, t.TempDir())
	if err != nil {
		t.Fatalf("build fixtures: %v", err)
	}

	realFfprobe, err := ffprobe.New()
	if err != nil {
		t.Fatalf("ffprobe.New: %v", err)
	}

	app := setupTestApp(t)
	app.FFmpeg = realFFmpeg
	app.Ffprobe = realFfprobe
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	matroskaID := insertTestHLSMovieFixtureAt(t, app, "h264", 180, fixtures.Matroska, "mkv")
	insertTestSubtitle(t, app, matroskaID, subtitletestutil.MatroskaPGSStream, "hdmv_pgs_subtitle", "eng")
	insertTestSubtitle(t, app, matroskaID, subtitletestutil.MatroskaSubRipStream, "subrip", "spa")
	insertTestSubtitle(t, app, matroskaID, subtitletestutil.MatroskaDVDStream, "dvd_subtitle", "eng")
	mp4ID := insertTestHLSMovieFixtureAt(t, app, "h264", 180, fixtures.MP4, "mp4")
	insertTestSubtitle(t, app, mp4ID, subtitletestutil.MP4MovTextStream, "mov_text", "eng")
	shiftedID := insertTestHLSMovieFixtureAt(t, app, "h264", 180, fixtures.ShiftedMatroska, "mkv")
	insertTestSubtitle(t, app, shiftedID, subtitletestutil.MatroskaSubRipStream, "subrip", "spa")
	server := subtitleStreamServer(t, app, viewer.ID)

	get := func(t *testing.T, path string) []byte {
		t.Helper()
		response, getErr := http.Get(server.URL + path)
		if getErr != nil {
			t.Fatalf("GET %s: %v", path, getErr)
		}
		defer response.Body.Close()
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, read error %v, %d bytes", path, response.StatusCode, readErr, len(body))
		}
		return body
	}
	packetTimes := func(t *testing.T, stream []byte) []float64 {
		t.Helper()
		path := filepath.Join(t.TempDir(), "stream.mkv")
		writeErr := os.WriteFile(path, stream, 0o644)
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		times, probeErr := subtitletestutil.PacketTimes(ffprobePath, path, 0)
		if probeErr != nil {
			t.Fatal(probeErr)
		}
		return times
	}
	requirePrimedAfterTracks := func(t *testing.T, stream []byte) {
		t.Helper()
		header, primeErr := helpers.ReadMatroskaHeader(bufio.NewReader(bytes.NewReader(stream)))
		if primeErr != nil {
			t.Fatalf("served stream does not parse: %v", primeErr)
		}
		// ReadMatroskaHeader stops at Tracks and appends the primer, so a
		// stream primed by the server starts with exactly what it returns.
		if !bytes.HasPrefix(stream, header) {
			t.Fatal("the served stream does not carry the primer right after Tracks")
		}
	}

	tracks := []struct {
		name       string
		movieID    int64
		trackIndex string
		ref        subtitleTrackRef
		want       []float64
	}{
		{
			name: "PGS", movieID: matroskaID, trackIndex: "0",
			ref: subtitleTrackRef{Kind: mediaKindMovie, FileID: matroskaID, StreamIndex: subtitletestutil.MatroskaPGSStream, SourcePath: fixtures.Matroska},
			// Show and clear display sets at or after 3.5 s: the straddling
			// cue's clear, then both sets of the last two cues.
			want: []float64{1.0, 2.5, 3.5, 4.5, 6.0},
		},
		{
			name: "DVD", movieID: matroskaID, trackIndex: "2",
			ref:  subtitleTrackRef{Kind: mediaKindMovie, FileID: matroskaID, StreamIndex: subtitletestutil.MatroskaDVDStream, SourcePath: fixtures.Matroska},
			want: []float64{2.5, 4.5},
		},
		{
			name: "mov_text", movieID: mp4ID, trackIndex: "0",
			ref:  subtitleTrackRef{Kind: mediaKindMovie, FileID: mp4ID, StreamIndex: subtitletestutil.MP4MovTextStream, SourcePath: fixtures.MP4, ConvertToSubRip: true},
			want: []float64{2.5, 4.5},
		},
	}
	for _, track := range tracks {
		t.Run(track.name, func(t *testing.T) {
			path := movieSubtitleStreamPath(track.movieID, track.trackIndex, "?start=3.5")
			fromSource := get(t, path)
			requirePrimedAfterTracks(t, fromSource)
			requireTimesNear(t, "from the media file", packetTimes(t, fromSource), track.want)

			app.cacheSubtitleTrack(track.ref)
			if app.cachedSubtitleTrack(track.ref) == "" {
				t.Fatal("the track was not cached")
			}
			fromCache := get(t, path)
			requirePrimedAfterTracks(t, fromCache)
			requireTimesNear(t, "from the cached copy", packetTimes(t, fromCache), track.want)
			// A player reloading the stream from byte 0 requires the Segment
			// at the same offset; past it, FFmpeg writes a random SegmentUID.
			segmentPayload := bytes.Index(fromSource, []byte{0x18, 0x53, 0x80, 0x67}) + 12
			if !bytes.Equal(fromCache[:segmentPayload], fromSource[:segmentPayload]) {
				t.Fatalf("the cached copy opens differently:\n%x\n%x", fromCache[:segmentPayload], fromSource[:segmentPayload])
			}
		})
	}

	webVTTTracks := []struct {
		name       string
		movieID    int64
		trackIndex string
		sourcePath string
	}{
		{name: "WebVTT from the cache", movieID: matroskaID, trackIndex: "1", sourcePath: fixtures.Matroska},
		// Its container starts at 2 s, which only the media file's
		// conversion subtracts unless the server shifts the copy's.
		{name: "WebVTT from the cache of a source with a nonzero start", movieID: shiftedID, trackIndex: "0", sourcePath: fixtures.ShiftedMatroska},
	}
	for _, track := range webVTTTracks {
		t.Run(track.name, func(t *testing.T) {
			vttPath := fmt.Sprintf("/api/movies/%d/subtitles/%s/web.vtt?start=3.5", track.movieID, track.trackIndex)
			fromMediaFile := get(t, vttPath)
			app.SubtitleVTTCache.Flush()
			ref := subtitleTrackRef{Kind: mediaKindMovie, FileID: track.movieID, StreamIndex: subtitletestutil.MatroskaSubRipStream, SourcePath: track.sourcePath}
			app.cacheSubtitleTrack(ref)
			if app.cachedSubtitleTrack(ref) == "" {
				t.Fatal("the track was not cached")
			}
			fromCache := get(t, vttPath)
			if !bytes.Equal(fromCache, fromMediaFile) {
				t.Fatalf("WebVTT differs:\nmedia file:\n%s\ncache:\n%s", fromMediaFile, fromCache)
			}
			if !bytes.Contains(fromCache, []byte("-->")) {
				t.Fatalf("no cues to compare:\n%s", fromCache)
			}
		})
	}
}

func requireTimesNear(t *testing.T, label string, got []float64, want []float64) {
	t.Helper()
	matches := len(got) == len(want)
	for i := 0; matches && i < len(got); i++ {
		matches = math.Abs(got[i]-want[i]) <= 0.001
	}
	if !matches {
		t.Fatalf("%s: packet times %v, want %v", label, got, want)
	}
}
