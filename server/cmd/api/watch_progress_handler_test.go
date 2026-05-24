package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

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

	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Test Movie",
		FilePath:  "/movies/test.mkv",
		FileName:  "test.mkv",
		Size:      1024,
		Container: "mkv",
		MimeType:  "video/x-matroska",
	})
	if err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}

	return user.ID, movie.ID
}

func TestWatchProgress_UpsertAndGet(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 300.5,
		DurationSec: 7200.0,
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

func TestWatchProgress_UpsertUpdatesExisting(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 100.0,
		DurationSec: 7200.0,
	})
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 500.0,
		DurationSec: 7200.0,
	})
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	row, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("GetMovieWatchProgress failed: %v", err)
	}

	if row.ProgressSec != 500.0 {
		t.Errorf("expected progress_sec 500.0 after second upsert, got %f", row.ProgressSec)
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
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 60.0,
		DurationSec: 7200.0,
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

func TestWatchProgress_GetNoRowsReturnsError(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	_, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err == nil {
		t.Fatal("expected sql.ErrNoRows, got nil")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestWatchProgress_Delete(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 300.0,
		DurationSec: 7200.0,
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

func TestWatchProgress_DeleteNonExistentIsNoOp(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.DeleteMovieWatchProgress(ctx, database.DeleteMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("DeleteMovieWatchProgress on non-existent row should not error, got: %v", err)
	}
}

func TestWatchProgress_MarkWatched(t *testing.T) {
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
		t.Error("expected watched=true")
	}
	if row.ProgressSec != 0 {
		t.Errorf("expected progress_sec=0 after MarkMovieWatched, got %f", row.ProgressSec)
	}
}

func TestWatchProgress_MarkWatchedClearsExistingProgress(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 3600.0,
		DurationSec: 7200.0,
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

	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
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

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      user1.ID,
		MovieID:     movie.ID,
		ProgressSec: 600.0,
		DurationSec: 7200.0,
	})
	if err != nil {
		t.Fatalf("upsert for user1 failed: %v", err)
	}

	err = app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      user2.ID,
		MovieID:     movie.ID,
		ProgressSec: 1800.0,
		DurationSec: 7200.0,
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

func TestWatchProgress_CascadeDeleteMovie(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	userID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 300.0,
		DurationSec: 7200.0,
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
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: 300.0,
		DurationSec: 7200.0,
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

func TestWatchProgress_CompletionThreshold(t *testing.T) {
	tests := []struct {
		name        string
		progressSec float64
		durationSec float64
		wantWatched bool
	}{
		{"50% - not watched", 3600, 7200, false},
		{"97% - not watched", 6984, 7200, false},
		{"98% - watched", 7056, 7200, true},
		{"99% - watched", 7128, 7200, true},
		{"100% - watched", 7200, 7200, true},
		{"above duration clamped to 100% - watched", 8000, 7200, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clamped := helpers.ClampFloat64(tt.progressSec, 0, tt.durationSec)
			ratio := clamped / tt.durationSec
			isWatched := ratio >= helpers.WATCH_COMPLETION_THRESHOLD
			if isWatched != tt.wantWatched {
				t.Errorf("progress=%.0f duration=%.0f ratio=%.4f: got watched=%v, want %v",
					tt.progressSec, tt.durationSec, ratio, isWatched, tt.wantWatched)
			}
		})
	}
}

func TestUpdateMovieWatchProgressRequest_JSONOmission(t *testing.T) {
	tests := []struct {
		name            string
		json            string
		wantProgressNil bool
		wantDurationNil bool
	}{
		{"empty object", `{}`, true, true},
		{"only progress", `{"progress_sec": 10}`, false, true},
		{"only duration", `{"duration_sec": 100}`, true, false},
		{"both present", `{"progress_sec": 10, "duration_sec": 100}`, false, false},
		{"null progress", `{"progress_sec": null, "duration_sec": 100}`, true, false},
		{"null duration", `{"progress_sec": 10, "duration_sec": null}`, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req updateMovieWatchProgressRequest
			err := json.Unmarshal([]byte(tt.json), &req)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if (req.ProgressSec == nil) != tt.wantProgressNil {
				t.Errorf("ProgressSec nil=%v, want nil=%v", req.ProgressSec == nil, tt.wantProgressNil)
			}
			if (req.DurationSec == nil) != tt.wantDurationNil {
				t.Errorf("DurationSec nil=%v, want nil=%v", req.DurationSec == nil, tt.wantDurationNil)
			}
		})
	}
}

func TestSetMovieWatchedRequest_JSONOmission(t *testing.T) {
	var req setMovieWatchedRequest
	err := json.Unmarshal([]byte(`{}`), &req)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Watched != nil {
		t.Errorf("expected Watched nil, got %v", req.Watched)
	}

	err = json.Unmarshal([]byte(`{"watched": true}`), &req)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Watched == nil || !*req.Watched {
		t.Errorf("expected Watched true, got %v", req.Watched)
	}

	err = json.Unmarshal([]byte(`{"watched": null}`), &req)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Watched != nil {
		t.Errorf("expected Watched nil for JSON null, got %v", req.Watched)
	}
}

func setupWatchProgressHTTPTestApp(t *testing.T) *Application {
	t.Helper()
	app := setupTestApp(t)
	app.InitSession()
	return app
}

func TestUpdateMovieWatchProgress_HTTPMissingFields(t *testing.T) {
	app := setupWatchProgressHTTPTestApp(t)
	defer app.DB.Close()

	userID, movieID := createTestUserAndMovie(t, app)

	r := chi.NewRouter()
	r.Put("/api/movies/{id}/watch-progress", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
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
	t.Run("valid body", func(t *testing.T) {
		run(t, `{"progress_sec": 100, "duration_sec": 7200}`, http.StatusOK)
	})
}

func TestSetMovieWatched_HTTPMissingWatched(t *testing.T) {
	app := setupWatchProgressHTTPTestApp(t)
	defer app.DB.Close()

	userID, movieID := createTestUserAndMovie(t, app)

	r := chi.NewRouter()
	r.Put("/api/movies/{id}/watch-progress/watched", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
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
	var req updateMovieWatchProgressRequest
	body := strings.NewReader(`{"progress_sec": 1, "duration_sec": 2, "extra": true}`)
	r := httptest.NewRequest(http.MethodPut, "/", body)
	w := httptest.NewRecorder()
	err := helpers.ReadJSON(w, r, &req, 1024)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestReadJSON_SetMovieWatchedRequest_UsesReadJSON(t *testing.T) {
	var req setMovieWatchedRequest
	body := strings.NewReader(`{"watched": false}`)
	r := httptest.NewRequest(http.MethodPut, "/", body)
	w := httptest.NewRecorder()
	err := helpers.ReadJSON(w, r, &req, 1024)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if req.Watched == nil || *req.Watched {
		t.Errorf("expected watched false, got %v", req.Watched)
	}
}
