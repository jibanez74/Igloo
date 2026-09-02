package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func (app *Application) ToggleLikeTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	trackID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid track id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// user_liked_tracks.track_id rejects an unknown track, so the happy path is
	// the toggle transaction alone; only a failure pays the TrackExists probe
	// that tells "no such track" (404) from a real error (500).
	isLiked, err := app.toggleTrackLike(ctx, userID, trackID)
	if err != nil {
		trackOK, existsErr := app.Queries.TrackExists(ctx, trackID)
		if existsErr == nil && !trackOK {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to toggle track like", "error", err, "trackID", trackID, "userID", userID)
		helpers.ErrorJSON(w, errors.New("failed to update like"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"track_id": trackID,
			"is_liked": isLiked,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// toggleTrackLike flips like state in a single DB transaction, mirroring
// toggleMovieLike: DELETE first, and only when nothing was removed INSERT
// (LikeTrack is idempotent via ON CONFLICT DO NOTHING). No read-then-write
// race.
func (app *Application) toggleTrackLike(ctx context.Context, userID, trackID int64) (isLiked bool, err error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)

	removed, err := qtx.UnlikeTrack(ctx, database.UnlikeTrackParams{
		UserID:  userID,
		TrackID: trackID,
	})
	if err != nil {
		return false, err
	}
	if removed > 0 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	if err := qtx.LikeTrack(ctx, database.LikeTrackParams{
		UserID:  userID,
		TrackID: trackID,
	}); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// GetLikedTrackIDsForUser returns the IDs of every track the authenticated user has liked.
// Used by the frontend to initialize liked-heart state on page load without a paginated fetch.
func (app *Application) GetLikedTrackIDsForUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	trackIDs, err := app.Queries.GetLikedTrackIDsByUserID(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get liked track IDs", "error", err, "userID", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch liked tracks"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"liked_track_ids": trackIDs,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetLikedTracks returns the current user's liked tracks as a paginated list,
// ordered by most recently liked first. Query params: page (default 1), per_page (default 50, max 100).
func (app *Application) GetLikedTracks(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	page := int64(1)
	if p := r.URL.Query().Get("page"); p != "" {
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}

	perPage := int64(50)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		parsed, err := strconv.ParseInt(pp, 10, 64)
		if err == nil && parsed > 0 {
			perPage = parsed
		}
	}
	if perPage > 100 {
		perPage = 100
	}

	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.CountUserLikedTracks(ctx, userID)
	if err != nil {
		app.Logger.Error("failed to count liked tracks", "error", err, "userID", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch liked tracks count"))
		return
	}

	tracks, err := app.Queries.GetLikedTracksForUser(ctx, database.GetLikedTracksForUserParams{
		UserID: userID,
		Limit:  perPage,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get liked tracks", "error", err, "userID", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch liked tracks"))
		return
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks":      tracks,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
			"has_more":    page < totalPages,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetTrackByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid track id"), http.StatusBadRequest)
		return
	}

	track, err := app.Queries.GetTrack(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get track", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch track from server"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"track": track,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// StreamTrack streams the audio file for playback.
// Uses http.ServeContent which handles:
//   - Range requests (for seeking/scrubbing)
//   - If-Modified-Since / If-None-Match headers (caching, via the ETag set below)
//   - Content-Type and Content-Length headers
func (app *Application) StreamTrack(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid track id"), http.StatusBadRequest)
		return
	}

	track, err := app.trackStreamFile(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get track for streaming", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch track from server"))
		return
	}

	err = serveMediaFile(w, r, track.Path, track.Name, track.ContentType)
	if err != nil {
		app.Logger.Error("failed to stream track file", "error", err, "path", track.Path, "id", id)
	}
}

func (app *Application) GetTracksAlphabetical(w http.ResponseWriter, r *http.Request) {
	limit := int64(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.ParseInt(l, 10, 64)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := int64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		parsed, err := strconv.ParseInt(o, 10, 64)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	total, err := app.Queries.GetTracksCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get tracks count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch tracks count"))
		return
	}

	tracks, err := app.Queries.GetTracksAlphabetical(r.Context(), database.GetTracksAlphabeticalParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get tracks", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch tracks"))
		return
	}

	hasMore := offset+limit < total

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks":   tracks,
			"total":    total,
			"offset":   offset,
			"limit":    limit,
			"has_more": hasMore,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// A shuffle client sends back what it already holds so a refill does not hand
// it the same tracks again. The cap matches the shuffle limit cap: beyond that
// the exclusion list stops being worth the round trip.
const maxShuffleExcludeIDs = 200

// Ids are skipped rather than rejected, so the accepted-id cap alone does not
// bound the work: a request made entirely of junk never reaches it. Cap the
// fields examined too, generously enough that no honest client notices.
const maxShuffleExcludeFields = maxShuffleExcludeIDs * 4

// parseShuffleExcludeIDs turns repeated `exclude=` query values into the JSON
// array GetRandomTracks binds to json_each. Unparseable or out-of-range values
// are skipped rather than rejected: an exclusion is an optimization, and
// failing the whole request over one bad id would stop playback.
func parseShuffleExcludeIDs(values []string) string {
	capacity := min(len(values), maxShuffleExcludeIDs)
	ids := make([]int64, 0, capacity)
	seen := make(map[int64]struct{}, capacity)
	scanned := 0

	for _, value := range values {
		for field := range strings.SplitSeq(value, ",") {
			if scanned == maxShuffleExcludeFields {
				break
			}
			scanned++

			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}

			id, err := strconv.ParseInt(field, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}

			seen[id] = struct{}{}
			ids = append(ids, id)

			if len(ids) == maxShuffleExcludeIDs {
				encoded, err := json.Marshal(ids)
				if err != nil {
					return "[]"
				}
				return string(encoded)
			}
		}

		if scanned == maxShuffleExcludeFields {
			break
		}
	}

	encoded, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}

	return string(encoded)
}

func (app *Application) GetShuffleTracks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := int64(50)
	if l := query.Get("limit"); l != "" {
		parsed, err := strconv.ParseInt(l, 10, 64)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 200 {
		limit = 200
	}

	tracks, err := app.Queries.GetRandomTracks(r.Context(), database.GetRandomTracksParams{
		ExcludeIds: parseShuffleExcludeIDs(query["exclude"]),
		RowLimit:   limit,
	})
	if err != nil {
		app.Logger.Error("failed to get random tracks", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch random tracks"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks": tracks,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetMusicStats(w http.ResponseWriter, r *http.Request) {
	counts, err := app.Queries.GetMusicLibraryCounts(r.Context())
	if err != nil {
		app.Logger.Error("failed to get music library counts", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch music stats"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"total_albums":    counts.AlbumsCount,
			"total_tracks":    counts.TracksCount,
			"total_musicians": counts.MusiciansCount,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
