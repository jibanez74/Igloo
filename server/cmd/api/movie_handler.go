package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// GetLatestMovies returns the 12 most recently added movies from the database.
// Response includes id, title, poster (full URL when available), and year.
func (app *Application) GetLatestMovies(w http.ResponseWriter, r *http.Request) {
	rows, err := app.Queries.GetLatestMovies(r.Context())
	if err != nil {
		app.Logger.Error("failed to get latest movies", "error", err)
		helpers.ErrorJSON(w, err)
		return
	}

	movies := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		var poster any
		if row.PosterPath.Valid && row.PosterPath.String != "" {
			poster = helpers.TmdbImageURL(row.PosterPath.String, helpers.TMDB_POSTER_SIZE)
		} else {
			poster = nil
		}

		var year any
		if row.Year.Valid {
			year = row.Year.Int64
		} else {
			year = nil
		}

		movies = append(movies, map[string]any{
			"id":     row.ID,
			"title":  row.Title,
			"poster": poster,
			"year":   year,
		})
	}

	// Ensure we return [] not null when empty
	if movies == nil {
		movies = []map[string]any{}
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"movies": movies},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMovieDetails returns a movie with all related data (cast, crew, genres, production companies, extra videos).
// Uses a read-only transaction so all data is from a single consistent snapshot.
func (app *Application) GetMovieDetails(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.Logger.Error("failed to begin transaction", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movie, err := qtx.GetMovieByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get movie", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}

	cast, err := qtx.GetCastByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get cast for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie cast from server"))
		return
	}

	crew, err := qtx.GetCrewByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get crew for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie crew from server"))
		return
	}

	genres, err := qtx.GetGenresByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get genres for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie genres from server"))
		return
	}

	productionCompanies, err := qtx.GetProductionCompaniesByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get production companies for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie production companies from server"))
		return
	}

	extraVideos, err := qtx.GetMovieExtraVideos(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get extra videos for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie extra videos from server"))
		return
	}

	// Build movie response with poster as full URL
	movieData := movieDetailsMovieToMap(movie)

	// Build cast with artist_profile as full URL
	castData := make([]map[string]any, 0, len(cast))
	for _, c := range cast {
		profile := any(nil)
		if c.ArtistProfile.Valid && c.ArtistProfile.String != "" {
			profile = helpers.TmdbImageURL(c.ArtistProfile.String, helpers.TMDB_PROFILE_SIZE)
		}
		castData = append(castData, map[string]any{
			"id":             c.ID,
			"movie_id":       c.MovieID,
			"artist_id":      c.ArtistID,
			"character":      c.Character,
			"cast_order":     c.CastOrder,
			"artist_name":    c.ArtistName,
			"artist_profile": profile,
		})
	}

	// Build crew with artist_profile as full URL
	crewData := make([]map[string]any, 0, len(crew))
	for _, c := range crew {
		profile := any(nil)
		if c.ArtistProfile.Valid && c.ArtistProfile.String != "" {
			profile = helpers.TmdbImageURL(c.ArtistProfile.String, helpers.TMDB_PROFILE_SIZE)
		}
		crewData = append(crewData, map[string]any{
			"id":             c.ID,
			"movie_id":       c.MovieID,
			"artist_id":      c.ArtistID,
			"job":            c.Job,
			"department":     c.Department,
			"artist_name":    c.ArtistName,
			"artist_profile": profile,
		})
	}

	// Build genres (id, tag)
	genresData := make([]map[string]any, 0, len(genres))
	for _, g := range genres {
		genresData = append(genresData, map[string]any{"id": g.ID, "tag": g.Tag})
	}

	// Build production companies with logo as full URL
	companiesData := make([]map[string]any, 0, len(productionCompanies))
	for _, pc := range productionCompanies {
		logo := any(nil)
		if pc.Logo.Valid && pc.Logo.String != "" {
			logo = helpers.TmdbImageURL(pc.Logo.String, helpers.TMDB_LOGO_SIZE)
		}
		country := any(nil)
		if pc.Country.Valid && pc.Country.String != "" {
			country = pc.Country.String
		}
		companiesData = append(companiesData, map[string]any{
			"id":      pc.ID,
			"name":    pc.Name,
			"tmdb_id": pc.TmdbID,
			"logo":    logo,
			"country": country,
		})
	}

	// Build extra videos (trailers, featurettes) for playback
	extraVideosData := make([]map[string]any, 0, len(extraVideos))
	for _, v := range extraVideos {
		externalID := any(nil)
		if v.ExternalID.Valid {
			externalID = v.ExternalID.String
		}
		extraVideosData = append(extraVideosData, map[string]any{
			"id":          v.ID,
			"title":       v.Title,
			"external_id": externalID,
			"key":         v.Key,
			"type":        v.Type,
			"site":        v.Site,
			"official":    v.Official,
		})
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie":                movieData,
			"cast":                 castData,
			"crew":                 crewData,
			"genres":               genresData,
			"production_companies": companiesData,
			"extra_videos":         extraVideosData,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

// movieDetailsMovieToMap converts a Movie row to a response map with poster as full URL and snake_case keys.
func movieDetailsMovieToMap(m database.Movie) map[string]any {
	poster := any(nil)
	if m.PosterPath.Valid && m.PosterPath.String != "" {
		poster = helpers.TmdbImageURL(m.PosterPath.String, helpers.TMDB_POSTER_SIZE)
	}
	year := any(nil)
	if m.Year.Valid {
		year = m.Year.Int64
	}
	releaseDate := any(nil)
	if m.ReleaseDate.Valid {
		releaseDate = m.ReleaseDate.String
	}
	overview := any(nil)
	if m.Overview.Valid {
		overview = m.Overview.String
	}
	tagLine := any(nil)
	if m.TagLine.Valid {
		tagLine = m.TagLine.String
	}
	certification := any(nil)
	if m.Certification.Valid {
		certification = m.Certification.String
	}
	criticRating := any(nil)
	if m.CriticRating.Valid {
		criticRating = m.CriticRating.Float64
	}
	audienceRating := any(nil)
	if m.AudienceRating.Valid {
		audienceRating = m.AudienceRating.Float64
	}
	revenue := any(nil)
	if m.Revenue.Valid {
		revenue = m.Revenue.Float64
	}
	budget := any(nil)
	if m.Budget.Valid {
		budget = m.Budget.Float64
	}
	runTime := any(nil)
	if m.RunTime.Valid {
		runTime = m.RunTime.Int64
	}
	tmdbID := any(nil)
	if m.TmdbID.Valid {
		tmdbID = m.TmdbID.Int64
	}
	imdbID := any(nil)
	if m.ImdbID.Valid {
		imdbID = m.ImdbID.String
	}
	language := any(nil)
	if m.Language.Valid {
		language = m.Language.String
	}

	return map[string]any{
		"id":              m.ID,
		"title":           m.Title,
		"file_path":       m.FilePath,
		"file_name":       m.FileName,
		"size":            m.Size,
		"container":       m.Container,
		"adult":           m.Adult,
		"tmdb_id":         tmdbID,
		"imdb_id":         imdbID,
		"poster":          poster,
		"language":        language,
		"year":            year,
		"release_date":    releaseDate,
		"overview":        overview,
		"tag_line":        tagLine,
		"certification":   certification,
		"critic_rating":   criticRating,
		"audience_rating": audienceRating,
		"revenue":         revenue,
		"budget":          budget,
		"run_time":        runTime,
		"created_at":      m.CreatedAt,
		"updated_at":      m.UpdatedAt,
	}
}

// StreamMovie streams the movie file for playback (direct stream, no transcoding).
func (app *Application) StreamMovie(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	movie, err := app.Queries.GetMovieByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get movie for streaming", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}

	file, err := os.Open(movie.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			app.Logger.Error("movie file not found on disk", "path", movie.FilePath, "id", id)
			helpers.ErrorJSON(w, errors.New("movie file not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to open movie file", "error", err, "path", movie.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to open movie file"))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		app.Logger.Error("failed to stat movie file", "error", err, "path", movie.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to read movie file"))
		return
	}

	contentType := movie.MimeType
	if contentType == "" {
		ext := filepath.Ext(movie.FileName)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)

	http.ServeContent(w, r, movie.FileName, stat.ModTime(), file)
}
