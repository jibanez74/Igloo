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
	maxPlaylistRequestSize       = 1024 * 1024 // 1 MB
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
	playlist, err := app.Queries.GetPlaylistById(ctx, playlistID)
	if err != nil {
		return database.Playlist{}, PermissionNone, err
	}

	if playlist.UserID == userID {
		return playlist, PermissionOwner, nil
	}

	canEdit, err := app.Queries.CanUserEditPlaylist(ctx, database.CanUserEditPlaylistParams{
		ID:       playlistID,
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		return database.Playlist{}, PermissionNone, err
	}

	if canEdit {
		return playlist, PermissionEdit, nil
	}

	isCollaborator, err := app.Queries.IsUserCollaborator(ctx, database.IsUserCollaboratorParams{
		PlaylistID: playlistID,
		UserID:     userID,
	})
	if err != nil {
		return database.Playlist{}, PermissionNone, err
	}

	if isCollaborator {
		return playlist, PermissionView, nil
	}

	if playlist.IsPublic {
		return playlist, PermissionView, nil
	}

	return playlist, PermissionNone, nil
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
