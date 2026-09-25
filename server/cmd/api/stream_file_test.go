package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
)

type streamTestTrack struct {
	database.GetTrackRow
	FilePath string
}

func seedStreamTestTrack(t *testing.T, app *Application, albumID sql.NullInt64, content []byte) streamTestTrack {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "Stream Test.flac")
	err := os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("write track file: %v", err)
	}

	trackID, err := app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
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
	track, err := app.Queries.GetTrack(context.Background(), trackID)
	if err != nil {
		t.Fatal(err)
	}
	return streamTestTrack{GetTrackRow: track, FilePath: path}
}

// The file delivery itself is covered for tracks by
// TestMediaResponsesConformToOpenAPI; only the missing-row path is unique here.
func TestStreamTrack_MissingTrackReturnsNotFound(t *testing.T) {
	app := setupTestApp(t)
	listener := createTestUser(t, app, "Listener", "listener@example.com", false)

	w := httptest.NewRecorder()
	authenticatedRouter(t, app, listener.ID).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/music/tracks/999999/stream", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestMovieStreamFileServesFromCacheUntilEvicted(t *testing.T) {
	app := setupTestApp(t)

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
	app := setupSessionTestApp(t)
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
	app := setupSessionTestApp(t)
	app.InitRouter()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)

	albumIdentity, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Stream Test Album",
		SortTitle: "stream test album",
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}
	album, err := app.Queries.GetAlbumByID(context.Background(), albumIdentity.ID)
	if err != nil {
		t.Fatal(err)
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
