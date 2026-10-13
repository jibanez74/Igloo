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

func (app *Application) GetMoviePlaylists(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlists, err := app.Queries.GetMoviePlaylistsWithCollaboratorAccess(r.Context(), userID)
	if err != nil {
		app.serverError(w, err, "failed to get movie playlists", "failed to fetch playlists")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"playlists": playlists,
		},
	})
}

type CreateMoviePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	MovieID     *int64 `json:"movie_id"`
}

func (app *Application) CreateMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req CreateMoviePlaylistRequest
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

	ctx := r.Context()
	var movieID sql.NullInt64
	if req.MovieID != nil {
		movieOK, err := app.Queries.MovieExists(ctx, *req.MovieID)
		if err != nil {
			app.serverError(w, err, "failed to verify movie for playlist", "failed to verify movie")
			return
		}
		if !movieOK {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusBadRequest)
			return
		}
		// The request separates an absent movie_id from a present one, which is
		// what the nil check above tests. helpers.NullInt64 tests for 0 instead
		// and cannot express that distinction.
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
		app.serverError(w, err, "failed to create movie playlist", createPlaylistMessage)
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Message: "Playlist created successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	})
}

func (app *Application) GetMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, playlistPermissionLogMessage, fetchPlaylistMessage)
		return
	}

	if permission == PermissionNone {
		helpers.ErrorJSON(w, errors.New(accessDeniedMessage), http.StatusForbidden)
		return
	}

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	owner, err := app.getPlaylistOwner(r.Context(), playlist.UserID)
	if err != nil {
		app.serverError(w, err, getPlaylistOwnerLogMessage, fetchPlaylistMessage)
		return
	}

	movieCount, err := app.Queries.CountPlaylistMovies(r.Context(), playlistID)
	if err != nil {
		app.serverError(w, err, "failed to count playlist movies", fetchPlaylistMessage, "playlist_id", playlistID)
		return
	}

	var collaborators []database.GetPlaylistCollaboratorsRow
	if permission == PermissionOwner {
		collaborators, err = app.Queries.GetPlaylistCollaborators(r.Context(), playlistID)
		if err != nil {
			app.serverError(w, err, "failed to get playlist collaborators", fetchPlaylistMessage, "playlist_id", playlistID)
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"playlist":      playlist,
			"movie_count":   movieCount,
			"is_owner":      permission == PermissionOwner,
			"can_edit":      permission >= PermissionEdit,
			"owner":         owner,
			"collaborators": collaborators,
		},
	})
}

type UpdateMoviePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverImage  string `json:"cover_image"`
	IsPublic    bool   `json:"is_public"`
	MovieID     *int64 `json:"movie_id"`
}

func (app *Application) UpdateMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	existing, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, playlistPermissionLogMessage, updatePlaylistMessage)
		return
	}

	if permission != PermissionOwner {
		helpers.ErrorJSON(w, errors.New("only the playlist owner can update metadata"), http.StatusForbidden)
		return
	}

	if !app.mustBeMoviePlaylist(w, existing) {
		return
	}

	var req UpdateMoviePlaylistRequest
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

	ctx := r.Context()
	var movieID sql.NullInt64
	if req.MovieID != nil {
		movieOK, err := app.Queries.MovieExists(ctx, *req.MovieID)
		if err != nil {
			app.serverError(w, err, "failed to verify movie for playlist", "failed to verify movie")
			return
		}
		if !movieOK {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusBadRequest)
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
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "failed to update movie playlist", updatePlaylistMessage)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Playlist updated successfully",
		Data: map[string]any{
			"playlist": playlist,
		},
	})
}

func (app *Application) DeleteMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, getPlaylistLogMessage, deletePlaylistMessage)
		return
	}

	if permission != PermissionOwner {
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
		app.serverError(w, err, "failed to delete movie playlist", deletePlaylistMessage)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Playlist deleted successfully"})
}

func (app *Application) GetMoviePlaylistMovies(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, playlistPermissionLogMessage, fetchPlaylistMessage)
		return
	}

	if permission == PermissionNone {
		helpers.ErrorJSON(w, errors.New(accessDeniedMessage), http.StatusForbidden)
		return
	}

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	page, perPage, sortParam := parseLibraryQuery(r)
	ctx := r.Context()

	total, err := app.Queries.CountPlaylistMovies(ctx, playlistID)
	if err != nil {
		app.serverError(w, err, "failed to count playlist movies", "failed to fetch playlist items")
		return
	}

	params := database.GetPlaylistMoviesPaginatedAscParams{PlaylistID: playlistID, Limit: perPage, Offset: (page - 1) * perPage}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		var rows []database.GetPlaylistMoviesPaginatedDescRow
		rows, err = app.Queries.GetPlaylistMoviesPaginatedDesc(ctx, database.GetPlaylistMoviesPaginatedDescParams(params))
		movies = movieLibraryRows(rows)
	} else {
		var rows []database.GetPlaylistMoviesPaginatedAscRow
		rows, err = app.Queries.GetPlaylistMoviesPaginatedAsc(ctx, params)
		movies = movieLibraryRows(rows)
	}
	if err != nil {
		app.serverError(w, err, "failed to get playlist movies", "failed to fetch playlist movies")
		return
	}

	writeMoviesLibrary(w, movies, total, page, perPage, sortParam)
}

type AddMoviesRequest struct {
	MovieIds []int64 `json:"movie_ids"`
}

func (app *Application) AddMoviesToMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, playlistPermissionLogMessage, "failed to add movies")
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to edit this playlist"), http.StatusForbidden)
		return
	}

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	var req AddMoviesRequest
	readErr := helpers.ReadJSON(w, r, &req)
	if readErr != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if len(req.MovieIds) == 0 {
		helpers.ErrorJSON(w, errors.New("at least one movie id is required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// One transaction for the whole batch. ON CONFLICT DO NOTHING replaces the
	// per-movie membership pre-check, and the movie_id foreign key replaces the
	// per-movie existence lookup -- an unknown id surfaces as an FK violation
	// and is counted as skipped, exactly as before.
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.serverError(w, err, "failed to begin add-movies transaction", "failed to add movies to playlist",
			"playlist_id", playlistID)
		return
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)

	addedCount := 0
	skippedCount := 0

	for _, movieID := range req.MovieIds {
		rowsAffected, err := qtx.AddMovieToPlaylist(ctx, database.AddMovieToPlaylistParams{
			PlaylistID: playlistID,
			MovieID:    movieID,
			AddedBy:    sql.NullInt64{Int64: userID, Valid: true},
		})
		if err != nil {
			// An unknown movie id fails the movie_id foreign key. Confirm that is
			// what happened instead of matching the driver's error text:
			// playlist_movies also has foreign keys on playlist_id and added_by, so
			// a playlist or user deleted mid-request must surface as an error
			// rather than a silent skip. A constraint violation aborts only the
			// failed statement, so the transaction is still usable for this probe.
			movieOK, existsErr := qtx.MovieExists(ctx, movieID)
			if existsErr == nil && !movieOK {
				skippedCount++
				app.Logger.Warn("skip unknown movie id for playlist", "movie_id", movieID)
				continue
			}
			app.serverError(w, err, "failed to add movie to playlist", "failed to add movies to playlist",
				"movie_id", movieID, "playlist_id", playlistID)
			return
		}
		if rowsAffected == 0 {
			skippedCount++
			continue
		}
		addedCount++
	}

	if addedCount > 0 {
		timestampErr := qtx.UpdatePlaylistTimestamp(ctx, playlistID)
		if timestampErr != nil {
			app.serverError(w, timestampErr, updatePlaylistTimestampLogMessage, finalizePlaylistUpdateMessage,
				"playlist_id", playlistID)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		app.serverError(w, err, "failed to commit add-movies transaction", "failed to add movies to playlist",
			"playlist_id", playlistID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Movies processed",
		Data: map[string]any{
			"added":   addedCount,
			"skipped": skippedCount,
		},
	})
}

func (app *Application) RemoveMovieFromMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlistID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidPlaylistIDMessage), http.StatusBadRequest)
		return
	}

	movieID, err := strconv.ParseInt(chi.URLParam(r, "movieId"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidMovieIDMessage), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(playlistNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, playlistPermissionLogMessage, "failed to remove movie")
		return
	}

	if permission < PermissionEdit {
		helpers.ErrorJSON(w, errors.New("you don't have permission to edit this playlist"), http.StatusForbidden)
		return
	}

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	err = app.removeFromPlaylist(r.Context(), playlistID, func(qtx *database.Queries) error {
		return qtx.RemoveMovieFromPlaylist(r.Context(), database.RemoveMovieFromPlaylistParams{
			PlaylistID: playlistID,
			MovieID:    movieID,
		})
	})
	if err != nil {
		app.serverError(w, err, "failed to remove movie from playlist", "failed to remove movie",
			"playlist_id", playlistID, "movie_id", movieID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Movie removed from playlist"})
}
