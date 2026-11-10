package main

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"igloo/cmd/internal/database"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"igloo/cmd/internal/helpers"
)

// The subtitle fixtures all live on stream 2, matching a typical container
// where the subtitle track follows one video and one audio stream.
const testSubtitleStreamIndex = 2

func serveSubtitleWebVTTWithQuery(
	t *testing.T,
	app *Application,
	userID int64,
	movieID int64,
	trackIndex string,
	query string,
) *httptest.ResponseRecorder {
	t.Helper()
	router := authenticatedRouter(t, app, userID)

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/%d/subtitles/%s/web.vtt%s", movieID, trackIndex, query),
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// A rebased HLS session's media timeline starts at zero while these cues carry
// absolute source timestamps, so `start` rebases them onto the session they
// will be played against (audit H4).
func TestSubtitleWebVTT_ShiftsCuesBySessionStart(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	absolute := "WEBVTT\n\n00:10:05.000 --> 00:10:07.000\nLine\n"
	app.SubtitleVTTCache.Set(helpers.SubtitleCacheKey("movie", movieID, 2), []byte(absolute), subtitleCacheTTL)

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "?start=600")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "00:00:05.000 --> 00:00:07.000") {
		t.Fatalf("cues were not rebased onto the session: %s", recorder.Body.String())
	}

	// The cache holds the absolute payload, so an unshifted request still gets
	// absolute cues without a second extraction.
	unshifted := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if !strings.Contains(unshifted.Body.String(), "00:10:05.000 --> 00:10:07.000") {
		t.Fatalf("cache must keep absolute cues: %s", unshifted.Body.String())
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("shifting must not trigger extraction, got %d calls", fake.SubtitleCallCount())
	}
}

// A film's cues are sent compressed to a client that accepts gzip, shifted or
// not, and unchanged to one that does not.
func TestSubtitleWebVTT_CompressesCuesForClientsThatAcceptGzip(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	var cues strings.Builder
	cues.WriteString("WEBVTT\n\n")
	for minute := 10; minute < 50; minute++ {
		fmt.Fprintf(&cues, "00:%02d:05.000 --> 00:%02d:07.000\nA line somebody says in minute %d\n\n", minute, minute, minute)
	}
	app.SubtitleVTTCache.Set(helpers.SubtitleCacheKey("movie", movieID, 2), []byte(cues.String()), subtitleCacheTTL)

	router := authenticatedRouter(t, app, viewer.ID)
	for _, query := range []string{"", "?start=600"} {
		path := fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt%s", movieID, query)

		plainRequest := httptest.NewRequest(http.MethodGet, path, nil)
		plain := httptest.NewRecorder()
		router.ServeHTTP(plain, plainRequest)
		if plain.Code != http.StatusOK || plain.Header().Get("Content-Encoding") != "" {
			t.Fatalf("%q: plain response = %d, Content-Encoding %q", query, plain.Code, plain.Header().Get("Content-Encoding"))
		}
		if !slices.Contains(plain.Header().Values("Vary"), "Accept-Encoding") {
			t.Errorf("%q: Vary = %q, want it to name Accept-Encoding", query, plain.Header().Values("Vary"))
		}
		assertOpenAPIExchange(t, "subtitleWebVTT", plainRequest, plain)

		gzipRequest := httptest.NewRequest(http.MethodGet, path, nil)
		gzipRequest.Header.Set("Accept-Encoding", "gzip")
		compressed := httptest.NewRecorder()
		router.ServeHTTP(compressed, gzipRequest)
		if compressed.Code != http.StatusOK || compressed.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%q: gzip response = %d, Content-Encoding %q", query, compressed.Code, compressed.Header().Get("Content-Encoding"))
		}
		if compressed.Header().Get("Content-Type") != subtitleWebVTTContentType {
			t.Errorf("%q: Content-Type = %q, want %q", query, compressed.Header().Get("Content-Type"), subtitleWebVTTContentType)
		}
		if compressed.Header().Get("Content-Length") != strconv.Itoa(compressed.Body.Len()) {
			t.Errorf("%q: Content-Length = %q for a body of %d bytes", query, compressed.Header().Get("Content-Length"), compressed.Body.Len())
		}
		if compressed.Body.Len() >= plain.Body.Len()/2 {
			t.Errorf("%q: compressed cues are %d bytes of %d", query, compressed.Body.Len(), plain.Body.Len())
		}
		assertOpenAPIExchange(t, "subtitleWebVTT", gzipRequest, compressed)

		reader, err := gzip.NewReader(compressed.Body)
		if err != nil {
			t.Fatalf("%q: cues are not gzip: %v", query, err)
		}
		decoded, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("%q: read gzip cues: %v", query, err)
		}
		if string(decoded) != plain.Body.String() {
			t.Errorf("%q: decompressed cues differ from the uncompressed ones", query)
		}
	}
}

func TestSubtitleWebVTT_RejectsNegativeStart(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "?start=-5")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative start, got %d", recorder.Code)
	}
}

func TestSubtitleWebVTT_ExtractsOnceAndServesFromCache(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != subtitleWebVTTContentType {
		t.Fatalf("Content-Type = %q, want %q", got, subtitleWebVTTContentType)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if recorder.Body.String() != "WEBVTT\n" {
		t.Fatalf("body = %q, want extractor output", recorder.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1", fake.SubtitleCallCount())
	}

	second := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if second.Code != http.StatusOK || second.Body.String() != "WEBVTT\n" {
		t.Fatalf("second request = %d %q, want the cached extractor output", second.Code, second.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1 (second request must hit the cache)", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_RejectsBitmapSubtitle(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "hdmv_pgs_subtitle", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var envelope struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not the JSON error envelope: %v", err)
	}
	if !envelope.Error || envelope.Message == "" {
		t.Fatalf("envelope = %+v, want error=true with a message", envelope)
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("extractor call count = %d, want 0 for bitmap codec", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_TrackIndexOutOfRange(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "5", "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// blockingSubtitleFFmpeg holds every extraction until release closes, and
// records whether the extraction's context had been canceled by then.
type blockingSubtitleFFmpeg struct {
	*fakeFFmpeg
	started  chan struct{}
	release  chan struct{}
	canceled atomic.Bool
}

func (f *blockingSubtitleFFmpeg) ExtractSubtitleAsWebVTT(ctx context.Context, path string, streamIndex int64) ([]byte, error) {
	close(f.started)
	<-f.release
	if ctx.Err() != nil {
		f.canceled.Store(true)
		return nil, ctx.Err()
	}
	return f.fakeFFmpeg.ExtractSubtitleAsWebVTT(ctx, path, streamIndex)
}

// A player drops its subtitle request whenever it rebuilds the track, which
// every rebase does. The extraction that request started is shared, so it has
// to finish and be cached rather than die with the request.
func TestSubtitleWebVTT_ExtractionOutlivesTheRequestThatStartedIt(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &blockingSubtitleFFmpeg{
		fakeFFmpeg: &fakeFFmpeg{},
		started:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	router := authenticatedRouter(t, app, viewer.ID)
	ctx, cancel := context.WithCancel(context.Background())
	first := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt", movieID), nil).WithContext(ctx)
	firstRecorder := httptest.NewRecorder()
	served := make(chan struct{})
	go func() {
		router.ServeHTTP(firstRecorder, first)
		close(served)
	}()

	select {
	case <-fake.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the extraction never started")
	}
	cancel()
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the canceled request kept waiting on the extraction")
	}
	if firstRecorder.Body.Len() != 0 {
		t.Fatalf("a request whose client left was answered: %d %q", firstRecorder.Code, firstRecorder.Body.String())
	}

	close(fake.release)
	app.Wait.Wait()
	if fake.canceled.Load() {
		t.Fatal("the shared extraction was canceled with the request that started it")
	}

	second := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if second.Code != http.StatusOK || second.Body.String() != "WEBVTT\n" {
		t.Fatalf("next request = %d %q, want the cached extraction", second.Code, second.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1", fake.SubtitleCallCount())
	}
}

// The run is counted in app.Wait before its caller can return: counted only
// once the shared goroutine got going, a caller that left first could let
// shutdown pass app.Wait.Wait() with the extraction still to run.
func TestExtractSubtitleWebVTT_ShutdownWaitsForARunItsCallerLeft(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake
	cacheKey := helpers.SubtitleCacheKey("movie", 1, testSubtitleStreamIndex)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = app.extractSubtitleWebVTT(ctx, cacheKey, subtitleTrackRef{
		Kind:        mediaKindMovie,
		FileID:      1,
		StreamIndex: testSubtitleStreamIndex,
		SourcePath:  "/media/movie.mkv",
	}, 0)

	app.Wait.Wait()
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count after app.Wait.Wait() = %d, want 1", fake.SubtitleCallCount())
	}
	_, found := app.SubtitleVTTCache.Get(cacheKey)
	if !found {
		t.Fatal("app.Wait.Wait() returned before the extraction was cached")
	}
}

// Without a refresh, a track extracted at the start of a long film expires
// mid-watch, and the next rebase pays for a whole extraction again.
func TestSubtitleWebVTT_CacheHitRestartsTheTTL(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	key := helpers.SubtitleCacheKey("movie", movieID, testSubtitleStreamIndex)
	app.SubtitleVTTCache.Set(key, []byte("WEBVTT\n"), time.Minute)

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	_, expiration, found := app.SubtitleVTTCache.GetWithExpiration(key)
	if !found || time.Until(expiration) < subtitleCacheTTL-time.Minute {
		t.Fatalf("cache entry expires in %s after a hit, want about %s", time.Until(expiration), subtitleCacheTTL)
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("extractor call count = %d, want 0 for a cache hit", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_BitmapContract(t *testing.T) {
	for _, kind := range []mediaKind{mediaKindMovie, mediaKindEpisode} {
		for _, codec := range []string{"hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle", " XSUB "} {
			t.Run(string(kind)+"/"+codec, func(t *testing.T) {
				app := setupTestApp(t)
				viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
				fake := &fakeFFmpeg{}
				app.FFmpeg = fake
				var path, operation string
				if kind == mediaKindMovie {
					id := insertTestHLSMovieFixture(t, app, "h264", 1080)
					insertTestSubtitle(t, app, id, testSubtitleStreamIndex, codec, "eng")
					path = fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt", id)
					operation = "subtitleWebVTT"
				} else {
					fixture := seedPlaybackEpisode(t, app)
					_, err := app.DB.Exec(`UPDATE show_subtitles SET codec = ? WHERE file_id = ?`, codec, fixture.FileID)
					if err != nil {
						t.Fatal(err)
					}
					path = fmt.Sprintf("/api/shows/episodes/%d/subtitles/0/web.vtt", fixture.Episode1)
					operation = "episodeSubtitleWebVTT"
				}
				req := httptest.NewRequest(http.MethodGet, path, nil)
				response := httptest.NewRecorder()
				authenticatedRouter(t, app, viewer.ID).ServeHTTP(response, req)
				if response.Code != http.StatusUnsupportedMediaType {
					t.Fatalf("status = %d, want 415", response.Code)
				}
				assertOpenAPIExchange(t, operation, req, response)
				if fake.SubtitleCallCount() != 0 {
					t.Fatal("bitmap reached extraction")
				}
			})
		}
	}
}

// Each generation has an independently controlled extraction, so the new
// request must finish even while the stale extraction remains blocked.
type generationSubtitleFFmpeg struct {
	*fakeFFmpeg
	calls   atomic.Int32
	started chan int
	release [2]chan struct{}
}

func (f *generationSubtitleFFmpeg) ExtractSubtitleAsWebVTT(ctx context.Context, _ string, _ int64) ([]byte, error) {
	call := int(f.calls.Add(1)) - 1
	f.started <- call
	if call < len(f.release) {
		select {
		case <-f.release[call]:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []byte(fmt.Sprintf("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\ngeneration %d\n", call)), nil
}

func TestSubtitleWebVTT_ConcurrentInvalidation(t *testing.T) {
	for _, kind := range []mediaKind{mediaKindMovie, mediaKindEpisode} {
		t.Run(string(kind), func(t *testing.T) {
			app := setupTestApp(t)
			viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
			fake := &generationSubtitleFFmpeg{fakeFFmpeg: &fakeFFmpeg{}, started: make(chan int, 4), release: [2]chan struct{}{make(chan struct{}), make(chan struct{})}}
			app.FFmpeg = fake
			defer func() { app.beginHLSShutdown(); app.Wait.Wait() }()
			var path string
			var fileID int64
			if kind == mediaKindMovie {
				fileID = insertTestHLSMovieFixture(t, app, "h264", 1080)
				insertTestSubtitle(t, app, fileID, testSubtitleStreamIndex, "subrip", "eng")
				path = fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt", fileID)
			} else {
				fixture := seedPlaybackEpisode(t, app)
				fileID = fixture.FileID
				path = fmt.Sprintf("/api/shows/episodes/%d/subtitles/0/web.vtt", fixture.Episode1)
			}
			router := authenticatedRouter(t, app, viewer.ID)
			serve := func() <-chan *httptest.ResponseRecorder {
				done := make(chan *httptest.ResponseRecorder, 1)
				go func() {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
					done <- response
				}()
				return done
			}
			first := serve()
			select {
			case <-fake.started:
			case <-time.After(5 * time.Second):
				t.Fatal("first extraction did not start")
			}
			// Eviction must stay file-specific, including the separate ID spaces.
			otherKey := helpers.SubtitleCacheKey("movie", fileID+100, 2)
			app.SubtitleVTTCache.Set(otherKey, []byte("WEBVTT\n"), subtitleCacheTTL)
			app.invalidateSubtitleVTTCache(kind, fileID)
			second := serve()
			select {
			case <-fake.started:
			case <-time.After(time.Second):
				t.Fatal("post-invalidation request joined stale extraction")
			}
			close(fake.release[1])
			select {
			case response := <-second:
				if response.Code != 200 || !strings.Contains(response.Body.String(), "generation 1") {
					t.Fatalf("new request: %d %s", response.Code, response.Body)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("new request did not complete")
			}
			close(fake.release[0])
			<-first
			app.Wait.Wait()
			key := helpers.SubtitleCacheKey(string(kind), fileID, 2)
			cached, _ := app.SubtitleVTTCache.Get(key)
			if !strings.Contains(string(cached.([]byte)), "generation 1") {
				t.Fatalf("stale extraction replaced new cache: %s", cached)
			}
			_, found := app.SubtitleVTTCache.Get(otherKey)
			if !found {
				t.Fatal("invalidation removed an unrelated file")
			}
		})
	}
}

// Pause source resolution after the query starts but before the handler has
// its file ID. Capturing the generation only at extraction would be too late.
type pausedSubtitleSourceDB struct {
	database.DBTX
	started, release chan struct{}
}

func (db *pausedSubtitleSourceDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	row := db.DBTX.QueryRowContext(ctx, query, args...)
	if strings.Contains(query, "-- name: GetMovieByID ") {
		close(db.started)
		<-db.release
	}
	return row
}

func TestSubtitleWebVTT_InvalidationDuringSourceResolution(t *testing.T) {
	app := setupTestApp(t)
	app.FFmpeg = &fakeFFmpeg{}
	id := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, id, 2, "subrip", "eng")
	db := &pausedSubtitleSourceDB{DBTX: app.DB, started: make(chan struct{}), release: make(chan struct{})}
	app.Queries = database.New(db)
	router := chi.NewRouter()
	router.Get("/api/movies/{id}/subtitles/{trackIndex}/web.vtt", app.SubtitleWebVTT)
	done := make(chan struct{})
	response := httptest.NewRecorder()
	go func() {
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt", id), nil))
		close(done)
	}()
	select {
	case <-db.started:
	case <-time.After(5 * time.Second):
		t.Fatal("source query did not start")
	}
	app.invalidateSubtitleVTTCache(mediaKindMovie, id)
	close(db.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not finish")
	}
	if response.Code != 200 {
		t.Fatalf("request = %d %s", response.Code, response.Body)
	}
	_, found := app.SubtitleVTTCache.Get(helpers.SubtitleCacheKey("movie", id, 2))
	if found {
		t.Fatal("metadata read across invalidation was published")
	}
}

func TestSubtitleWebVTT_StaleGenerationCannotRenewNewEntry(t *testing.T) {
	app := setupTestApp(t)
	key := helpers.SubtitleCacheKey("movie", 1, 2)
	app.invalidateSubtitleVTTCache(mediaKindMovie, 1)
	app.SubtitleVTTCache.Set(key, []byte("new cues"), time.Minute)
	_, expiration, _ := app.SubtitleVTTCache.GetWithExpiration(key)
	_, hit := app.cachedSubtitleWebVTT(key, 0)
	if hit {
		t.Fatal("stale request read new generation's cache")
	}
	_, after, _ := app.SubtitleVTTCache.GetWithExpiration(key)
	if !after.Equal(expiration) {
		t.Fatal("stale request renewed cache")
	}
}

func TestSubtitleWebVTT_ExtractionTimeoutAndShutdown(t *testing.T) {
	for _, cause := range []string{"timeout", "shutdown"} {
		t.Run(cause, func(t *testing.T) {
			app := setupTestApp(t)
			viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
			fake := &generationSubtitleFFmpeg{fakeFFmpeg: &fakeFFmpeg{}, started: make(chan int, 1), release: [2]chan struct{}{make(chan struct{}), make(chan struct{})}}
			app.FFmpeg = fake
			if cause == "timeout" {
				previous := subtitleExtractTimeout
				subtitleExtractTimeout = 20 * time.Millisecond
				defer func() { subtitleExtractTimeout = previous }()
			}
			id := insertTestHLSMovieFixture(t, app, "h264", 1080)
			insertTestSubtitle(t, app, id, 2, "subrip", "eng")
			router := authenticatedRouter(t, app, viewer.ID)
			request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt", id), nil)
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { router.ServeHTTP(response, request); close(done) }()
			select {
			case <-fake.started:
			case <-time.After(5 * time.Second):
				t.Fatal("extraction did not start")
			}
			if cause == "shutdown" {
				app.beginHLSShutdown()
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("extraction did not stop")
			}
			app.Wait.Wait()
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d: %s", response.Code, response.Body)
			}
			assertOpenAPIExchange(t, "subtitleWebVTT", request, response)
			_, found := app.SubtitleVTTCache.Get(helpers.SubtitleCacheKey("movie", id, 2))
			if found {
				t.Fatal("failed extraction entered cache")
			}
		})
	}
}
