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
	userID := app.userIDFromRequest(r)

	var req RecordPlayEventRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.TrackID <= 0 {
		helpers.ErrorJSON(w, errors.New("track_id is required"), http.StatusBadRequest)
		return
	}
	if req.DurationPlayed < 0 {
		helpers.ErrorJSON(w, errors.New("duration_played must be nonnegative"), http.StatusBadRequest)
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
			helpers.ErrorJSON(w, errors.New(trackNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "failed to record play event", "failed to record play event",
			"track_id", req.TrackID, "user_id", userID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"recorded": true}})
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
	userID := app.userIDFromRequest(r)

	stats, err := app.Queries.GetUserListeningStats(r.Context(), userID)
	if err != nil {
		app.serverError(w, err, "failed to get listening stats", "failed to fetch listening stats")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"total_plays":          stats.TotalPlays,
			"total_time_listened":  stats.TotalTimeListened,
			"unique_tracks_played": stats.UniqueTracksPlayed,
			"liked_tracks_count":   stats.LikedTracksCount,
		},
	})
}

func (app *Application) GetUserTopTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	limit, offset := parseLimitOffsetParams(r, 20, 100)

	tracks, err := app.Queries.GetUserTopTracks(r.Context(), database.GetUserTopTracksParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.serverError(w, err, "failed to get top tracks", "failed to fetch top tracks")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"tracks": tracks,
			"limit":  limit,
			"offset": offset,
		},
	})
}

func (app *Application) GetUserTopMusicians(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	limit, offset := parseLimitOffsetParams(r, 10, 50)

	musicians, err := app.Queries.GetUserTopMusicians(r.Context(), database.GetUserTopMusiciansParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.serverError(w, err, "failed to get top musicians", "failed to fetch top musicians")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"musicians": musicians,
			"limit":     limit,
			"offset":    offset,
		},
	})
}

func (app *Application) GetUserTopGenres(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	limit := min(positiveQueryInt(r, "limit", 10), 20)

	genres, err := app.Queries.GetUserTopGenres(r.Context(), database.GetUserTopGenresParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		app.serverError(w, err, "failed to get top genres", "failed to fetch top genres")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"genres": genres,
			"limit":  limit,
		},
	})
}

func (app *Application) GetUserTopAlbums(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	limit, offset := parseLimitOffsetParams(r, 10, 50)

	albums, err := app.Queries.GetUserTopAlbums(r.Context(), database.GetUserTopAlbumsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.serverError(w, err, "failed to get top albums", "failed to fetch top albums")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"albums": albums,
			"limit":  limit,
			"offset": offset,
		},
	})
}

func (app *Application) GetUserRecentlyPlayed(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	limit, offset := parseLimitOffsetParams(r, 20, 50)

	tracks, err := app.Queries.GetUserRecentlyPlayed(r.Context(), database.GetUserRecentlyPlayedParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.serverError(w, err, "failed to get recently played", "failed to fetch recently played")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"tracks": tracks,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// parseLimitOffsetParams reads limit and offset. A missing, non-integer or
// nonpositive limit uses defaultLimit and a larger one is clamped to maxLimit;
// an invalid offset is 0.
func parseLimitOffsetParams(r *http.Request, defaultLimit, maxLimit int64) (int64, int64) {
	limit := min(positiveQueryInt(r, "limit", defaultLimit), maxLimit)

	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		offset = 0
	}
	return limit, offset
}
