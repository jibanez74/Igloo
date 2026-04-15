package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// background is a package-level background context used for operations that must
// outlive the originating HTTP request (e.g. HLS warm-up after room creation).
var background = context.Background()

type watchRoomMemberSummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
}

type watchRoomListItem struct {
	ID           int64                    `json:"id"`
	MovieID      int64                    `json:"movie_id"`
	MovieTitle   string                   `json:"movie_title"`
	MoviePoster  *string                  `json:"movie_poster"`
	Owner        watchRoomMemberSummary   `json:"owner"`
	Members      []watchRoomMemberSummary `json:"members"`
	PlaybackMode string                   `json:"playback_mode"`
	IsOwner      bool                     `json:"is_owner"`
	CreatedAt    string                   `json:"created_at"`
}

type watchRoomDetail struct {
	ID            int64                    `json:"id"`
	MovieID       int64                    `json:"movie_id"`
	MovieTitle    string                   `json:"movie_title"`
	MoviePoster   *string                  `json:"movie_poster"`
	Owner         watchRoomMemberSummary   `json:"owner"`
	Members       []watchRoomMemberSummary `json:"members"`
	PlaybackMode  string                   `json:"playback_mode"`
	AudioTrack    int64                    `json:"audio_track"`
	SubtitleTrack *int64                   `json:"subtitle_track"`
	IsOwner       bool                     `json:"is_owner"`
	CreatedAt     string                   `json:"created_at"`
}

type createWatchRoomRequest struct {
	MovieID        int64   `json:"movie_id"`
	Mode           string  `json:"mode"`
	AudioTrack     int64   `json:"audio_track"`
	SubtitleTrack  *int64  `json:"subtitle_track"`
	InvitedUserIDs []int64 `json:"invited_user_ids"`
}

func parseRoomID(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	roomID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || roomID <= 0 {
		return 0, errors.New("invalid room id")
	}
	return roomID, nil
}

func isValidPlaybackMode(mode string) bool {
	if mode == helpers.WATCH_ROOM_PLAYBACK_MODE_DIRECT {
		return true
	}
	return helpers.IsAllowedHLSProfile(mode)
}

// deduplicateAndFilterUserIDs removes duplicates and the owner from invited user IDs.
func deduplicateAndFilterUserIDs(ids []int64, ownerID int64) []int64 {
	seen := make(map[int64]bool)
	result := []int64{}
	for _, id := range ids {
		if id == ownerID || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func (app *Application) loadRoomMembers(ctx context.Context, roomID int64) ([]watchRoomMemberSummary, error) {
	rows, err := app.Queries.GetWatchRoomMembers(ctx, roomID)
	if err != nil {
		return nil, err
	}
	members := make([]watchRoomMemberSummary, len(rows))
	for i, row := range rows {
		var avatar *string
		if row.Avatar.Valid {
			avatar = &row.Avatar.String
		}
		members[i] = watchRoomMemberSummary{
			ID:     row.ID,
			Name:   row.Name,
			Avatar: avatar,
		}
	}
	return members, nil
}

func findMemberByID(members []watchRoomMemberSummary, id int64) (watchRoomMemberSummary, bool) {
	for _, m := range members {
		if m.ID == id {
			return m, true
		}
	}
	return watchRoomMemberSummary{}, false
}

// GetWatchRooms serves GET /api/watch-rooms.
// Returns all rooms the authenticated user owns or is invited to.
func (app *Application) GetWatchRooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	rooms, err := app.Queries.GetWatchRoomsForUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to fetch watch rooms", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	roomIDs := make([]int64, 0, len(rooms))
	movieIDs := make([]int64, 0, len(rooms))
	seenMovieIDs := make(map[int64]bool)
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
		if seenMovieIDs[room.MovieID] {
			continue
		}
		seenMovieIDs[room.MovieID] = true
		movieIDs = append(movieIDs, room.MovieID)
	}

	movies, err := app.Queries.GetMoviesByIDs(r.Context(), movieIDs)
	if err != nil {
		app.Logger.Error("failed to fetch movies for watch rooms", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	memberRows, err := app.Queries.GetWatchRoomMembersByRoomIDs(r.Context(), roomIDs)
	if err != nil {
		app.Logger.Error("failed to fetch members for watch rooms", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	movieByID := make(map[int64]database.Movie, len(movies))
	for _, movie := range movies {
		movieByID[movie.ID] = movie
	}

	membersByRoomID := make(map[int64][]watchRoomMemberSummary, len(roomIDs))
	for _, row := range memberRows {
		var avatar *string
		if row.Avatar.Valid {
			avatar = &row.Avatar.String
		}
		membersByRoomID[row.RoomID] = append(membersByRoomID[row.RoomID], watchRoomMemberSummary{
			ID:     row.ID,
			Name:   row.Name,
			Avatar: avatar,
		})
	}

	items := make([]watchRoomListItem, 0, len(rooms))
	for _, room := range rooms {
		movie, movieOK := movieByID[room.MovieID]
		if !movieOK {
			app.Logger.Error("failed to find movie for watch room", "room_id", room.ID, "movie_id", room.MovieID)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
			return
		}

		members := membersByRoomID[room.ID]
		owner, _ := findMemberByID(members, room.OwnerUserID)

		var moviePoster *string
		if movie.PosterPath.Valid {
			moviePoster = &movie.PosterPath.String
		}

		items = append(items, watchRoomListItem{
			ID:           room.ID,
			MovieID:      room.MovieID,
			MovieTitle:   movie.Title,
			MoviePoster:  moviePoster,
			Owner:        owner,
			Members:      members,
			PlaybackMode: room.PlaybackMode,
			IsOwner:      room.OwnerUserID == userID,
			CreatedAt:    room.CreatedAt,
		})
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"rooms": items},
	})
}

// GetWatchRoom serves GET /api/watch-rooms/{id}.
// Returns room details. Only room members can access.
func (app *Application) GetWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	room, err := app.Queries.GetWatchRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("room not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to fetch watch room", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	_, err = app.Queries.IsWatchRoomMember(r.Context(), database.IsWatchRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
			return
		}
		app.Logger.Error("failed to check room membership", "error", err, "room_id", roomID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	movie, err := app.Queries.GetMovieByID(r.Context(), room.MovieID)
	if err != nil {
		app.Logger.Error("failed to fetch movie for watch room", "error", err, "room_id", roomID, "movie_id", room.MovieID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	members, err := app.loadRoomMembers(r.Context(), roomID)
	if err != nil {
		app.Logger.Error("failed to fetch members for watch room", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	owner, _ := findMemberByID(members, room.OwnerUserID)

	var moviePoster *string
	if movie.PosterPath.Valid {
		moviePoster = &movie.PosterPath.String
	}

	var subtitleTrack *int64
	if room.SubtitleTrack.Valid {
		subtitleTrack = &room.SubtitleTrack.Int64
	}

	detail := watchRoomDetail{
		ID:            room.ID,
		MovieID:       room.MovieID,
		MovieTitle:    movie.Title,
		MoviePoster:   moviePoster,
		Owner:         owner,
		Members:       members,
		PlaybackMode:  room.PlaybackMode,
		AudioTrack:    room.AudioTrack,
		SubtitleTrack: subtitleTrack,
		IsOwner:       room.OwnerUserID == userID,
		CreatedAt:     room.CreatedAt,
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"room": detail},
	})
}

// CreateWatchRoom serves POST /api/watch-rooms.
// Creates a room with the caller as owner and adds all invited users as members in a transaction.
func (app *Application) CreateWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	var req createWatchRoomRequest
	if err := helpers.ReadJSON(w, r, &req, helpers.MAX_WATCH_ROOM_REQUEST_SIZE); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.MovieID <= 0 {
		helpers.ErrorJSON(w, errors.New("movie_id is required"), http.StatusBadRequest)
		return
	}

	if req.Mode == "" {
		helpers.ErrorJSON(w, errors.New("mode is required"), http.StatusBadRequest)
		return
	}

	if !isValidPlaybackMode(req.Mode) {
		helpers.ErrorJSON(w, errors.New("invalid playback mode"), http.StatusBadRequest)
		return
	}

	if req.AudioTrack < 0 {
		helpers.ErrorJSON(w, errors.New("audio_track must be non-negative"), http.StatusBadRequest)
		return
	}

	if req.SubtitleTrack != nil && *req.SubtitleTrack < 0 {
		helpers.ErrorJSON(w, errors.New("subtitle_track must be non-negative"), http.StatusBadRequest)
		return
	}

	_, err := app.Queries.GetMovieByID(r.Context(), req.MovieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusBadRequest)
			return
		}
		app.Logger.Error("failed to verify movie for watch room", "error", err, "movie_id", req.MovieID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	invitedIDs := deduplicateAndFilterUserIDs(req.InvitedUserIDs, userID)

	if len(invitedIDs) > 0 {
		count, err := app.Queries.CountUsersByIDs(r.Context(), invitedIDs)
		if err != nil {
			app.Logger.Error("failed to validate invited user IDs", "error", err)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
			return
		}
		if count != int64(len(invitedIDs)) {
			helpers.ErrorJSON(w, errors.New("one or more invited users do not exist"), http.StatusBadRequest)
			return
		}
	}

	var subtitleTrack sql.NullInt64
	if req.SubtitleTrack != nil {
		subtitleTrack = sql.NullInt64{Int64: *req.SubtitleTrack, Valid: true}
	}

	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.Logger.Error("failed to begin transaction for watch room creation", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	qtx := app.Queries.WithTx(tx)

	room, err := qtx.CreateWatchRoom(r.Context(), database.CreateWatchRoomParams{
		OwnerUserID:   userID,
		MovieID:       req.MovieID,
		PlaybackMode:  req.Mode,
		AudioTrack:    req.AudioTrack,
		SubtitleTrack: subtitleTrack,
	})
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("failed to create watch room", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	err = qtx.AddWatchRoomMember(r.Context(), database.AddWatchRoomMemberParams{
		RoomID: room.ID,
		UserID: userID,
	})
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("failed to add owner as room member", "error", err, "room_id", room.ID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	for _, invitedID := range invitedIDs {
		err = qtx.AddWatchRoomMember(r.Context(), database.AddWatchRoomMemberParams{
			RoomID: room.ID,
			UserID: invitedID,
		})
		if err != nil {
			_ = tx.Rollback()
			app.Logger.Error("failed to add invited user as room member", "error", err, "room_id", room.ID, "user_id", invitedID)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
			return
		}
	}

	if err = tx.Commit(); err != nil {
		_ = tx.Rollback()
		app.Logger.Error("failed to commit watch room transaction", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// If the room uses HLS, start FFmpeg immediately so participants experience
	// minimal startup latency when they join. Per plan, failure here rolls the
	// room back: we delete it and surface the error to the caller.
	if req.Mode != helpers.WATCH_ROOM_PLAYBACK_MODE_DIRECT {
		warmErr := app.WarmUpRoomHLSSession(background, room.ID, req.MovieID, req.Mode, int(req.AudioTrack))
		if warmErr != nil {
			deleteErr := app.Queries.DeleteWatchRoom(background, room.ID)
			if deleteErr != nil {
				app.Logger.Error(
					"hls warm-up failed and watch room rollback delete failed",
					"warm_up_error", warmErr,
					"delete_error", deleteErr,
					"room_id", room.ID,
				)
				helpers.ErrorJSON(w, errors.New("failed to start playback session for this room and failed to roll back the room"), http.StatusInternalServerError)
				return
			}
			app.Logger.Error("hls warm-up failed; rolled back watch room", "warm_up_error", warmErr, "room_id", room.ID)
			helpers.ErrorJSON(w, errors.New("failed to start playback session for this room"), http.StatusInternalServerError)
			return
		}
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"room_id": room.ID},
	})
}

// JoinWatchRoom serves POST /api/watch-rooms/{id}/join.
// Validates membership and returns a join confirmation.
func (app *Application) JoinWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	_, err = app.Queries.GetWatchRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("room not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to fetch watch room for join", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	_, err = app.Queries.IsWatchRoomMember(r.Context(), database.IsWatchRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
			return
		}
		app.Logger.Error("failed to check room membership for join", "error", err, "room_id", roomID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"room_id": roomID, "joined": true},
	})
}

// DeleteWatchRoom serves DELETE /api/watch-rooms/{id}.
// Only the room owner can delete. Cascade removes all member rows.
func (app *Application) DeleteWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	_, err = app.Queries.GetWatchRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("room not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to fetch watch room for delete", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	_, err = app.Queries.IsWatchRoomOwner(r.Context(), database.IsWatchRoomOwnerParams{
		ID:          roomID,
		OwnerUserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("only the room owner can delete this room"), http.StatusForbidden)
			return
		}
		app.Logger.Error("failed to verify room ownership for delete", "error", err, "room_id", roomID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	err = app.Queries.DeleteWatchRoom(r.Context(), roomID)
	if err != nil {
		app.Logger.Error("failed to delete watch room", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Stop and remove the room HLS session if one was running.
	// This is a no-op for direct-play rooms or rooms that never successfully warmed up.
	app.CleanupRoomHLSSession(roomID)
	app.WatchRoomHub.deleteRoom(roomID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"deleted": true},
	})
}
