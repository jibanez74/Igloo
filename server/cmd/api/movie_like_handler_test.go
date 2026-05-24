package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func TestToggleLikeMovie_HTTPPersistsLikeAndUnlike(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	userID, movieID := createTestUserAndMovie(t, app)

	r := chi.NewRouter()
	r.Post("/api/movies/{id}/like", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.ToggleLikeMovie(w, r)
	})
	handler := app.SessionManager.LoadAndSave(r)

	postLike := func(t *testing.T) bool {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/movies/%d/like", movieID), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp struct {
			Error bool `json:"error"`
			Data  struct {
				MovieID int64 `json:"movie_id"`
				IsLiked bool  `json:"is_liked"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.Error {
			t.Fatalf("expected success response, got %s", w.Body.String())
		}
		if resp.Data.MovieID != movieID {
			t.Fatalf("movie_id=%d, want %d", resp.Data.MovieID, movieID)
		}

		return resp.Data.IsLiked
	}

	if isLiked := postLike(t); !isLiked {
		t.Fatal("first like toggle returned is_liked=false, want true")
	}
	persistedLike, err := app.Queries.IsMovieLiked(context.Background(), database.IsMovieLikedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("IsMovieLiked after like: %v", err)
	}
	if !persistedLike {
		t.Fatal("expected liked row after first toggle")
	}

	if isLiked := postLike(t); isLiked {
		t.Fatal("second like toggle returned is_liked=true, want false")
	}
	persistedLike, err = app.Queries.IsMovieLiked(context.Background(), database.IsMovieLikedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("IsMovieLiked after unlike: %v", err)
	}
	if persistedLike {
		t.Fatal("expected liked row to be removed after second toggle")
	}
}
