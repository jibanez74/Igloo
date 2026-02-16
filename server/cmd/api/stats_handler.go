package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// RecordPlayEventRequest represents the request body for recording a play event.
type RecordPlayEventRequest struct {
	TrackID        int64 `json:"track_id"`
	DurationPlayed int64 `json:"duration_played"`
	Completed      bool  `json:"completed"`
}

// RecordPlayEvent records when a user plays a track.
// Called by the frontend after playback threshold is met (30s or 80% completion).
func (app *Application) RecordPlayEvent(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req RecordPlayEventRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.TrackID == 0 {
		helpers.ErrorJSON(w, errors.New("track_id is required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Verify track exists
	_, err := app.Queries.GetTrack(ctx, req.TrackID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get track for play event", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to record play event"))
		return
	}

	// Record the play event
	err = app.Queries.RecordPlayEvent(ctx, database.RecordPlayEventParams{
		UserID:         userID,
		TrackID:        req.TrackID,
		DurationPlayed: req.DurationPlayed,
		Completed:      req.Completed,
	})
	if err != nil {
		app.Logger.Error("failed to record play event", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to record play event"))
		return
	}

	// Update aggregated stats
	err = app.Queries.UpsertUserTrackStats(ctx, database.UpsertUserTrackStatsParams{
		UserID:          userID,
		TrackID:         req.TrackID,
		TotalTimePlayed: req.DurationPlayed,
	})
	if err != nil {
		app.Logger.Error("failed to update track stats", "error", err)
		// Don't fail the request, the play was still recorded
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"recorded": true},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetUserListeningStats returns overall listening statistics for the authenticated user.
func (app *Application) GetUserListeningStats(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	stats, err := app.Queries.GetUserListeningStats(r.Context(), database.GetUserListeningStatsParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		app.Logger.Error("failed to get listening stats", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch listening stats"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"total_plays":          stats.TotalPlays,
			"total_time_listened":  stats.TotalTimeListened,
			"unique_tracks_played": stats.UniqueTracksPlayed,
			"liked_tracks_count":   stats.LikedTracksCount,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetUserTopTracks returns the user's most played tracks.
func (app *Application) GetUserTopTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	limit, offset := parseStatsPaginationParams(r, 20, 100)

	tracks, err := app.Queries.GetUserTopTracks(r.Context(), database.GetUserTopTracksParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get top tracks", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch top tracks"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks": tracks,
			"limit":  limit,
			"offset": offset,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetUserTopMusicians returns the user's most listened musicians.
func (app *Application) GetUserTopMusicians(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	limit, offset := parseStatsPaginationParams(r, 10, 50)

	musicians, err := app.Queries.GetUserTopMusicians(r.Context(), database.GetUserTopMusiciansParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get top musicians", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch top musicians"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"musicians": musicians,
			"limit":     limit,
			"offset":    offset,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetUserTopGenres returns the user's most listened genres.
func (app *Application) GetUserTopGenres(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	limit := int64(10)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 20 {
		limit = 20
	}

	genres, err := app.Queries.GetUserTopGenres(r.Context(), database.GetUserTopGenresParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		app.Logger.Error("failed to get top genres", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch top genres"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"genres": genres,
			"limit":  limit,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetUserTopAlbums returns the user's most listened albums.
func (app *Application) GetUserTopAlbums(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	limit, offset := parseStatsPaginationParams(r, 10, 50)

	albums, err := app.Queries.GetUserTopAlbums(r.Context(), database.GetUserTopAlbumsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get top albums", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch top albums"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"albums": albums,
			"limit":  limit,
			"offset": offset,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetUserRecentlyPlayed returns recently played tracks for the user.
func (app *Application) GetUserRecentlyPlayed(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	limit, offset := parseStatsPaginationParams(r, 20, 50)

	tracks, err := app.Queries.GetUserRecentlyPlayed(r.Context(), database.GetUserRecentlyPlayedParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get recently played", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch recently played"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks": tracks,
			"limit":  limit,
			"offset": offset,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// parseStatsPaginationParams extracts limit and offset from query parameters.
func parseStatsPaginationParams(r *http.Request, defaultLimit, maxLimit int64) (int64, int64) {
	limit := defaultLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := int64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.ParseInt(o, 10, 64); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}
