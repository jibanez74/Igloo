package main

import (
	"context"
	"database/sql"
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
	playlistNotFoundMessage    = "playlist not found"
	invalidPlaylistIDMessage   = "invalid playlist id"
	getPlaylistOwnerLogMessage = "failed to get playlist owner"

	fetchPlaylistMessage  = "failed to fetch playlist"
	createPlaylistMessage = "failed to create playlist"
	updatePlaylistMessage = "failed to update playlist"
	deletePlaylistMessage = "failed to delete playlist"
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
	case row.CollaboratorCanEdit.Valid || row.IsPublic:
		return playlist, PermissionView, nil
	default:
		return playlist, PermissionNone, nil
	}
}

// authorizePlaylist loads a playlist for the user and answers the request itself
// when the handler cannot go on: 404 for no playlist, 500 with failMessage for a
// failed lookup, 403 with deniedMessage below required, and 400 for a playlist
// of another content type.
func (app *Application) authorizePlaylist(
	w http.ResponseWriter,
	r *http.Request,
	playlistID, userID int64,
	contentType string,
	required PlaylistPermission,
	deniedMessage, failMessage string,
) (database.Playlist, PlaylistPermission, bool) {
	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
		return playlist, permission, false
	}
	if err != nil {
		app.serverError(w, err, "failed to check playlist permission", failMessage, "playlist_id", playlistID)
		return playlist, permission, false
	}

	if permission < required {
		helpers.ErrorJSON(w, errors.New(deniedMessage), http.StatusForbidden)
		return playlist, permission, false
	}

	if playlist.ContentType != contentType {
		helpers.ErrorJSON(w, fmt.Errorf("not a %s playlist", contentType), http.StatusBadRequest)
		return playlist, permission, false
	}

	return playlist, permission, true
}

// addToPlaylist inserts every id in one transaction. ON CONFLICT DO NOTHING
// replaces a membership pre-check, so no affected row means already present.
// An id whose insert fails and that exists reports missing is skipped as
// unknown; any other failure aborts the batch. Probing existence instead of
// matching the driver's error text keeps a playlist or user deleted mid-request
// (the other foreign keys) an error, and SQLite aborts only the failed
// statement, so the probe can run in the same transaction.
func (app *Application) addToPlaylist(
	ctx context.Context,
	playlistID int64,
	ids []int64,
	insert func(q *database.Queries, id int64) (int64, error),
	exists func(q *database.Queries, ctx context.Context, id int64) (bool, error),
) (added, skipped int, err error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)
	for _, id := range ids {
		rowsAffected, insertErr := insert(qtx, id)
		if insertErr != nil {
			found, existsErr := exists(qtx, ctx, id)
			if existsErr == nil && !found {
				skipped++
				app.Logger.Warn("skip unknown id for playlist", "playlist_id", playlistID, "id", id)
				continue
			}
			return 0, 0, fmt.Errorf("add %d: %w", id, insertErr)
		}
		if rowsAffected == 0 {
			skipped++
			continue
		}
		added++
	}

	if added > 0 {
		err = qtx.UpdatePlaylistTimestamp(ctx, playlistID)
		if err != nil {
			return 0, 0, err
		}
	}
	return added, skipped, tx.Commit()
}

// getPlaylistOwner is the detail endpoints' owner lookup: a lean query rather
// than a users JOIN in GetPlaylistWithAccess, which every playlist mutation
// runs as its permission check.
func (app *Application) getPlaylistOwner(ctx context.Context, ownerID int64) (userSummary, error) {
	row, err := app.Queries.GetUserSummary(ctx, ownerID)
	if err != nil {
		return userSummary{}, err
	}

	owner := userSummary{
		ID:     row.ID,
		Name:   row.Name,
		Avatar: helpers.StringPtrFromNull(row.Avatar),
	}

	return owner, nil
}

// removeFromPlaylist deletes one item and moves the playlist's updated_at in
// one transaction, so a failed timestamp never answers 500 for an item that is
// already gone.
func (app *Application) removeFromPlaylist(ctx context.Context, playlistID int64, remove func(*database.Queries) error) error {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)
	err = remove(qtx)
	if err != nil {
		return err
	}
	err = qtx.UpdatePlaylistTimestamp(ctx, playlistID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
