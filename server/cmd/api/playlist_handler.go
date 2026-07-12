package main

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type PlaylistPermission int

const (
	PermissionNone PlaylistPermission = iota
	PermissionView
	PermissionEdit
	PermissionOwner
)

const (
	playlistContentTypeTrack = "track"
	playlistContentTypeMovie = "movie"
	maxPlaylistRequestSize   = 1024 * 1024 // 1 MB
)

type CreatePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type UpdatePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverImage  string `json:"cover_image"`
	IsPublic    bool   `json:"is_public"`
}

type AddTracksRequest struct {
	TrackIds []int64 `json:"track_ids"`
}

type ReorderTracksRequest struct {
	TrackIds []int64 `json:"track_ids"`
}

type AddCollaboratorRequest struct {
	UserId  int64 `json:"user_id"`
	CanEdit bool  `json:"can_edit"`
}

type CreateMoviePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	MovieID     *int64 `json:"movie_id"`
}

type UpdateMoviePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverImage  string `json:"cover_image"`
	IsPublic    bool   `json:"is_public"`
	MovieID     *int64 `json:"movie_id"`
}

type AddMoviesRequest struct {
	MovieIds []int64 `json:"movie_ids"`
}

func (app *Application) getPlaylistPermission(ctx context.Context, playlistId, userId int64) (PlaylistPermission, error) {
	playlist, err := app.Queries.GetPlaylistById(ctx, playlistId)
	if err != nil {
		return PermissionNone, err
	}

	if playlist.UserID == userId {
		return PermissionOwner, nil
	}

	canEdit, err := app.Queries.CanUserEditPlaylist(ctx, database.CanUserEditPlaylistParams{
		ID:       playlistId,
		UserID:   userId,
		UserID_2: userId,
	})
	if err != nil {
		return PermissionNone, err
	}

	if canEdit {
		return PermissionEdit, nil
	}

	isCollaborator, err := app.Queries.IsUserCollaborator(ctx, database.IsUserCollaboratorParams{
		PlaylistID: playlistId,
		UserID:     userId,
	})
	if err != nil {
		return PermissionNone, err
	}

	if isCollaborator {
		return PermissionView, nil
	}

	if playlist.IsPublic {
		return PermissionView, nil
	}

	return PermissionNone, nil
}

func (app *Application) mustBeTrackPlaylist(w http.ResponseWriter, playlist database.Playlist) bool {
	if playlist.ContentType != playlistContentTypeTrack {
		helpers.ErrorJSON(w, errors.New("not a track playlist"), http.StatusBadRequest)
		return false
	}
	return true
}

func (app *Application) mustBeMoviePlaylist(w http.ResponseWriter, playlist database.Playlist) bool {
	if playlist.ContentType != playlistContentTypeMovie {
		helpers.ErrorJSON(w, errors.New("not a movie playlist"), http.StatusBadRequest)
		return false
	}
	return true
}

func (app *Application) GetPlaylists(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	playlists, err := app.Queries.GetPlaylistsWithCollaboratorAccess(r.Context(), database.GetPlaylistsWithCollaboratorAccessParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		app.Logger.Error("failed to get playlists", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlists"))
		return
	}

	type PlaylistResponse struct {
		database.GetPlaylistsWithCollaboratorAccessRow
		IsOwner bool `json:"is_owner"`
		CanEdit bool `json:"can_edit"`
	}

	response := make([]PlaylistResponse, len(playlists))
	for i, p := range playlists {
		isOwner := p.UserID == userID
		canEdit := isOwner
		if !isOwner {
			canEditResult, _ := app.Queries.CanUserEditPlaylist(r.Context(), database.CanUserEditPlaylistParams{
				ID:       p.ID,
				UserID:   userID,
				UserID_2: userID,
			})
			canEdit = canEditResult
		}
		response[i] = PlaylistResponse{
			GetPlaylistsWithCollaboratorAccessRow: p,
			IsOwner:                               isOwner,
			CanEdit:                               canEdit,
		}
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"playlists": response,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}

	if permission == PermissionNone {
		helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	playlist, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	trackCount, _ := app.Queries.CountPlaylistTracks(r.Context(), playlistId)
	duration, _ := app.Queries.GetPlaylistDuration(r.Context(), playlistId)

	var collaborators []database.GetPlaylistCollaboratorsRow
	if permission == PermissionOwner {
		collaborators, _ = app.Queries.GetPlaylistCollaborators(r.Context(), playlistId)
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"playlist":      playlist,
			"track_count":   trackCount,
			"duration":      duration,
			"is_owner":      permission == PermissionOwner,
			"can_edit":      permission >= PermissionEdit,
			"collaborators": collaborators,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}

	if permission == PermissionNone {
		helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}
	if !app.mustBeTrackPlaylist(w, pl) {
		return
	}

	limit := int64(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.ParseInt(l, 10, 64)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := int64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		parsed, err := strconv.ParseInt(o, 10, 64)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	tracks, err := app.Queries.GetPlaylistTracksInfinite(r.Context(), database.GetPlaylistTracksInfiniteParams{
		PlaylistID: playlistId,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		app.Logger.Error("failed to get playlist tracks", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist tracks"))
		return
	}

	total, _ := app.Queries.CountPlaylistTracks(r.Context(), playlistId)
	hasMore := offset+int64(len(tracks)) < total

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks":      tracks,
			"total":       total,
			"has_more":    hasMore,
			"next_offset": offset + int64(len(tracks)),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	var req CreatePlaylistRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("playlist name is required"), http.StatusBadRequest)
		return
	}
	if len(req.Name) > 255 {
		helpers.ErrorJSON(w, errors.New("playlist name is too long (max 255 characters)"), http.StatusBadRequest)
		return
	}
	if len(req.Description) > 1000 {
		helpers.ErrorJSON(w, errors.New("description is too long (max 1000 characters)"), http.StatusBadRequest)
		return
	}

	playlist, err := app.Queries.CreatePlaylist(r.Context(), database.CreatePlaylistParams{
		UserID:      userID,
		Name:        req.Name,
		Description: helpers.NullString(req.Description),
		CoverImage:  sql.NullString{Valid: false},
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		app.Logger.Error("failed to create playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to create playlist"))
		return
	}

	app.Logger.Info("playlist created", "id", playlist.ID, "name", playlist.Name, "user_id", userID)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Playlist created successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (app *Application) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playlist"))
		return
	}

	if permission != PermissionOwner {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can update metadata"), http.StatusForbidden)
		return
	}

	existing, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playlist"))
		return
	}
	if !app.mustBeTrackPlaylist(w, existing) {
		return
	}

	var req UpdatePlaylistRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("playlist name is required"), http.StatusBadRequest)
		return
	}
	if len(req.Name) > 255 {
		helpers.ErrorJSON(w, errors.New("playlist name is too long (max 255 characters)"), http.StatusBadRequest)
		return
	}
	if len(req.Description) > 1000 {
		helpers.ErrorJSON(w, errors.New("description is too long (max 1000 characters)"), http.StatusBadRequest)
		return
	}

	playlist, err := app.Queries.UpdatePlaylist(r.Context(), database.UpdatePlaylistParams{
		ID:          playlistId,
		Name:        req.Name,
		Description: helpers.NullString(req.Description),
		CoverImage:  helpers.NullString(req.CoverImage),
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		app.Logger.Error("failed to update playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playlist"))
		return
	}

	app.Logger.Info("playlist updated", "id", playlist.ID, "name", playlist.Name)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Playlist updated successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to delete playlist"))
		return
	}

	if playlist.UserID != userID {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can delete it"), http.StatusForbidden)
		return
	}

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	err = app.Queries.DeletePlaylist(r.Context(), database.DeletePlaylistParams{
		ID:     playlistId,
		UserID: userID,
	})
	if err != nil {
		app.Logger.Error("failed to delete playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to delete playlist"))
		return
	}

	app.Logger.Info("playlist deleted", "id", playlistId, "name", playlist.Name)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Playlist deleted successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) AddTracksToPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add tracks"))
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to add tracks to this playlist"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add tracks"))
		return
	}
	if !app.mustBeTrackPlaylist(w, pl) {
		return
	}

	var req AddTracksRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if len(req.TrackIds) == 0 {
		helpers.ErrorJSON(w, errors.New("at least one track id is required"), http.StatusBadRequest)
		return
	}

	addedCount := 0
	skippedCount := 0

	for _, trackId := range req.TrackIds {
		inPlaylist, _ := app.Queries.IsTrackInPlaylist(r.Context(), database.IsTrackInPlaylistParams{
			PlaylistID: playlistId,
			TrackID:    trackId,
		})
		if inPlaylist {
			skippedCount++
			continue
		}

		_, err := app.Queries.AddTrackToPlaylist(r.Context(), database.AddTrackToPlaylistParams{
			PlaylistID: playlistId,
			TrackID:    trackId,
			AddedBy:    sql.NullInt64{Int64: userID, Valid: true},
		})
		if err != nil {
			app.Logger.Warn("failed to add track to playlist", "error", err, "track_id", trackId, "playlist_id", playlistId)
			continue
		}
		addedCount++
	}

	_ = app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistId)

	app.Logger.Info("tracks added to playlist", "playlist_id", playlistId, "added", addedCount, "skipped", skippedCount)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Tracks added successfully",
		Data: map[string]any{
			"added":   addedCount,
			"skipped": skippedCount,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) RemoveTrackFromPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	playlistIdParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(playlistIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	trackIdParam := chi.URLParam(r, "trackId")
	trackId, err := strconv.ParseInt(trackIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid track id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove track"))
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to remove tracks from this playlist"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove track"))
		return
	}
	if !app.mustBeTrackPlaylist(w, pl) {
		return
	}

	err = app.Queries.RemoveTrackFromPlaylist(r.Context(), database.RemoveTrackFromPlaylistParams{
		PlaylistID: playlistId,
		TrackID:    trackId,
	})
	if err != nil {
		app.Logger.Error("failed to remove track from playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove track"))
		return
	}

	_ = app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistId)

	app.Logger.Info("track removed from playlist", "playlist_id", playlistId, "track_id", trackId)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Track removed successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) ReorderPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to reorder tracks"))
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to reorder tracks in this playlist"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to reorder tracks"))
		return
	}
	if !app.mustBeTrackPlaylist(w, pl) {
		return
	}

	var req ReorderTracksRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if len(req.TrackIds) == 0 {
		helpers.ErrorJSON(w, errors.New("track ids are required"), http.StatusBadRequest)
		return
	}

	for i, trackId := range req.TrackIds {
		err := app.Queries.UpdateTrackPosition(r.Context(), database.UpdateTrackPositionParams{
			Position:   int64(i),
			PlaylistID: playlistId,
			TrackID:    trackId,
		})
		if err != nil {
			app.Logger.Warn("failed to update track position", "error", err, "track_id", trackId, "position", i)
		}
	}

	_ = app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistId)

	app.Logger.Info("playlist tracks reordered", "playlist_id", playlistId)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Tracks reordered successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetPlaylistCollaborators(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch collaborators"))
		return
	}

	if permission != PermissionOwner {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can view collaborators"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch collaborators"))
		return
	}
	if !app.mustBeTrackPlaylist(w, pl) {
		return
	}

	collaborators, err := app.Queries.GetPlaylistCollaborators(r.Context(), playlistId)
	if err != nil {
		app.Logger.Error("failed to get collaborators", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch collaborators"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"collaborators": collaborators,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) AddCollaborator(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add collaborator"))
		return
	}

	if playlist.UserID != userID {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can add collaborators"), http.StatusForbidden)
		return
	}

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	var req AddCollaboratorRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.UserId == userID {
		helpers.ErrorJSON(w, errors.New("you cannot add yourself as a collaborator"), http.StatusBadRequest)
		return
	}

	_, err = app.Queries.GetUser(r.Context(), req.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("user not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to verify user", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add collaborator"))
		return
	}

	collaborator, err := app.Queries.AddCollaborator(r.Context(), database.AddCollaboratorParams{
		PlaylistID: playlistId,
		UserID:     req.UserId,
		CanEdit:    req.CanEdit,
	})
	if err != nil {
		app.Logger.Error("failed to add collaborator", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add collaborator"))
		return
	}

	app.Logger.Info("collaborator added", "playlist_id", playlistId, "user_id", req.UserId)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Collaborator added successfully",
		Data: map[string]any{
			"collaborator": collaborator,
		},
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (app *Application) RemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	playlistIdParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(playlistIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	collaboratorIdParam := chi.URLParam(r, "userId")
	collaboratorId, err := strconv.ParseInt(collaboratorIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid user id"), http.StatusBadRequest)
		return
	}

	playlist, err := app.Queries.GetPlaylistById(r.Context(), playlistId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove collaborator"))
		return
	}

	if playlist.UserID != userID {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can remove collaborators"), http.StatusForbidden)
		return
	}

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	err = app.Queries.RemoveCollaborator(r.Context(), database.RemoveCollaboratorParams{
		PlaylistID: playlistId,
		UserID:     collaboratorId,
	})
	if err != nil {
		app.Logger.Error("failed to remove collaborator", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove collaborator"))
		return
	}

	app.Logger.Info("collaborator removed", "playlist_id", playlistId, "user_id", collaboratorId)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Collaborator removed successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func libraryRowsFromPlaylistAsc(rows []database.GetPlaylistMoviesPaginatedAscRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
}

func libraryRowsFromPlaylistDesc(rows []database.GetPlaylistMoviesPaginatedDescRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
}

func (app *Application) GetMoviePlaylists(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	playlists, err := app.Queries.GetMoviePlaylistsWithCollaboratorAccess(r.Context(), database.GetMoviePlaylistsWithCollaboratorAccessParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		app.Logger.Error("failed to get movie playlists", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlists"))
		return
	}

	type moviePlaylistResponse struct {
		database.GetMoviePlaylistsWithCollaboratorAccessRow
		IsOwner bool `json:"is_owner"`
		CanEdit bool `json:"can_edit"`
	}

	response := make([]moviePlaylistResponse, len(playlists))
	for i, p := range playlists {
		isOwner := p.UserID == userID
		canEdit := isOwner
		if !isOwner {
			canEditResult, _ := app.Queries.CanUserEditPlaylist(r.Context(), database.CanUserEditPlaylistParams{
				ID:       p.ID,
				UserID:   userID,
				UserID_2: userID,
			})
			canEdit = canEditResult
		}
		response[i] = moviePlaylistResponse{
			GetMoviePlaylistsWithCollaboratorAccessRow: p,
			IsOwner: isOwner,
			CanEdit: canEdit,
		}
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"playlists": response,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) CreateMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	var req CreateMoviePlaylistRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("playlist name is required"), http.StatusBadRequest)
		return
	}
	if len(req.Name) > 255 {
		helpers.ErrorJSON(w, errors.New("playlist name is too long (max 255 characters)"), http.StatusBadRequest)
		return
	}
	if len(req.Description) > 1000 {
		helpers.ErrorJSON(w, errors.New("description is too long (max 1000 characters)"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var movieID sql.NullInt64
	if req.MovieID != nil {
		_, err := app.Queries.GetMovieByID(ctx, *req.MovieID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusBadRequest)
				return
			}
			app.Logger.Error("failed to verify movie for playlist", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to verify movie"))
			return
		}
		movieID = sql.NullInt64{Int64: *req.MovieID, Valid: true}
	}

	playlist, err := app.Queries.CreateMoviePlaylist(ctx, database.CreateMoviePlaylistParams{
		UserID:      userID,
		Name:        req.Name,
		Description: helpers.NullString(req.Description),
		CoverImage:  sql.NullString{Valid: false},
		IsPublic:    req.IsPublic,
		MovieID:     movieID,
	})
	if err != nil {
		app.Logger.Error("failed to create movie playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to create playlist"))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Playlist created successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (app *Application) GetMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}

	if permission == PermissionNone {
		helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	playlist, err := app.Queries.GetPlaylistById(r.Context(), playlistID)
	if err != nil {
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	movieCount, _ := app.Queries.CountPlaylistMovies(r.Context(), playlistID)

	var collaborators []database.GetPlaylistCollaboratorsRow
	if permission == PermissionOwner {
		collaborators, _ = app.Queries.GetPlaylistCollaborators(r.Context(), playlistID)
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"playlist":      playlist,
			"movie_count":   movieCount,
			"is_owner":      permission == PermissionOwner,
			"can_edit":      permission >= PermissionEdit,
			"collaborators": collaborators,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) UpdateMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playlist"))
		return
	}

	if permission != PermissionOwner {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can update metadata"), http.StatusForbidden)
		return
	}

	existing, err := app.Queries.GetPlaylistById(r.Context(), playlistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playlist"))
		return
	}

	if !app.mustBeMoviePlaylist(w, existing) {
		return
	}

	var req UpdateMoviePlaylistRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("playlist name is required"), http.StatusBadRequest)
		return
	}
	if len(req.Name) > 255 {
		helpers.ErrorJSON(w, errors.New("playlist name is too long (max 255 characters)"), http.StatusBadRequest)
		return
	}
	if len(req.Description) > 1000 {
		helpers.ErrorJSON(w, errors.New("description is too long (max 1000 characters)"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var movieID sql.NullInt64
	if req.MovieID != nil {
		_, err := app.Queries.GetMovieByID(ctx, *req.MovieID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusBadRequest)
				return
			}
			app.Logger.Error("failed to verify movie for playlist", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to verify movie"))
			return
		}
		movieID = sql.NullInt64{Int64: *req.MovieID, Valid: true}
	} else {
		movieID = existing.MovieID
	}

	playlist, err := app.Queries.UpdateMoviePlaylist(ctx, database.UpdateMoviePlaylistParams{
		Name:        req.Name,
		Description: helpers.NullString(req.Description),
		CoverImage:  helpers.NullString(req.CoverImage),
		IsPublic:    req.IsPublic,
		MovieID:     movieID,
		ID:          playlistID,
		UserID:      userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to update movie playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playlist"))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Playlist updated successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) DeleteMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, err := app.Queries.GetPlaylistById(r.Context(), playlistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to delete playlist"))
		return
	}

	if playlist.UserID != userID {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can delete it"), http.StatusForbidden)
		return
	}

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	err = app.Queries.DeletePlaylist(r.Context(), database.DeletePlaylistParams{
		ID:     playlistID,
		UserID: userID,
	})
	if err != nil {
		app.Logger.Error("failed to delete movie playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to delete playlist"))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Playlist deleted successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetMoviePlaylistMovies(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}

	if permission == PermissionNone {
		helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistID)
	if err != nil {
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist"))
		return
	}
	if !app.mustBeMoviePlaylist(w, pl) {
		return
	}

	page, perPage, sortParam := parseMoviesLibraryQuery(r)
	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.CountPlaylistMovies(ctx, playlistID)
	if err != nil {
		app.Logger.Error("failed to count playlist movies", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlist items"))
		return
	}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		descRows, err := app.Queries.GetPlaylistMoviesPaginatedDesc(ctx, database.GetPlaylistMoviesPaginatedDescParams{
			PlaylistID: playlistID,
			Limit:      perPage,
			Offset:     offset,
		})
		if err != nil {
			app.Logger.Error("failed to get playlist movies", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to fetch playlist movies"))
			return
		}
		movies = libraryRowsFromPlaylistDesc(descRows)
	} else {
		ascRows, err := app.Queries.GetPlaylistMoviesPaginatedAsc(ctx, database.GetPlaylistMoviesPaginatedAscParams{
			PlaylistID: playlistID,
			Limit:      perPage,
			Offset:     offset,
		})
		if err != nil {
			app.Logger.Error("failed to get playlist movies", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to fetch playlist movies"))
			return
		}
		movies = libraryRowsFromPlaylistAsc(ascRows)
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) AddMoviesToMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add movies"))
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to edit this playlist"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add movies"))
		return
	}
	if !app.mustBeMoviePlaylist(w, pl) {
		return
	}

	var req AddMoviesRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if len(req.MovieIds) == 0 {
		helpers.ErrorJSON(w, errors.New("at least one movie id is required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	addedCount := 0
	skippedCount := 0

	for _, movieID := range req.MovieIds {
		_, err := app.Queries.GetMovieByID(ctx, movieID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				skippedCount++
				app.Logger.Warn("skip unknown movie id for playlist", "movie_id", movieID)
				continue
			}
			app.Logger.Error("failed to look up movie", "error", err, "movie_id", movieID)
			helpers.ErrorJSON(w, errors.New("failed to verify movies"), http.StatusInternalServerError)
			return
		}

		inPl, err := app.Queries.IsMovieInPlaylist(ctx, database.IsMovieInPlaylistParams{
			PlaylistID: playlistID,
			MovieID:    movieID,
		})
		if err != nil {
			app.Logger.Error("failed to check movie in playlist", "error", err, "movie_id", movieID, "playlist_id", playlistID)
			helpers.ErrorJSON(w, errors.New("failed to update playlist"), http.StatusInternalServerError)
			return
		}
		if inPl {
			skippedCount++
			continue
		}

		_, err = app.Queries.AddMovieToPlaylist(ctx, database.AddMovieToPlaylistParams{
			PlaylistID: playlistID,
			MovieID:    movieID,
			AddedBy:    sql.NullInt64{Int64: userID, Valid: true},
		})
		if err != nil {
			app.Logger.Error("failed to add movie to playlist", "error", err, "movie_id", movieID, "playlist_id", playlistID)
			helpers.ErrorJSON(w, errors.New("failed to add movies to playlist"), http.StatusInternalServerError)
			return
		}
		addedCount++
	}

	if addedCount > 0 {
		if err := app.Queries.UpdatePlaylistTimestamp(ctx, playlistID); err != nil {
			app.Logger.Error("failed to update playlist timestamp", "error", err, "playlist_id", playlistID)
			helpers.ErrorJSON(w, errors.New("failed to finalize playlist update"), http.StatusInternalServerError)
			return
		}
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Movies processed",
		Data: map[string]any{
			"added":   addedCount,
			"skipped": skippedCount,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) RemoveMovieFromMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	playlistID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	movieID, err := strconv.ParseInt(chi.URLParam(r, "movieId"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	permission, err := app.getPlaylistPermission(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to check playlist permission", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove movie"))
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to edit this playlist"), http.StatusForbidden)
		return
	}

	pl, err := app.Queries.GetPlaylistById(r.Context(), playlistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove movie"))
		return
	}
	if !app.mustBeMoviePlaylist(w, pl) {
		return
	}

	err = app.Queries.RemoveMovieFromPlaylist(r.Context(), database.RemoveMovieFromPlaylistParams{
		PlaylistID: playlistID,
		MovieID:    movieID,
	})
	if err != nil {
		app.Logger.Error("failed to remove movie from playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove movie"))
		return
	}

	if err := app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistID); err != nil {
		app.Logger.Error("failed to update playlist timestamp", "error", err, "playlist_id", playlistID)
		helpers.ErrorJSON(w, errors.New("failed to finalize playlist update"), http.StatusInternalServerError)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Movie removed from playlist",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
