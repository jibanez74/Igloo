package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func (app *Application) GetPlaylists(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	playlists, err := app.Queries.GetPlaylistsWithCollaboratorAccess(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get playlists", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch playlists"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"playlists": playlists,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	limit := int64(50)
	l := r.URL.Query().Get("limit")
	if l != "" {
		parsed, err := strconv.ParseInt(l, 10, 64)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := int64(0)
	o := r.URL.Query().Get("offset")
	if o != "" {
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	var req CreatePlaylistRequest
	readErr := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	validationErr := validatePlaylistMetadata(req.Name, req.Description)
	if validationErr != nil {
		helpers.ErrorJSON(w, validationErr, http.StatusBadRequest)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	existing, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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

	if !app.mustBeTrackPlaylist(w, existing) {
		return
	}

	var req UpdatePlaylistRequest
	readErr := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	validationErr := validatePlaylistMetadata(req.Name, req.Description)
	if validationErr != nil {
		helpers.ErrorJSON(w, validationErr, http.StatusBadRequest)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to delete playlist"))
		return
	}

	if permission != PermissionOwner {
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	var req AddTracksRequest
	readErr := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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

	if !app.mustBeTrackPlaylist(w, playlist) {
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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

	if !app.mustBeTrackPlaylist(w, playlist) {
		return
	}

	var req ReorderTracksRequest
	readErr := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
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
	app.getPlaylistCollaborators(w, r, app.mustBeTrackPlaylist)
}

func (app *Application) GetMoviePlaylistCollaborators(w http.ResponseWriter, r *http.Request) {
	app.getPlaylistCollaborators(w, r, app.mustBeMoviePlaylist)
}

func (app *Application) getPlaylistCollaborators(
	w http.ResponseWriter,
	r *http.Request,
	mustBePlaylistType func(http.ResponseWriter, database.Playlist) bool,
) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
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

	if !mustBePlaylistType(w, playlist) {
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
	app.addCollaborator(w, r, app.mustBeTrackPlaylist)
}

func (app *Application) AddMoviePlaylistCollaborator(w http.ResponseWriter, r *http.Request) {
	app.addCollaborator(w, r, app.mustBeMoviePlaylist)
}

func (app *Application) addCollaborator(
	w http.ResponseWriter,
	r *http.Request,
	mustBePlaylistType func(http.ResponseWriter, database.Playlist) bool,
) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add collaborator"))
		return
	}

	if permission != PermissionOwner {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can add collaborators"), http.StatusForbidden)
		return
	}

	if !mustBePlaylistType(w, playlist) {
		return
	}

	var req AddCollaboratorRequest
	readErr := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.UserId == userID {
		helpers.ErrorJSON(w, errors.New("you cannot add yourself as a collaborator"), http.StatusBadRequest)
		return
	}

	userOK, err := app.Queries.UserExists(r.Context(), req.UserId)
	if err != nil {
		app.Logger.Error("failed to verify user", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to add collaborator"))
		return
	}
	if !userOK {
		helpers.ErrorJSON(w, errors.New("user not found"), http.StatusNotFound)
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
	app.removeCollaborator(w, r, app.mustBeTrackPlaylist)
}

func (app *Application) RemoveMoviePlaylistCollaborator(w http.ResponseWriter, r *http.Request) {
	app.removeCollaborator(w, r, app.mustBeMoviePlaylist)
}

func (app *Application) removeCollaborator(
	w http.ResponseWriter,
	r *http.Request,
	mustBePlaylistType func(http.ResponseWriter, database.Playlist) bool,
) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("playlist not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove collaborator"))
		return
	}

	if permission != PermissionOwner {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can remove collaborators"), http.StatusForbidden)
		return
	}

	if !mustBePlaylistType(w, playlist) {
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
