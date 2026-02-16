package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
)

// processMovieFile extracts metadata from a movie file and upserts it into the database.
// Handles TMDB lookup, FFPROBE extraction, and related entities (cast, crew, genres, etc.).
func (app *Application) processMovieFile(ctx context.Context, qtx *database.Queries, path, ext string, fileSize int64, cache *movieScannerCache) error {
	// Step 1: Extract title and year from filename
	titleYear, err := helpers.GetTitleAndYearFromFileName(filepath.Base(path))
	if err != nil {
		// Fallback: use filename without extension as title, year = 0
		baseName := filepath.Base(path)
		ext := filepath.Ext(baseName)
		titleYear = &helpers.TitleYearResponse{
			Title: strings.TrimSuffix(baseName, ext),
			Year:  0,
		}
	}

	// Step 2: TMDB Search (if TMDB is configured)
	var tmdbMovie *tmdb.TmdbMovie

	if app.Tmdb != nil {
		searchResults, err := app.Tmdb.SearchMoviesByTitleAndYear(titleYear.Title, titleYear.Year)
		if err == nil && len(searchResults) > 0 {
			bestMatch := selectBestTmdbMatch(searchResults, titleYear.Year)
			if bestMatch == nil {
				// No year match: use first result only if title is a plausible match (avoid wrong film)
				first := &searchResults[0]
				if titleMatchConfidence(titleYear.Title, first.Title) {
					bestMatch = first
				}
			}

			if bestMatch != nil {
				err = app.Tmdb.GetTmdbMovieByID(bestMatch)
				if err == nil {
					tmdbMovie = bestMatch
				}
			}
		}
	}

	// Step 4: FFPROBE Metadata Extraction (required)
	info, err := app.Ffprobe.GetMetadata(path)
	if err != nil {
		return fmt.Errorf("ffprobe failed (required): %w", err)
	}

	// Step 5: Build movie parameters
	mimeType := mime.TypeByExtension("." + ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	params := database.UpsertMovieParams{
		Title:     titleYear.Title,
		FilePath:  path,
		FileName:  filepath.Base(path),
		Container: ext,
		MimeType:  mimeType,
		Adult:     false, // Default to false, will be set from TMDB if available
	}

	// Parse size from FFPROBE, fallback to fileSize from directory walk
	params.Size = fileSize
	if info.Format.Size != "" {
		size, err := strconv.ParseInt(info.Format.Size, 10, 64)
		if err == nil && size > 0 {
			params.Size = size
		}
	}

	// Map TMDB data if available
	if tmdbMovie != nil {
		params.TmdbID = helpers.NullInt64(int64(tmdbMovie.TmdbID))
		params.ImdbID = helpers.NullString(tmdbMovie.ImdbID)
		params.PosterPath = helpers.NullString(tmdbMovie.PosterPath)
		params.BackdropPath = helpers.NullString(tmdbMovie.BackdropPath)
		params.Title = tmdbMovie.Title
		params.Adult = tmdbMovie.Adult
		params.Language = helpers.NullString(tmdbMovie.OriginalLang)
		params.Overview = helpers.NullString(tmdbMovie.Overview)
		params.TagLine = helpers.NullString(tmdbMovie.Tagline)
		params.Certification = helpers.NullString(tmdbMovie.Certification())
		params.CriticRating = helpers.NullFloat64(tmdbMovie.VoteAverage)
		params.Revenue = helpers.NullFloat64(float64(tmdbMovie.Revenue))
		params.Budget = helpers.NullFloat64(float64(tmdbMovie.Budget))
		params.RunTime = helpers.NullInt64(int64(tmdbMovie.Runtime))

		// Parse release date
		if tmdbMovie.ReleaseDate != "" {
			params.ReleaseDate = helpers.NullString(tmdbMovie.ReleaseDate)
			// Extract year from release date
			if year := extractYearFromReleaseDate(tmdbMovie.ReleaseDate); year > 0 {
				params.Year = helpers.NullInt64(int64(year))
			}
		}
	} else {
		// Use year from filename if TMDB not available
		if titleYear.Year > 0 {
			params.Year = helpers.NullInt64(int64(titleYear.Year))
		}
	}

	// Step 6: Upsert movie
	movie, err := qtx.UpsertMovie(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert movie failed: %w", err)
	}

	// Step 7: Process related entities (only if TMDB data available)
	if tmdbMovie != nil {
		// Process production companies
		if err := app.processProductionCompanies(ctx, qtx, movie.ID, tmdbMovie.ProductionCompanies, cache); err != nil {
			return fmt.Errorf("process production companies failed: %w", err)
		}

		// Process cast
		if err := app.processCast(ctx, qtx, movie.ID, tmdbMovie.Credits.Cast, cache); err != nil {
			return fmt.Errorf("process cast failed: %w", err)
		}

		// Process crew
		if err := app.processCrew(ctx, qtx, movie.ID, tmdbMovie.Credits.Crew, cache); err != nil {
			return fmt.Errorf("process crew failed: %w", err)
		}

		// Process genres
		if err := app.processMovieGenres(ctx, qtx, movie.ID, tmdbMovie.Genres); err != nil {
			return fmt.Errorf("process genres failed: %w", err)
		}

		// Process extra videos (trailers, special features)
		if err := app.processExtraVideos(ctx, qtx, movie.ID, tmdbMovie.Videos.Results); err != nil {
			return fmt.Errorf("process extra videos failed: %w", err)
		}
	}

	// Step 8: Process streams and chapters (from FFPROBE)
	// Video streams are required - if none found, skip movie (invalid file)
	videoStreamCount, err := app.processMovieStreams(ctx, qtx, movie.ID, info.Streams)
	if err != nil {
		return fmt.Errorf("process movie streams failed: %w", err)
	}
	if videoStreamCount == 0 {
		return fmt.Errorf("no video stream found - invalid movie file")
	}

	if err := app.processChapters(ctx, qtx, movie.ID, info.Chapters); err != nil {
		return fmt.Errorf("process chapters failed: %w", err)
	}

	return nil
}

// titleMatchConfidence returns true if the search title (from filename) plausibly
// matches the TMDB movie title (e.g. one contains the other after normalizing),
// to avoid assigning the wrong film when falling back to "first result".
func titleMatchConfidence(searchTitle, movieTitle string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		var b strings.Builder
		for _, r := range s {
			if r == ' ' || r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				if r == ' ' || r == '.' || r == '-' {
					b.WriteRune(' ')
				} else {
					b.WriteRune(r)
				}
			}
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	s, m := norm(searchTitle), norm(movieTitle)
	if len(s) < 2 || len(m) < 2 {
		return true
	}
	return strings.Contains(s, m) || strings.Contains(m, s)
}

// selectBestTmdbMatch selects the best matching movie from TMDB search results when
// at least one result has an exact year match. Priority: exact year match → highest
// popularity → highest vote average. Returns nil when targetYear > 0 but no result
// matches the year (caller can fall back to first result).
func selectBestTmdbMatch(results []tmdb.TmdbMovie, targetYear int) *tmdb.TmdbMovie {
	if len(results) == 0 {
		return nil
	}

	// When we have a target year, only consider results that match it
	if targetYear > 0 {
		var bestMatch *tmdb.TmdbMovie
		var bestScore float64 = -1
		for i := range results {
			movie := &results[i]
			movieYear := extractYearFromReleaseDate(movie.ReleaseDate)
			if movieYear != targetYear {
				continue
			}
			score := helpers.TMDB_YEAR_MATCH_SCORE + movie.Popularity + movie.VoteAverage*10
			if score > bestScore {
				bestScore = score
				bestMatch = movie
			}
		}
		return bestMatch
	}

	// No year to match: pick best by popularity and vote average
	var bestMatch *tmdb.TmdbMovie
	var bestScore float64 = -1
	for i := range results {
		movie := &results[i]
		score := movie.Popularity + movie.VoteAverage*10
		if score > bestScore {
			bestScore = score
			bestMatch = movie
		}
	}
	return bestMatch
}
