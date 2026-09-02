package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

type RecordPlayEventRequest struct {
	TrackID        int64 `json:"track_id"`
	DurationPlayed int64 `json:"duration_played"`
	Completed      bool  `json:"completed"`
}

// The frontend records a play after its playback threshold is met.
func (app *Application) RecordPlayEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	var req RecordPlayEventRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.TrackID == 0 {
		helpers.ErrorJSON(w, errors.New("track_id is required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// The play-event log and the aggregate stats are one fact recorded twice;
	// they used to be two untransacted writes, so a failure in between left
	// them disagreeing. The track_id foreign key replaces the TrackExists
	// pre-check that ran first: an unknown track fails the insert, and only
	// then do we probe to tell 404 from 500.
	err := app.recordTrackPlay(ctx, userID, req)
	if err != nil {
		trackOK, existsErr := app.Queries.TrackExists(ctx, req.TrackID)
		if existsErr == nil && !trackOK {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to record play event", "error", err, "track_id", req.TrackID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to record play event"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"recorded": true},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// recordTrackPlay writes the play event and the rolled-up per-track stats in
// one transaction so the two cannot diverge.
func (app *Application) recordTrackPlay(ctx context.Context, userID int64, req RecordPlayEventRequest) error {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)

	err = qtx.RecordPlayEvent(ctx, database.RecordPlayEventParams{
		UserID:         userID,
		TrackID:        req.TrackID,
		DurationPlayed: req.DurationPlayed,
		Completed:      req.Completed,
	})
	if err != nil {
		return err
	}

	err = qtx.UpsertUserTrackStats(ctx, database.UpsertUserTrackStatsParams{
		UserID:          userID,
		TrackID:         req.TrackID,
		TotalTimePlayed: req.DurationPlayed,
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (app *Application) GetUserListeningStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	stats, err := app.Queries.GetUserListeningStats(r.Context(), userID)
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

func (app *Application) GetUserTopTracks(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

func (app *Application) GetUserTopMusicians(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

func (app *Application) GetUserTopGenres(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

func (app *Application) GetUserTopAlbums(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

func (app *Application) GetUserRecentlyPlayed(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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
