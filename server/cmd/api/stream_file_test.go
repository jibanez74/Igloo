package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"igloo/cmd/internal/database"

	"github.com/go-chi/chi/v5"
)

func seedStreamTestTrack(t *testing.T, app *Application, albumID sql.NullInt64, content []byte) database.Track {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "Stream Test.flac")
	err := os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("write track file: %v", err)
	}

	track, err := app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
		Title:     "Stream Test",
		SortTitle: "stream test",
		FilePath:  path,
		FileName:  filepath.Base(path),
		Container: "flac",
		MimeType:  "audio/flac",
		Codec:     "flac",
		Size:      int64(len(content)),
		AlbumID:   albumID,
	})
	if err != nil {
		t.Fatalf("insert track: %v", err)
	}
	return track
}

// StreamTrack shares serveMediaFile with the movie handlers, so this covers the
// music side of that helper: content type, validator and range support.
func TestStreamTrackServesFileWithRangeSupport(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	content := bytes.Repeat([]byte("abcdefghij"), 30)
	track := seedStreamTestTrack(t, app, sql.NullInt64{}, content)

	router := chi.NewRouter()
	router.Get("/api/music/tracks/{id}/stream", app.StreamTrack)
	target := fmt.Sprintf("/api/music/tracks/%d/stream", track.ID)

	t.Run("full body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Header().Get("Content-Type"); got != "audio/flac" {
			t.Errorf("Content-Type = %q, want audio/flac", got)
		}
		if w.Header().Get("ETag") == "" {
			t.Error("ETag was not set")
		}
		if !bytes.Equal(w.Body.Bytes(), content) {
			t.Error("body does not match file content")
		}
	})

	t.Run("range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Range", "bytes=5-14")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusPartialContent)
		}
		if !bytes.Equal(w.Body.Bytes(), content[5:15]) {
			t.Errorf("body = %q, want %q", w.Body.Bytes(), content[5:15])
		}
	})

	t.Run("missing row", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/999999/stream", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestMovieStreamFileServesFromCacheUntilEvicted(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", []byte("payload"))
	ctx := context.Background()

	first, err := app.movieStreamFile(ctx, movie.ID)
	if err != nil {
		t.Fatalf("resolve movie: %v", err)
	}

	// Deleting the row would make an uncached lookup fail, so a successful
	// second resolve can only have come from the cache.
	err = app.Queries.DeleteMovie(ctx, movie.ID)
	if err != nil {
		t.Fatalf("delete movie: %v", err)
	}

	cached, err := app.movieStreamFile(ctx, movie.ID)
	if err != nil {
		t.Fatalf("resolve cached movie: %v", err)
	}
	if cached != first {
		t.Errorf("cached resolve = %+v, want %+v", cached, first)
	}

	app.StreamFileCache.invalidate(movieStreamFileKey(movie.ID))

	_, err = app.movieStreamFile(ctx, movie.ID)
	if err == nil {
		t.Error("resolve after eviction succeeded; the deleted row was still served")
	}
}

func TestTrackStreamFileServesFromCacheUntilEvicted(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	track := seedStreamTestTrack(t, app, sql.NullInt64{}, []byte("payload"))
	ctx := context.Background()

	first, err := app.trackStreamFile(ctx, track.ID)
	if err != nil {
		t.Fatalf("resolve track: %v", err)
	}

	// Tracks have no delete query of their own; they go with their album.
	_, err = app.DB.Exec("DELETE FROM tracks WHERE id = ?", track.ID)
	if err != nil {
		t.Fatalf("delete track: %v", err)
	}

	cached, err := app.trackStreamFile(ctx, track.ID)
	if err != nil {
		t.Fatalf("resolve cached track: %v", err)
	}
	if cached != first {
		t.Errorf("cached resolve = %+v, want %+v", cached, first)
	}

	app.StreamFileCache.invalidate(trackStreamFileKey(track.ID))

	_, err = app.trackStreamFile(ctx, track.ID)
	if err == nil {
		t.Error("resolve after eviction succeeded; the deleted row was still served")
	}
}

// Deleting a movie must drop its resolved file the same way it drops cached
// subtitles, so a re-added movie at the same id cannot serve the old path.
func TestDeleteMovieEvictsStreamFileCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", []byte("payload"))

	_, err := app.movieStreamFile(context.Background(), movie.ID)
	if err != nil {
		t.Fatalf("resolve movie: %v", err)
	}

	req := newOpenAPIJSONRequest(http.MethodDelete, fmt.Sprintf("/api/movies/%d", movie.ID), `{}`)
	req.AddCookie(newAuthSessionCookie(t, app, admin.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "deleteMovie", req, w)

	_, cached := app.StreamFileCache.get(movieStreamFileKey(movie.ID))
	if cached {
		t.Error("deleting a movie left its resolved file in the stream cache")
	}

	// The eviction has to outlast the request: a lookup afterwards must fail
	// rather than serve the path the handler just removed.
	_, err = app.movieStreamFile(context.Background(), movie.ID)
	if err == nil {
		t.Error("resolving a deleted movie succeeded; the stream cache outlived the row")
	}
}

// Albums cascade to their tracks, so the resolved files of those tracks must go
// with them. Without this the deleted tracks stay streamable until the TTL.
func TestDeleteAlbumEvictsTrackStreamFileCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)

	album, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Stream Test Album",
		SortTitle: "stream test album",
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	track := seedStreamTestTrack(t, app, sql.NullInt64{Int64: album.ID, Valid: true}, []byte("payload"))

	_, err = app.trackStreamFile(context.Background(), track.ID)
	if err != nil {
		t.Fatalf("resolve track: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/music/albums/%d", album.ID), nil)
	req.AddCookie(newAuthSessionCookie(t, app, admin.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "deleteAlbum", req, w)

	_, err = app.trackStreamFile(context.Background(), track.ID)
	if err == nil {
		t.Error("resolving a cascaded-deleted track succeeded; the stream cache outlived the row")
	}
}

// A reader that misses the cache reads its row before the query returns, so a
// delete can land in between. The generation guard is what stops that reader
// from republishing what it read.
func TestStreamFileCacheDropsFillRacingAnInvalidation(t *testing.T) {
	c := newStreamFileCache(time.Minute, 2*time.Minute)
	key := movieStreamFileKey(1)

	// Captured before the query, as resolveStreamFile does.
	gen := c.generation()

	// The row is deleted and the key evicted while that query is in flight.
	c.invalidate(key)

	c.setIfCurrent(key, gen, streamFile{Path: "/deleted.mp4"})

	_, hit := c.get(key)
	if hit {
		t.Error("a fill started before the invalidation was published; the deleted file stays streamable until the TTL")
	}

	// A fill that starts after the invalidation is still cached normally.
	fresh := streamFile{Path: "/current.mp4"}
	c.setIfCurrent(key, c.generation(), fresh)

	got, hit := c.get(key)
	if !hit || got != fresh {
		t.Errorf("post-invalidation fill = %+v (hit %v), want %+v (hit true)", got, hit, fresh)
	}
}
