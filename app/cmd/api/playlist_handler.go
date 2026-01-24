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

const maxPlaylistRequestSize = 1024 * 1024 // 1MB

// Permission levels for playlist access
type PlaylistPermission int

const (
	PermissionNone PlaylistPermission = iota
	PermissionView
	PermissionEdit
	PermissionOwner
)

// Request types
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

// getPlaylistPermission checks the user's permission level for a playlist
func (app *Application) getPlaylistPermission(ctx context.Context, playlistId, userId int64) (PlaylistPermission, error) {
	playlist, err := app.Queries.GetPlaylistById(ctx, playlistId)
	if err != nil {
		return PermissionNone, err
	}

	// Owner has full permissions
	if playlist.UserID == userId {
		return PermissionOwner, nil
	}

	// Check if user can edit (owner or collaborator with edit rights)
	canEdit, err := app.Queries.CanUserEditPlaylist(ctx, database.CanUserEditPlaylistParams{
		ID:       playlistId,
		UserID:   userId,
		UserID_2: userId,
	})
	if err != nil {
		return PermissionNone, err
	}

	if canEdit == 1 {
		return PermissionEdit, nil
	}

	// Check if user is a view-only collaborator
	isCollaborator, err := app.Queries.IsUserCollaborator(ctx, database.IsUserCollaboratorParams{
		PlaylistID: playlistId,
		UserID:     userId,
	})
	if err != nil {
		return PermissionNone, err
	}

	if isCollaborator == 1 {
		return PermissionView, nil
	}

	// Check if playlist is public
	if playlist.IsPublic {
		return PermissionView, nil
	}

	return PermissionNone, nil
}

// GetPlaylists returns all playlists the user owns or has collaborator access to
func (app *Application) GetPlaylists(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
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

	// Add owner/edit info to each playlist
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
			// Check collaborator edit permission
			canEditResult, _ := app.Queries.CanUserEditPlaylist(r.Context(), database.CanUserEditPlaylistParams{
				ID:       p.ID,
				UserID:   userID,
				UserID_2: userID,
			})
			canEdit = canEditResult == 1
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

// GetPlaylist returns a single playlist's details
func (app *Application) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Check permission
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

	// Get track count and duration
	trackCount, _ := app.Queries.CountPlaylistTracks(r.Context(), playlistId)
	duration, _ := app.Queries.GetPlaylistDuration(r.Context(), playlistId)

	// Get collaborators if owner
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

// GetPlaylistTracks returns tracks in a playlist with pagination for infinite scroll
func (app *Application) GetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Check permission
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

	// Parse pagination params
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

// CreatePlaylist creates a new playlist
func (app *Application) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req CreatePlaylistRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	// Validate
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

// UpdatePlaylist updates a playlist's metadata (owner only)
func (app *Application) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Only owner can update metadata
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

	var req UpdatePlaylistRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	// Validate
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

// DeletePlaylist deletes a playlist (owner only)
func (app *Application) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Get playlist to verify ownership
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

// AddTracksToPlaylist adds tracks to a playlist
func (app *Application) AddTracksToPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Check edit permission
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
		// Check if track already in playlist
		inPlaylist, _ := app.Queries.IsTrackInPlaylist(r.Context(), database.IsTrackInPlaylistParams{
			PlaylistID: playlistId,
			TrackID:    trackId,
		})
		if inPlaylist == 1 {
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

	// Update playlist timestamp
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

// RemoveTrackFromPlaylist removes a track from a playlist
func (app *Application) RemoveTrackFromPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
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

	// Check edit permission
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

	err = app.Queries.RemoveTrackFromPlaylist(r.Context(), database.RemoveTrackFromPlaylistParams{
		PlaylistID: playlistId,
		TrackID:    trackId,
	})
	if err != nil {
		app.Logger.Error("failed to remove track from playlist", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to remove track"))
		return
	}

	// Update playlist timestamp
	_ = app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistId)

	app.Logger.Info("track removed from playlist", "playlist_id", playlistId, "track_id", trackId)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Track removed successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// ReorderPlaylistTracks reorders tracks in a playlist
func (app *Application) ReorderPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Check edit permission
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

	var req ReorderTracksRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if len(req.TrackIds) == 0 {
		helpers.ErrorJSON(w, errors.New("track ids are required"), http.StatusBadRequest)
		return
	}

	// Update positions based on new order
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

	// Update playlist timestamp
	_ = app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistId)

	app.Logger.Info("playlist tracks reordered", "playlist_id", playlistId)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Tracks reordered successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetPlaylistCollaborators returns the collaborators for a playlist (owner only)
func (app *Application) GetPlaylistCollaborators(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Only owner can view collaborators
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

// AddCollaborator adds a collaborator to a playlist (owner only)
func (app *Application) AddCollaborator(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	// Only owner can add collaborators
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

	var req AddCollaboratorRequest
	if err := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	// Can't add yourself as collaborator
	if req.UserId == userID {
		helpers.ErrorJSON(w, errors.New("you cannot add yourself as a collaborator"), http.StatusBadRequest)
		return
	}

	// Verify user exists
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

// RemoveCollaborator removes a collaborator from a playlist (owner only)
func (app *Application) RemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
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

	// Only owner can remove collaborators
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
