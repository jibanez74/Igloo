package main

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"slices"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const watchCompletionThreshold = 0.98

type updateWatchProgressRequest struct {
	ProgressSec   *float64 `json:"progress_sec"`
	DurationSec   *float64 `json:"duration_sec"`
	SaveSessionID *string  `json:"save_session_id"`
	SaveSequence  *int64   `json:"save_sequence"`
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}

	hexValue := strings.ReplaceAll(value, "-", "")
	_, err := hex.DecodeString(hexValue)
	return err == nil
}

type setWatchedRequest struct {
	Watched *bool `json:"watched"`
}

type watchProgressResponse struct {
	ProgressSec *float64 `json:"progress_sec"`
	DurationSec *float64 `json:"duration_sec"`
	Watched     bool     `json:"watched"`
	UpdatedAt   *string  `json:"updated_at"`
}

// watchProgressRow is the stored progress shape shared by movies and episodes.
type watchProgressRow struct {
	ProgressSec float64
	DurationSec float64
	Watched     bool
	UpdatedAt   string
}

// watchProgressWrite is one progress save. SaveSessionID and SaveSequence let
// the store reject a stale write from an earlier save of the same session.
type watchProgressWrite struct {
	UserID        int64
	MediaID       int64
	ProgressSec   float64
	DurationSec   float64
	SaveSessionID string
	SaveSequence  int64
}

// watchProgressStore is what the shared handlers need from a progress table.
// Movies and episodes keep separate tables (each with a foreign key to its own
// catalog row) behind the same handler logic.
type watchProgressStore interface {
	exists(ctx context.Context, id int64) (bool, error)
	get(ctx context.Context, userID int64, id int64) (watchProgressRow, error)
	upsert(ctx context.Context, write watchProgressWrite) error
	remove(ctx context.Context, userID int64, id int64) error
	markWatched(ctx context.Context, userID int64, id int64) error
	markWatchedFromProgress(ctx context.Context, write watchProgressWrite) error
	markUnwatched(ctx context.Context, userID int64, id int64) error
}

// movieWatchProgressStore is the movie adapter over movie_watch_progress.
type movieWatchProgressStore struct {
	q *database.Queries
}

func (s movieWatchProgressStore) exists(ctx context.Context, id int64) (bool, error) {
	return s.q.MovieExists(ctx, id)
}

func (s movieWatchProgressStore) get(ctx context.Context, userID int64, id int64) (watchProgressRow, error) {
	row, err := s.q.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{UserID: userID, MovieID: id})
	if err != nil {
		return watchProgressRow{}, err
	}
	return watchProgressRow{ProgressSec: row.ProgressSec, DurationSec: row.DurationSec, Watched: row.Watched, UpdatedAt: row.UpdatedAt}, nil
}

func (s movieWatchProgressStore) upsert(ctx context.Context, write watchProgressWrite) error {
	return s.q.UpsertMovieWatchProgress(ctx, database.UpsertMovieWatchProgressParams{
		UserID:        write.UserID,
		MovieID:       write.MediaID,
		ProgressSec:   write.ProgressSec,
		DurationSec:   write.DurationSec,
		SaveSessionID: write.SaveSessionID,
		SaveSequence:  write.SaveSequence,
	})
}

func (s movieWatchProgressStore) remove(ctx context.Context, userID int64, id int64) error {
	return s.q.DeleteMovieWatchProgress(ctx, database.DeleteMovieWatchProgressParams{UserID: userID, MovieID: id})
}

func (s movieWatchProgressStore) markWatched(ctx context.Context, userID int64, id int64) error {
	return s.q.MarkMovieWatched(ctx, database.MarkMovieWatchedParams{UserID: userID, MovieID: id})
}

func (s movieWatchProgressStore) markWatchedFromProgress(ctx context.Context, write watchProgressWrite) error {
	return s.q.MarkMovieWatchedFromProgress(ctx, database.MarkMovieWatchedFromProgressParams{
		UserID:        write.UserID,
		MovieID:       write.MediaID,
		SaveSessionID: write.SaveSessionID,
		SaveSequence:  write.SaveSequence,
	})
}

func (s movieWatchProgressStore) markUnwatched(ctx context.Context, userID int64, id int64) error {
	return s.q.MarkMovieUnwatched(ctx, database.MarkMovieUnwatchedParams{UserID: userID, MovieID: id})
}

// watchProgressStoreFor picks the table for a media kind.
func (app *Application) watchProgressStoreFor(kind mediaKind) watchProgressStore {
	if kind == mediaKindEpisode {
		return showEpisodeWatchProgressStore{q: app.Queries}
	}
	return movieWatchProgressStore{q: app.Queries}
}

// watchedIDField names the media id in the watched response: "movie_id" or
// "episode_id".
func watchedIDField(kind mediaKind) string {
	return string(kind) + "_id"
}

func (app *Application) ensureMediaExists(r *http.Request, store watchProgressStore, media mediaRef) error {
	exists, err := store.exists(r.Context(), media.ID)
	if err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

// rejectWatchProgressWrite turns a failed watch-progress write into a
// response. The media foreign key already rejects an unknown row, so the write
// paths do not pre-check existence -- they pay one query on success and only
// the failure path probes existence to tell "no such media" (404) from a real
// error (500). Same shape, and the same reasoning, as the playlist add path in
// movie_playlist_handler.go. The read and delete paths keep their pre-check:
// neither can trip a foreign key, so nothing else would produce the documented
// 404.
func (app *Application) rejectWatchProgressWrite(w http.ResponseWriter, r *http.Request, store watchProgressStore, writeErr error, media mediaRef, logMessage, userMessage string) {
	exists, existsErr := store.exists(r.Context(), media.ID)
	if existsErr == nil && !exists {
		helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
		return
	}

	app.Logger.Error(logMessage, "error", writeErr, "media", media.String())
	helpers.ErrorJSON(w, errors.New(userMessage))
}

func emptyWatchProgressResponse() watchProgressResponse {
	return watchProgressResponse{
		ProgressSec: nil,
		DurationSec: nil,
		Watched:     false,
		UpdatedAt:   nil,
	}
}

func watchProgressToResponse(row watchProgressRow) watchProgressResponse {
	progressSec := row.ProgressSec
	durationSec := row.DurationSec
	updatedAt := row.UpdatedAt

	return watchProgressResponse{
		ProgressSec: &progressSec,
		DurationSec: &durationSec,
		Watched:     row.Watched,
		UpdatedAt:   &updatedAt,
	}
}

// GetMovieWatchProgress returns the caller's progress on one movie.
func (app *Application) GetMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	app.getWatchProgress(w, r, mediaKindMovie)
}

func (app *Application) getWatchProgress(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	media, err := parseMediaID(chi.URLParam(r, "id"), kind)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}
	store := app.watchProgressStoreFor(kind)

	err = app.ensureMediaExists(r, store, media)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to verify media exists for watch progress", "error", err, "media", media.String())
		helpers.ErrorJSON(w, errors.New("failed to fetch watch progress"))
		return
	}

	row, err := store.get(r.Context(), userID, media.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
				Error: false,
				Data:  emptyWatchProgressResponse(),
			})
			return
		}
		app.Logger.Error("failed to get watch progress", "error", err, "media", media.String(), "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch watch progress"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  watchProgressToResponse(row),
	})
}

// continueWatchingLimit caps the merged row, and is also the limit handed to
// each half's query, so the three caps cannot drift apart. It only bites twice:
// once per query, then again once movies and episodes are combined.
const continueWatchingLimit = 12

// continueWatchingItem is one card in the home row. Movies and episodes come
// from different queries with different columns, so this is the one read path
// here that shapes a response instead of returning sqlc rows verbatim. The
// episode fields are pointers so a movie omits them entirely, which is what the
// contract's discriminated oneOf expects.
type continueWatchingItem struct {
	Kind          string         `json:"kind"`
	ID            int64          `json:"id"`
	Title         string         `json:"title"`
	PosterPath    sql.NullString `json:"poster_path"`
	Year          sql.NullInt64  `json:"year"`
	ProgressSec   float64        `json:"progress_sec"`
	DurationSec   float64        `json:"duration_sec"`
	ShowID        *int64         `json:"show_id,omitempty"`
	SeasonNumber  *int64         `json:"season_number,omitempty"`
	EpisodeNumber *int64         `json:"episode_number,omitempty"`
	EpisodeName   *string        `json:"episode_name,omitempty"`

	// updatedAt orders the merge and is never serialized.
	updatedAt string
}

// GetContinueWatching lists what the current user has started and not finished,
// movies and episodes together, most recently watched first.
func (app *Application) GetContinueWatching(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movies, err := app.Queries.GetContinueWatchingMovies(r.Context(), database.GetContinueWatchingMoviesParams{
		UserID: userID,
		Limit:  continueWatchingLimit,
	})
	if err != nil {
		app.Logger.Error("failed to get continue watching movies", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch continue watching"))
		return
	}

	episodes, err := app.Queries.GetContinueWatchingEpisodes(r.Context(), database.GetContinueWatchingEpisodesParams{
		UserID: userID,
		Limit:  continueWatchingLimit,
	})
	if err != nil {
		app.Logger.Error("failed to get continue watching episodes", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch continue watching"))
		return
	}

	items := make([]continueWatchingItem, 0, len(movies)+len(episodes))

	for _, movie := range movies {
		items = append(items, continueWatchingItem{
			Kind:        string(mediaKindMovie),
			ID:          movie.ID,
			Title:       movie.Title,
			PosterPath:  movie.PosterPath,
			Year:        movie.Year,
			ProgressSec: movie.ProgressSec,
			DurationSec: movie.DurationSec,
			updatedAt:   movie.UpdatedAt,
		})
	}

	for _, episode := range episodes {
		items = append(items, continueWatchingItem{
			Kind:          string(mediaKindEpisode),
			ID:            episode.ID,
			Title:         episode.ShowName,
			PosterPath:    episode.ShowPosterPath,
			Year:          episode.ShowPremiereYear,
			ProgressSec:   episode.ProgressSec,
			DurationSec:   episode.DurationSec,
			ShowID:        &episode.ShowID,
			SeasonNumber:  &episode.SeasonNumber,
			EpisodeNumber: &episode.EpisodeNumber,
			EpisodeName:   &episode.Name,
			updatedAt:     episode.UpdatedAt,
		})
	}

	// updated_at is "YYYY-MM-DD HH:MM:SS" text, so comparing the strings
	// compares the instants. CURRENT_TIMESTAMP only has second resolution and
	// the two halves arrive from separate queries, so ties are broken on kind
	// then id: without that the order of two rows saved in the same second
	// would depend on which slice happened to be appended first, and the row
	// could reshuffle between refetches.
	slices.SortStableFunc(items, func(a, b continueWatchingItem) int {
		if order := cmp.Compare(b.updatedAt, a.updatedAt); order != 0 {
			return order
		}
		if order := cmp.Compare(a.Kind, b.Kind); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})

	if len(items) > continueWatchingLimit {
		items = items[:continueWatchingLimit]
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"items": items,
		},
	})
}

// UpdateMovieWatchProgress records a playback position for one movie.
func (app *Application) UpdateMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	app.updateWatchProgress(w, r, mediaKindMovie)
}

func (app *Application) updateWatchProgress(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	media, err := parseMediaID(chi.URLParam(r, "id"), kind)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}
	store := app.watchProgressStoreFor(kind)

	var req updateWatchProgressRequest
	err = helpers.ReadJSON(w, r, &req)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.ProgressSec == nil {
		helpers.ErrorJSON(w, errors.New("progress_sec is required"), http.StatusBadRequest)
		return
	}
	if req.DurationSec == nil {
		helpers.ErrorJSON(w, errors.New("duration_sec is required"), http.StatusBadRequest)
		return
	}
	if req.SaveSessionID == nil {
		helpers.ErrorJSON(w, errors.New("save_session_id is required"), http.StatusBadRequest)
		return
	}
	if !validUUID(*req.SaveSessionID) {
		helpers.ErrorJSON(w, errors.New("save_session_id must be a valid UUID"), http.StatusBadRequest)
		return
	}
	if req.SaveSequence == nil {
		helpers.ErrorJSON(w, errors.New("save_sequence is required"), http.StatusBadRequest)
		return
	}
	if *req.SaveSequence <= 0 {
		helpers.ErrorJSON(w, errors.New("save_sequence must be greater than 0"), http.StatusBadRequest)
		return
	}

	progressVal := *req.ProgressSec
	durationVal := *req.DurationSec

	if math.IsNaN(progressVal) || math.IsNaN(durationVal) || math.IsInf(progressVal, 0) || math.IsInf(durationVal, 0) {
		helpers.ErrorJSON(w, errors.New("progress and duration must be finite numbers"), http.StatusBadRequest)
		return
	}
	if durationVal <= 0 {
		helpers.ErrorJSON(w, errors.New("duration_sec must be greater than 0"), http.StatusBadRequest)
		return
	}

	write := watchProgressWrite{
		UserID:        userID,
		MediaID:       media.ID,
		ProgressSec:   helpers.ClampFloat64(progressVal, 0, durationVal),
		DurationSec:   durationVal,
		SaveSessionID: *req.SaveSessionID,
		SaveSequence:  *req.SaveSequence,
	}

	if write.ProgressSec/write.DurationSec >= watchCompletionThreshold {
		err = store.markWatchedFromProgress(r.Context(), write)
		if err != nil {
			app.rejectWatchProgressWrite(w, r, store, err, media,
				"failed to mark media watched from progress update", "failed to update watch progress")
			return
		}

		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: map[string]any{
				"watched": true,
			},
		})
		return
	}

	err = store.upsert(r.Context(), write)
	if err != nil {
		app.rejectWatchProgressWrite(w, r, store, err, media,
			"failed to upsert watch progress", "failed to update watch progress")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"watched": false,
		},
	})
}

// DeleteMovieWatchProgress clears the caller's progress on one movie.
func (app *Application) DeleteMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	app.deleteWatchProgress(w, r, mediaKindMovie)
}

func (app *Application) deleteWatchProgress(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	media, err := parseMediaID(chi.URLParam(r, "id"), kind)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}
	store := app.watchProgressStoreFor(kind)

	err = app.ensureMediaExists(r, store, media)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to verify media exists for watch progress delete", "error", err, "media", media.String())
		helpers.ErrorJSON(w, errors.New("failed to clear watch progress"))
		return
	}

	err = store.remove(r.Context(), userID, media.ID)
	if err != nil {
		app.Logger.Error("failed to delete watch progress", "error", err, "media", media.String(), "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to clear watch progress"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"cleared": true,
		},
	})
}

// SetMovieWatched marks one movie watched or unwatched for the caller.
func (app *Application) SetMovieWatched(w http.ResponseWriter, r *http.Request) {
	app.setWatched(w, r, mediaKindMovie)
}

func (app *Application) setWatched(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	media, err := parseMediaID(chi.URLParam(r, "id"), kind)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}
	store := app.watchProgressStoreFor(kind)

	var req setWatchedRequest
	err = helpers.ReadJSON(w, r, &req)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.Watched == nil {
		helpers.ErrorJSON(w, errors.New("watched is required"), http.StatusBadRequest)
		return
	}

	watched := *req.Watched

	if watched {
		err = store.markWatched(r.Context(), userID, media.ID)
		if err != nil {
			app.rejectWatchProgressWrite(w, r, store, err, media,
				"failed to mark media watched", "failed to update watched status")
			return
		}
	} else {
		err = store.markUnwatched(r.Context(), userID, media.ID)
		if err != nil {
			app.rejectWatchProgressWrite(w, r, store, err, media,
				"failed to mark media unwatched", "failed to update watched status")
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			watchedIDField(kind): media.ID,
			"watched":            watched,
		},
	})
}
