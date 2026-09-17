package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

type PlaylistPermission int

const (
	PermissionNone PlaylistPermission = iota
	PermissionView
	PermissionEdit
	PermissionOwner
)

const (
	playlistContentTypeTrack     = "track"
	playlistContentTypeMovie     = "movie"
	playlistNameMaxLength        = 255
	playlistDescriptionMaxLength = 1000
)

// Said by both the track and the movie playlist handlers.
const (
	playlistNotFoundMessage      = "playlist not found"
	invalidPlaylistIDMessage     = "invalid playlist id"
	playlistPermissionLogMessage = "failed to check playlist permission"
)

func validatePlaylistMetadata(name, description string) error {
	if name == "" {
		return errors.New("playlist name is required")
	}

	nameLength := utf8.RuneCountInString(name)
	if nameLength > playlistNameMaxLength {
		return fmt.Errorf("playlist name is too long (max %d characters)", playlistNameMaxLength)
	}

	descriptionLength := utf8.RuneCountInString(description)
	if descriptionLength > playlistDescriptionMaxLength {
		return fmt.Errorf("description is too long (max %d characters)", playlistDescriptionMaxLength)
	}

	return nil
}

func (app *Application) getPlaylistAccess(ctx context.Context, playlistID, userID int64) (database.Playlist, PlaylistPermission, error) {
	row, err := app.Queries.GetPlaylistWithAccess(ctx, database.GetPlaylistWithAccessParams{
		PlaylistID: playlistID,
		UserID:     userID,
	})
	if err != nil {
		return database.Playlist{}, PermissionNone, err
	}

	playlist := database.Playlist{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.Name,
		Description: row.Description,
		CoverImage:  row.CoverImage,
		IsPublic:    row.IsPublic,
		MovieID:     row.MovieID,
		ContentType: row.ContentType,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	// CollaboratorCanEdit is non-NULL exactly when the user has a collaborator
	// row on this playlist.
	switch {
	case row.UserID == userID:
		return playlist, PermissionOwner, nil
	case row.CollaboratorCanEdit.Valid && row.CollaboratorCanEdit.Bool:
		return playlist, PermissionEdit, nil
	case row.CollaboratorCanEdit.Valid:
		return playlist, PermissionView, nil
	case row.IsPublic:
		return playlist, PermissionView, nil
	default:
		return playlist, PermissionNone, nil
	}
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
