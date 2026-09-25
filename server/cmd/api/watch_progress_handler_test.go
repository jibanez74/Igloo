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
)

const testWatchProgressSaveSessionID = "11111111-1111-4111-8111-111111111111"

func TestWatchProgressHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	userID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, userID)

	// The empty and populated shapes differ (nulls versus values), so both are
	// validated against the contract.
	progressPath := fmt.Sprintf("/api/movies/%d/watch-progress", movieID)
	empty := serveOpenAPIExchange(t, handler, "getMovieWatchProgress", httptest.NewRequest(http.MethodGet, progressPath, nil), http.StatusOK)
	if !strings.Contains(empty.Body.String(), `"progress_sec":null`) {
		t.Fatalf("fresh progress = %s, want nulls", empty.Body.String())
	}
	updateBody := `{"progress_sec":120,"duration_sec":7200,"save_session_id":"11111111-1111-4111-8111-111111111111","save_sequence":1}`
	serveOpenAPIExchange(t, handler, "updateMovieWatchProgress", newOpenAPIJSONRequest(http.MethodPut, progressPath, updateBody), http.StatusOK)
	populated := serveOpenAPIExchange(t, handler, "getMovieWatchProgress", httptest.NewRequest(http.MethodGet, progressPath, nil), http.StatusOK)
	if !strings.Contains(populated.Body.String(), `"progress_sec":120`) || !strings.Contains(populated.Body.String(), `"duration_sec":7200`) {
		t.Fatalf("saved progress = %s, want 120/7200", populated.Body.String())
	}
	watchedPath := fmt.Sprintf("/api/movies/%d/watch-progress/watched", movieID)
	serveOpenAPIExchange(t, handler, "setMovieWatched", newOpenAPIJSONRequest(http.MethodPut, watchedPath, `{"watched":false}`), http.StatusOK)
	serveOpenAPIExchange(t, handler, "deleteMovieWatchProgress", httptest.NewRequest(http.MethodDelete, progressPath, nil), http.StatusOK)
}

func TestWatchProgress_UpsertResetsWatchedFlag(t *testing.T) {
	app := setupTestApp(t)
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

func TestWatchProgress_MarkUnwatchedIdempotentCreatesRow(t *testing.T) {
	app := setupTestApp(t)
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
	ctx := context.Background()

	user1 := createTestUser(t, app, "User One", "one@example.com", false)

	user2 := createTestUser(t, app, "User Two", "two@example.com", false)

	movieID := createTestMovie(t, app, "Shared Movie", "/movies/shared.mkv")

	err := app.Queries.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        user1.ID,
		MovieID:       movieID,
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
		MovieID:       movieID,
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
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("get for user1 failed: %v", err)
	}

	row2, err := app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{
		UserID:  user2.ID,
		MovieID: movieID,
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
	ctx := context.Background()

	user := createTestUser(t, app, "Watcher", "watcher@example.com", false)

	otherUser := createTestUser(t, app, "Other", "other@example.com", false)

	createMovie := func(title, fileName string) int64 {
		return createTestMovie(t, app, title, "/movies/"+fileName)
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

	err := app.Queries.MarkMovieWatched(ctx, database.MarkMovieWatchedParams{
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

	rows, err := app.Queries.GetContinueWatchingMovies(ctx, database.GetContinueWatchingMoviesParams{UserID: user.ID, Limit: continueWatchingLimit})
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

	otherRows, err := app.Queries.GetContinueWatchingMovies(ctx, database.GetContinueWatchingMoviesParams{UserID: otherUser.ID, Limit: continueWatchingLimit})
	if err != nil {
		t.Fatalf("GetContinueWatchingMovies for other user failed: %v", err)
	}
	if len(otherRows) != 1 || otherRows[0].ID != otherUserID {
		t.Errorf("expected other user to only see their own in-progress movie, got %+v", otherRows)
	}
}

// TestUpdateMovieWatchProgress_HTTPCompletionThreshold drives the real handler
// so the 95% auto-watched rule and the progress clamp are asserted end to end:
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
		{name: "just below the threshold is not watched", progressSec: 6839, durationSec: 7200, wantProgress: 6839},
		{name: "negative progress clamps to zero", progressSec: -50, durationSec: 7200, wantProgress: 0},
		{name: "exactly at the threshold is watched", progressSec: 6840, durationSec: 7200, wantWatched: true},
		{name: "above the duration is watched", progressSec: 8000, durationSec: 7200, wantWatched: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupSessionTestApp(t)

			userID, movieID := createTestUserAndMovie(t, app)

			handler := authenticatedRouter(t, app, userID)

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

	userID, movieID := createTestUserAndMovie(t, app)

	handler := authenticatedRouter(t, app, userID)

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

func TestSetMovieWatched_HTTPTogglesWatchedAndRequiresField(t *testing.T) {
	app := setupSessionTestApp(t)

	userID, movieID := createTestUserAndMovie(t, app)
	seedWatchProgress(t, app, userID, movieID)
	handler := authenticatedRouter(t, app, userID)
	target := fmt.Sprintf("/api/movies/%d/watch-progress/watched", movieID)

	type watchedResponse struct {
		Error bool `json:"error"`
		Data  struct {
			MovieID int64 `json:"movie_id"`
			Watched bool  `json:"watched"`
		} `json:"data"`
	}
	setWatched := func(t *testing.T, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	expectPersisted := func(t *testing.T, w *httptest.ResponseRecorder, watched bool) database.GetMovieWatchProgressRow {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
		var resp watchedResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err != nil {
			t.Fatalf("unmarshal watched response: %v", err)
		}
		if resp.Error || resp.Data.MovieID != movieID || resp.Data.Watched != watched {
			t.Fatalf("response = %s, want movie %d watched=%t", w.Body.String(), movieID, watched)
		}
		row, err := app.Queries.GetMovieWatchProgress(context.Background(), database.GetMovieWatchProgressParams{
			UserID:  userID,
			MovieID: movieID,
		})
		if err != nil {
			t.Fatalf("GetMovieWatchProgress: %v", err)
		}
		if row.Watched != watched {
			t.Fatalf("persisted watched = %t, want %t", row.Watched, watched)
		}
		return row
	}

	missing := setWatched(t, `{}`)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d, body=%s", missing.Code, http.StatusBadRequest, missing.Body.String())
	}

	// Marking a movie watched clears the resume position it had.
	watched := expectPersisted(t, setWatched(t, `{"watched": true}`), true)
	if watched.ProgressSec != 0 {
		t.Fatalf("progress_sec after watched=true = %v, want 0", watched.ProgressSec)
	}

	expectPersisted(t, setWatched(t, `{"watched": false}`), false)
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

	user := createTestUser(t, app, "Watcher", "continue-watching@example.com", false)
	movieID := createTestMovie(t, app, "Contract Movie", "/movies/contract-continue.mkv")
	seedWatchProgress(t, app, user.ID, movieID)

	fixture := seedPlaybackEpisode(t, app)
	seedEpisodeWatchProgress(t, app, user.ID, fixture.Episode1, 900.0, 1)
	backdateProgress(t, app, "movie_watch_progress", "movie_id", user.ID, movieID, "-1 hour")

	// Another account watching the same movie and the same episode. Both halves
	// of the merged row are per-user, and the handler runs them off the session
	// user, so neither may leak into this response.
	otherUser := createTestUser(t, app, "Other", "continue-watching-other@example.com", false)
	otherMovieID := createTestMovie(t, app, "Other Movie", "/movies/other-continue.mkv")
	seedWatchProgress(t, app, otherUser.ID, otherMovieID)
	seedWatchProgress(t, app, otherUser.ID, movieID)
	seedEpisodeWatchProgress(t, app, otherUser.ID, fixture.Episode2, 900.0, 1)

	request := httptest.NewRequest(http.MethodGet, "/api/continue-watching", nil)
	response := serveOpenAPIExchange(t, authenticatedRouter(t, app, user.ID), "getContinueWatching", request, http.StatusOK)
	assertResponseListNotEmpty(t, "getContinueWatching", response.Body.Bytes(), "items")

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
		t.Fatalf("expected only this user's movie and episode, got %d items: %s", len(items), response.Body.String())
	}
	for _, item := range items {
		if item.Kind == "movie" && item.ID == otherMovieID {
			t.Fatalf("another user's movie reached the row: %+v", item)
		}
		if item.Kind == "episode" && item.ID == fixture.Episode2 {
			t.Fatalf("another user's episode reached the row: %+v", item)
		}
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

	rows, err := app.Queries.GetContinueWatchingEpisodes(ctx, database.GetContinueWatchingEpisodesParams{UserID: user.ID, Limit: continueWatchingLimit})
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

	rows, err = app.Queries.GetContinueWatchingEpisodes(ctx, database.GetContinueWatchingEpisodesParams{UserID: user.ID, Limit: continueWatchingLimit})
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
