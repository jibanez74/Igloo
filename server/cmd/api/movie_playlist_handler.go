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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	playlists, err := app.Queries.GetMoviePlaylistsWithCollaboratorAccess(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get movie playlists", "error", err)
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

func (app *Application) CreateMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	var req CreateMoviePlaylistRequest
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

	ctx := r.Context()
	var movieID sql.NullInt64
	if req.MovieID != nil {
		movieOK, err := app.Queries.MovieExists(ctx, *req.MovieID)
		if err != nil {
			app.Logger.Error("failed to verify movie for playlist", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to verify movie"))
			return
		}
		if !movieOK {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusBadRequest)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	existing, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
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

	if !app.mustBeMoviePlaylist(w, existing) {
		return
	}

	var req UpdateMoviePlaylistRequest
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

	ctx := r.Context()
	var movieID sql.NullInt64
	if req.MovieID != nil {
		movieOK, err := app.Queries.MovieExists(ctx, *req.MovieID)
		if err != nil {
			app.Logger.Error("failed to verify movie for playlist", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to verify movie"))
			return
		}
		if !movieOK {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusBadRequest)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
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

	if !app.mustBeMoviePlaylist(w, playlist) {
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid playlist id"), http.StatusBadRequest)
		return
	}

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
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

	if !app.mustBeMoviePlaylist(w, playlist) {
		return
	}

	var req AddMoviesRequest
	readErr := helpers.ReadJSON(w, r, &req, maxPlaylistRequestSize)
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
		app.Logger.Error("failed to begin add-movies transaction", "error", err, "playlist_id", playlistID)
		helpers.ErrorJSON(w, errors.New("failed to add movies to playlist"), http.StatusInternalServerError)
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
			app.Logger.Error("failed to add movie to playlist", "error", err, "movie_id", movieID, "playlist_id", playlistID)
			helpers.ErrorJSON(w, errors.New("failed to add movies to playlist"), http.StatusInternalServerError)
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
			app.Logger.Error("failed to update playlist timestamp", "error", timestampErr, "playlist_id", playlistID)
			helpers.ErrorJSON(w, errors.New("failed to finalize playlist update"), http.StatusInternalServerError)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		app.Logger.Error("failed to commit add-movies transaction", "error", err, "playlist_id", playlistID)
		helpers.ErrorJSON(w, errors.New("failed to add movies to playlist"), http.StatusInternalServerError)
		return
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
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

	playlist, permission, err := app.getPlaylistAccess(r.Context(), playlistID, userID)
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

	if !app.mustBeMoviePlaylist(w, playlist) {
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

	timestampErr := app.Queries.UpdatePlaylistTimestamp(r.Context(), playlistID)
	if timestampErr != nil {
		app.Logger.Error("failed to update playlist timestamp", "error", timestampErr, "playlist_id", playlistID)
		helpers.ErrorJSON(w, errors.New("failed to finalize playlist update"), http.StatusInternalServerError)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Movie removed from playlist",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
