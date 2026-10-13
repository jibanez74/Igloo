package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func (app *Application) GetPlaylists(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlists, err := app.Queries.GetPlaylistsWithCollaboratorAccess(r.Context(), userID)
	if err != nil {
		app.serverError(w, err, "failed to get playlists", "failed to fetch playlists")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"playlists": playlists,
		},
	})
}

func (app *Application) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	playlist, permission, ok := app.authorizePlaylist(w, r, playlistId, userID, playlistContentTypeTrack, PermissionView, accessDeniedMessage, fetchPlaylistMessage)
	if !ok {
		return
	}

	owner, err := app.getPlaylistOwner(r.Context(), playlist.UserID)
	if err != nil {
		app.serverError(w, err, getPlaylistOwnerLogMessage, fetchPlaylistMessage)
		return
	}

	summary, err := app.Queries.GetPlaylistTrackSummary(r.Context(), playlistId)
	if err != nil {
		app.serverError(w, err, "failed to summarize playlist tracks", fetchPlaylistMessage, "playlist_id", playlistId)
		return
	}

	var collaborators []database.GetPlaylistCollaboratorsRow
	if permission == PermissionOwner {
		collaborators, err = app.Queries.GetPlaylistCollaborators(r.Context(), playlistId)
		if err != nil {
			app.serverError(w, err, "failed to get playlist collaborators", fetchPlaylistMessage, "playlist_id", playlistId)
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"playlist":      playlist,
			"track_count":   summary.TrackCount,
			"duration":      summary.TotalDuration,
			"is_owner":      permission == PermissionOwner,
			"can_edit":      permission >= PermissionEdit,
			"owner":         owner,
			"collaborators": collaborators,
		},
	})
}

func (app *Application) GetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, playlistContentTypeTrack, PermissionView, accessDeniedMessage, fetchPlaylistMessage)
	if !ok {
		return
	}

	limit, offset := parseLimitOffsetParams(r, 50, 100)

	tracks, err := app.Queries.GetPlaylistTracksInfinite(r.Context(), database.GetPlaylistTracksInfiniteParams{
		PlaylistID: playlistId,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		app.serverError(w, err, "failed to get playlist tracks", "failed to fetch playlist tracks")
		return
	}

	total, err := app.Queries.CountPlaylistTracks(r.Context(), playlistId)
	if err != nil {
		app.serverError(w, err, "failed to count playlist tracks", "failed to fetch playlist tracks", "playlist_id", playlistId)
		return
	}
	hasMore := offset+int64(len(tracks)) < total

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"tracks":      tracks,
			"total":       total,
			"has_more":    hasMore,
			"next_offset": offset + int64(len(tracks)),
		},
	})
}

type CreatePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

func (app *Application) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req CreatePlaylistRequest
	readErr := helpers.ReadJSON(w, r, &req)
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
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		app.serverError(w, err, "failed to create playlist", createPlaylistMessage)
		return
	}

	app.Logger.Info("playlist created", "id", playlist.ID, "name", playlist.Name, "user_id", userID)

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Message: "Playlist created successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	})
}

type UpdatePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverImage  string `json:"cover_image"`
	IsPublic    bool   `json:"is_public"`
}

func (app *Application) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, playlistContentTypeTrack, PermissionOwner, "only the playlist owner can update metadata", updatePlaylistMessage)
	if !ok {
		return
	}

	var req UpdatePlaylistRequest
	readErr := helpers.ReadJSON(w, r, &req)
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
		app.serverError(w, err, "failed to update playlist", updatePlaylistMessage)
		return
	}

	app.Logger.Info("playlist updated", "id", playlist.ID, "name", playlist.Name)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Playlist updated successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	})
}

func (app *Application) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	app.deletePlaylist(w, r, playlistContentTypeTrack)
}

func (app *Application) DeleteMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	app.deletePlaylist(w, r, playlistContentTypeMovie)
}

func (app *Application) deletePlaylist(w http.ResponseWriter, r *http.Request, contentType string) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	playlist, _, ok := app.authorizePlaylist(w, r, playlistId, userID, contentType, PermissionOwner, "only the playlist owner can delete it", deletePlaylistMessage)
	if !ok {
		return
	}

	err = app.Queries.DeletePlaylist(r.Context(), database.DeletePlaylistParams{
		ID:     playlistId,
		UserID: userID,
	})
	if err != nil {
		app.serverError(w, err, "failed to delete playlist", deletePlaylistMessage)
		return
	}

	app.Logger.Info("playlist deleted", "id", playlistId, "name", playlist.Name)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Playlist deleted successfully"})
}

type AddTracksRequest struct {
	TrackIds []int64 `json:"track_ids"`
}

func (app *Application) AddTracksToPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, playlistContentTypeTrack, PermissionEdit, "you don't have permission to add tracks to this playlist", "failed to add tracks")
	if !ok {
		return
	}

	var req AddTracksRequest
	readErr := helpers.ReadJSON(w, r, &req)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if len(req.TrackIds) == 0 {
		helpers.ErrorJSON(w, errors.New("at least one track id is required"), http.StatusBadRequest)
		return
	}

	added, skipped, err := app.addToPlaylist(r.Context(), playlistId, req.TrackIds,
		func(q *database.Queries, id int64) (int64, error) {
			return q.AddTrackToPlaylist(r.Context(), database.AddTrackToPlaylistParams{
				PlaylistID: playlistId,
				TrackID:    id,
				AddedBy:    helpers.NullInt64(userID),
			})
		},
		(*database.Queries).TrackExists,
	)
	if err != nil {
		app.serverError(w, err, "failed to add tracks to playlist", "failed to add tracks", "playlist_id", playlistId)
		return
	}

	app.Logger.Info("tracks added to playlist", "playlist_id", playlistId, "added", added, "skipped", skipped)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Tracks added successfully",
		Data: map[string]any{
			"added":   added,
			"skipped": skipped,
		},
	})
}

func (app *Application) RemoveTrackFromPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlistIdParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(playlistIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	trackIdParam := chi.URLParam(r, "trackId")
	trackId, err := strconv.ParseInt(trackIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidTrackIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, playlistContentTypeTrack, PermissionEdit, "you don't have permission to remove tracks from this playlist", "failed to remove track")
	if !ok {
		return
	}

	err = app.removeFromPlaylist(r.Context(), playlistId, func(qtx *database.Queries) error {
		return qtx.RemoveTrackFromPlaylist(r.Context(), database.RemoveTrackFromPlaylistParams{
			PlaylistID: playlistId,
			TrackID:    trackId,
		})
	})
	if err != nil {
		app.serverError(w, err, "failed to remove track from playlist", "failed to remove track",
			"playlist_id", playlistId, "track_id", trackId)
		return
	}

	app.Logger.Info("track removed from playlist", "playlist_id", playlistId, "track_id", trackId)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Track removed successfully"})
}

type ReorderTracksRequest struct {
	TrackIds []int64 `json:"track_ids"`
}

func (app *Application) ReorderPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, playlistContentTypeTrack, PermissionEdit, "you don't have permission to reorder tracks in this playlist", "failed to reorder tracks")
	if !ok {
		return
	}

	var req ReorderTracksRequest
	readErr := helpers.ReadJSON(w, r, &req)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if len(req.TrackIds) == 0 {
		helpers.ErrorJSON(w, errors.New("track ids are required"), http.StatusBadRequest)
		return
	}

	// One transaction for the whole reorder, so a long playlist costs one fsync
	// and a failed update leaves no partial order behind. An id that is not in
	// the playlist updates no row, which is not an error.
	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err, "failed to begin reorder transaction", "failed to reorder tracks", "playlist_id", playlistId)
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)
	for i, trackId := range req.TrackIds {
		err := qtx.UpdateTrackPosition(r.Context(), database.UpdateTrackPositionParams{
			Position:   int64(i),
			PlaylistID: playlistId,
			TrackID:    trackId,
		})
		if err != nil {
			app.serverError(w, err, "failed to update track position", "failed to reorder tracks",
				"track_id", trackId, "position", i)
			return
		}
	}

	timestampErr := qtx.UpdatePlaylistTimestamp(r.Context(), playlistId)
	if timestampErr != nil {
		app.serverError(w, timestampErr, "failed to update playlist timestamp", "failed to reorder tracks", "playlist_id", playlistId)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverError(w, err, "failed to commit reorder transaction", "failed to reorder tracks", "playlist_id", playlistId)
		return
	}

	app.Logger.Info("playlist tracks reordered", "playlist_id", playlistId)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Tracks reordered successfully"})
}

func (app *Application) GetPlaylistCollaborators(w http.ResponseWriter, r *http.Request) {
	app.getPlaylistCollaborators(w, r, playlistContentTypeTrack)
}

func (app *Application) GetMoviePlaylistCollaborators(w http.ResponseWriter, r *http.Request) {
	app.getPlaylistCollaborators(w, r, playlistContentTypeMovie)
}

func (app *Application) getPlaylistCollaborators(
	w http.ResponseWriter,
	r *http.Request,
	contentType string,
) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, contentType, PermissionOwner, "only the playlist owner can view collaborators", "failed to fetch collaborators")
	if !ok {
		return
	}

	collaborators, err := app.Queries.GetPlaylistCollaborators(r.Context(), playlistId)
	if err != nil {
		app.serverError(w, err, "failed to get collaborators", "failed to fetch collaborators")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"collaborators": collaborators,
		},
	})
}

func (app *Application) AddCollaborator(w http.ResponseWriter, r *http.Request) {
	app.addCollaborator(w, r, playlistContentTypeTrack)
}

func (app *Application) AddMoviePlaylistCollaborator(w http.ResponseWriter, r *http.Request) {
	app.addCollaborator(w, r, playlistContentTypeMovie)
}

type AddCollaboratorRequest struct {
	UserId  int64 `json:"user_id"`
	CanEdit bool  `json:"can_edit"`
}

func (app *Application) addCollaborator(
	w http.ResponseWriter,
	r *http.Request,
	contentType string,
) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, contentType, PermissionOwner, "only the playlist owner can add collaborators", "failed to add collaborator")
	if !ok {
		return
	}

	var req AddCollaboratorRequest
	readErr := helpers.ReadJSON(w, r, &req)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	// A missing user_id decodes to 0; it is a bad request, not an unknown user.
	if req.UserId < 1 {
		helpers.ErrorJSON(w, errors.New("user_id is required"), http.StatusBadRequest)
		return
	}

	if req.UserId == userID {
		helpers.ErrorJSON(w, errors.New("you cannot add yourself as a collaborator"), http.StatusBadRequest)
		return
	}

	userOK, err := app.Queries.UserExists(r.Context(), req.UserId)
	if err != nil {
		app.serverError(w, err, "failed to verify user", "failed to add collaborator")
		return
	}
	if !userOK {
		helpers.ErrorJSON(w, errors.New(userNotFoundMessage), http.StatusNotFound)
		return
	}

	collaborator, err := app.Queries.AddCollaborator(r.Context(), database.AddCollaboratorParams{
		PlaylistID: playlistId,
		UserID:     req.UserId,
		CanEdit:    req.CanEdit,
	})
	if err != nil {
		if strings.Contains(err.Error(), uniqueConstraintErrorFragment) {
			helpers.ErrorJSON(w, errors.New("that user is already a collaborator on this playlist"), http.StatusConflict)
			return
		}
		app.serverError(w, err, "failed to add collaborator", "failed to add collaborator")
		return
	}

	app.Logger.Info("collaborator added", "playlist_id", playlistId, "user_id", req.UserId)

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Message: "Collaborator added successfully",
		Data: map[string]any{
			"collaborator": collaborator,
		},
	})
}

func (app *Application) RemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	app.removeCollaborator(w, r, playlistContentTypeTrack)
}

func (app *Application) RemoveMoviePlaylistCollaborator(w http.ResponseWriter, r *http.Request) {
	app.removeCollaborator(w, r, playlistContentTypeMovie)
}

func (app *Application) removeCollaborator(
	w http.ResponseWriter,
	r *http.Request,
	contentType string,
) {
	userID := app.userIDFromRequest(r)

	playlistIdParam := chi.URLParam(r, "id")
	playlistId, err := strconv.ParseInt(playlistIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	collaboratorIdParam := chi.URLParam(r, "userId")
	collaboratorId, err := strconv.ParseInt(collaboratorIdParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid user id"), http.StatusBadRequest)
		return
	}

	_, _, ok := app.authorizePlaylist(w, r, playlistId, userID, contentType, PermissionOwner, "only the playlist owner can remove collaborators", "failed to remove collaborator")
	if !ok {
		return
	}

	err = app.Queries.RemoveCollaborator(r.Context(), database.RemoveCollaboratorParams{
		PlaylistID: playlistId,
		UserID:     collaboratorId,
	})
	if err != nil {
		app.serverError(w, err, "failed to remove collaborator", "failed to remove collaborator")
		return
	}

	app.Logger.Info("collaborator removed", "playlist_id", playlistId, "user_id", collaboratorId)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Collaborator removed successfully"})
}
