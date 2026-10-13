package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
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
	movieID, ok := app.resolvePlaylistMovieID(w, ctx, req.MovieID, sql.NullInt64{})
	if !ok {
		return
	}

	playlist, err := app.Queries.CreateMoviePlaylist(ctx, database.CreateMoviePlaylistParams{
		UserID:      userID,
		Name:        req.Name,
		Description: helpers.NullString(req.Description),
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

	playlistID, ok := parseIDParam(w, r, "id", invalidPlaylistIDMessage)
	if !ok {
		return
	}

	playlist, permission, ok := app.authorizePlaylist(w, r, playlistID, userID, playlistContentTypeMovie, PermissionView, accessDeniedMessage, fetchPlaylistMessage)
	if !ok {
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

	playlistID, ok := parseIDParam(w, r, "id", invalidPlaylistIDMessage)
	if !ok {
		return
	}

	existing, _, ok := app.authorizePlaylist(w, r, playlistID, userID, playlistContentTypeMovie, PermissionOwner, "only the playlist owner can update metadata", updatePlaylistMessage)
	if !ok {
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
	movieID, ok := app.resolvePlaylistMovieID(w, ctx, req.MovieID, existing.MovieID)
	if !ok {
		return
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

func (app *Application) GetMoviePlaylistMovies(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlistID, ok := parseIDParam(w, r, "id", invalidPlaylistIDMessage)
	if !ok {
		return
	}

	_, _, ok = app.authorizePlaylist(w, r, playlistID, userID, playlistContentTypeMovie, PermissionView, accessDeniedMessage, fetchPlaylistMessage)
	if !ok {
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

// resolvePlaylistMovieID answers 400 when the requested movie does not exist.
// A nil request keeps current: the request separates an absent movie_id from a
// present one, which helpers.NullInt64 (testing for 0) cannot express.
func (app *Application) resolvePlaylistMovieID(w http.ResponseWriter, ctx context.Context, requested *int64, current sql.NullInt64) (sql.NullInt64, bool) {
	if requested == nil {
		return current, true
	}

	movieOK, err := app.Queries.MovieExists(ctx, *requested)
	if err != nil {
		app.serverError(w, err, "failed to verify movie for playlist", "failed to verify movie")
		return current, false
	}
	if !movieOK {
		helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusBadRequest)
		return current, false
	}
	return sql.NullInt64{Int64: *requested, Valid: true}, true
}

type AddMoviesRequest struct {
	MovieIds []int64 `json:"movie_ids"`
}

func (app *Application) AddMoviesToMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlistID, ok := parseIDParam(w, r, "id", invalidPlaylistIDMessage)
	if !ok {
		return
	}

	_, _, ok = app.authorizePlaylist(w, r, playlistID, userID, playlistContentTypeMovie, PermissionEdit, "you don't have permission to edit this playlist", "failed to add movies")
	if !ok {
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

	added, skipped, err := app.addToPlaylist(ctx, playlistID, req.MovieIds,
		func(q *database.Queries, id int64) (int64, error) {
			return q.AddMovieToPlaylist(ctx, database.AddMovieToPlaylistParams{
				PlaylistID: playlistID,
				MovieID:    id,
				AddedBy:    helpers.NullInt64(userID),
			})
		},
		(*database.Queries).MovieExists,
	)
	if err != nil {
		app.serverError(w, err, "failed to add movies to playlist", "failed to add movies to playlist", "playlist_id", playlistID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Movies processed",
		Data: map[string]any{
			"added":   added,
			"skipped": skipped,
		},
	})
}

func (app *Application) RemoveMovieFromMoviePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	playlistID, ok := parseIDParam(w, r, "id", invalidPlaylistIDMessage)
	if !ok {
		return
	}

	movieID, ok := parseIDParam(w, r, "movieId", invalidMovieIDMessage)
	if !ok {
		return
	}

	_, _, ok = app.authorizePlaylist(w, r, playlistID, userID, playlistContentTypeMovie, PermissionEdit, "you don't have permission to edit this playlist", "failed to remove movie")
	if !ok {
		return
	}

	err := app.removeFromPlaylist(r.Context(), playlistID, func(qtx *database.Queries) error {
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
