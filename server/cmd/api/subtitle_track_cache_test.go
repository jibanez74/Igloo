package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

// testTrackRef names the subtitle insertSubtitleStreamMovie inserted.
func testTrackRef(movieID int64, sourcePath string) subtitleTrackRef {
	return subtitleTrackRef{
		Kind:        mediaKindMovie,
		FileID:      movieID,
		StreamIndex: testSubtitleStreamIndex,
		SourcePath:  sourcePath,
	}
}

// expectedCachePath is where the copy of ref made from its source as it is
// now belongs.
func expectedCachePath(t *testing.T, app *Application, ref subtitleTrackRef) string {
	t.Helper()
	info, err := os.Stat(ref.SourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	return subtitleTrackCachePath(app.hlsTranscodeRoot(), ref, info)
}

func writeCachedTrack(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, []byte("cached track"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chtimes(path, modTime, modTime)
	if err != nil {
		t.Fatal(err)
	}
}

// copyingTracks answers ExtractSubtitleMatroska by writing the output file.
func copyingTracks(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
	return os.WriteFile(params.OutputPath, []byte("copied track"), 0o644)
}

func requireNoFile(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s exists (stat error %v), want it gone", path, err)
	}
}

func requireNoTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*"+subtitleTrackTempExt))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestSubtitleTrackCachePath_KeysOnTheTrackAndTheSourceVersion(t *testing.T) {
	app := setupTestApp(t)
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	ref := testTrackRef(movieID, sourcePath)
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	path := subtitleTrackCachePath("/transcode", ref, info)

	name := fmt.Sprintf("%d-%d-%d.mkv", testSubtitleStreamIndex, info.Size(), info.ModTime().UnixNano())
	want := filepath.Join("/transcode", "igloo-subtitle-cache", "movie", fmt.Sprint(movieID), name)
	if path != want {
		t.Fatalf("cache path = %q, want %q", path, want)
	}
}

func TestCachedSubtitleTrack_FindsTheCurrentCopyAndMarksItUsed(t *testing.T) {
	app := setupTestApp(t)
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	ref := testTrackRef(movieID, sourcePath)
	cachePath := expectedCachePath(t, app, ref)

	if app.cachedSubtitleTrack(ref) != "" {
		t.Fatal("found a copy before one was made")
	}

	longAgo := time.Now().Add(-20 * 24 * time.Hour)
	writeCachedTrack(t, cachePath, longAgo)
	staleCopy := filepath.Join(filepath.Dir(cachePath), fmt.Sprintf("%d-1-1.mkv", testSubtitleStreamIndex))
	writeCachedTrack(t, staleCopy, longAgo)
	otherStream := filepath.Join(filepath.Dir(cachePath), "9-1-1.mkv")
	writeCachedTrack(t, otherStream, longAgo)

	found := app.cachedSubtitleTrack(ref)

	if found != cachePath {
		t.Fatalf("cachedSubtitleTrack = %q, want %q", found, cachePath)
	}
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("copy last used %v, want it marked used now", info.ModTime())
	}
	requireNoFile(t, staleCopy)
	_, err = os.Stat(otherStream)
	if err != nil {
		t.Fatalf("another stream's copy was deleted: %v", err)
	}
}

func TestCachedSubtitleTrack_IgnoresAnEmptyCopyAndAMissingSource(t *testing.T) {
	app := setupTestApp(t)
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	ref := testTrackRef(movieID, sourcePath)
	cachePath := expectedCachePath(t, app, ref)
	err := os.MkdirAll(filepath.Dir(cachePath), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(cachePath, nil, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if app.cachedSubtitleTrack(ref) != "" {
		t.Fatal("an empty copy was served")
	}

	writeCachedTrack(t, cachePath, time.Now())
	err = os.Remove(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if app.cachedSubtitleTrack(ref) != "" {
		t.Fatal("a copy of a deleted source was served")
	}
}

func TestCacheSubtitleTrack_CopiesTheTrackWithTheSourcesTimestamps(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{extractTrackFunc: copyingTracks}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mp4", "mov_text")
	ref := testTrackRef(movieID, sourcePath)
	ref.ConvertToSubRip = true

	app.cacheSubtitleTrack(ref)

	cachePath := expectedCachePath(t, app, ref)
	content, err := os.ReadFile(cachePath)
	if err != nil || string(content) != "copied track" {
		t.Fatalf("published copy = %q, %v", content, err)
	}
	calls := fake.ExtractTrackCalls()
	if len(calls) != 1 {
		t.Fatalf("extract calls = %d, want 1", len(calls))
	}
	call := calls[0]
	inCacheDir := filepath.Dir(call.OutputPath) == filepath.Dir(cachePath)
	if call.SourcePath != sourcePath || call.StreamIndex != testSubtitleStreamIndex || !call.ConvertToSubRip || !inCacheDir {
		t.Fatalf("extract params = %+v", call)
	}
	requireNoTemporaryFiles(t, filepath.Dir(cachePath))

	app.cacheSubtitleTrack(ref)
	if len(fake.ExtractTrackCalls()) != 1 {
		t.Fatal("a track already in the cache was copied again")
	}
}

func TestCacheSubtitleTrack_SharesOneCopyBetweenRequests(t *testing.T) {
	app := setupTestApp(t)
	release := make(chan struct{})
	var runs atomic.Int32
	fake := &fakeFFmpeg{extractTrackFunc: func(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
		runs.Add(1)
		<-release
		return copyingTracks(ctx, params)
	}}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	ref := testTrackRef(movieID, sourcePath)

	done := make(chan struct{}, 3)
	for range 3 {
		go func() {
			app.cacheSubtitleTrack(ref)
			done <- struct{}{}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	for range 3 {
		<-done
	}

	if runs.Load() != 1 {
		t.Fatalf("copies run = %d, want one shared copy", runs.Load())
	}
}

// Copying a track reads its whole media file; two at once over a network
// share would each take twice as long.
func TestCacheSubtitleTrack_CopiesOneTrackAtATime(t *testing.T) {
	app := setupTestApp(t)
	started := make(chan string, 2)
	release := make(chan struct{})
	fake := &fakeFFmpeg{extractTrackFunc: func(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
		started <- params.SourcePath
		<-release
		return copyingTracks(ctx, params)
	}}
	app.FFmpeg = fake
	firstID, firstSource := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	secondID, secondSource := insertSubtitleStreamMovie(t, app, "mkv", "subrip")

	go app.cacheSubtitleTrack(testTrackRef(firstID, firstSource))
	first := <-started
	go app.cacheSubtitleTrack(testTrackRef(secondID, secondSource))
	select {
	case second := <-started:
		t.Fatalf("%s started while %s was still being copied", second, first)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the second copy never ran")
	}
}

func TestCacheSubtitleTrack_DiscardsACopyThatWouldNotBeFound(t *testing.T) {
	tests := []struct {
		name   string
		during func(t *testing.T, app *Application, ref subtitleTrackRef, params ffmpeg.SubtitleExtractParams)
	}{
		{
			name: "source rewritten while it was read",
			during: func(t *testing.T, app *Application, ref subtitleTrackRef, params ffmpeg.SubtitleExtractParams) {
				err := os.WriteFile(ref.SourcePath, []byte("a different, longer file"), 0o644)
				if err != nil {
					t.Error(err)
				}
			},
		},
		{
			name: "transcode directory moved",
			during: func(t *testing.T, app *Application, ref subtitleTrackRef, params ffmpeg.SubtitleExtractParams) {
				moved := *app.CurrentSettings()
				moved.TranscodeDir = filepath.Join(t.TempDir(), "moved")
				app.SetSettings(&moved)
			},
		},
		{
			// A source replaced keeping its size and modification time looks
			// unchanged to the copy; only the scanner's invalidation tells.
			// The invalidation's removal finishes, and the copy's file is
			// still written afterwards, as it is when the two interleave.
			name: "track invalidated while it was read",
			during: func(t *testing.T, app *Application, ref subtitleTrackRef, params ffmpeg.SubtitleExtractParams) {
				app.invalidateSubtitleTrackCache(ref.Kind, ref.FileID)
				app.Wait.Wait()
				err := os.MkdirAll(filepath.Dir(params.OutputPath), 0o755)
				if err != nil {
					t.Error(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
			ref := testTrackRef(movieID, sourcePath)
			cachePath := expectedCachePath(t, app, ref)
			app.FFmpeg = &fakeFFmpeg{extractTrackFunc: func(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
				tt.during(t, app, ref, params)
				return copyingTracks(ctx, params)
			}}

			app.cacheSubtitleTrack(ref)

			requireNoFile(t, cachePath)
			requireNoTemporaryFiles(t, filepath.Dir(cachePath))
		})
	}
}

func TestCacheSubtitleTrack_DiscardsACopyThatReachesTheSizeCap(t *testing.T) {
	previous := subtitleTrackCacheMaxBytes
	subtitleTrackCacheMaxBytes = int64(len("copied track"))
	t.Cleanup(func() { subtitleTrackCacheMaxBytes = previous })
	app := setupTestApp(t)
	fake := &fakeFFmpeg{extractTrackFunc: copyingTracks}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "hdmv_pgs_subtitle")
	ref := testTrackRef(movieID, sourcePath)

	app.cacheSubtitleTrack(ref)

	calls := fake.ExtractTrackCalls()
	if len(calls) != 1 || calls[0].MaxBytes != subtitleTrackCacheMaxBytes {
		t.Fatalf("extract calls %+v, want one capped at %d bytes", calls, subtitleTrackCacheMaxBytes)
	}
	cachePath := expectedCachePath(t, app, ref)
	requireNoFile(t, cachePath)
	requireNoTemporaryFiles(t, filepath.Dir(cachePath))
}

func TestCacheSubtitleTrack_FailedOrCanceledCopyLeavesNothing(t *testing.T) {
	app := setupTestApp(t)
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	ref := testTrackRef(movieID, sourcePath)
	cachePath := expectedCachePath(t, app, ref)
	app.FFmpeg = &fakeFFmpeg{extractTrackFunc: func(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
		_ = os.WriteFile(params.OutputPath, []byte("half a track"), 0o644)
		return errors.New("read error")
	}}

	app.cacheSubtitleTrack(ref)

	requireNoFile(t, cachePath)
	requireNoTemporaryFiles(t, filepath.Dir(cachePath))

	started := make(chan struct{})
	app.FFmpeg = &fakeFFmpeg{extractTrackFunc: func(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	done := make(chan struct{})
	go func() {
		app.cacheSubtitleTrack(ref)
		close(done)
	}()
	<-started
	app.beginHLSShutdown()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not cancel the copy")
	}
	requireNoFile(t, cachePath)
	requireNoTemporaryFiles(t, filepath.Dir(cachePath))
}

func TestInvalidateCommitted_RemovesTheFilesCachedTracks(t *testing.T) {
	app := setupTestApp(t)
	root := app.hlsTranscodeRoot()
	movieDir := subtitleTrackCacheDir(root, mediaKindMovie, 5)
	otherMovieDir := subtitleTrackCacheDir(root, mediaKindMovie, 6)
	showFileDir := subtitleTrackCacheDir(root, mediaKindEpisode, 5)
	for _, dir := range []string{movieDir, otherMovieDir, showFileDir} {
		writeCachedTrack(t, filepath.Join(dir, "2-1-1.mkv"), time.Now())
	}

	app.invalidateCommittedMovie(5)
	app.Wait.Wait()

	requireNoFile(t, movieDir)
	for _, dir := range []string{otherMovieDir, showFileDir} {
		_, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("%s was removed with another file's tracks: %v", dir, err)
		}
	}

	app.invalidateCommittedShowFile(5, nil)
	app.Wait.Wait()
	requireNoFile(t, showFileDir)
}

// A source replaced keeping its size and modification time still finds its
// old copy by name, until the invalidation's removal, which runs in the
// background, is done.
func TestCachedSubtitleTrack_IsNotServedWhileItsInvalidationRuns(t *testing.T) {
	removing := make(chan struct{})
	release := make(chan struct{})
	previous := removeSubtitleTrackCacheDir
	removeSubtitleTrackCacheDir = func(dir string) error {
		close(removing)
		<-release
		return os.RemoveAll(dir)
	}
	t.Cleanup(func() { removeSubtitleTrackCacheDir = previous })
	app := setupTestApp(t)
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	ref := testTrackRef(movieID, sourcePath)
	writeCachedTrack(t, expectedCachePath(t, app, ref), time.Now())

	app.invalidateSubtitleTrackCache(ref.Kind, ref.FileID)
	<-removing
	found := app.cachedSubtitleTrack(ref)
	close(release)
	app.Wait.Wait()

	if found != "" {
		t.Fatalf("served %s while its invalidation was still removing it", found)
	}
	writeCachedTrack(t, expectedCachePath(t, app, ref), time.Now())
	if app.cachedSubtitleTrack(ref) == "" {
		t.Fatal("a copy made after the invalidation finished is not served")
	}
}

func TestSweepSubtitleTrackCache_RemovesWhatIsNoLongerUsed(t *testing.T) {
	app := setupTestApp(t)
	root := app.hlsTranscodeRoot()
	now := time.Now()
	unused := filepath.Join(subtitleTrackCacheDir(root, mediaKindMovie, 1), "2-1-1.mkv")
	used := filepath.Join(subtitleTrackCacheDir(root, mediaKindMovie, 2), "2-1-1.mkv")
	abandonedTemp := filepath.Join(subtitleTrackCacheDir(root, mediaKindEpisode, 3), "2-123.tmp")
	runningTemp := filepath.Join(subtitleTrackCacheDir(root, mediaKindEpisode, 4), "2-456.tmp")
	writeCachedTrack(t, unused, now.Add(-31*24*time.Hour))
	writeCachedTrack(t, used, now.Add(-29*24*time.Hour))
	writeCachedTrack(t, abandonedTemp, now.Add(-4*time.Hour))
	writeCachedTrack(t, runningTemp, now.Add(-time.Minute))
	outside := filepath.Join(root, "igloo-hls-session", "old.m4s")
	writeCachedTrack(t, outside, now.Add(-365*24*time.Hour))

	app.sweepSubtitleTrackCache(now)

	requireNoFile(t, unused)
	requireNoFile(t, filepath.Dir(unused))
	requireNoFile(t, abandonedTemp)
	for _, kept := range []string{used, runningTemp, outside} {
		_, err := os.Stat(kept)
		if err != nil {
			t.Fatalf("%s was swept: %v", kept, err)
		}
	}
}

func TestSweepSubtitleTrackCache_WithoutACacheDoesNothing(t *testing.T) {
	app := setupTestApp(t)

	app.sweepSubtitleTrackCache(time.Now())

	requireNoFile(t, subtitleTrackCacheRoot(app.hlsTranscodeRoot()))
}

func TestUpdateGeneralSettings_MovingTheTranscodeDirectoryRemovesTheOldTrackCache(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	oldRoot := app.hlsTranscodeRoot()
	cached := filepath.Join(subtitleTrackCacheDir(oldRoot, mediaKindMovie, 1), "2-1-1.mkv")
	writeCachedTrack(t, cached, time.Now())
	sibling := filepath.Join(oldRoot, "igloo-hls-kept", "init.mp4")
	writeCachedTrack(t, sibling, time.Now())

	// The watcher stays off: once on, it runs in app.Wait until the test ends.
	staticDir := filepath.Join(t.TempDir(), "static")
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBodyWithWatcher(staticDir, false))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if app.hlsTranscodeRoot() == oldRoot {
		t.Fatal("the save did not move the transcode directory")
	}
	oldCache := subtitleTrackCacheRoot(oldRoot)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(oldCache)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	requireNoFile(t, oldCache)
	_, err := os.Stat(sibling)
	if err != nil {
		t.Fatalf("the old transcode directory lost more than the track cache: %v", err)
	}
}

func TestSubtitleStream_ServesTheCachedCopy(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, 0)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{streamFunc: writingSubtitleStreams(testStreamHeader, nil)}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "hdmv_pgs_subtitle")
	cachePath := expectedCachePath(t, app, testTrackRef(movieID, sourcePath))
	writeCachedTrack(t, cachePath, time.Now().Add(-time.Hour))

	recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", "?start=90"))
	app.Wait.Wait()

	calls := fake.StreamCalls()
	want := ffmpeg.SubtitleStreamParams{InputPath: cachePath, FromCache: true, StartSec: 90}
	if recorder.Code != http.StatusOK || len(calls) != 1 || calls[0] != want {
		t.Fatalf("status %d, stream calls %+v, want [%+v]", recorder.Code, calls, want)
	}
	if len(fake.ExtractTrackCalls()) != 0 {
		t.Fatal("a cached track was copied again")
	}
}

func TestSubtitleStream_FallsBackToTheMediaFileWhenTheCachedCopyFails(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	readable := writingSubtitleStreams(testStreamHeader, nil)
	unreadable := writingSubtitleStreams(nil, errors.New("Invalid data found when processing input"))
	fake := &fakeFFmpeg{streamFunc: func(ctx context.Context, params ffmpeg.SubtitleStreamParams) (ffmpeg.SubtitleStream, error) {
		if params.FromCache {
			return unreadable(ctx, params)
		}
		return readable(ctx, params)
	}}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	cachePath := expectedCachePath(t, app, testTrackRef(movieID, sourcePath))
	writeCachedTrack(t, cachePath, time.Now())

	recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))

	calls := fake.StreamCalls()
	if recorder.Code != http.StatusOK || len(calls) != 2 || !calls[0].FromCache || calls[1].InputPath != sourcePath {
		t.Fatalf("status %d, stream calls %+v, want the cache then the media file", recorder.Code, calls)
	}
	requireNoFile(t, cachePath)
}

func TestSubtitleStream_RemovesACachedCopyThatBreaksOffMidStream(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	broken := append(slices.Clone(testStreamHeader), testStreamCluster[:4]...)
	app.FFmpeg = &fakeFFmpeg{streamFunc: writingSubtitleStreams(broken, errors.New("Invalid data found when processing input"))}
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	cachePath := expectedCachePath(t, app, testTrackRef(movieID, sourcePath))
	writeCachedTrack(t, cachePath, time.Now())
	server := subtitleStreamServer(t, app, viewer.ID)

	response, err := http.Get(server.URL + movieSubtitleStreamPath(movieID, "0", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, err = io.ReadAll(response.Body)
	if err == nil {
		t.Fatal("a stream that broke off ended like a complete one")
	}

	// The copy goes before the connection is cut, so the next stream reads
	// the media file instead of breaking off the same way.
	requireNoFile(t, cachePath)
}

// FFmpeg closes its output a moment before it exits; a player leaving in that
// moment kills it, and the run it cut short says nothing about the copy.
func TestSubtitleStream_KeepsACachedCopyWhenThePlayerLeavesAsFFmpegExits(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	streams := make(chan *scriptedSubtitleStream, 1)
	app.FFmpeg = &fakeFFmpeg{streamFunc: heldSubtitleStreams(streams)}
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	cachePath := expectedCachePath(t, app, testTrackRef(movieID, sourcePath))
	writeCachedTrack(t, cachePath, time.Now())
	server := subtitleStreamServer(t, app, viewer.ID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+movieSubtitleStreamPath(movieID, "0", ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	responses := make(chan *http.Response, 1)
	go func() {
		response, doErr := http.DefaultClient.Do(request)
		if doErr == nil {
			responses <- response
		}
	}()
	stream := receiveStream(t, streams)
	if !stream.params.FromCache {
		t.Fatalf("stream = %+v, want the cached copy", stream.params)
	}
	// The whole stream, then its output closes while FFmpeg has yet to exit.
	stream.write(t, append(slices.Clone(testStreamHeader), testStreamCluster...))
	_ = stream.writer.Close()
	response := <-responses
	defer response.Body.Close()
	_ = readExactly(t, response.Body, len(testPrimedStream(testStreamCluster)))

	cancel()
	waitForStreamEnd(t, stream)
	deadline := time.Now().Add(5 * time.Second)
	for len(app.SubtitleStreamSlots) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(app.SubtitleStreamSlots) != 0 {
		t.Fatal("the handler did not finish")
	}

	_, err = os.Stat(cachePath)
	if err != nil {
		t.Fatalf("a good copy was deleted when its player left: %v", err)
	}
}

func TestWriteSubtitleStreamHeader_MarksFailuresAsThePlayersSide(t *testing.T) {
	gone := errors.New("connection reset by peer")
	tests := map[string]struct {
		writer io.Writer
		flush  func() error
	}{
		"write fails": {writer: failingWriter{err: gone}, flush: func() error { return nil }},
		"flush fails": {writer: io.Discard, flush: func() error { return gone }},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := writeSubtitleStreamHeader(tt.writer, tt.flush, testPrimedStream())
			if !errors.Is(err, gone) || !errors.Is(err, helpers.ErrMatroskaRelayWrite) {
				t.Fatalf("error = %v, want %v marked as the player's side", err, gone)
			}
		})
	}
	err := writeSubtitleStreamHeader(io.Discard, func() error { return nil }, testPrimedStream())
	if err != nil {
		t.Fatalf("a header that was sent reported %v", err)
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestSubtitleStream_CopiesATrackIntoTheCacheOnceItsStreamStaysOpen(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, 20*time.Millisecond)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	streams := make(chan *scriptedSubtitleStream, 2)
	copied := make(chan struct{})
	fake := &fakeFFmpeg{
		streamFunc: heldSubtitleStreams(streams),
		extractTrackFunc: func(ctx context.Context, params ffmpeg.SubtitleExtractParams) error {
			defer close(copied)
			return copyingTracks(ctx, params)
		},
	}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "hdmv_pgs_subtitle")
	cachePath := expectedCachePath(t, app, testTrackRef(movieID, sourcePath))

	served := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		served <- serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))
	}()
	stream := receiveStream(t, streams)
	stream.write(t, testStreamHeader)
	select {
	case <-copied:
	case <-time.After(5 * time.Second):
		t.Fatal("a stream open past the gate was never copied")
	}
	stream.end(nil)
	<-served
	app.Wait.Wait()

	_, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("the copy was not published: %v", err)
	}

	go func() {
		served <- serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))
	}()
	second := receiveStream(t, streams)
	if !second.params.FromCache || second.params.InputPath != cachePath {
		t.Fatalf("second stream = %+v, want the cached copy", second.params)
	}
	second.write(t, testStreamHeader)
	second.end(nil)
	<-served
}

func TestSubtitleStream_DoesNotCopyATrackWhoseStreamEndsBeforeTheGate(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{
		streamFunc:       writingSubtitleStreams(testStreamHeader, nil),
		extractTrackFunc: copyingTracks,
	}
	app.FFmpeg = fake
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")

	recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))
	app.Wait.Wait()

	if recorder.Code != http.StatusOK || len(fake.ExtractTrackCalls()) != 0 {
		t.Fatalf("status %d, copies %d, want no copy of a stream that ended first", recorder.Code, len(fake.ExtractTrackCalls()))
	}
}

func TestSubtitleWebVTT_ConvertsTheCachedCopyWhenThereIsOne(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{extractTrackFunc: copyingTracks}
	app.FFmpeg = fake
	app.Ffprobe = startTimeProbe{startTime: "0.000000"}
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	cachePath := expectedCachePath(t, app, testTrackRef(movieID, sourcePath))
	writeCachedTrack(t, cachePath, time.Now())

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	app.Wait.Wait()

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "from the cached track") {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
	cachedCalls := fake.CachedSubtitleCalls()
	if len(cachedCalls) != 1 || cachedCalls[0] != cachePath || fake.SubtitleCallCount() != 0 {
		t.Fatalf("cached conversions %v, media file conversions %d", cachedCalls, fake.SubtitleCallCount())
	}
	if starts := fake.CachedSubtitleStarts(); starts[0] != 0 {
		t.Fatalf("cached conversion shifted by %v, want no shift for a file starting at zero", starts)
	}
}

// The copy keeps the media file's own timestamps; converting the media file
// subtracts its container start, so converting the copy must too.
func TestSubtitleWebVTT_ShiftsTheCachedCopyOntoTheMediaFilesTimeline(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake
	app.Ffprobe = startTimeProbe{startTime: "1.400000"}
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	writeCachedTrack(t, expectedCachePath(t, app, testTrackRef(movieID, sourcePath)), time.Now())

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	starts := fake.CachedSubtitleStarts()
	if recorder.Code != http.StatusOK || len(starts) != 1 || starts[0] != 1.4 {
		t.Fatalf("status %d, cached conversions shifted by %v, want 1.4 s", recorder.Code, starts)
	}
}

// A probe stuck on a slow share must not spend the conversion's whole
// deadline, or there is none left to read the media file instead.
func TestSubtitleWebVTT_ReadsTheMediaFileWhenTheStartTimeProbeStalls(t *testing.T) {
	previous := subtitleStartProbeTimeout
	subtitleStartProbeTimeout = 20 * time.Millisecond
	t.Cleanup(func() { subtitleStartProbeTimeout = previous })
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake
	app.Ffprobe = stuckProbe{}
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	writeCachedTrack(t, expectedCachePath(t, app, testTrackRef(movieID, sourcePath)), time.Now())

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	if recorder.Code != http.StatusOK || fake.SubtitleCallCount() != 1 {
		t.Fatalf("status %d, media file conversions %d, want the media file read after the probe gave up",
			recorder.Code, fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_ReadsTheMediaFileWhenItsStartTimeIsUnknown(t *testing.T) {
	for name, probe := range map[string]startTimeProbe{
		"probe failed":       {err: errors.New("ffprobe: Invalid data found when processing input")},
		"invalid start time": {startTime: "soon"},
	} {
		t.Run(name, func(t *testing.T) {
			app := setupTestApp(t)
			viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
			fake := &fakeFFmpeg{}
			app.FFmpeg = fake
			app.Ffprobe = probe
			movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
			writeCachedTrack(t, expectedCachePath(t, app, testTrackRef(movieID, sourcePath)), time.Now())

			recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

			if recorder.Code != http.StatusOK || len(fake.CachedSubtitleCalls()) != 0 || fake.SubtitleCallCount() != 1 {
				t.Fatalf("status %d, cached conversions %v, media file conversions %d",
					recorder.Code, fake.CachedSubtitleCalls(), fake.SubtitleCallCount())
			}
		})
	}
}

func TestSubtitleWebVTT_FallsBackToTheMediaFileWhenTheCachedCopyFails(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{cachedSubtitleErr: errors.New("Invalid data found when processing input")}
	app.FFmpeg = fake
	app.Ffprobe = startTimeProbe{startTime: "0.000000"}
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	writeCachedTrack(t, expectedCachePath(t, app, testTrackRef(movieID, sourcePath)), time.Now())

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	if recorder.Code != http.StatusOK || recorder.Body.String() != "WEBVTT\n" || fake.SubtitleCallCount() != 1 {
		t.Fatalf("response = %d %q, media file conversions %d", recorder.Code, recorder.Body.String(), fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_NeverCopiesATrackIntoTheCache(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, 0)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{extractTrackFunc: copyingTracks}
	app.FFmpeg = fake
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	app.Wait.Wait()

	if recorder.Code != http.StatusOK || len(fake.ExtractTrackCalls()) != 0 || fake.SubtitleCallCount() != 1 {
		t.Fatalf("status %d, copies %d, media file conversions %d", recorder.Code, len(fake.ExtractTrackCalls()), fake.SubtitleCallCount())
	}
}
