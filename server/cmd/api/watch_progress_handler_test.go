package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const testWatchProgressSaveSessionID = "11111111-1111-4111-8111-111111111111"

func createTestUserAndMovie(t *testing.T, app *Application) (userID, movieID int64) {
	t.Helper()
	ctx := context.Background()

	user, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashed",
		IsAdmin:  false,
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// MP4 on purpose: watch-room tests create direct-mode rooms with this
	// movie, and direct playback is refused for non-MP4 containers.
	movieID, err = app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileName:  "test.mp4",
		Size:      1024,
		Container: "mp4",
		MimeType:  helpers.VideoMimeTypes["mp4"],
	})
	if err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		t.Fatal(err)
	}

	return user.ID, movie.ID
}

func TestWatchProgressHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	userID, movieID := createTestUserAndMovie(t, app)
	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, userID)

	assertRequest := func(operationID string, req *http.Request) {
		t.Helper()
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", operationID, response.Code, response.Body.String())
		}
		assertOpenAPIExchange(t, operationID, req, response)
	}

	progressPath := fmt.Sprintf("/api/movies/%d/watch-progress", movieID)
	assertRequest("getMovieWatchProgress", httptest.NewRequest(http.MethodGet, progressPath, nil))
	updateBody := `{"progress_sec":120,"duration_sec":7200,"save_session_id":"11111111-1111-4111-8111-111111111111","save_sequence":1}`
	assertRequest("updateMovieWatchProgress", newOpenAPIJSONRequest(http.MethodPut, progressPath, updateBody))
	watchedPath := fmt.Sprintf("/api/movies/%d/watch-progress/watched", movieID)
	assertRequest("setMovieWatched", newOpenAPIJSONRequest(http.MethodPut, watchedPath, `{"watched":false}`))
	assertRequest("deleteMovieWatchProgress", httptest.NewRequest(http.MethodDelete, progressPath, nil))
}

func TestWatchProgress_UpsertAndGet(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   300.5,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("UpsertMovieWatchProgress failed: %v", err)
	}

	row, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress failed: %v", err)
	}

	if row.ProgressSec != 300.5 {
		t.Errorf("expected progress_sec 300.5, got %f", row.ProgressSec)
	}
	if row.DurationSec != 7200.0 {
		t.Errorf("expected duration_sec 7200.0, got %f", row.DurationSec)
	}
	if row.Watched {
		t.Error("expected watched to be false after upsert")
	}
}

func TestWatchProgress_UpsertResetsWatchedFlag(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.MarkMovieWatched(ctx, database.MarkMovieWatchedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("MarkMovieWatched failed: %v", err)
	}

	row, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress failed: %v", err)
	}
	if !row.Watched {
		t.Fatal("expected watched=true after MarkMovieWatched")
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   60.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("UpsertMovieWatchProgress failed: %v", err)
	}

	row, err = app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress after upsert failed: %v", err)
	}
	if row.Watched {
		t.Error("expected watched=false after UpsertMovieWatchProgress on a watched movie")
	}
	if row.ProgressSec != 60.0 {
		t.Errorf("expected progress_sec 60.0, got %f", row.ProgressSec)
	}
}

func TestWatchProgress_Delete(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   300.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("UpsertMovieWatchProgress failed: %v", err)
	}

	err = app.Queries.DeleteMovieWatchProgress(ctx, database.DeleteMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("DeleteMovieWatchProgress failed: %v", err)
	}

	_, err = app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got: %v", err)
	}
}

func TestWatchProgress_MarkWatchedClearsExistingProgress(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   3600.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("UpsertMovieWatchProgress failed: %v", err)
	}

	err = app.Queries.MarkMovieWatched(ctx, database.MarkMovieWatchedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("MarkMovieWatched failed: %v", err)
	}

	row, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress failed: %v", err)
	}

	if !row.Watched {
		t.Error("expected watched=true after MarkMovieWatched")
	}
	if row.ProgressSec != 0 {
		t.Errorf("expected progress_sec=0 (cleared), got %f", row.ProgressSec)
	}
}

func TestWatchProgress_MarkUnwatched(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.MarkMovieWatched(ctx, database.MarkMovieWatchedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("MarkMovieWatched failed: %v", err)
	}

	err = app.Queries.MarkMovieUnwatched(ctx, database.MarkMovieUnwatchedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("MarkMovieUnwatched failed: %v", err)
	}

	row, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress failed: %v", err)
	}

	if row.Watched {
		t.Error("expected watched=false after MarkMovieUnwatched")
	}
}

func TestWatchProgress_MarkUnwatchedIdempotentCreatesRow(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.MarkMovieUnwatched(ctx, database.MarkMovieUnwatchedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("MarkMovieUnwatched failed: %v", err)
	}

	row, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress failed: %v", err)
	}
	if row.Watched {
		t.Error("expected watched=false when inserting unwatched row")
	}
	if row.ProgressSec != 0 || row.DurationSec != 0 {
		t.Errorf("expected zero progress/duration for new row, got progress_sec=%f duration_sec=%f",
			row.ProgressSec, row.DurationSec)
	}
}

func TestWatchProgress_PerUserIsolation(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	user1, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "User One",
		Email:    "one@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}

	user2, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "User Two",
		Email:    "two@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Shared Movie",
		FilePath:  "/movies/shared.mkv",
		FileName:  "shared.mkv",
		Size:      2048,
		Container: "mkv",
		MimeType:  "video/x-matroska",
	})
	if err != nil {
		t.Fatalf("failed to create movie: %v", err)
	}
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		t.Fatal(err)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        user1.ID,
		MovieID:       movie.ID,
		ProgressSec:   600.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("upsert for user1 failed: %v", err)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        user2.ID,
		MovieID:       movie.ID,
		ProgressSec:   1800.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("upsert for user2 failed: %v", err)
	}

	row1, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  user1.ID,
		MovieID: movie.ID,
	})
	if err != nil {
		t.Fatalf("get for user1 failed: %v", err)
	}

	row2, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  user2.ID,
		MovieID: movie.ID,
	})
	if err != nil {
		t.Fatalf("get for user2 failed: %v", err)
	}

	if row1.ProgressSec != 600.0 {
		t.Errorf("user1 progress_sec: expected 600.0, got %f", row1.ProgressSec)
	}
	if row2.ProgressSec != 1800.0 {
		t.Errorf("user2 progress_sec: expected 1800.0, got %f", row2.ProgressSec)
	}
}

func TestGetContinueWatchingMovies(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	user, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Watcher",
		Email:    "watcher@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	otherUser, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Other",
		Email:    "other@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("failed to create other user: %v", err)
	}

	createMovie := func(title, fileName string) int64 {
		movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
			Title:     title,
			FilePath:  "/movies/" + fileName,
			FileName:  fileName,
			Size:      1024,
			Container: "mkv",
			MimeType:  "video/x-matroska",
		})
		if err != nil {
			t.Fatalf("failed to create movie %q: %v", title, err)
		}
		movie, err := app.Queries.GetMovieByID(ctx, movieID)
		if err != nil {
			t.Fatal(err)
		}
		return movie.ID
	}

	oldInProgressID := createMovie("Old In Progress", "old-in-progress.mkv")
	recentInProgressID := createMovie("Recent In Progress", "recent-in-progress.mkv")
	completedID := createMovie("Completed", "completed.mkv")
	watchedID := createMovie("Watched", "watched.mkv")
	unwatchedZeroID := createMovie("Unwatched Zero", "unwatched-zero.mkv")
	belowFloorID := createMovie("Below Floor", "below-floor.mkv")
	atFloorID := createMovie("At Floor", "at-floor.mkv")
	otherUserID := createMovie("Other User Movie", "other-user.mkv")

	upsertProgress := func(userID, movieID int64, progressSec float64) {
		err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
			UserID:        userID,
			MovieID:       movieID,
			ProgressSec:   progressSec,
			DurationSec:   7200.0,
			SaveSessionID: testWatchProgressSaveSessionID,
			SaveSequence:  1,
		})
		if err != nil {
			t.Fatalf("failed to upsert progress for movie %d: %v", movieID, err)
		}
	}

	upsertProgress(user.ID, oldInProgressID, 300.0)
	upsertProgress(user.ID, recentInProgressID, 1200.0)
	upsertProgress(user.ID, completedID, 7200.0)
	upsertProgress(user.ID, belowFloorID, 29.0)
	upsertProgress(user.ID, atFloorID, 30.0)
	upsertProgress(otherUser.ID, otherUserID, 900.0)

	err = app.Queries.MarkMovieWatched(ctx, database.MarkMovieWatchedParams{
		UserID:  user.ID,
		MovieID: watchedID,
	})
	if err != nil {
		t.Fatalf("failed to mark movie watched: %v", err)
	}

	err = app.Queries.MarkMovieUnwatched(ctx, database.MarkMovieUnwatchedParams{
		UserID:  user.ID,
		MovieID: unwatchedZeroID,
	})
	if err != nil {
		t.Fatalf("failed to mark movie unwatched: %v", err)
	}

	// CURRENT_TIMESTAMP has second resolution, so force distinct older
	// timestamps to make the recency ordering deterministic.
	_, err = app.DB.ExecContext(ctx,
		"UPDATE movie_watch_progress SET updated_at = datetime('now', '-1 hour') WHERE user_id = ? AND movie_id = ?",
		user.ID, oldInProgressID,
	)
	if err != nil {
		t.Fatalf("failed to backdate progress row: %v", err)
	}
	_, err = app.DB.ExecContext(ctx,
		"UPDATE movie_watch_progress SET updated_at = datetime('now', '-2 hours') WHERE user_id = ? AND movie_id = ?",
		user.ID, atFloorID,
	)
	if err != nil {
		t.Fatalf("failed to backdate at-floor progress row: %v", err)
	}

	rows, err := app.Queries.GetContinueWatchingMovies(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetContinueWatchingMovies failed: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 continue watching movies, got %d", len(rows))
	}
	if rows[0].ID != recentInProgressID {
		t.Errorf("expected most recently watched movie %d first, got %d", recentInProgressID, rows[0].ID)
	}
	if rows[1].ID != oldInProgressID {
		t.Errorf("expected older movie %d second, got %d", oldInProgressID, rows[1].ID)
	}
	if rows[2].ID != atFloorID {
		t.Errorf("expected at-floor movie %d third, got %d", atFloorID, rows[2].ID)
	}
	for _, row := range rows {
		if row.ID == belowFloorID {
			t.Error("expected below-floor progress (29s) to be excluded from continue watching")
		}
	}
	if rows[0].ProgressSec != 1200.0 {
		t.Errorf("expected progress_sec 1200.0, got %f", rows[0].ProgressSec)
	}
	if rows[0].DurationSec != 7200.0 {
		t.Errorf("expected duration_sec 7200.0, got %f", rows[0].DurationSec)
	}

	otherRows, err := app.Queries.GetContinueWatchingMovies(ctx, otherUser.ID)
	if err != nil {
		t.Fatalf("GetContinueWatchingMovies for other user failed: %v", err)
	}
	if len(otherRows) != 1 || otherRows[0].ID != otherUserID {
		t.Errorf("expected other user to only see their own in-progress movie, got %+v", otherRows)
	}
}

func TestWatchProgress_CascadeDeleteMovie(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   300.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("UpsertMovieWatchProgress failed: %v", err)
	}

	err = app.Queries.DeleteMovie(ctx, movieID)
	if err != nil {
		t.Fatalf("DeleteMovie failed: %v", err)
	}

	_, err = app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != sql.ErrNoRows {
		t.Errorf("expected progress row to be cascade-deleted with movie, got: %v", err)
	}
}

func TestWatchProgress_CascadeDeleteUser(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   300.0,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("UpsertMovieWatchProgress failed: %v", err)
	}

	err = app.Queries.DeleteUser(ctx, userID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	_, err = app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != sql.ErrNoRows {
		t.Errorf("expected progress row to be cascade-deleted with user, got: %v", err)
	}
}

// TestUpdateMovieWatchProgress_HTTPCompletionThreshold drives the real handler
// so the 98% auto-watched rule and the progress clamp are asserted end to end:
// on the response body and on the persisted row, not on arithmetic repeated in
// the test.
func TestUpdateMovieWatchProgress_HTTPCompletionThreshold(t *testing.T) {
	tests := []struct {
		name         string
		progressSec  float64
		durationSec  float64
		wantWatched  bool
		wantProgress float64
	}{
		{name: "half way is not watched", progressSec: 3600, durationSec: 7200, wantProgress: 3600},
		{name: "just below the threshold is not watched", progressSec: 7055, durationSec: 7200, wantProgress: 7055},
		{name: "negative progress clamps to zero", progressSec: -50, durationSec: 7200, wantProgress: 0},
		{name: "exactly at the threshold is watched", progressSec: 7056, durationSec: 7200, wantWatched: true},
		{name: "above the duration is watched", progressSec: 8000, durationSec: 7200, wantWatched: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupSessionTestApp(t)
			defer app.DB.Close()

			userID, movieID := createTestUserAndMovie(t, app)

			r := chi.NewRouter()
			r.Put("/api/movies/{id}/watch-progress", func(w http.ResponseWriter, r *http.Request) {
				app.SessionManager.Put(r.Context(), cookieUserID, userID)
				app.UpdateMovieWatchProgress(w, r)
			})
			handler := app.SessionManager.LoadAndSave(r)

			body := fmt.Sprintf(
				`{"progress_sec": %v, "duration_sec": %v, "save_session_id": %q, "save_sequence": 1}`,
				tt.progressSec, tt.durationSec, testWatchProgressSaveSessionID,
			)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/movies/%d/watch-progress", movieID), strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}

			var resp struct {
				Error bool `json:"error"`
				Data  struct {
					Watched *bool `json:"watched"`
				} `json:"data"`
			}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if resp.Error {
				t.Fatalf("expected success response, got %s", w.Body.String())
			}
			if resp.Data.Watched == nil {
				t.Fatalf("response omitted data.watched, got %s", w.Body.String())
			}
			if *resp.Data.Watched != tt.wantWatched {
				t.Errorf("response watched = %v, want %v", *resp.Data.Watched, tt.wantWatched)
			}

			row, err := app.Queries.GetMovieWatchProgress(context.Background(), database.GetMovieWatchProgressParams{
				UserID:  userID,
				MovieID: movieID,
			})
			if err != nil {
				t.Fatalf("GetMovieWatchProgress: %v", err)
			}
			if row.Watched != tt.wantWatched {
				t.Errorf("persisted watched = %v, want %v", row.Watched, tt.wantWatched)
			}
			if !tt.wantWatched && row.ProgressSec != tt.wantProgress {
				t.Errorf("persisted progress_sec = %v, want %v", row.ProgressSec, tt.wantProgress)
			}
		})
	}
}

func TestWatchProgress_SaveOrdering(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)
	otherSessionID := "22222222-2222-4222-8222-222222222222"

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   500,
		DurationSec:   1000,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  2,
	})
	if err != nil {
		t.Fatalf("save sequence 2: %v", err)
	}

	const preservedUpdatedAt = "2001-02-03 04:05:06"
	_, err = app.DB.ExecContext(ctx,
		"UPDATE movie_watch_progress SET updated_at = ? WHERE user_id = ? AND movie_id = ?",
		preservedUpdatedAt, userID, movieID,
	)
	if err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   100,
		DurationSec:   1000,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("stale save: %v", err)
	}

	readStoredWatchProgress := func() (struct {
		database.GetMovieWatchProgressRow
		SaveSessionID string
		SaveSequence  int64
	}, error) {
		var stored struct {
			database.GetMovieWatchProgressRow
			SaveSessionID string
			SaveSequence  int64
		}
		err := app.DB.QueryRowContext(ctx, "SELECT progress_sec, duration_sec, watched, updated_at, save_session_id, save_sequence FROM movie_watch_progress WHERE user_id = ? AND movie_id = ?", userID, movieID).Scan(&stored.ProgressSec, &stored.DurationSec, &stored.Watched, &stored.UpdatedAt, &stored.SaveSessionID, &stored.SaveSequence)
		return stored, err
	}

	row, err := readStoredWatchProgress()
	if err != nil {
		t.Fatalf("get after stale save: %v", err)
	}
	if row.ProgressSec != 500 || row.SaveSequence != 2 || row.UpdatedAt != preservedUpdatedAt {
		t.Fatalf("stale save changed row: %+v", row)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   200,
		DurationSec:   1000,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  2,
	})
	if err != nil {
		t.Fatalf("equal-sequence save: %v", err)
	}
	row, err = readStoredWatchProgress()
	if err != nil {
		t.Fatalf("get after equal-sequence save: %v", err)
	}
	if row.ProgressSec != 500 || row.SaveSequence != 2 || row.UpdatedAt != preservedUpdatedAt {
		t.Fatalf("equal-sequence save changed row: %+v", row)
	}

	err = app.Queries.MarkMovieWatchedFromProgress(ctx, database.MarkMovieWatchedFromProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  3,
	})
	if err != nil {
		t.Fatalf("completion save: %v", err)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   700,
		DurationSec:   1000,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  2,
	})
	if err != nil {
		t.Fatalf("stale save after completion: %v", err)
	}
	row, err = readStoredWatchProgress()
	if err != nil {
		t.Fatalf("get after stale completion overwrite: %v", err)
	}
	if !row.Watched || row.SaveSequence != 3 {
		t.Fatalf("stale save overwrote completion: %+v", row)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   60,
		DurationSec:   1000,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  4,
	})
	if err != nil {
		t.Fatalf("higher-sequence rewind: %v", err)
	}
	row, err = readStoredWatchProgress()
	if err != nil {
		t.Fatalf("get after rewind: %v", err)
	}
	if row.Watched || row.ProgressSec != 60 {
		t.Fatalf("higher-sequence rewind was not saved: %+v", row)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   250,
		DurationSec:   1000,
		SaveSessionID: otherSessionID,
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("different-session save: %v", err)
	}
	row, err = readStoredWatchProgress()
	if err != nil {
		t.Fatalf("get after different-session save: %v", err)
	}
	if row.ProgressSec != 250 || row.SaveSessionID != otherSessionID || row.SaveSequence != 1 {
		t.Fatalf("different-session last write was not saved: %+v", row)
	}
}

func TestUpdateMovieWatchProgress_HTTPMissingFields(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	userID, movieID := createTestUserAndMovie(t, app)

	r := chi.NewRouter()
	r.Put("/api/movies/{id}/watch-progress", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.UpdateMovieWatchProgress(w, r)
	})
	handler := app.SessionManager.LoadAndSave(r)

	run := func(t *testing.T, body string, wantStatus int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/movies/%d/watch-progress", movieID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != wantStatus {
			t.Errorf("body=%q status=%d, want %d, resp=%s", body, w.Code, wantStatus, w.Body.String())
		}
	}

	t.Run("empty object", func(t *testing.T) {
		run(t, `{}`, http.StatusBadRequest)
	})
	t.Run("missing duration", func(t *testing.T) {
		run(t, `{"progress_sec": 100}`, http.StatusBadRequest)
	})
	t.Run("missing progress", func(t *testing.T) {
		run(t, `{"duration_sec": 7200}`, http.StatusBadRequest)
	})
	t.Run("missing save session", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200, "save_sequence": 1}`, http.StatusBadRequest)
	})
	t.Run("malformed save session", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200, "save_session_id": "invalid", "save_sequence": 1}`, http.StatusBadRequest)
	})
	t.Run("missing save sequence", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200, "save_session_id": "11111111-1111-4111-8111-111111111111"}`, http.StatusBadRequest)
	})
	t.Run("zero save sequence", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200, "save_session_id": "11111111-1111-4111-8111-111111111111", "save_sequence": 0}`, http.StatusBadRequest)
	})
	t.Run("negative save sequence", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200, "save_session_id": "11111111-1111-4111-8111-111111111111", "save_sequence": -1}`, http.StatusBadRequest)
	})
	t.Run("valid body", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200, "save_session_id": "11111111-1111-4111-8111-111111111111", "save_sequence": 1}`, http.StatusOK)
	})
}

func TestSetMovieWatched_HTTPMissingWatched(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	userID, movieID := createTestUserAndMovie(t, app)

	r := chi.NewRouter()
	r.Put("/api/movies/{id}/watch-progress/watched", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.SetMovieWatched(w, r)
	})
	handler := app.SessionManager.LoadAndSave(r)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/movies/%d/watch-progress/watched", movieID), strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/movies/%d/watch-progress/watched", movieID), strings.NewReader(`{"watched": true}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("status=%d, want %d, body=%s", w2.Code, http.StatusOK, w2.Body.String())
	}
	var watchedResp struct {
		Error bool `json:"error"`
		Data  struct {
			MovieID int64 `json:"movie_id"`
			Watched bool  `json:"watched"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &watchedResp); err != nil {
		t.Fatalf("unmarshal watched response: %v", err)
	}
	if watchedResp.Error {
		t.Fatalf("expected success response, got %s", w2.Body.String())
	}
	if watchedResp.Data.MovieID != movieID {
		t.Errorf("movie_id=%d, want %d", watchedResp.Data.MovieID, movieID)
	}
	if !watchedResp.Data.Watched {
		t.Error("expected watched=true response")
	}
	row, err := app.Queries.GetMovieWatchProgress(context.Background(), database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress after watched=true: %v", err)
	}
	if !row.Watched {
		t.Error("expected watched=true to persist")
	}

	req3 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/movies/%d/watch-progress/watched", movieID), strings.NewReader(`{"watched": false}`))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("status=%d, want %d, body=%s", w3.Code, http.StatusOK, w3.Body.String())
	}
	watchedResp = struct {
		Error bool `json:"error"`
		Data  struct {
			MovieID int64 `json:"movie_id"`
			Watched bool  `json:"watched"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(w3.Body.Bytes(), &watchedResp); err != nil {
		t.Fatalf("unmarshal watched=false response: %v", err)
	}
	if watchedResp.Error {
		t.Fatalf("expected success response, got %s", w3.Body.String())
	}
	if watchedResp.Data.MovieID != movieID {
		t.Errorf("movie_id=%d, want %d", watchedResp.Data.MovieID, movieID)
	}
	if watchedResp.Data.Watched {
		t.Error("expected watched=false response")
	}
	row, err = app.Queries.GetMovieWatchProgress(context.Background(), database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress after watched=false: %v", err)
	}
	if row.Watched {
		t.Error("expected watched=false to persist")
	}
}

func TestReadJSON_WatchProgressRequest_DisallowUnknownFields(t *testing.T) {
	var req updateWatchProgressRequest
	body := strings.NewReader(`{"progress_sec": 1, "duration_sec": 2, "save_session_id": "11111111-1111-4111-8111-111111111111", "save_sequence": 1, "extra": true}`)
	r := httptest.NewRequest(http.MethodPut, "/", body)
	w := httptest.NewRecorder()
	err := helpers.ReadJSON(w, r, &req, 1024)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

// backdateProgress forces a distinct, older updated_at so recency ordering is
// deterministic: CURRENT_TIMESTAMP only has second resolution.
func backdateProgress(t *testing.T, app *Application, table, column string, userID, mediaID int64, offset string) {
	t.Helper()

	query := fmt.Sprintf("UPDATE %s SET updated_at = datetime('now', ?) WHERE user_id = ? AND %s = ?", table, column)
	_, err := app.DB.Exec(query, offset, userID, mediaID)
	if err != nil {
		t.Fatalf("backdate %s row: %v", table, err)
	}
}

// saveSequence orders writes within a playback session: a later write to the
// same episode has to advance it or the upsert's guard discards it.
func seedEpisodeWatchProgress(t *testing.T, app *Application, userID, episodeID int64, progressSec float64, saveSequence int64) {
	t.Helper()

	err := app.Queries.UpsertShowEpisodeWatchProgress(context.Background(), database.UpsertShowEpisodeWatchProgressParams{
		UserID:        userID,
		EpisodeID:     episodeID,
		ProgressSec:   progressSec,
		DurationSec:   7200.0,
		SaveSessionID: testWatchProgressSaveSessionID,
		SaveSequence:  saveSequence,
	})
	if err != nil {
		t.Fatalf("seed episode watch progress for episode %d: %v", episodeID, err)
	}
}

// The merged row is the only place a movie and an episode share a payload, so
// it is validated with both kinds present: an empty or single-kind list would
// leave one branch of the contract's oneOf unexercised.
func TestGetContinueWatching_ConformsToOpenAPIWithRows(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Watcher", "continue-watching@example.com", false)
	movieID := createSearchMovie(t, app, "Contract Movie", "/movies/contract-continue.mkv")
	seedWatchProgress(t, app, user.ID, movieID)

	fixture := seedPlaybackEpisode(t, app)
	seedEpisodeWatchProgress(t, app, user.ID, fixture.Episode1, 900.0, 1)
	backdateProgress(t, app, "movie_watch_progress", "movie_id", user.ID, movieID, "-1 hour")

	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/continue-watching", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	assertResponseListNotEmpty(t, "getContinueWatching", response.Body.Bytes(), "items")
	assertOpenAPIExchange(t, "getContinueWatching", request, response)

	var payload struct {
		Data struct {
			Items []struct {
				Kind          string  `json:"kind"`
				ID            int64   `json:"id"`
				Title         string  `json:"title"`
				ProgressSec   float64 `json:"progress_sec"`
				DurationSec   float64 `json:"duration_sec"`
				ShowID        int64   `json:"show_id"`
				SeasonNumber  int64   `json:"season_number"`
				EpisodeNumber int64   `json:"episode_number"`
				EpisodeName   string  `json:"episode_name"`
			} `json:"items"`
		} `json:"data"`
	}
	err := json.Unmarshal(response.Body.Bytes(), &payload)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	items := payload.Data.Items
	if len(items) != 2 {
		t.Fatalf("expected the movie and the episode, got %d items: %s", len(items), response.Body.String())
	}

	episode := items[0]
	if episode.Kind != "episode" || episode.ID != fixture.Episode1 {
		t.Fatalf("expected the just-watched episode first, got %+v", episode)
	}
	if episode.Title != "Playback Show" || episode.ShowID != fixture.ShowID {
		t.Errorf("episode item should carry the show's identity, got %+v", episode)
	}
	if episode.SeasonNumber != 1 || episode.EpisodeNumber != 1 || episode.EpisodeName != "Pilot" {
		t.Errorf("episode item season/episode/name = %d/%d/%q", episode.SeasonNumber, episode.EpisodeNumber, episode.EpisodeName)
	}
	if episode.ProgressSec != 900.0 || episode.DurationSec != 7200.0 {
		t.Errorf("episode progress = %f/%f, want 900/7200", episode.ProgressSec, episode.DurationSec)
	}

	movie := items[1]
	if movie.Kind != "movie" || movie.ID != movieID {
		t.Fatalf("expected the older movie second, got %+v", movie)
	}
	if movie.ShowID != 0 || movie.EpisodeName != "" {
		t.Errorf("movie item must omit the episode fields, got %+v", movie)
	}
}

func TestGetContinueWatchingEpisodes_OnePerShowAndOnlyWhatCanBeResumed(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	user := createTestUser(t, app, "Watcher", "episode-continue@example.com", false)
	otherUser := createTestUser(t, app, "Other", "episode-continue-other@example.com", false)

	fixture := seedPlaybackEpisode(t, app)

	// A third episode the scanner knows but has no file for: nothing to resume.
	fileless, err := app.Queries.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
		SeasonID:      fixture.SeasonID,
		EpisodeNumber: 3,
		Name:          "Fileless",
	})
	if err != nil {
		t.Fatalf("upsert fileless episode: %v", err)
	}

	otherShow := seedSecondShowEpisode(t, app)

	seedEpisodeWatchProgress(t, app, user.ID, fixture.Episode1, 300.0, 1)
	seedEpisodeWatchProgress(t, app, user.ID, fixture.Episode2, 1200.0, 1)
	seedEpisodeWatchProgress(t, app, user.ID, fileless.ID, 600.0, 1)
	seedEpisodeWatchProgress(t, app, user.ID, otherShow, 900.0, 1)
	seedEpisodeWatchProgress(t, app, otherUser.ID, fixture.Episode1, 900.0, 1)

	backdateProgress(t, app, "show_episode_watch_progress", "episode_id", user.ID, fixture.Episode1, "-1 hour")
	backdateProgress(t, app, "show_episode_watch_progress", "episode_id", user.ID, otherShow, "-2 hours")

	rows, err := app.Queries.GetContinueWatchingEpisodes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetContinueWatchingEpisodes failed: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected one row per show, got %d: %+v", len(rows), rows)
	}
	if rows[0].ID != fixture.Episode2 {
		t.Errorf("expected the show's most recent in-progress episode %d, got %d", fixture.Episode2, rows[0].ID)
	}
	if rows[0].ShowID != fixture.ShowID || rows[0].ShowName != "Playback Show" {
		t.Errorf("row should carry the show identity, got %+v", rows[0])
	}
	if rows[1].ID != otherShow {
		t.Errorf("expected the older show's episode %d second, got %d", otherShow, rows[1].ID)
	}
	for _, row := range rows {
		if row.ID == fileless.ID {
			t.Error("an episode with no file cannot be resumed and must be excluded")
		}
	}

	// Finishing the surviving episode drops the show out of the row entirely:
	// the sibling is only hidden by the one-per-show collapse.
	err = app.Queries.MarkShowEpisodeWatched(ctx, database.MarkShowEpisodeWatchedParams{
		UserID:    user.ID,
		EpisodeID: fixture.Episode2,
	})
	if err != nil {
		t.Fatalf("mark episode watched: %v", err)
	}
	seedEpisodeWatchProgress(t, app, user.ID, fixture.Episode1, 29.0, 2)

	rows, err = app.Queries.GetContinueWatchingEpisodes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetContinueWatchingEpisodes after watching failed: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != otherShow {
		t.Fatalf("watched and below-floor episodes must be excluded, got %+v", rows)
	}
}

// seedSecondShowEpisode adds a second show with one season, one file and one
// episode linked to it, and returns that episode's id.
func seedSecondShowEpisode(t *testing.T, app *Application) int64 {
	t.Helper()
	ctx := context.Background()
	q := app.Queries

	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: "/shows/Second Show (2022)",
		LocalName:     "Second Show",
		PremiereYear:  sql.NullInt64{Int64: 2022, Valid: true},
		Name:          "Second Show",
	})
	if err != nil {
		t.Fatalf("upsert second show: %v", err)
	}

	season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
		ShowID:       show.ID,
		SeasonNumber: 2,
		Name:         "Season 2",
	})
	if err != nil {
		t.Fatalf("upsert second show season: %v", err)
	}

	path := fmt.Sprintf("/tmp/%s-second-S02E01.mkv", sanitizeTestPathComponent(t.Name()))
	file, err := q.UpsertShowFile(ctx, database.UpsertShowFileParams{
		SeasonID:  season.ID,
		FilePath:  path,
		FileName:  filepath.Base(path),
		Size:      1_000_000,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Duration:  sql.NullFloat64{Float64: 7200, Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert second show file: %v", err)
	}

	episode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
		SeasonID:      season.ID,
		EpisodeNumber: 1,
		Name:          "Second Pilot",
	})
	if err != nil {
		t.Fatalf("upsert second show episode: %v", err)
	}

	err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{
		EpisodeID:    episode.ID,
		FileID:       file.ID,
		SeasonID:     season.ID,
		EpisodeOrder: 0,
	})
	if err != nil {
		t.Fatalf("link second show episode: %v", err)
	}

	return episode.ID
}
