package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func TestReconcileMissingMoviesDeletesStaleRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	existingPath := filepath.Join(moviesDir, "Casino.Royale.2006.mkv")
	err := os.WriteFile(existingPath, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	existingMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Casino Royale",
		FilePath:  existingPath,
		FileName:  filepath.Base(existingPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert existing movie: %v", err)
	}

	deletedPath := filepath.Join(moviesDir, "Deleted.Movie.1999.mkv")
	deletedMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Deleted Movie",
		FilePath:  deletedPath,
		FileName:  filepath.Base(deletedPath),
		Size:      7,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert deleted movie: %v", err)
	}

	reconciled, err := app.reconcileMissingMovies(ctx, moviesDir, map[string]bool{
		filepath.Clean(existingPath): true,
	})
	if err != nil {
		t.Fatalf("reconcileMissingMovies: %v", err)
	}
	if reconciled != 1 {
		t.Fatalf("expected 1 reconciled movie, got %d", reconciled)
	}

	_, err = app.Queries.GetMovieByID(ctx, existingMovie.ID)
	if err != nil {
		t.Fatalf("expected existing movie to remain: %v", err)
	}

	_, err = app.Queries.GetMovieByID(ctx, deletedMovie.ID)
	if err == nil {
		t.Fatal("expected deleted movie row to be removed")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMovieMetadataLocksPreventScannerOverwrite(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	path := "/movies/Moneyball.2011.mkv"

	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball",
		FilePath:  path,
		FileName:  "Moneyball.2011.mkv",
		Size:      100,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Overview:  helpers.NullString("Original overview"),
	})
	if err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	err = app.Queries.LockMovieMetadataFields(ctx, movieMetadataLocks{
		Title:    true,
		Overview: true,
	}.toParams(movie.ID))
	if err != nil {
		t.Fatalf("lock metadata: %v", err)
	}

	updated, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball Remastered",
		FilePath:  path,
		FileName:  "Moneyball.2011.mkv",
		Size:      200,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Overview:  helpers.NullString("Scanner overview"),
	})
	if err != nil {
		t.Fatalf("scanner upsert: %v", err)
	}

	if updated.Title != "Moneyball" {
		t.Fatalf("expected locked title to stay Moneyball, got %q", updated.Title)
	}
	if !updated.Overview.Valid || updated.Overview.String != "Original overview" {
		t.Fatalf("expected locked overview to stay original, got %+v", updated.Overview)
	}
	if updated.Size != 200 {
		t.Fatalf("expected scanner-owned size to update to 200, got %d", updated.Size)
	}
}
